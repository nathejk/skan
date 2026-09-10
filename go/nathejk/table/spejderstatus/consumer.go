package spejderstatus

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// consumer projects the member lifecycle onto the spejderstatus table.
//
// Three properties are worth stating up front, because the rest follows from
// them:
//
//   - **racing is derived, not published.** Nothing in the platform announces
//     "this member is on the route"; what it announces is that a patrol started,
//     carrying the list of members who actually did. So the one status that
//     matters most comes from an event that already existed, with no new producer
//     and no migration — see the patrulje.started case below.
//   - **Replay is idempotent.** Every write is an upsert keyed by (year,
//     memberId), so replaying the whole history rebuilds exactly the same rows.
//     This is not a nicety: the read model is rebuilt from JetStream on every API
//     restart.
//   - **Statuses are normalised on the way in.** History contains half a dozen
//     superseded spellings, and an unrecognised one is refused rather than stored
//     — see status.go for why that is the safer half of the trade.
type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() []stream.Subject {
	return []stream.Subject{
		// Where racing comes from.
		subject.FromStr("NATHEJK:*.patrulje.*.started"),

		// StartPatrulje publishes this for every member who did *not* start. A
		// projection that ignores it keeps rows for no-shows, and every one of
		// those inflates its team's strength — so the 3-member requirement would
		// be judged against members who never turned up.
		subject.FromStr("NATHEJK:*.spejder.*.deleted"),

		// The lifecycle proper. `pickup.accepted` is still published by nobody: it
		// belongs to the car interface, and subscribing now is what makes this
		// projection ready for it rather than something that has to be revisited when
		// it ships. The shelter's three arrived with PRD 007.
		subject.FromStr("NATHEJK:*.spejder.*.withdrawal.requested"),
		subject.FromStr("NATHEJK:*.spejder.*.withdrawal.cancelled"),
		subject.FromStr("NATHEJK:*.spejder.*.status.overridden"),
		subject.FromStr("NATHEJK:*.spejder.*.team.moved"),
		subject.FromStr("NATHEJK:*.spejder.*.pickup.accepted"),
		subject.FromStr("NATHEJK:*.spejder.*.shelter.accepted"),
		subject.FromStr("NATHEJK:*.spejder.*.shelter.placed"),
		subject.FromStr("NATHEJK:*.spejder.*.handover.completed"),
	}
}

