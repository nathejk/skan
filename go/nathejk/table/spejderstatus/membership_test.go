package spejderstatus

import (
	"testing"
	"time"

	"github.com/nathejk/shared-go/types"
)

// The interval walk, tested as a pure function over log rows.
//
// TeamMemberships itself needs a database, and the SQL is straightforward; what is not straightforward
// is deciding where one membership ends and another begins from a sequence of events, so that is
// extracted and tested directly. Three cases here are ones I would otherwise have got wrong: an event
// carrying no team, a member who returns, and a member with no history at all.

func at(min int) time.Time {
	return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)
}

// intervalsFor is the loop inside TeamMemberships, lifted so it can be tested without a db.
//
// Kept as a package-level function rather than a method so the test drives exactly the logic the
// query uses — if this diverges from TeamMemberships, the test stops meaning anything, so
// TeamMemberships calls it too.
func TestIntervalsFor(t *testing.T) {
	const team = types.TeamID("t-1")

	tests := []struct {
		name    string
		entries []logEntry
		want    []Membership
	}{
		{
			name:    "still on the team",
			entries: []logEntry{{team: "t-1", createdAt: at(0)}},
			want:    []Membership{{From: at(0)}},
		},
		{
			name: "moved away closes the interval",
			entries: []logEntry{
				{team: "t-1", createdAt: at(0)},
				{team: "t-2", createdAt: at(30)},
			},
			want: []Membership{{From: at(0), To: ptrTime(at(30))}},
		},
		{
			// A member who joins, leaves and comes back yields two intervals rather than one long
			// one spanning their absence — otherwise the other patrol's movement would appear on
			// this patrol's map.
			name: "returning member yields two intervals",
			entries: []logEntry{
				{team: "t-1", createdAt: at(0)},
				{team: "t-2", createdAt: at(30)},
				{team: "t-1", createdAt: at(60)},
			},
			want: []Membership{
				{From: at(0), To: ptrTime(at(30))},
				{From: at(60)},
			},
		},
		{
			// The case worth being careful about. Several lifecycle events legitimately carry no
			// team (consumer.teamID returns "" for events that do not name one), and treating that
			// as a departure would cut a member's track short at, say, a withdrawal request.
			name: "event with no team does not end the membership",
			entries: []logEntry{
				{team: "t-1", createdAt: at(0)},
				{team: "", createdAt: at(30)},
				{team: "t-1", createdAt: at(60)},
			},
			want: []Membership{{From: at(0)}},
		},
		{
			name: "never on this team",
			entries: []logEntry{
				{team: "t-2", createdAt: at(0)},
				{team: "t-3", createdAt: at(30)},
			},
			want: nil,
		},
		{
			// Repeated events while on the team must not restart the interval — the membership began
			// at the first of them.
			name: "repeated events keep the original start",
			entries: []logEntry{
				{team: "t-1", createdAt: at(0)},
				{team: "t-1", createdAt: at(10)},
				{team: "t-1", createdAt: at(20)},
			},
			want: []Membership{{From: at(0)}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := intervalsFor("m-1", team, tc.entries)

			if len(got) != len(tc.want) {
				t.Fatalf("want %d intervals, got %d: %+v", len(tc.want), len(got), got)
			}
			for i := range got {
				if !got[i].From.Equal(tc.want[i].From) {
					t.Errorf("interval %d: From = %v, want %v", i, got[i].From, tc.want[i].From)
				}
				switch {
				case tc.want[i].To == nil && got[i].To != nil:
					t.Errorf("interval %d: want open, got closed at %v", i, *got[i].To)
				case tc.want[i].To != nil && got[i].To == nil:
					t.Errorf("interval %d: want closed at %v, got open", i, *tc.want[i].To)
				case tc.want[i].To != nil && !got[i].To.Equal(*tc.want[i].To):
					t.Errorf("interval %d: To = %v, want %v", i, *got[i].To, *tc.want[i].To)
				}
				if got[i].MemberID != "m-1" {
					t.Errorf("interval %d: MemberID = %q", i, got[i].MemberID)
				}
			}
		})
	}
}

func TestMembershipOpen(t *testing.T) {
	end := at(10)
	if !(Membership{}).Open() {
		t.Error("a membership with no To is open")
	}
	if (Membership{To: &end}).Open() {
		t.Error("a membership with a To is closed")
	}
}

// A member with no lifecycle events yields nothing from the interval walk — which is correct, and is
// exactly why the roster query exists alongside it.
//
// This is the regression guard for task 154: a patrol that has not started has no `spejderstatuslog`
// rows at all, so a membership query built on history alone returned an empty patrol and the track map
// showed nothing while a position glyph sat next to the scout's name. The walk returning nothing here
// is the *expected* half of that; `rosterWithoutHistory` supplies the member, and it reads `spejder`
// (the current roster) as well as `spejderstatus`.
func TestIntervalsForNoHistoryYieldsNothing(t *testing.T) {
	if got := intervalsFor("m-1", "t-1", nil); len(got) != 0 {
		t.Errorf("want no intervals from an empty log, got %+v", got)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
