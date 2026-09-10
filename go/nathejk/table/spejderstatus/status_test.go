package spejderstatus

import (
	"testing"

	"github.com/nathejk/shared-go/types"
)

// The whole mapping documented in shared-go's types/member.go, plus the current
// values that must survive it untouched.
//
// Table-driven rather than a handful of assertions because the failure mode is
// per-value and silent: one missing entry means one legacy state that never
// reaches the in-our-care count, and nothing else in the system would complain.
func TestParseMemberStatus(t *testing.T) {
	tests := []struct {
		in   string
		want types.MemberStatus
		ok   bool
	}{
		// Superseded values, per the documented mapping.
		{"REGISTERED", types.MemberStatusRegistered, true},
		{"STARTED", types.MemberStatusRacing, true},
		{"active", types.MemberStatusRacing, true},
		{"emergency", types.MemberStatusWaiting, true},
		{"hq", types.MemberStatusSheltered, true},
		{"out", types.MemberStatusReleased, true},

		// Documented as unchanged. These are current values that happen also to
		// be legacy ones, which is why they must not be mapped to anything.
		{"waiting", types.MemberStatusWaiting, true},
		{"transit", types.MemberStatusTransit, true},

		// The rest of the current set, passing through.
		{"registered", types.MemberStatusRegistered, true},
		{"seated", types.MemberStatusSeated, true},
		{"racing", types.MemberStatusRacing, true},
		{"finished", types.MemberStatusFinished, true},
		{"sheltered", types.MemberStatusSheltered, true},
		{"reunited", types.MemberStatusReunited, true},
		{"released", types.MemberStatusReleased, true},

		// No status recorded: readable, not an error.
		{"", types.MemberStatusNone, true},

		// Values hq's own SQL invents (internal/data/member.go:42) but which have
		// never been events. 'started' happens to be a documented legacy value and
		// maps; 'paid' is not a status at all and must be rejected rather than
		// quietly accepted, or task 067's rework would have nothing to catch it.
		{"paid", types.MemberStatusNone, false},

		// Unknown.
		{"nonsense", types.MemberStatusNone, false},
		{"RACING_", types.MemberStatusNone, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseMemberStatus(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Errorf("ParseMemberStatus(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// Whatever the mapping produces must be storable.
//
// The projection writes the result straight into a row that the rest of the system
// reads back as a types.MemberStatus, so a mapping target that Valid() rejects
// would be a value nothing downstream can reason about — the same silent
// under-reporting the mapping exists to prevent, reintroduced by the fix.
func TestEveryMappedValueIsValid(t *testing.T) {
	for legacy, mapped := range legacyMemberStatus {
		if !mapped.Valid() {
			t.Errorf("%q maps to %q, which types.MemberStatus does not recognise", legacy, mapped)
		}
	}
}

// Case folding, asserted separately because the reason is historical rather than
// obvious: the same state was published as both REGISTERED and registered, and
// STARTED and started, depending on the era of the producer.
func TestParseMemberStatusFoldsCase(t *testing.T) {
	for _, in := range []string{"ACTIVE", "Active", "active", "aCtIvE"} {
		got, ok := ParseMemberStatus(in)
		if !ok || got != types.MemberStatusRacing {
			t.Errorf("ParseMemberStatus(%q) = (%q, %v), want (racing, true)", in, got, ok)
		}
	}
}

// The in-our-care set is derived from shared-go's helper, never listed by hand.
//
// Asserted because the whole point is that a fourth in-care state added to
// types.MemberStatus starts counting without anybody editing a query — and the way
// that breaks is somebody "simplifying" InOurCareStatuses() into a literal slice.
func TestInOurCareStatusesIsDerived(t *testing.T) {
	got := InOurCareStatuses()

	want := []types.MemberStatus{
		types.MemberStatusWaiting,
		types.MemberStatusTransit,
		types.MemberStatusSheltered,
	}
	if len(got) != len(want) {
		t.Fatalf("InOurCareStatuses() = %v, want %v", got, want)
	}
	for i, s := range want {
		if got[i] != s {
			t.Errorf("InOurCareStatuses()[%d] = %q, want %q", i, got[i], s)
		}
	}
	// Every listed status must agree with the helper, and nothing outside the set may.
	for _, s := range allMemberStatuses {
		inSet := false
		for _, c := range got {
			if c == s {
				inSet = true
			}
		}
		if inSet != s.InOurCare() {
			t.Errorf("%q: in set = %v, InOurCare() = %v", s, inSet, s.InOurCare())
		}
	}
}

// allMemberStatuses stands in for an enumeration shared-go cannot provide (Valid()
// is a switch), so every entry must at least be one it recognises. This catches a
// typo; nothing can catch an omission, which is why the list carries a comment
// saying so — the count assertion is the closest available substitute.
func TestAllMemberStatusesAreValid(t *testing.T) {
	for _, s := range allMemberStatuses {
		if !s.Valid() {
			t.Errorf("%q is listed in allMemberStatuses but types.MemberStatus rejects it", s)
		}
	}
	if len(allMemberStatuses) != 9 {
		t.Errorf("allMemberStatuses has %d entries, want the 9 the lifecycle defines — "+
			"if a status was added to shared-go, add it here too", len(allMemberStatuses))
	}
}

// A legacy value must never be mistaken for a member who can still finish.
//
// 'active' and 'STARTED' map to racing, which *can* finish — correctly, those
// members are on the route. But 'hq' and 'out' describe members who were driven in,
// and if either mapped to anything CanFinish() accepts, replaying old history would
// hand a finish to somebody who took a lift. Cheap to assert, impossible to notice
// otherwise.
func TestLegacyOutcomesCannotFinish(t *testing.T) {
	for _, legacy := range []string{"emergency", "hq", "out"} {
		got, ok := ParseMemberStatus(legacy)
		if !ok {
			t.Fatalf("ParseMemberStatus(%q) failed", legacy)
		}
		if got.CanFinish() {
			t.Errorf("legacy %q maps to %q, which CanFinish() accepts", legacy, got)
		}
	}
}
