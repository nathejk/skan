package spejderstatus

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/nathejk/shared-go/types"
)

// ErrRecordNotFound is returned when a member has no status row.
//
// Its own sentinel rather than a copy of shared-go's: an errors.New duplicate
// would be a distinct value and errors.Is would silently stop matching, which is
// the trap the go-bff-layout notes record about internal/data. When this package
// is lifted (task 083) this alias goes and shared-go's is used directly.
var ErrRecordNotFound = errors.New("record not found")

type Filter struct {
	YearSlug types.YearSlug
	TeamID   types.TeamID
}

type Queries interface {
	GetByMemberID(context.Context, types.YearSlug, types.MemberID) (*SpejderStatus, error)
	GetByMemberIDs(context.Context, types.YearSlug, []types.MemberID) (map[types.MemberID]SpejderStatus, error)
	GetByStatuses(context.Context, types.YearSlug, []types.MemberStatus) ([]SpejderStatus, error)
	GetByTeam(context.Context, Filter) ([]SpejderStatus, error)
	GetHistory(context.Context, types.YearSlug, types.MemberID) ([]StatusEvent, error)
	InOurCare(context.Context, types.YearSlug) (*Care, error)
	// TeamMemberships lists everyone who has ever been on a team, with the intervals they were —
	// including members whose `spejder` row has since been deleted (PRD 011, membership.go).
	TeamMemberships(context.Context, types.YearSlug, types.TeamID) ([]Membership, error)
}

// StatusEvent is one step in a member's lifecycle, as the member detail view shows it.
//
// Event is kept alongside Status because they answer different questions: the status is
// where the member ended up, the event is what happened to them. `racing` reached by
// carrying on under their own steam and `racing` reached by being moved to another patrol
// are the same status and very different facts, and a timeline that showed only the status
// would render those two identically.
type StatusEvent struct {
	Seq       uint64             `json:"seq"`
	Status    types.MemberStatus `json:"status"`
	Event     string             `json:"event"`
	TeamID    types.TeamID       `json:"teamId"`
	Actor     types.UserID       `json:"actorUserId"`
	CreatedAt time.Time          `json:"createdAt"`
}

// Care is how many members Nathejk is currently responsible for.
//
// The number that has to reach zero before the organisers can go home — which is why
// it is a type with a breakdown rather than an int: an operator who sees 3 needs to
// know whether that is three people waiting by a road or three asleep at HQ, because
// the first three are blocking three patrols and somebody has to drive.
type Care struct {
	Total    int                        `json:"total"`
	ByStatus map[types.MemberStatus]int `json:"byStatus"`

	// OldestWaitingAt is when the longest-waiting member asked to leave, or nil if
	// nobody is waiting. The threshold is applied by the caller, not here: it is
	// configuration and still unsettled (PRD 006 §11), so the query returns the fact
	// and lets the rule live where it can change.
	//
	// Only `waiting` is measured. A member in a car or asleep at HQ is accounted for
	// and their patrol has stopped waiting for them; a member by the trailside is
	// neither.
	OldestWaitingAt *time.Time `json:"oldestWaitingAt"`
}

type querier struct {
	db *sql.DB
	r  *goqu.Database
}

