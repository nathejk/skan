package spejderstatus

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// --- fakes ---

// recordingWriter captures the statements the projection produces. Asserting on
// emitted SQL rather than against a database is what the rest of the repo does,
// and it pins the parts that actually matter here: whether a write is an upsert,
// which columns it is entitled to overwrite, and whether a second identical event
// changes anything.
type recordingWriter struct {
	stmts []string
}

func (w *recordingWriter) Consume(stmt string) error {
	w.stmts = append(w.stmts, stmt)
	return nil
}

func (w *recordingWriter) last() string {
	if len(w.stmts) == 0 {
		return ""
	}
	return w.stmts[len(w.stmts)-1]
}

// Every member write is now followed by a recompute of the team's strength, so
// tests select the statement they mean rather than counting or taking the last
// one. Splitting by target table keeps each assertion about one thing: the member
// row, or the count derived from it.
func (w *recordingWriter) memberStmts() []string { return w.stmtsFor("spejderstatus") }
func (w *recordingWriter) countStmts() []string  { return w.stmtsFor("patrulje") }

func (w *recordingWriter) stmtsFor(table string) []string {
	var out []string
	for _, s := range w.stmts {
		// The recompute reads spejderstatus in a subquery, so match on what the
		// statement *writes* rather than on any mention of the table.
		if strings.HasPrefix(s, "UPDATE `"+table+"`") ||
			strings.HasPrefix(s, "INSERT IGNORE INTO `"+table+"`") ||
			strings.HasPrefix(s, "INSERT INTO `"+table+"`") ||
			// goqu's mysql dialect renders a delete as DELETE `t` FROM `t` WHERE ...
			strings.HasPrefix(s, "DELETE `"+table+"`") {
			out = append(out, s)
		}
	}
	return out
}

// firstMemberStmt is the member-row write, which most tests are about.
func (w *recordingWriter) firstMemberStmt(t *testing.T) string {
	t.Helper()
	stmts := w.memberStmts()
	if len(stmts) == 0 {
		t.Fatalf("no spejderstatus statement emitted; subject unmatched?")
	}
	return stmts[0]
}

// updateClause is the ON DUPLICATE KEY UPDATE tail, i.e. exactly which columns an
// event is entitled to overwrite on a row that already exists.
func updateClause(t *testing.T, stmt string) string {
	t.Helper()
	i := strings.Index(stmt, "ON DUPLICATE KEY UPDATE")
	if i < 0 {
		t.Fatalf("statement is not an upsert, so replay would duplicate or clobber: %s", stmt)
	}
	return stmt[i:]
}

type fakeMessage struct {
	subject stream.Subject
	body    any
	at      time.Time
}

func (m fakeMessage) Subject() stream.Subject { return m.subject }
func (m fakeMessage) Time() time.Time         { return m.at }
func (m fakeMessage) Sequence() uint64        { return 1 }
func (m fakeMessage) Body(dst any) error {
	b, err := json.Marshal(m.body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
func (m fakeMessage) Meta(any) error { return nil }
func (m fakeMessage) RawBody() any   { return m.body }
func (m fakeMessage) RawMeta() any   { return nil }

func msg(subj string, body any) stream.Message {
	return fakeMessage{
		subject: subject.FromStr(subj),
		body:    body,
		at:      time.Date(2026, 8, 11, 20, 30, 0, 0, time.UTC),
	}
}

// msgAt is for the one test that cares about the clock: a message published in a
// different year from the one in its subject.
func msgAt(subj string, body any, at time.Time) stream.Message {
	return fakeMessage{subject: subject.FromStr(subj), body: body, at: at}
}

func handle(t *testing.T, c *consumer, m stream.Message) {
	t.Helper()
	if err := c.HandleMessage(m); err != nil {
		t.Fatalf("HandleMessage(%s): %v", m.Subject().Subject(), err)
	}
}

// --- tests ---

// The event that makes the whole feature possible: racing is derived from a patrol
// starting, with no new producer anywhere in the platform.
func TestTeamStartedMakesEveryStartingMemberRacing(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.patrulje.team-1.started", messages.NathejkTeamStarted{
		TeamID: "team-1",
		Members: []messages.NathejkTeamStarted_Member{
			{MemberID: "m-1"},
			{MemberID: "m-2"},
		},
	}))

	members := w.memberStmts()
	if len(members) != 2 {
		t.Fatalf("got %d member statements, want one per starting member: %v", len(members), members)
	}
	for _, stmt := range members {
		if !strings.Contains(stmt, "racing") {
			t.Errorf("starting member not set racing: %s", stmt)
		}
		updateClause(t, stmt)
	}
	if !strings.Contains(members[0], "'m-1'") || !strings.Contains(members[1], "'m-2'") {
		t.Errorf("member ids not taken from the body: %v", members)
	}
	// One recompute for the team, not one per member: the count is derived from the
	// table rather than accumulated, so N statements would ask the same question N
	// times.
	if got := len(w.countStmts()); got != 1 {
		t.Errorf("got %d strength recomputes for a two-member start, want 1", got)
	}
}