func (c *consumer) HandleMessage(msg stream.Message) error {
	switch {
	// Checked before the four-part spejder patterns: these subjects have five
	// parts, and NATHEJK.*.spejder.*.deleted would not match them, but the
	// reverse ordering has bitten this codebase before (see the sos consumer's
	// section-subject comment), so keep the specific ones first.
	case msg.Subject().Match("NATHEJK.*.spejder.*.withdrawal.requested"):
		var body WithdrawalRequested
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.withdrawal.cancelled"):
		var body WithdrawalCancelled
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.status.overridden"):
		var body StatusOverridden
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.pickup.accepted"):
		var body PickupAccepted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.shelter.accepted"):
		var body ShelterAccepted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.shelter.placed"):
		var body ShelterPlaced
		if err := msg.Body(&body); err != nil {
			return err
		}
		// Resolves to `sheltered`, which the member already is, so this write is a
		// no-op here by design — the placering itself belongs to hq's `shelter` table.
		// It still goes through setStatus rather than being skipped: it puts the move
		// between tents on the member's timeline, and it is the one path that would
		// heal a member whose acceptance event was lost.
		return c.setStatus(msg, body)

	case msg.Subject().Match("NATHEJK.*.spejder.*.handover.completed"):
		var body HandoverCompleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.setStatus(msg, body)

	// A move is the one event that writes a column the others do not, so it does
	// not go through setStatus.
	case msg.Subject().Match("NATHEJK.*.spejder.*.team.moved"):
		var body TeamMoved
		if err := msg.Body(&body); err != nil {
			return err
		}
		// initialTeamId is deliberately absent from the update: it records where
		// the member started and must survive any number of moves.
		if err := c.upsert(msg, goqu.Record{
			"id":            c.memberID(msg),
			"year":          c.year(msg),
			"initialTeamId": string(body.FromTeamID),
			"currentTeamId": string(body.ToTeamID),
			"status":        string(types.MemberStatusRacing),
			"updatedAt":     c.at(msg),
		}, "currentTeamId", "status", "updatedAt"); err != nil {
			return err
		}
		if err := c.appendHistory(msg, body.MemberID, body.ToTeamID, types.MemberStatusRacing); err != nil {
			return err
		}
		// Both teams: the origin lost a member and the destination gained one. This
		// is the case that makes carrying FromTeamID on the event worth it — the row
		// no longer says where the member came from.
		if err := c.recomputeActiveMemberCount(msg, body.FromTeamID); err != nil {
			return err
		}
		return c.recomputeActiveMemberCount(msg, body.ToTeamID)

	// The member did not start. Delete rather than mark: a no-show has no place in
	// the lifecycle at all, and leaving a row would make them count somewhere.
	case msg.Subject().Match("NATHEJK.*.spejder.*.deleted"):
		// The body is read only for its team: the delete itself is keyed on the
		// subject, but the team whose strength just changed cannot be recovered from
		// a row that no longer exists, so it has to come from the event.
		var body messages.NathejkMemberDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		if err := c.exec(goqu.Dialect("mysql").
			Delete("spejderstatus").
			Where(
				goqu.C("year").Eq(c.year(msg)),
				goqu.C("id").Eq(c.memberID(msg)),
			)); err != nil {
			return err
		}
		// The history goes with them. A member who did not start has no lifecycle to
		// show, and leaving orphaned rows behind would put a history on a person the
		// rest of the read model says does not exist.
		if err := c.exec(goqu.Dialect("mysql").
			Delete("spejderstatuslog").
			Where(
				goqu.C("year").Eq(c.year(msg)),
				goqu.C("id").Eq(c.memberID(msg)),
			)); err != nil {
			return err
		}
		return c.recomputeActiveMemberCount(msg, body.TeamID)

	// Where racing comes from.
	//
	// body.Members is precisely the members who signed in at the start — the same
	// slice the patrulje projection already uses for memberCount — so the 3-member
	// requirement is judged against the same population the patrol's own count
	// reports, rather than a second definition that could drift from it.
	case msg.Subject().Match("NATHEJK.*.patrulje.*.started"):
		var body messages.NathejkTeamStarted
		if err := msg.Body(&body); err != nil {
			return err
		}
		for _, m := range body.Members {
			if m.MemberID == "" {
				continue
			}
			// initialTeamId and currentTeamId are both the starting team, and
			// currentTeamId is *not* overwritten on conflict: replaying the start
			// event after a member has been moved must not drag them back to the
			// patrol they began with. Status is refreshed, because a replay should
			// re-derive racing, but only for a member whose row this event still
			// legitimately describes.
			if err := c.upsert(msg, goqu.Record{
				"id":            string(m.MemberID),
				"year":          c.year(msg),
				"initialTeamId": string(body.TeamID),
				"currentTeamId": string(body.TeamID),
				"status":        string(types.MemberStatusRacing),
				"updatedAt":     c.at(msg),
			}, "initialTeamId"); err != nil {
				return err
			}
			// The first line of every member's timeline. Keyed by this one event's
			// sequence plus the member id, which is why the log's key is composite.
			if err := c.appendHistory(msg, m.MemberID, body.TeamID, types.MemberStatusRacing); err != nil {
				return err
			}
		}
		// Once for the team, not once per member: the count is recomputed from the
		// table rather than accumulated, so repeating it per member would issue N
		// identical statements for the same answer.
		return c.recomputeActiveMemberCount(msg, body.TeamID)

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
	}
	return nil
}

