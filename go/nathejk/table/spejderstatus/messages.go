// Package spejderstatus is the member lifecycle: where every participant is, from
// the start line to the moment somebody else takes charge of them.
//
// It owns two facts. The first is each member's status and team membership, keyed
// by MemberID — the canonical read model that shared-go's types/member.go refers
// to when it says "these strings live in the spejderstatus projection". The
// second is derived from the first and written onto the team:
// patrulje.activeMemberCount, which is simultaneously a patrol's strength on the
// route and, when it reaches zero, the fact that the patrol is discontinued.
//
// The package is written to shared-go's guidelines so it can be lifted to
// shared-go/tables/spejderstatus unchanged (PRD 006 §8, roadmap task 083). The
// rule that makes that a file move rather than a rewrite is simple and unenforced
// by the compiler: nothing here may import nathejk.dk/... — see the guard in
// lift_test.go. In particular the acting user is passed in by the caller rather
// than read from the request context.
//
// # Who may write what
//
// Not every transition belongs to this repo, and that boundary is the point of
// the design rather than an accident of scheduling. A member is *self-carrying*
// up to and including MemberStatusWaiting: they have covered every metre on their
// own legs and a phone call has not changed that. Those transitions belong to the
// nødtelefon, which is hq. From the car door onwards each step is an
// **acceptance by the receiving party** — the driver accepts the member aboard,
// the shelter accepts them on arrival, a guardian or their own team accepts them
// at the end — and each of those interfaces will publish its own event when it
// exists. Custody is always confirmed by the receiver, never claimed by the party
// letting go, which is what makes the chain trustworthy.
//
// So this package publishes the withdrawal request, its cancellation, the
// override and the team move; it *consumes* everything downstream, tolerating
// transitions it did not cause, arriving in any order, for members on teams it
// has never heard of.
package spejderstatus

import "github.com/nathejk/shared-go/types"

// The bodies of the member lifecycle events, published on
// NATHEJK.{year}.spejder.{memberId}.{event} — see commands.go for the subjects.
//
// # One event per act, not one "status changed"
//
// A single generic status-changed event would fit this model badly. Each
// transition is a distinct act by a distinct party — a request to leave, a
// decision to carry on, an acceptance into a car, an acceptance at the shelter, a
// handover — so each gets its own type carrying the party responsible. That is
// what makes the *acceptor* recordable, which a bare {memberId, status} payload
// cannot express, and it answers "who is holding this member right now?" for
// free, because the car's acceptance event names the car.
//
// Each body resolves to exactly one types.MemberStatus, via Status(). The
// projection therefore never needs to know which event it is looking at in order
// to write a row, and a new transition added later cannot invent a status the
// lifecycle does not define.
//
// # No sosId, deliberately
//
// None of these carry a case id, even though the nødtelefon always has one when
// it publishes them. The case is a fact about *why the operator was on the
// phone*, not about the member, and the future car and shelter interfaces will
// publish these same events knowing nothing about cases at all. Putting a sosId
// here would either make it a lie for them or force them to invent one.
//
// The case link is carried by a separate summarising event on the sos entity,
// published once per operation after the member events it describes (PRD 006 §8).
// So sosId is a parameter of the *command*, never a field on the event.

// MemberEvent is the shape every lifecycle event shares: it concerns one member
// and resolves to one status.
//
// The projection depends on this interface rather than on the concrete types, so
// adding a transition — including one this repo does not publish — means adding a
// type and a subject, not editing the write path.
type MemberEvent interface {
	// Status is the status the member is in after this event.
	Status() types.MemberStatus
}

// Actor is who performed the act, resolved by the HTTP layer and passed in.
//
// It is defined here rather than reused from the sos package because this package
// may not import nathejk.dk/... — see the package comment. Today the value is
// empty in practice: authentication is perimeter-only, so the middleware puts an
// anonymous user with no id on every request (PRD 001 §6 Auth). It is recorded
// anyway, so that identity arriving later needs no change in the domain.
type Actor struct {
	UserID types.UserID `json:"userId,omitempty"`
	Name   string       `json:"name,omitempty"`
}