// A member with no id is skipped rather than written as a row keyed on the empty
// string — which would collide with every other such member in the year, since the
// primary key is (year, id).
func TestTeamStartedSkipsMembersWithNoID(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.patrulje.team-1.started", messages.NathejkTeamStarted{
		TeamID:  "team-1",
		Members: []messages.NathejkTeamStarted_Member{{MemberID: ""}, {MemberID: "m-1"}},
	}))

	if got := len(w.memberStmts()); got != 1 {
		t.Fatalf("got %d member statements, want 1: %v", got, w.memberStmts())
	}
}

// **The subtlest rule in the projection.** Replaying the start event after a member
// has been moved must not drag them back to the patrol they began with.
//
// The start event is replayed on every restart, and it legitimately knows the
// member's *initial* team — but nothing about where they are now. So its upsert may
// refresh initialTeamId and nothing else. If currentTeamId were in the update list,
// every restart would silently undo every move made during the event, and the
// strength of two patrols would be wrong with no trace of why.
func TestTeamStartedReplayDoesNotUndoAMove(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.patrulje.team-1.started", messages.NathejkTeamStarted{
		TeamID:  "team-1",
		Members: []messages.NathejkTeamStarted_Member{{MemberID: "m-1"}},
	}))

	update := updateClause(t, w.firstMemberStmt(t))
	if strings.Contains(update, "currentTeamId") {
		t.Errorf("start event overwrites currentTeamId, so replay would undo moves: %s", update)
	}
	if !strings.Contains(update, "initialTeamId") {
		t.Errorf("start event should refresh initialTeamId: %s", update)
	}
}

// Non-starters are deleted, not left behind. A leftover row inflates its team's
// strength, so the 3-member requirement would be judged against members who never
// turned up.
func TestSpejderDeletedRemovesTheRow(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.deleted", messages.NathejkMemberDeleted{
		MemberID: "m-1", TeamID: "team-1",
	}))

	stmt := w.firstMemberStmt(t)
	if !strings.HasPrefix(strings.ToUpper(stmt), "DELETE") {
		t.Errorf("want a DELETE, got: %s", stmt)
	}
	if !strings.Contains(stmt, "'m-1'") || !strings.Contains(stmt, "'2026'") {
		t.Errorf("delete is not scoped to the member and year: %s", stmt)
	}
	// The deleted member's team loses strength, and the row is gone, so the team can
	// only come from the event body.
	if got := len(w.countStmts()); got != 1 {
		t.Errorf("a non-starter's removal did not recompute their team's strength")
	}
}

// Every lifecycle event resolves to its own status through the body, so one code
// path serves all of them — including the three this repo does not publish.
func TestLifecycleEventsWriteTheirStatus(t *testing.T) {
	tests := []struct {
		subject string
		body    any
		want    types.MemberStatus
	}{
		{"NATHEJK.2026.spejder.m-1.withdrawal.requested", WithdrawalRequested{MemberID: "m-1", TeamID: "team-1"}, types.MemberStatusWaiting},
		{"NATHEJK.2026.spejder.m-1.withdrawal.cancelled", WithdrawalCancelled{MemberID: "m-1", TeamID: "team-1"}, types.MemberStatusRacing},
		{"NATHEJK.2026.spejder.m-1.pickup.accepted", PickupAccepted{MemberID: "m-1", TeamID: "team-1", SectionSlug: "bil-3"}, types.MemberStatusTransit},
		{"NATHEJK.2026.spejder.m-1.shelter.accepted", ShelterAccepted{MemberID: "m-1", TeamID: "team-1", Placement: "Telt 4"}, types.MemberStatusSheltered},
		{"NATHEJK.2026.spejder.m-1.shelter.placed", ShelterPlaced{MemberID: "m-1", TeamID: "team-1", Placement: "Telt 4"}, types.MemberStatusSheltered},
		{"NATHEJK.2026.spejder.m-1.status.overridden", StatusOverridden{MemberID: "m-1", TeamID: "team-1", To: types.MemberStatusSheltered}, types.MemberStatusSheltered},
		{"NATHEJK.2026.spejder.m-1.handover.completed", HandoverCompleted{MemberID: "m-1", TeamID: "team-1", To: types.MemberStatusReleased}, types.MemberStatusReleased},
	}
	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			w := &recordingWriter{}
			c := &consumer{w: w}
			handle(t, c, msg(tt.subject, tt.body))

			stmt := w.firstMemberStmt(t)
			if !strings.Contains(stmt, string(tt.want)) {
				t.Errorf("want status %q in: %s", tt.want, stmt)
			}
			updateClause(t, stmt)
			if got := len(w.countStmts()); got != 1 {
				t.Errorf("status change did not recompute the team's strength")
			}
		})
	}
}