// setStatus writes the status a lifecycle event resolves to.
//
// Every event except the move goes through here, and none of them names a status
// directly: the body's Status() does. That is what stops a consumer case from
// writing a value the lifecycle does not define, and it means the car and shelter
// interfaces' events need no new write path — only a subject and a type.
func (c *consumer) setStatus(msg stream.Message, e MemberEvent) error {
	status, ok := ParseMemberStatus(string(e.Status()))
	if !ok {
		// Refused rather than stored. An unreadable status in the read model would
		// be counted by nothing — neither InOurCare() nor a strength query — so a
		// member would go missing from the one number that has to reach zero.
		// Loud, and not an error: a poison message must not wedge the replay.
		log.Printf("spejderstatus: refusing unknown status %q for member %q", e.Status(), c.memberID(msg))
		return nil
	}
	if err := c.upsert(msg, goqu.Record{
		"id":            c.memberID(msg),
		"year":          c.year(msg),
		"currentTeamId": string(c.teamID(e)),
		"status":        string(status),
		"updatedAt":     c.at(msg),
	}, "status", "updatedAt"); err != nil {
		return err
	}
	if err := c.appendHistory(msg, types.MemberID(c.memberID(msg)), c.teamID(e), status); err != nil {
		return err
	}
	// Strictly after the row is written, which is the whole reason the two live in
	// one consumer — see below.
	return c.recomputeActiveMemberCount(msg, c.teamID(e))
}

// appendHistory records one step in a member's lifecycle.
//
// Takes the member id explicitly rather than reading it from the subject, because the
// start event's subject names the **team**: one message puts a whole roster into
// `racing`, and taking parts[3] there would file every one of them under the team's id.
//
// Keyed by the event's stream sequence, so replaying history rebuilds the same rows
// rather than appending a second copy of every member's past on each restart. That is
// the same decision sos_activity rests on, and it is the only thing making an
// append-only table safe in a projection that is replayed by design.
//
// The event name is stored alongside the status because they answer different
// questions: the status is where the member ended up, the event is what happened to
// them. "racing" reached by carrying on themselves and "racing" reached by being moved
// to another patrol are the same status and very different facts.
func (c *consumer) appendHistory(msg stream.Message, memberID types.MemberID, teamID types.TeamID, status types.MemberStatus) error {
	if memberID == "" {
		return nil
	}
	return c.exec(goqu.Dialect("mysql").
		Insert("spejderstatuslog").Rows(goqu.Record{
		"seq":         msg.Sequence(),
		"id":          string(memberID),
		"year":        c.year(msg),
		"teamId":      string(teamID),
		"status":      string(status),
		"event":       c.event(msg),
		"actorUserId": string(c.actor(msg)),
		"createdAt":   c.at(msg),
	}).
		OnConflict(goqu.DoNothing()))
}

// event is everything after the member id in the subject — "withdrawal.requested",
// "team.moved" — so a dotted name survives whole.
func (c *consumer) event(msg stream.Message) string {
	parts := msg.Subject().Parts()
	if len(parts) < 5 {
		return ""
	}
	return strings.Join(parts[4:], ".")
}

// actor is the user the event was published by, empty until the platform
// authenticates anybody (PRD 001 §6 Auth).
func (c *consumer) actor(msg stream.Message) types.UserID {
	var meta messages.Metadata
	if err := msg.Meta(&meta); err != nil {
		return ""
	}
	return meta.UserID
}

