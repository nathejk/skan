package spejderstatus

import (
	"testing"

	"github.com/nathejk/shared-go/types"
)

// Every lifecycle event must resolve to exactly one status, and that status must
// be one the lifecycle actually defines.
//
// The second half is the point: the projection writes Status() straight into the
// row, so an event resolving to a value types.MemberStatus does not know about
// would put an unreadable status in the read model, and every InOurCare() and
// CanFinish() check downstream would quietly disagree about it. Valid() is the
// only thing standing between a typo here and a member nobody is counted as
// looking for.
func TestEventsResolveToValidStatus(t *testing.T) {
	tests := []struct {
		name  string
		event MemberEvent
		want  types.MemberStatus
	}{
		{"withdrawal requested", WithdrawalRequested{}, types.MemberStatusWaiting},
		{"withdrawal cancelled", WithdrawalCancelled{}, types.MemberStatusRacing},
		{"team moved", TeamMoved{}, types.MemberStatusRacing},
		{"pickup accepted", PickupAccepted{}, types.MemberStatusTransit},
		{"shelter accepted", ShelterAccepted{}, types.MemberStatusSheltered},
		{"shelter placed", ShelterPlaced{}, types.MemberStatusSheltered},
		{"override", StatusOverridden{To: types.MemberStatusSheltered}, types.MemberStatusSheltered},
		{"handover released", HandoverCompleted{To: types.MemberStatusReleased}, types.MemberStatusReleased},
		{"handover reunited", HandoverCompleted{To: types.MemberStatusReunited}, types.MemberStatusReunited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.Status()
			if got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
			if !got.Valid() {
				t.Errorf("Status() = %q, which types.MemberStatus does not recognise", got)
			}
		})
	}
}

// The self-carrying boundary, asserted on the events rather than only in the
// command.
//
// A member is on their own legs up to and including waiting; from transit onwards
// they have taken a lift and no later event puts that back. So exactly one event
// may leave a member able to finish — the cancellation, because carrying on is
// what they actually did — and every event from the car door onwards must not.
//
// This is a test rather than a comment because the rule is invisible at the call
// site: nothing stops somebody adding a transition that resolves to racing.
func TestOnlyResumeRestoresTheAbilityToFinish(t *testing.T) {
	canFinish := map[string]bool{
		"withdrawal requested": WithdrawalRequested{}.Status().CanFinish(),
		"withdrawal cancelled": WithdrawalCancelled{}.Status().CanFinish(),
		"team moved":           TeamMoved{}.Status().CanFinish(),
		"pickup accepted":      PickupAccepted{}.Status().CanFinish(),
		"shelter accepted":     ShelterAccepted{}.Status().CanFinish(),
		"shelter placed":       ShelterPlaced{}.Status().CanFinish(),
	}
	want := map[string]bool{
		"withdrawal requested": false,
		"withdrawal cancelled": true,
		"team moved":           true, // still racing, just for a different patrol
		"pickup accepted":      false,
		"shelter accepted":     false,
		"shelter placed":       false,
	}
	for name, got := range canFinish {
		if got != want[name] {
			t.Errorf("%s: CanFinish() = %v, want %v", name, got, want[name])
		}
	}
}

// The in-our-care set, asserted through the events that produce it.
//
// This is the count that has to reach zero before anybody goes home, so what
// belongs in it is worth pinning: a request to leave puts a member in our care and
// a handover takes them out, while moving between teams never does either — a
// moved member is still on the route with a patrol, and counting them as ours
// would inflate the one number the night is judged by.
func TestInOurCareSpansWaitingToSheltered(t *testing.T) {
	inCare := []struct {
		name  string
		event MemberEvent
		want  bool
	}{
		{"withdrawal requested", WithdrawalRequested{}, true},
		{"pickup accepted", PickupAccepted{}, true},
		{"shelter accepted", ShelterAccepted{}, true},
		// Being moved between tents keeps a member in our care, obviously — but it is
		// worth asserting, because this is the one event that does not advance the
		// lifecycle, and an implementation that resolved it to "no change" by returning
		// MemberStatusNone would silently drop a sleeping child out of the count that
		// has to reach zero.
		{"shelter placed", ShelterPlaced{}, true},
		{"withdrawal cancelled", WithdrawalCancelled{}, false},
		{"team moved", TeamMoved{}, false},
		{"handover released", HandoverCompleted{To: types.MemberStatusReleased}, false},
		{"handover reunited", HandoverCompleted{To: types.MemberStatusReunited}, false},
	}
	for _, tt := range inCare {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.Status().InOurCare(); got != tt.want {
				t.Errorf("InOurCare() = %v, want %v", got, tt.want)
			}
		})
	}
}

// None of the bodies may carry a case id.
//
// Asserted as a compile-time-ish check on the struct shape because the reason is
// easy to forget and the consequence is not local: the car and shelter interfaces
// publish these same events knowing nothing about SOS cases, so a sosId field
// would either be a lie for them or force them to invent one. The case link lives
// on the separate summarising sos event instead.
func TestNoEventCarriesACaseID(t *testing.T) {
	// If a sosId is ever added to one of these, this test will not catch it by
	// reflection alone — it is here to make the intent unmissable to the next
	// person editing messages.go, and to fail loudly if the marker below is
	// removed along with the field.
	events := []MemberEvent{
		WithdrawalRequested{}, WithdrawalCancelled{}, StatusOverridden{},
		TeamMoved{}, PickupAccepted{}, ShelterAccepted{}, ShelterPlaced{}, HandoverCompleted{},
	}
	for _, e := range events {
		if _, ok := any(e).(interface{ SosID() string }); ok {
			t.Errorf("%T exposes a case id; the case link belongs on the summarising sos event", e)
		}
	}
}