// The placering is none of this projection's business.
//
// `shelter.placed` goes through the same write path as every other lifecycle event, and
// what it must *not* do is carry the placement into spejderstatus. That table is queued for
// lifting to shared-go verbatim (task 083), and a placering column appearing in it is
// exactly how that lift stops being a file move. The placement lives in hq's own `shelter`
// table, written by its own projection.
func TestShelterPlacedDoesNotWriteThePlacering(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}
	handle(t, c, msg("NATHEJK.2026.spejder.m-1.shelter.placed",
		ShelterPlaced{MemberID: "m-1", TeamID: "team-1", Placement: "Telt 4"}))

	for _, stmt := range w.stmts {
		if strings.Contains(stmt, "Telt 4") || strings.Contains(strings.ToLower(stmt), "placement") {
			t.Errorf("the placering reached spejderstatus: %s", stmt)
		}
	}
}

// A move between tents still lands on the member's timeline.
//
// The status write is a no-op — the member was already sheltered — so the only lasting
// effect of this event here is the history row. If it were skipped as "nothing changed",
// the record would show a child arriving and never moving, which is precisely the question
// somebody asks when they cannot find them.
func TestShelterPlacedIsRecordedInHistory(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}
	handle(t, c, msg("NATHEJK.2026.spejder.m-1.shelter.placed",
		ShelterPlaced{MemberID: "m-1", TeamID: "team-1", Placement: "Telt 4"}))

	var found bool
	for _, stmt := range w.stmts {
		if strings.Contains(stmt, "spejderstatuslog") && strings.Contains(stmt, "shelter.placed") {
			found = true
		}
	}
	if !found {
		t.Errorf("no history row named the event; statements were: %v", w.stmts)
	}
}

// A move writes currentTeamId and must not touch initialTeamId — the member races
// on with a patrol that is not the one they signed up with, and both facts are
// wanted afterwards.
func TestTeamMovedChangesCurrentTeamOnly(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.team.moved", TeamMoved{
		MemberID: "m-1", FromTeamID: "team-1", ToTeamID: "team-2",
	}))

	stmt := w.firstMemberStmt(t)
	if !strings.Contains(stmt, "'team-2'") {
		t.Errorf("destination team not written: %s", stmt)
	}
	update := updateClause(t, stmt)
	if !strings.Contains(update, "currentTeamId") {
		t.Errorf("move does not update currentTeamId: %s", update)
	}
	if strings.Contains(update, "initialTeamId") {
		t.Errorf("move overwrites initialTeamId, losing where the member started: %s", update)
	}

	// **Both** teams are recomputed: the origin lost a member and the destination
	// gained one. Recomputing only the destination is the plausible half-fix that
	// would leave the patrol the member left overstating its strength — and therefore
	// not showing as under styrke when it should, which is the whole point of the
	// number.
	counts := w.countStmts()
	if len(counts) != 2 {
		t.Fatalf("got %d recomputes for a move, want one per team: %v", len(counts), counts)
	}
	if !strings.Contains(counts[0], "'team-1'") {
		t.Errorf("origin team's strength not recomputed: %s", counts[0])
	}
	if !strings.Contains(counts[1], "'team-2'") {
		t.Errorf("destination team's strength not recomputed: %s", counts[1])
	}
}

// The recompute derives the count from the member rows rather than adjusting it.
//
// An increment would need the member's previous status, which no event carries, and
// would make the result depend on the order history happens to arrive in — fatal for
// a table rebuilt by replay on every restart. Asserting on the shape of the SQL is
// the only way to pin this without a database.
func TestStrengthIsRecomputedNotIncremented(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.withdrawal.requested", WithdrawalRequested{
		MemberID: "m-1", TeamID: "team-1",
	}))

	stmt := w.countStmts()[0]
	if !strings.Contains(stmt, "SELECT COUNT(*)") {
		t.Errorf("strength is not recomputed from the member rows: %s", stmt)
	}
	if strings.Contains(stmt, "activeMemberCount`+") || strings.Contains(stmt, "activeMemberCount`-") {
		t.Errorf("strength is adjusted rather than recomputed, so replay order matters: %s", stmt)
	}
	// Only racing members count. waiting members are still on the team but not on
	// the route, and counting them would hide every breach of the 3-member rule.
	if !strings.Contains(stmt, "'racing'") {
		t.Errorf("strength does not filter on racing: %s", stmt)
	}
	if !strings.Contains(stmt, "'2026'") {
		t.Errorf("strength is not year-scoped: %s", stmt)
	}
}