// WithdrawalRequested records that the member wants to leave the race and is
// waiting to be collected.
//
// The member is still self-carrying: they have accepted no help beyond a phone
// call, so this is not yet a withdrawal and their finish is intact. What it does
// mean is that their patrol may not continue until they are either collected or
// back on their feet — this state blocks the whole team, which is why it is the
// one worth an alarm when it lasts too long.
//
// It also starts the clock on the count that must reach zero before the
// organisers can go home. From here until somebody takes charge of the member,
// they are in our care.
type WithdrawalRequested struct {
	MemberID types.MemberID `json:"memberId"`
	TeamID   types.TeamID   `json:"teamId"`
	Actor    Actor          `json:"actor"`
}

func (WithdrawalRequested) Status() types.MemberStatus { return types.MemberStatusWaiting }

// WithdrawalCancelled records that the member decided to carry on under their own
// steam.
//
// Legitimate and expected, not a correction: plenty of members stop for a
// blister, a bad stretch or a cry and then walk the rest of it themselves. They
// go back to racing with their finish intact, because sitting by the trail costs
// them time, not the route.
//
// Valid only while the member is still waiting. Once a car has accepted them the
// lift cannot be uncrossed, so the command dirty-checks the current row and
// rejects this otherwise — see commands.go, where the race between an operator
// pressing resume and a driver accepting the member aboard is resolved in the
// driver's favour.
type WithdrawalCancelled struct {
	MemberID types.MemberID `json:"memberId"`
	TeamID   types.TeamID   `json:"teamId"`
	Actor    Actor          `json:"actor"`
}

func (WithdrawalCancelled) Status() types.MemberStatus { return types.MemberStatusRacing }

// StatusOverridden corrects a member's status by hand.
//
// This is the admission that something happened which the interface that owns it
// did not record — most often, before the car and shelter interfaces exist, a
// pickup or an arrival that only reached us by radio. It is deliberately a
// separate event from the transitions above rather than a parameterised setter,
// so that "how often are we correcting by hand?" stays answerable: a high count
// means the chain of custody is fiction, and that is worth knowing.
//
// MemberStatusFinished is never a valid target. Only a member who walked the
// route unaided has finished, and CanFinish() is true only for racing, so the
// finish can never be conferred by correction.
type StatusOverridden struct {
	MemberID types.MemberID     `json:"memberId"`
	TeamID   types.TeamID       `json:"teamId"`
	To       types.MemberStatus `json:"to"`
	Actor    Actor              `json:"actor"`
}

func (e StatusOverridden) Status() types.MemberStatus { return e.To }

// TeamMoved records that the member now belongs to a different team.
//
// This is what replaces the legacy patrulje.merged / patrulje.splited pair. Teams
// are not merged and split; a member is moved, and a team left with nobody racing
// is thereby discontinued. The old encoding pointed a teamId at a parentTeamId
// and had to be deleted again to undo itself, which is precisely the drift this
// avoids: membership is the only input, so the team fact follows and reverses on
// its own.
//
// FromTeamID is carried so the projection can recompute activeMemberCount for
// *both* teams without reading the previous row — the origin team loses a member
// and the destination gains one, and a replay must produce the same two counts
// whatever order it sees things in.
//
// The member's status does not change: a survivor moved into another patrol is
// still racing and still self-carrying, so they can still finish — with a team
// that is not the one they started with, which is why initialTeamId is never
// overwritten.
type TeamMoved struct {
	MemberID   types.MemberID `json:"memberId"`
	FromTeamID types.TeamID   `json:"fromTeamId"`
	ToTeamID   types.TeamID   `json:"toTeamId"`
	Actor      Actor          `json:"actor"`
}

func (TeamMoved) Status() types.MemberStatus { return types.MemberStatusRacing }

