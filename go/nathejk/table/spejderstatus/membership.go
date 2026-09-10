package spejderstatus

import (
	"context"
	"strings"
	"time"

	"github.com/nathejk/shared-go/types"
)

// Who has been on this patrol, and when (PRD 011, task 148).
//
// # Why this cannot be a query against `spejder`
//
// The obvious implementation is `SELECT memberId FROM spejder WHERE teamId = ?`, and it silently
// answers a different question. `NATHEJK.*.spejder.*.deleted` **hard-deletes** the row
// (`DELETE FROM spejder WHERE memberId = …`, in shared-go), so a withdrawn or removed scout leaves no
// trace there at all. A patrol's map drawn from that table would omit exactly the people whose
// movement somebody is most likely to be reconstructing.
//
// The history survives in this package's own tables instead: `spejderstatuslog` has one append-only
// row per lifecycle event, each carrying the team the member was on at the time.
//
// # Why intervals rather than a set of ids
//
// Scouts move between patrols mid-race (`spejder.*.team.moved`). Their positions before and after the
// move belong to different patrols, and showing a member's whole track on both teams' maps would put
// one patrol's movement on another's picture. An interval is what lets the caller clip.
//
// # Two sources, neither sufficient alone
//
// The log holds everyone who has *ever* been on the team, including scouts whose `spejder` row was
// hard-deleted when they withdrew. The `spejder` table holds the current roster, and is the only
// source that knows anything before the patrol starts — which is most of the season. See
// rosterWithoutHistory.

// logEntry is one row of a member's lifecycle log, reduced to what membership intervals need.
type logEntry struct {
	team      string
	createdAt time.Time
}

// Membership is one stretch during which a member belonged to a team.
type Membership struct {
	MemberID types.MemberID `json:"memberId"`

	// Name is empty when no `spejder` row survives — a scout deleted after withdrawing.
	//
	// Empty rather than an error or an omission: their track is still evidence of where the patrol
	// went, and dropping an unnamed member would quietly shrink the picture during precisely the
	// incident it is being consulted for. The UI labels these "tidligere medlem".
	Name string `json:"name"`

	// From is when this stretch began. Zero when unknown — a member whose patrol never started has
	// no lifecycle event to date it, so there is nothing to clip against at the start.
	From time.Time `json:"from"`

	// To is when it ended, or nil while the member is still on the team.
	To *time.Time `json:"to"`
}

// Open reports whether the member is still on the team.
func (m Membership) Open() bool { return m.To == nil }