// GetByMemberID reads one member's current place in the lifecycle.
//
// This is what the commands dirty-check against, which is the reason it exists:
// the resume action is valid only while the member is still self-carrying, and
// that can only be decided against the stored status rather than against whatever
// the operator's screen last showed.
func (q *querier) GetByMemberID(ctx context.Context, year types.YearSlug, id types.MemberID) (*SpejderStatus, error) {
	if id == "" {
		return nil, ErrRecordNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := selectSpejderStatus + ` WHERE year = ? AND id = ?`

	var s SpejderStatus
	var updatedAt time.Time
	err := q.db.QueryRowContext(ctx, query, string(year), string(id)).Scan(
		&s.MemberID, &s.YearSlug, &s.InitialTeamID, &s.CurrentTeamID, &s.Status, &updatedAt,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrRecordNotFound
	case err != nil:
		return nil, err
	}
	s.UpdatedAt = updatedAt
	return &s, nil
}

// InOurCare counts the members Nathejk is responsible for right now, per status,
// with the oldest `waiting` timestamp for the alarm.
//
// The status set comes from InOurCareStatuses(), so this query does not name
// waiting/transit/sheltered anywhere — see the comment there for why that is not
// fussiness.
//
// Every in-care status is present in ByStatus even at zero. A breakdown that omitted
// empty states would make the display flicker between three rows and one as the night
// went on, and "transit: 0" is information: it says no car is currently carrying
// anybody.
func (q *querier) InOurCare(ctx context.Context, year types.YearSlug) (*Care, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	statuses := InOurCareStatuses()
	care := &Care{ByStatus: make(map[types.MemberStatus]int, len(statuses))}
	args := []any{string(year)}
	placeholders := make([]string, 0, len(statuses))
	for _, s := range statuses {
		care.ByStatus[s] = 0
		placeholders = append(placeholders, "?")
		args = append(args, string(s))
	}

	query := `SELECT status, COUNT(*), MIN(updatedAt)
		FROM spejderstatus
		WHERE year = ? AND status IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY status`

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status types.MemberStatus
		var count int
		var oldest sql.NullTime
		if err := rows.Scan(&status, &count, &oldest); err != nil {
			return nil, err
		}
		care.ByStatus[status] = count
		care.Total += count
		if status == types.MemberStatusWaiting && oldest.Valid {
			t := oldest.Time
			care.OldestWaitingAt = &t
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return care, nil
}

// GetHistory reads one member's lifecycle in the order it happened.
//
// Ordered by stream sequence rather than by timestamp. They almost always agree, but the
// sequence is the order the platform actually applied things, and two events inside one
// operation share a timestamp to the second — so sorting by time would render a collection
// of three members in an arbitrary order while the sequence gives the true one.
func (q *querier) GetHistory(ctx context.Context, year types.YearSlug, id types.MemberID) ([]StatusEvent, error) {
	if id == "" {
		return nil, ErrRecordNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT seq, status, event, teamId, actorUserId, createdAt
		FROM spejderstatuslog
		WHERE year = ? AND id = ?
		ORDER BY seq`

	rows, err := q.db.QueryContext(ctx, query, string(year), string(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := []StatusEvent{}
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.Seq, &e.Status, &e.Event, &e.TeamID, &e.Actor, &e.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, e)
	}
	return history, rows.Err()
}

// GetByMemberIDs reads the status of specific members, whatever team they are on now.
//
// This exists because GetByTeam cannot answer the question a roster asks. A patrol's roster
// is its *signup* roster, and membership follows `currentTeamId` — so to decide whether a
// listed member has moved away, a caller needs their row **regardless of team**, which a
// team-scoped query by definition cannot return. Without it the two cases "has moved
// elsewhere" and "has no status at all" are indistinguishable, and a moved member renders as
// one who never started.
//
// One query with an IN clause rather than a lookup per member: a case with three patrols
// asks about eighteen people, and that is a screen an operator keeps open all night.
func (q *querier) GetByMemberIDs(ctx context.Context, year types.YearSlug, ids []types.MemberID) (map[types.MemberID]SpejderStatus, error) {
	out := map[types.MemberID]SpejderStatus{}
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := make([]any, 0, len(ids)+1)
	args = append(args, string(year))
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, string(id))
	}

	query := selectSpejderStatus + `
		WHERE year = ? AND id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s SpejderStatus
		var updatedAt time.Time
		if err := rows.Scan(&s.MemberID, &s.YearSlug, &s.InitialTeamID, &s.CurrentTeamID, &s.Status, &updatedAt); err != nil {
			return nil, err
		}
		s.UpdatedAt = updatedAt
		out[s.MemberID] = s
	}
	return out, rows.Err()
}

// selectSpejderStatus is the column list every SpejderStatus read shares, in the order
// scanSpejderStatus expects them. Spelled once so a column added to the struct cannot be
// added to three of the four queries.
const selectSpejderStatus = `SELECT id, year, initialTeamId, currentTeamId, status, updatedAt
	FROM spejderstatus`

// byStatusesQuery builds the statement and arguments for GetByStatuses.
//
// Separated from the method so it can be tested without a database. That is not a
// concession: what is worth pinning here is the shape of the SQL — that the statuses become
// placeholders rather than interpolated strings, that the year is always constrained, and
// that the ordering is the one the screen depends on. consumer_test.go asserts on emitted
// SQL for the same reason.
func byStatusesQuery(year types.YearSlug, statuses []types.MemberStatus) (string, []any) {
	args := make([]any, 0, len(statuses)+1)
	args = append(args, string(year))
	placeholders := make([]string, 0, len(statuses))
	for _, s := range statuses {
		placeholders = append(placeholders, "?")
		args = append(args, string(s))
	}
	// updatedAt DESC because the shelter reads the most recent arrivals first. The id is a
	// tiebreak, not decoration: a patrol starting writes its whole roster in one second, so
	// without it the order within a group is whatever the storage engine feels like and two
	// consecutive loads can disagree — which on screen looks like rows shuffling by
	// themselves.
	query := selectSpejderStatus + `
		WHERE year = ? AND status IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY updatedAt DESC, id`
	return query, args
}

// GetByStatuses reads every member of the year in any of the given statuses.
//
// Named for what it does rather than for who asks. The obvious alternative was
// ListNotActive(), and it would have been a mistake: the set of statuses is the caller's
// question, and baking one screen's policy into the query is how the next caller ends up
// with a query whose name lies about what it returns. The shelter derives its set from
// types.MemberStatus predicates where it can (InOurCare()), so a fourth in-care state added
// to shared-go starts appearing without this being touched.
//
// An empty status set returns nothing and issues no query. That is the honest answer to
// "which members are in none of these statuses" — and the alternative matters, because
// `status IN ()` is a syntax error in MySQL, so a caller who filtered its own list down to
// nothing would get a database error instead of an empty screen.
func (q *querier) GetByStatuses(ctx context.Context, year types.YearSlug, statuses []types.MemberStatus) ([]SpejderStatus, error) {
	if len(statuses) == 0 {
		return []SpejderStatus{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query, args := byStatusesQuery(year, statuses)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []SpejderStatus{}
	for rows.Next() {
		var s SpejderStatus
		var updatedAt time.Time
		if err := rows.Scan(&s.MemberID, &s.YearSlug, &s.InitialTeamID, &s.CurrentTeamID, &s.Status, &updatedAt); err != nil {
			return nil, err
		}
		s.UpdatedAt = updatedAt
		members = append(members, s)
	}
	return members, rows.Err()
}

// GetByTeam reads every member currently attached to a team, whatever their status.
//
// Keyed on currentTeamId rather than initialTeamId, because the question callers
// ask is "who is with this patrol now?" — a member who was moved away is no longer
// this patrol's business, and a member moved in is.
func (q *querier) GetByTeam(ctx context.Context, f Filter) ([]SpejderStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := selectSpejderStatus + `
		WHERE year = ? AND currentTeamId = ?
		ORDER BY id`

	rows, err := q.db.QueryContext(ctx, query, string(f.YearSlug), string(f.TeamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []SpejderStatus{}
	for rows.Next() {
		var s SpejderStatus
		var updatedAt time.Time
		if err := rows.Scan(&s.MemberID, &s.YearSlug, &s.InitialTeamID, &s.CurrentTeamID, &s.Status, &updatedAt); err != nil {
			return nil, err
		}
		s.UpdatedAt = updatedAt
		members = append(members, s)
	}
	return members, rows.Err()
}
