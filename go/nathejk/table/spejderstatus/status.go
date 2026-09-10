package spejderstatus

import (
	"strings"

	"github.com/nathejk/shared-go/types"
)

// Normalising the status values that predate the current lifecycle.
//
// # Why this exists at all
//
// The projection is rebuilt from the full JetStream history on every API restart,
// so replay does not encounter a clean stream of current values — it encounters
// everything the platform has ever published, including several spellings of the
// same idea from before types.MemberStatus was written down. shared-go's
// types/member.go documents the mapping under "Persisted values" but does not
// implement it.
//
// Leaving it un-normalised would not fail loudly. Every value would still be a
// string, every row would still be written, and every InOurCare() and
// CanFinish() check downstream would silently return false for statuses it did
// not recognise — so a member sitting in the legacy "hq" state would be someone
// we are responsible for who does not appear in the count of people we are
// responsible for. That is the exact failure the in-our-care number exists to
// prevent, which is why this is a mapping with tests rather than a switch in the
// consumer.
//
// # Deliberately not included
//
// hq's own SQL invents two more values — 'started' and 'paid', from the fallback
// at internal/data/member.go:42 — which have never been published as events and
// are not statuses at all, just a query papering over a missing row. They are not
// mapped here; they are deleted in task 067. Adding them would legitimise them.

// legacyMemberStatus maps superseded persisted values onto the current lifecycle,
// exactly as documented in shared-go's types/member.go.
//
// Keyed lowercase; ParseMemberStatus folds case before looking up, because the
// old values were published in both REGISTERED and registered form and the pair
// mean the same thing.
var legacyMemberStatus = map[string]types.MemberStatus{
	"registered": types.MemberStatusRegistered,
	"started":    types.MemberStatusRacing,
	"active":     types.MemberStatusRacing,
	"emergency":  types.MemberStatusWaiting,
	"hq":         types.MemberStatusSheltered,
	"out":        types.MemberStatusReleased,
}

// ParseMemberStatus turns a persisted or published status string into a current
// types.MemberStatus.
//
// Current values pass through unchanged. Superseded ones are mapped per
// legacyMemberStatus. The empty string is MemberStatusNone — a member read from a
// projection that predates status tracking, which is readable but not a valid
// thing to store.
//
// Anything else returns MemberStatusNone and ok=false. Returning the unknown
// value as-is was the alternative and is worse: it would let a typo or a value
// from some future producer land in the read model, where Valid() would reject it
// but only after it had already been written. Refusing it at the boundary means
// the projection can decide what to do — log it, skip it — while the row keeps a
// status the rest of the code can reason about. The caller gets to tell "not
// recorded" from "recorded as something I do not understand", which are different
// problems: the first is a member who has not started, the second is a bug.
func ParseMemberStatus(s string) (types.MemberStatus, bool) {
	if s == "" {
		return types.MemberStatusNone, true
	}
	if status := types.MemberStatus(s); status.Valid() {
		return status, true
	}
	if status, ok := legacyMemberStatus[strings.ToLower(s)]; ok {
		return status, true
	}
	return types.MemberStatusNone, false
}

// allMemberStatuses is every status the lifecycle defines.
//
// shared-go has no such list — Valid() is a switch, which cannot be enumerated — so
// it is spelled out here once and derived from thereafter. There is a test asserting
// every entry is Valid(), which catches a typo, but nothing can catch an *omission*
// from here, so anything added to types.MemberStatus must be added here too.
var allMemberStatuses = []types.MemberStatus{
	types.MemberStatusRegistered,
	types.MemberStatusSeated,
	types.MemberStatusRacing,
	types.MemberStatusFinished,
	types.MemberStatusWaiting,
	types.MemberStatusTransit,
	types.MemberStatusSheltered,
	types.MemberStatusReunited,
	types.MemberStatusReleased,
}

// InOurCareStatuses is the set of statuses that mean Nathejk is responsible for a
// member's physical whereabouts.
//
// Derived by asking InOurCare() rather than by listing waiting/transit/sheltered,
// which matters for the reason PRD 006 keeps insisting on: the set is shared-go's to
// define, and a fourth in-care state added there must start counting here without
// anybody remembering to edit a query. Listing them at the call site is how a member
// ends up in our care but not in the count of members in our care.
func InOurCareStatuses() []types.MemberStatus {
	var out []types.MemberStatus
	for _, s := range allMemberStatuses {
		if s.InOurCare() {
			out = append(out, s)
		}
	}
	return out
}