// recomputeActiveMemberCount rewrites the team's strength from the member rows.
//
// # Why this lives here, in the member projection, writing another package's table
//
// The count belongs to the team, so the obvious home is the patrulje consumer. That
// would be wrong, and quietly: the mux hands the same message to every consumer with
// **no ordering guarantee between them**, so a recompute in patrulje could read
// spejderstatus before this projection had written the row the event describes, and
// land a count that is one out. Nothing would fail. The number would simply be wrong
// in a way that looks entirely plausible — which, for a count of who is still on the
// route, is the worst available outcome.
//
// Writing both tables from the one consumer removes the race by construction: the
// row is in place before it is counted, because the same function did both in order.
//
// # Why recompute rather than increment
//
// A ±1 would need the member's previous status, which the event does not carry and
// which a replayed event cannot know. It would also make the result depend on
// arrival order: replay the same events in a different order and the count drifts.
// Recomputing converges on the same answer whatever order history arrives in, which
// is the only property that matters for a table rebuilt from the log on every
// restart. The subquery is indexed on (year, currentTeamId, status).
//
// # Why there is no event for discontinuation
//
// A team with activeMemberCount == 0 is discontinued. That needs no event, no flag
// and no reverse event: move a member back in and the recompute makes the team
// active again on its own. The legacy patruljemerged encoding needed .merged and
// .splited precisely because it stored the conclusion instead of the input.
func (c *consumer) recomputeActiveMemberCount(msg stream.Message, teamID types.TeamID) error {
	if teamID == "" {
		// Nothing to recompute. Not an error: a lifecycle event for a member whose
		// team we do not know still updates the member's own row, and a strength we
		// cannot attribute is better left alone than written against "".
		return nil
	}
	racing := goqu.Dialect("mysql").
		From("spejderstatus").
		Select(goqu.COUNT(goqu.Star())).
		Where(
			goqu.C("year").Eq(c.year(msg)),
			goqu.C("currentTeamId").Eq(string(teamID)),
			goqu.C("status").Eq(string(types.MemberStatusRacing)),
		)
	return c.exec(goqu.Dialect("mysql").
		Update("patrulje").
		Set(goqu.Record{"activeMemberCount": racing}).
		Where(goqu.C("teamId").Eq(string(teamID))))
}

// teamID reads the team off whichever event body carries one.
//
// The events all name the member's team, which lets a row be created by a
// lifecycle event alone — for a member whose start this projection never saw,
// because history was truncated or the car interface got there first. Without it
// such a member would have a status and no team, and would therefore be invisible
// to every per-team query.
func (c *consumer) teamID(e MemberEvent) types.TeamID {
	switch v := e.(type) {
	case WithdrawalRequested:
		return v.TeamID
	case WithdrawalCancelled:
		return v.TeamID
	case StatusOverridden:
		return v.TeamID
	case PickupAccepted:
		return v.TeamID
	case ShelterAccepted:
		return v.TeamID
	case ShelterPlaced:
		return v.TeamID
	case HandoverCompleted:
		return v.TeamID
	}
	return ""
}

// upsert writes a row, updating only the named columns if it already exists.
//
// The explicit column list is the point. An upsert that overwrote everything would
// make replay order-dependent — a start event arriving after a move would undo the
// move — so each caller states exactly which facts its event is entitled to
// change.
func (c *consumer) upsert(msg stream.Message, row goqu.Record, updates ...string) error {
	update := goqu.Record{}
	for _, col := range updates {
		update[col] = goqu.L(fmt.Sprintf("VALUES(%s)", col))
	}
	return c.exec(goqu.Dialect("mysql").
		Insert("spejderstatus").Rows(row).
		OnConflict(goqu.DoUpdate("year,id", update)))
}

func (c *consumer) exec(ds interface {
	ToSQL() (string, []any, error)
}) error {
	sqlStr, _, err := ds.ToSQL()
	if err != nil {
		return err
	}
	return c.w.Consume(sqlStr)
}

// memberID comes from the subject, not the body: the subject is what the stream
// routes on, so it is the one place the id cannot disagree with itself.
func (c *consumer) memberID(msg stream.Message) string {
	return msg.Subject().Parts()[3]
}

// year comes from the subject too, never from msg.Time().
//
// The bodies carry no year, and the old commented-out version of this projection
// used msg.Time().Year() — which is wrong the moment history is replayed across a
// year boundary, and replay is how this table is built.
func (c *consumer) year(msg stream.Message) string {
	return msg.Subject().Parts()[1]
}

// at renders the event time as a MySQL DATETIME in UTC, matching the sos
// projection: the driver reads DATETIME back as UTC, so the API emits a Z-offset
// timestamp and the browser converts to the operator's clock. Local wall-clock
// would be an hour or two wrong all summer, on the timestamp the waiting alarm
// measures.
func (c *consumer) at(msg stream.Message) string {
	return msg.Time().UTC().Format(time.DateTime)
}