// TeamMemberships lists every member who has ever belonged to a team, with the intervals they did.
//
// Members currently on the team, members who moved away, and members whose `spejder` row no longer
// exists are all included. A member who joined, left and returned yields two intervals.
func (q *querier) TeamMemberships(ctx context.Context, year types.YearSlug, teamID types.TeamID) ([]Membership, error) {
	if teamID == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Every log row of every member who has ever touched this team — not just the rows naming it.
	// The rows naming *other* teams are what close an interval, so filtering them out would make
	// every membership look open-ended and a moved scout's track would never be clipped.
	//
	// Ordered by (id, seq) rather than by createdAt: two events inside one operation share a
	// timestamp to the second, and the sequence is the order the platform actually applied them —
	// the same reason GetHistory orders by seq.
	const query = `SELECT id, teamId, createdAt
		FROM spejderstatuslog
		WHERE year = ?
		  AND id IN (SELECT DISTINCT id FROM spejderstatuslog WHERE year = ? AND teamId = ?)
		ORDER BY id, seq`

	rows, err := q.db.QueryContext(ctx, query, string(year), string(year), string(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type entry = logEntry
	history := map[types.MemberID][]entry{}
	order := []types.MemberID{}

	for rows.Next() {
		var id types.MemberID
		var e entry
		if err := rows.Scan(&id, &e.team, &e.createdAt); err != nil {
			return nil, err
		}
		if _, seen := history[id]; !seen {
			order = append(order, id)
		}
		history[id] = append(history[id], e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	memberships := []Membership{}
	for _, id := range order {
		memberships = append(memberships, intervalsFor(id, teamID, history[id])...)
	}

	// A member whose patrol never started has no log rows at all, so the loop above never saw them.
	// Before a race that is *every* member, which is the ordinary state for most of the season — and it
	// is why the current roster is a source here alongside the history.
	signed, err := q.rosterWithoutHistory(ctx, year, teamID, history)
	if err != nil {
		return nil, err
	}
	memberships = append(memberships, signed...)

	return q.withNames(ctx, year, memberships)
}

// intervalsFor walks one member's log and returns the stretches they spent on teamID.
//
// Separate from the query so the decisions here can be tested without a database — and they are
// decisions rather than mechanics:
//
//   - An event naming a **different** team ends the membership. This is what clips a moved scout's
//     track, so their movement with the next patrol does not appear on this one's map.
//   - An event naming **no** team does not. Several lifecycle events legitimately carry none (see
//     consumer.teamID), and reading that as a departure would cut a member's track short at, say, a
//     withdrawal request — losing exactly the positions somebody is looking for.
//   - Repeated events while already on the team do not restart the interval; the membership began at
//     the first of them.
func intervalsFor(id types.MemberID, teamID types.TeamID, entries []logEntry) []Membership {
	var out []Membership
	on := false
	var since time.Time

	for _, e := range entries {
		switch {
		case e.team == string(teamID):
			if !on {
				on, since = true, e.createdAt
			}

		case e.team == "":
			// Absence of information, not a move.

		default:
			if on {
				end := e.createdAt
				out = append(out, Membership{MemberID: id, From: since, To: &end})
				on = false
			}
		}
	}

	if on {
		out = append(out, Membership{MemberID: id, From: since})
	}
	return out
}

// rosterWithoutHistory finds members attached to the team who have no lifecycle events yet.
//
// # Why the current roster is a source, not just the log
//
// The log records what *happened* to a member, and before a patrol starts nothing has: no
// `patrulje.started`, no move, no withdrawal. Such a member has no `spejderstatuslog` rows and often
// no `spejderstatus` row either, so a query over history alone returns an empty patrol — which was a
// real bug (task 154): a scout whose phone was already reporting had a position glyph and an empty map.
//
// So both are read, and unioned:
//
//   - `spejder` is the **current roster**. It is authoritative for who is on the team now, and it is
//     the only source that knows anything at all before the race.
//   - `spejderstatus` catches a member attached to the team by lifecycle rather than by signup — one
//     moved in from elsewhere, whose `spejder` row may name a different team.
//
// `spejder` alone would still be wrong, which is what task 148 was right about:
// `NATHEJK.*.spejder.*.deleted` hard-deletes the row, so a withdrawn scout exists only in the log.
// Neither source is sufficient; the union is.
//
// Their interval is open with a zero From: there is no event to date the start, and inventing one from
// the signup time would be a guess presented as a fact. Zero means "do not clip", which is the honest
// handling — every position we hold for them was recorded while they belonged to this team.
func (q *querier) rosterWithoutHistory(
	ctx context.Context,
	year types.YearSlug,
	teamID types.TeamID,
	seen map[types.MemberID][]logEntry,
) ([]Membership, error) {
	// UNION rather than two round trips, and UNION rather than UNION ALL so a member present in both
	// — the normal case once a patrol has started — is returned once.
	const query = `SELECT id FROM spejderstatus
			WHERE year = ? AND (initialTeamId = ? OR currentTeamId = ?)
		UNION
		SELECT memberId FROM spejder
			WHERE year = ? AND teamId = ?`

	rows, err := q.db.QueryContext(ctx, query,
		string(year), string(teamID), string(teamID),
		string(year), string(teamID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Membership{}
	for rows.Next() {
		var id types.MemberID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		// Someone with history already has their intervals from the walk above; adding an open-ended
		// one here would put a moved-away scout back on this patrol's map for the whole race.
		if _, ok := seen[id]; ok {
			continue
		}
		out = append(out, Membership{MemberID: id})
	}
	return out, rows.Err()
}

// withNames fills in the names that still exist.
//
// A LEFT JOIN in the main query would have been fewer round trips, but the main query reads the log
// and the names live in `spejder` — a table whose rows disappear. Keeping them apart makes the
// asymmetry visible: the log is the source of truth for *who was here*, and `spejder` is a
// best-effort lookup for what to call them.
func (q *querier) withNames(ctx context.Context, year types.YearSlug, memberships []Membership) ([]Membership, error) {
	if len(memberships) == 0 {
		return memberships, nil
	}

	ids := make([]string, 0, len(memberships))
	args := []any{string(year)}
	for _, m := range memberships {
		ids = append(ids, "?")
		args = append(args, string(m.MemberID))
	}

	query := `SELECT memberId, name FROM spejder WHERE year = ? AND memberId IN (` +
		strings.Join(ids, ",") + `)`

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[types.MemberID]string{}
	for rows.Next() {
		var id types.MemberID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range memberships {
		memberships[i].Name = names[memberships[i].MemberID]
	}
	return memberships, nil
}

// Compile-time check that the querier satisfies the interface this file adds to Queries.
var _ interface {
	TeamMemberships(context.Context, types.YearSlug, types.TeamID) ([]Membership, error)
} = (*querier)(nil)