// A lifecycle event whose body names no team still updates the member, but must not
// write a strength against the empty team id.
func TestNoTeamMeansNoRecompute(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.withdrawal.requested", WithdrawalRequested{MemberID: "m-1"}))

	if len(w.memberStmts()) != 1 {
		t.Errorf("member row should still be written: %v", w.stmts)
	}
	if got := len(w.countStmts()); got != 0 {
		t.Errorf("recomputed a strength for no team: %v", w.countStmts())
	}
}

// An unrecognised status is refused rather than written, and the refusal is not an
// error: a poison message must not wedge the replay that rebuilds the whole table.
func TestUnknownStatusIsRefusedWithoutFailing(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.status.overridden", StatusOverridden{
		MemberID: "m-1", TeamID: "team-1", To: types.MemberStatus("nonsense"),
	}))

	if len(w.stmts) != 0 {
		t.Errorf("wrote an unreadable status into the read model: %v", w.stmts)
	}
}

// Replay is idempotent: the same event twice produces the same statement, so
// rebuilding from history cannot double-count anybody.
func TestReplayIsIdempotent(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	m := msg("NATHEJK.2026.spejder.m-1.withdrawal.requested", WithdrawalRequested{MemberID: "m-1", TeamID: "team-1"})
	handle(t, c, m)
	handle(t, c, m)

	members := w.memberStmts()
	if len(members) != 2 || members[0] != members[1] {
		t.Errorf("replay is not idempotent:\n%s\n%s", members[0], members[1])
	}
	// The recompute is idempotent for the same reason it is order-independent: it
	// asks the table, so running it twice lands the same number.
	counts := w.countStmts()
	if len(counts) != 2 || counts[0] != counts[1] {
		t.Errorf("strength recompute is not idempotent:\n%v", counts)
	}
}

// The year comes from the subject, never from the message clock.
//
// This is the bug the old commented-out projection shipped with: it used
// msg.Time().Year(). Since the table is rebuilt by replaying all of history, a 2026
// event replayed in 2027 would be filed under the wrong year — and every
// year-scoped query, which is all of them, would miss it.
func TestYearComesFromSubjectNotMessageTime(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msgAt(
		"NATHEJK.2026.spejder.m-1.withdrawal.requested",
		WithdrawalRequested{MemberID: "m-1", TeamID: "team-1"},
		time.Date(2027, 3, 1, 12, 0, 0, 0, time.UTC),
	))

	stmt := w.firstMemberStmt(t)
	if !strings.Contains(stmt, "'2026'") {
		t.Errorf("year not taken from the subject: %s", stmt)
	}
	if strings.Contains(stmt, "'2027'") {
		t.Errorf("year taken from the message clock: %s", stmt)
	}
}

// An unknown subject is a logged no-op rather than an error. The car and shelter
// interfaces will add subjects to this domain, and a projection that errored on
// anything unfamiliar would turn a forward-compatible deployment into a stuck
// replay.
func TestUnknownSubjectIsANoOp(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.spejder.m-1.teleported", nil))

	if len(w.stmts) != 0 {
		t.Errorf("unknown subject wrote something: %v", w.stmts)
	}
}

// Every subject the consumer declares must actually be handled.
//
// The two halves drift apart silently: a subject declared but unmatched means the
// projection subscribes to an event and quietly ignores it, which looks exactly
// like the event never being published.
func TestEverySubjectDeclaredIsHandled(t *testing.T) {
	c := &consumer{}
	for _, subj := range c.Consumes() {
		s := strings.ReplaceAll(subj.Subject(), ":", ".")
		s = strings.ReplaceAll(s, "*", "x")
		w := &recordingWriter{}
		c := &consumer{w: w}
		// Bodies differ per subject, so one permissive map stands in for all of
		// them: it carries every field any handler reads, including the members
		// list the start event needs to write anything at all. An unmatched subject
		// falls through to the default case and writes nothing, which is the
		// failure this test is looking for.
		body := map[string]any{
			"memberId": "m-1",
			"teamId":   "t-1",
			"toTeamId": "t-2",
			"to":       "racing",
			"members":  []map[string]any{{"memberId": "m-1"}},
		}
		if err := c.HandleMessage(msg(s, body)); err != nil {
			t.Fatalf("HandleMessage(%s): %v", s, err)
		}
		if len(w.stmts) == 0 {
			t.Errorf("subject %q is declared but not handled", subj.Subject())
		}
	}
}