// PickupAccepted records that a car has taken the member aboard.
//
// Published by the dispatch desk (PRD 009, task 118) via spejderstatus.AcceptPickup, and
// eventually by the driver's own app — the driver accepts the member, and until they have a
// screen the dispatcher records it on their behalf. Defined here because this package's
// projection consumes it, and it was defined here *before* anything could publish it, so that
// the seam existed for the interface to be built against.
//
// This is the point of no return. It is the first outside help the member has
// taken, so there is no way back onto the route and no finish to be had: the
// endings available from here are reunited and released.
//
// SectionSlug names the dispatch unit holding the member — the question a dashboard actually
// needs answered while somebody is in transit. A unit and not a vehicle id, deliberately: the
// unit is who took them, and it survives a car being swapped mid-night. (It replaced a `Car`
// string during task 118, which was safe to change outright because nothing had ever published
// this event.)
type PickupAccepted struct {
	MemberID     types.MemberID `json:"memberId"`
	TeamID       types.TeamID   `json:"teamId"`
	SectionSlug  types.Slug     `json:"sectionSlug,omitempty"`
	DriverUserID types.UserID   `json:"driverUserId,omitempty"`
	Actor        Actor          `json:"actor"`
}

func (PickupAccepted) Status() types.MemberStatus { return types.MemberStatusTransit }

// ShelterAccepted records that HQ has received the member and is looking after
// them — put to bed if it is the middle of the night, waiting in the warm if
// somebody is already on the way. Which of the two depends on the hour rather than
// on anything worth tracking, so it is one state.
//
// Published by the shelter interface (PRD 007), which is also the only party that can
// say it: the receiver confirms custody, never the party letting go.
//
// Placement is where in the shelter the member was put, and it is optional because the
// two facts arrive together in the ordinary case and separately in the awkward one. A
// crew member receiving three scouts off a car types the tent once for all of them; a
// crew member receiving somebody at a run records the arrival now and where they ended
// up when they get back. Requiring it would push the second case into either a lie or a
// second screen.
type ShelterAccepted struct {
	MemberID  types.MemberID `json:"memberId"`
	TeamID    types.TeamID   `json:"teamId"`
	Placement string         `json:"placement,omitempty"`
	Actor     Actor          `json:"actor"`
}

func (ShelterAccepted) Status() types.MemberStatus { return types.MemberStatusSheltered }

// ShelterPlaced records where in the shelter the member is — the answer to "which tent
// is she in?", asked at 3am by a parent standing at the door.
//
// Its own event rather than a re-published ShelterAccepted, because moving a sleeping
// child from one tent to another is a distinct act and reads as one on the timeline. A
// re-publish would also claim custody was taken twice, which is exactly the fiction the
// acceptance events exist to prevent.
//
// The status it resolves to is `sheltered`, unchanged: placing somebody does not move
// them through the lifecycle. The projection's write is therefore idempotent and the
// placement is the point — which is why the placering lives in hq's own `shelter` table
// rather than on spejderstatus. A bed is a fact about the shelter, not about the
// lifecycle, and this package is queued for lifting to shared-go verbatim.
//
// Placement is deliberately free text. The zones are not known until race start (PRD 007
// §6), so no vocabulary can be defined here; the interface suggests what is already in
// use and enforces nothing.
type ShelterPlaced struct {
	MemberID  types.MemberID `json:"memberId"`
	TeamID    types.TeamID   `json:"teamId"`
	Placement string         `json:"placement"`
	Actor     Actor          `json:"actor"`
}

func (ShelterPlaced) Status() types.MemberStatus { return types.MemberStatusSheltered }

// HandoverCompleted records that somebody else has taken charge of the member and
// we no longer track them. This is what takes them out of the in-our-care count.
//
// Two endings, and they are not interchangeable, which is why To is on the event
// rather than being guessed from the hour: released means a guardian came for them
// during the night, reunited means their own team reached the finish and the
// member was handed back to it. Neither is finished — see MemberStatusFinished for
// why keeping that apart is what lets a finish be counted as an achievement rather
// than as attendance.
type HandoverCompleted struct {
	MemberID types.MemberID     `json:"memberId"`
	TeamID   types.TeamID       `json:"teamId"`
	To       types.MemberStatus `json:"to"`
	Actor    Actor              `json:"actor"`
}

func (e HandoverCompleted) Status() types.MemberStatus { return e.To }
