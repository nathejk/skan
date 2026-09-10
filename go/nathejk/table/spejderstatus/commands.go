package spejderstatus

import (
	"context"
	"errors"
	"fmt"

	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// Commands is the write surface for the member lifecycle, as the application sees
// it.
//
// Every method takes an Actor rather than reading the acting user out of the
// request context, for the same reason the sos package does: reading the context
// here would mean importing nathejk.dk/internal/requestctx and turning the
// eventual lift into a rewrite (task 083).
//
// # What these do not do
//
// They publish the **member** events only. The summarising case event that puts the
// operation on a timeline is the sos package's, and this package may not import it
// — so the *handler* composes the two, which is the layer that is allowed to know
// about both. Each method therefore returns what it changed, in enough detail for
// the caller to build that summary without re-reading anything.
//
// They also do not validate the destination of a move beyond "not where the member
// already is". Whether a target patrol exists, started, and is still racing is a
// question about the patrulje entity, which this package equally may not import;
// the handler checks it before calling. Documented rather than silently assumed,
// because it is the one place a caller could pass something meaningless.
//
// # What they refuse
//
// The self-carrying boundary, and only that. A member is on their own legs up to
// and including waiting, so those are the transitions this interface owns; from the
// car door onwards the events belong to the car and shelter interfaces. The
// requirement that a patrol keep three racing members is deliberately **not**
// enforced here — see PRD 006 §8: the member is leaving regardless, and refusing to
// record it would only make the data wrong.
type Commands interface {
	RequestWithdrawal(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID) (*Change, error)
	CancelWithdrawal(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID) (*Change, error)
	OverrideStatus(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.MemberStatus) (*Change, error)
	MoveTeam(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.TeamID) (*Move, error)
	MoveMembers(ctx context.Context, actor Actor, year types.YearSlug, ids []types.MemberID, to types.TeamID) ([]Move, error)
	CollectTeam(ctx context.Context, actor Actor, year types.YearSlug, team types.TeamID) ([]Change, error)

	// The shelter's acceptances (PRD 007). Added here rather than in a package of their own
	// because they are status transitions, and this package owns the lifecycle; the
	// *placering* is not a status and lives in hq's own shelter package instead.
	//
	// Note what this does to the "self-carrying boundary" rule above: it is no longer quite
	// true that this package publishes only the transitions a member makes on their own
	// legs. The rule that survives, and the one that mattered, is that **custody is
	// confirmed by the receiver**: the shelter interface calls these, and it is the
	// receiving party. A driver's pickup is still not ours to publish.
	AcceptIntoShelter(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, placement string) (*Change, error)

	// The car's acceptance (PRD 009 §8, task 118). The other half of the same argument as
	// AcceptIntoShelter above: **custody is confirmed by the receiver**, and here the driver
	// is the receiver. Until the drivers have a screen of their own the dispatcher records it
	// on their behalf, which is a question of who is at the keyboard rather than of who took
	// the scout — hence the unit is passed in explicitly and never inferred from the actor.
	//
	// This is what closes the hole PRD 007 §8 named as the single biggest risk to
	// Hønsegården: before it, nothing in hq published `transit` except an operator remembering
	// to override a status, so *På vej* was empty while cars were on their way.
	//
	// A batch, because one stop collects several scouts and two members of one patrol leaving
	// together is one act — not two decisions that happen to coincide.
	//
	// The unit is a **section slug**, not a vehicle id: the unit is who took them, and it
	// survives a car being swapped mid-night. `driver` is recorded beside it for the same
	// reason the shelter records its actor — empty until HQ has login, and wired now so nothing
	// has to change when it arrives.
	AcceptPickup(ctx context.Context, actor Actor, year types.YearSlug, ids []types.MemberID, unit types.Slug, driver types.UserID) ([]Change, error)
	CompleteHandover(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.MemberStatus) (*Change, error)
}

var (
	// ErrAlreadyCollected is returned when an operator tries to put a member back
	// into the race after a car has accepted them.
	//
	// Its own error rather than a generic conflict because the operator can act on
	// it: it means "allerede hentet", and the answer is to tell the caller the
	// scout is on their way to HQ, not to retry. The race it guards is real and
	// narrow — the operator presses resume in the same moment the driver accepts
	// the member aboard — and the acceptance wins, because it reflects a member
	// physically sitting in a car.
	ErrAlreadyCollected = errors.New("member has already been collected")

	// ErrNotWaiting is returned when a resume is attempted on a member who never
	// asked to leave.
	ErrNotWaiting = errors.New("member is not waiting")

	// ErrNotSelfCarrying is returned when a withdrawal is requested for a member
	// who is no longer on their own legs. Requesting one is meaningless once a car
	// has them: they have already left the route.
	ErrNotSelfCarrying = errors.New("member is no longer self-carrying")

	// ErrCannotFinish is returned when a correction tries to mark a member
	// finished. Only walking the route unaided earns that, so no override may
	// confer it — see types.MemberStatus.CanFinish.
	ErrCannotFinish = errors.New("finished cannot be set by hand")

	// ErrInvalidStatus is returned for a status this version of the code does not
	// know.
	ErrInvalidStatus = errors.New("unknown member status")

	// ErrSameTeam is returned when a member is moved to the team they are already
	// on. Not an error the operator caused so much as one the UI should have
	// prevented, but publishing it would put a meaningless line on a timeline.
	ErrSameTeam = errors.New("member is already on that team")

	// ErrNotStarted is returned when the shelter tries to accept a member who never
	// started — one still `registered` or `seated`.
	//
	// The only refusal AcceptIntoShelter makes, and it is not pedantry: those members are at
	// home, so an acceptance for one of them is a mistyped or misclicked identity, and
	// recording it would invent a child in our care who is not on site. Every *started*
	// status is accepted, including `racing` — see AcceptIntoShelter.
	ErrNotStarted = errors.New("member has not started")

	// ErrNotAnEnding is returned when a handover names something that is not one of the two
	// ways a member leaves our care.
	//
	// `released` means a guardian came for them, `reunited` means their own patrol reached
	// the finish and took them back. They are not interchangeable and neither is `finished`,
	// which is why the caller must say which one rather than the command guessing from the
	// hour.
	ErrNotAnEnding = errors.New("handover must be to released or reunited")
)

// Change is what a status-changing command did, for the caller to summarise.
//
// From is included because the timeline entry has to say "fra racing til waiting"
// and keep saying it: the summary is stored, not re-derived, so the previous status
// has to be captured at the moment it stopped being true.
//
// TeamStrength is the team's racing count *after* this change, computed here rather
// than read back from patrulje.activeMemberCount. That is not an optimisation: the
// projection has not seen the event yet at this point, so the column still holds
// the old number. Computing it from the members this command can see is the only
// way to record the resulting strength without waiting for a projection to catch
// up.
type Change struct {
	MemberID     types.MemberID
	TeamID       types.TeamID
	From         types.MemberStatus
	To           types.MemberStatus
	TeamStrength int
}

// Move is what a team move did. Two strengths, because a move changes both teams.
type Move struct {
	MemberID         types.MemberID
	FromTeamID       types.TeamID
	ToTeamID         types.TeamID
	FromTeamStrength int
	ToTeamStrength   int
}

type commander struct {
	p stream.Publisher
	q Queries
}

// RequestWithdrawal records that a member wants to leave the race and is waiting to
// be collected.
//
// Idempotent: a member who is already waiting produces no event, so a double click
// or a retried request does not put two lines on a timeline. That is the house
// pattern — a no-op write publishes nothing and therefore emits no live signal
// either.
func (c commander) RequestWithdrawal(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID) (*Change, error) {
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.Status == types.MemberStatusWaiting {
		return nil, nil
	}
	// Only from racing. Anything past waiting means a car already has them, and a
	// member in a car has not "asked to leave" — they have left.
	if current.Status != types.MemberStatusRacing {
		return nil, ErrNotSelfCarrying
	}
	change, err := c.change(ctx, year, current, types.MemberStatusWaiting)
	if err != nil {
		return nil, err
	}
	body := &WithdrawalRequested{MemberID: id, TeamID: current.CurrentTeamID, Actor: actor}
	if err := c.publish(actor, year, id, "withdrawal.requested", body); err != nil {
		return nil, err
	}
	return change, nil
}

// CancelWithdrawal puts a member who changed their mind back into the race.
//
// Valid only while they are still self-carrying, and enforced here rather than only
// in the UI: the operator's screen may be a moment stale, and that moment is exactly
// when a car is accepting the member.
func (c commander) CancelWithdrawal(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID) (*Change, error) {
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.Status == types.MemberStatusRacing {
		return nil, nil
	}
	if current.Status != types.MemberStatusWaiting {
		// Distinguish the two so the operator gets an answer they can use. "Already
		// collected" is a fact about where the scout is; "not waiting" means the
		// screen was wrong about them.
		if current.Status.InOurCare() {
			return nil, ErrAlreadyCollected
		}
		return nil, ErrNotWaiting
	}
	change, err := c.change(ctx, year, current, types.MemberStatusRacing)
	if err != nil {
		return nil, err
	}
	body := &WithdrawalCancelled{MemberID: id, TeamID: current.CurrentTeamID, Actor: actor}
	if err := c.publish(actor, year, id, "withdrawal.cancelled", body); err != nil {
		return nil, err
	}
	return change, nil
}

// OverrideStatus corrects a member's status by hand.
//
// **Deliberately lenient about ordering** (decided 2026-08-17): any valid status is
// accepted from any other, without enforcing the documented sequence. This is the
// out-of-sync repair tool — its whole purpose is recording a reality that did not
// follow the diagram, and a version that rejected racing → sheltered because no
// pickup was logged would refuse precisely the correction it exists to make. The
// timeline shows what happened, which is the honest record.
//
// The one exception is `finished`, which no correction may confer.
func (c commander) OverrideStatus(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.MemberStatus) (*Change, error) {
	if to == types.MemberStatusFinished {
		return nil, ErrCannotFinish
	}
	if !to.Valid() {
		return nil, ErrInvalidStatus
	}
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.Status == to {
		return nil, nil
	}
	change, err := c.change(ctx, year, current, to)
	if err != nil {
		return nil, err
	}
	body := &StatusOverridden{MemberID: id, TeamID: current.CurrentTeamID, To: to, Actor: actor}
	if err := c.publish(actor, year, id, "status.overridden", body); err != nil {
		return nil, err
	}
	return change, nil
}

// MoveTeam moves a member to another patrol.
//
// The member keeps racing and keeps their initial team; only currentTeamId changes.
// Whether the destination is a real, started, still-racing patrol is the caller's to
// check — see the note on Commands.
func (c commander) MoveTeam(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.TeamID) (*Move, error) {
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.CurrentTeamID == to {
		return nil, ErrSameTeam
	}

	// Both teams' resulting strengths, computed before the event is published for
	// the same reason as Change.TeamStrength: the projection has not run yet.
	from, err := c.strengthAfter(ctx, year, current.CurrentTeamID, id, types.MemberStatusNone)
	if err != nil {
		return nil, err
	}
	dest, err := c.strengthAfter(ctx, year, to, id, types.MemberStatusRacing)
	if err != nil {
		return nil, err
	}

	body := &TeamMoved{MemberID: id, FromTeamID: current.CurrentTeamID, ToTeamID: to, Actor: actor}
	if err := c.publish(actor, year, id, "team.moved", body); err != nil {
		return nil, err
	}
	return &Move{
		MemberID:         id,
		FromTeamID:       current.CurrentTeamID,
		ToTeamID:         to,
		FromTeamStrength: from,
		ToTeamStrength:   dest,
	}, nil
}

// MoveMembers moves several members to the same patrol in one operation.
//
// # Why this exists alongside MoveTeam
//
// Moving is per member — two survivors may end up in two different patrols — so the
// per-member command stays the primitive and this is not a replacement for it. What this
// adds is the *operation*: when an operator moves the remnants of a below-strength patrol
// to one destination, that is one decision, and it should be one line on the timeline and
// one thing that either happened or did not.
//
// The alternative, which shipped first (task 077), was N requests from the browser. It
// records the same data but has the flaw task 073 rejected for collection: if the second
// of two calls fails, one member has moved and one has not, and the operator is told only
// that something went wrong.
//
// # Partial failure
//
// A failure part-way through returns the error and discards the moves so far, so no
// summary claims an operation that did not finish. Whatever was already published stays
// published — the log is the record and cannot be rewritten — but the caller learns the
// operation is incomplete rather than seeing a success that half happened.
//
// Strengths are computed **as the whole operation lands**, not per member: moving three
// members out one at a time would otherwise report the origin's strength as 3, then 2,
// then 1 for what is a single step to 0.
func (c commander) MoveMembers(ctx context.Context, actor Actor, year types.YearSlug, ids []types.MemberID, to types.TeamID) ([]Move, error) {
	if to == "" {
		return nil, ErrRecordNotFound
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Read every member first, so the operation is rejected before anything is published
	// if one of them cannot legally move. Half-validating and then discovering the problem
	// mid-loop is the failure this whole command exists to avoid.
	current := make([]*SpejderStatus, 0, len(ids))
	for _, id := range ids {
		m, err := c.q.GetByMemberID(ctx, year, id)
		if err != nil {
			return nil, err
		}
		if m.CurrentTeamID == to {
			return nil, ErrSameTeam
		}
		current = append(current, m)
	}

	// The origin loses all of them at once and the destination gains all of them at once,
	// so both strengths are the state after the operation rather than after each step.
	moving := make(map[types.MemberID]bool, len(current))
	for _, m := range current {
		moving[m.MemberID] = true
	}
	origin := current[0].CurrentTeamID
	fromStrength, err := c.strengthWithout(ctx, year, origin, moving)
	if err != nil {
		return nil, err
	}
	toStrength, err := c.strengthWithout(ctx, year, to, nil)
	if err != nil {
		return nil, err
	}
	// Everyone moved in is racing, and none of them was counted in the destination before.
	toStrength += len(current)

	moves := make([]Move, 0, len(current))
	for _, m := range current {
		body := &TeamMoved{MemberID: m.MemberID, FromTeamID: m.CurrentTeamID, ToTeamID: to, Actor: actor}
		if err := c.publish(actor, year, m.MemberID, "team.moved", body); err != nil {
			return nil, err
		}
		moves = append(moves, Move{
			MemberID:         m.MemberID,
			FromTeamID:       m.CurrentTeamID,
			ToTeamID:         to,
			FromTeamStrength: fromStrength,
			ToTeamStrength:   toStrength,
		})
	}
	return moves, nil
}

// strengthWithout counts a team's racing members, ignoring any that are about to leave it.
//
// Same reason as strengthAfter: the projection has not seen the events yet, so
// patrulje.activeMemberCount still holds the pre-operation number.
func (c commander) strengthWithout(ctx context.Context, year types.YearSlug, teamID types.TeamID, leaving map[types.MemberID]bool) (int, error) {
	if teamID == "" {
		return 0, nil
	}
	members, err := c.q.GetByTeam(ctx, Filter{YearSlug: year, TeamID: teamID})
	if err != nil {
		return 0, err
	}
	strength := 0
	for _, m := range members {
		if leaving[m.MemberID] {
			continue
		}
		if m.Status == types.MemberStatusRacing {
			strength++
		}
	}
	return strength, nil
}

// CollectTeam takes every remaining racing member of a patrol out of the race in one
// operation: the team leaves together.
//
// # One command, not N calls from the browser
//
// Three separate requests could half-succeed, and a team split across two states with
// nobody noticing is the worst available outcome — worse than the operator having to
// click three times, which is itself a way to forget two of them while on the phone.
// So the loop is here, on the server, where a partial failure is returned as a failure.
//
// # Members already out are skipped, not re-requested
//
// Collecting a patrol where one member is already in a car must not touch that member:
// they have left the route, and re-publishing a withdrawal request for them would put a
// second, false line in their history and move them backwards through the lifecycle.
//
// Returns one Change per member actually collected, in team order, for the caller to
// summarise as a single timeline entry. An empty result means there was nobody left to
// collect — not an error, just a no-op, which is what a double click produces.
func (c commander) CollectTeam(ctx context.Context, actor Actor, year types.YearSlug, team types.TeamID) ([]Change, error) {
	if team == "" {
		return nil, ErrRecordNotFound
	}
	members, err := c.q.GetByTeam(ctx, Filter{YearSlug: year, TeamID: team})
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, m := range members {
		if m.Status != types.MemberStatusRacing {
			continue
		}
		body := &WithdrawalRequested{MemberID: m.MemberID, TeamID: team, Actor: actor}
		if err := c.publish(actor, year, m.MemberID, "withdrawal.requested", body); err != nil {
			// Returned rather than swallowed: the caller must be able to tell the
			// operator that the collection did not complete, since some members are now
			// waiting and others are not. Whatever was published stays published — the
			// log is the record — and the changes so far are discarded so no summary
			// claims an operation that did not finish.
			return nil, err
		}
		changes = append(changes, Change{
			MemberID: m.MemberID,
			TeamID:   team,
			From:     types.MemberStatusRacing,
			To:       types.MemberStatusWaiting,
			// Zero by definition: every racing member of this team is being taken out
			// of the race, so none is left on the route. This is also what makes the
			// patrol discontinued, with no event of its own.
			TeamStrength: 0,
		})
	}
	return changes, nil
}

// AcceptIntoShelter records that HQ has received the member and is looking after them.
//
// Published by the shelter crew, because they are the receiving party and custody is always
// confirmed by the receiver — the driver letting go cannot say it happened.
//
// # What it accepts, and why that is broad
//
// Any *started* status: `transit` is the ordinary path, `waiting` is the scout who arrived in
// a car nobody logged, and `racing` is the scout whose withdrawal was never recorded at all.
// The last two look like violations of the lifecycle diagram and are in fact the reason this
// command exists: the child is standing in the doorway, and a command that refused to record
// it because no pickup event preceded it would leave the read model claiming they are still on
// the trail. The shelter's word is the better evidence, so it wins.
//
// The one refusal is a member who never started (`registered`, `seated`). They are at home, so
// an acceptance is a mistyped identity rather than an unrecorded arrival, and honouring it
// would invent a child in our care who is not on site.
//
// # Placement
//
// Carried on the event when the crew typed a tent while accepting, which is the ordinary
// gesture. A member who is **already** sheltered is a no-op here — and note what that means
// for the caller: a new placering for somebody already in the shelter is not this command's
// business, and the handler must use shelter.SetPlacement for it. Publishing a second
// acceptance to carry a tent would claim custody was taken twice.
func (c commander) AcceptIntoShelter(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, placement string) (*Change, error) {
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.Status == types.MemberStatusSheltered {
		return nil, nil
	}
	switch current.Status {
	case types.MemberStatusRegistered, types.MemberStatusSeated, types.MemberStatusNone:
		return nil, ErrNotStarted
	}
	change, err := c.change(ctx, year, current, types.MemberStatusSheltered)
	if err != nil {
		return nil, err
	}
	body := &ShelterAccepted{MemberID: id, TeamID: current.CurrentTeamID, Placement: placement, Actor: actor}
	if err := c.publish(actor, year, id, "shelter.accepted", body); err != nil {
		return nil, err
	}
	return change, nil
}

// AcceptPickup records that a car has taken members aboard (PRD 009).
//
// # Why it validates everything before publishing anything
//
// One stop collecting three scouts is one act. If the second member turns out to be somebody
// who never started — a mistyped or misclicked identity — then the dispatcher has the wrong
// task open, and publishing the first member's transit before finding that out would leave a
// scout recorded as sitting in a car nobody sent for them. So the batch is checked first and
// refused whole, naming the member at fault.
//
// # What it skips rather than refuses
//
// A member already in our care is skipped silently and contributes no Change. That is the
// idempotency the desk needs: pressing Hentet twice, or a driver ringing in about a scout the
// operator already recorded, must not put two custody changes on the log. It is also the
// resolution of the race `ErrAlreadyCollected` exists for — an acceptance beats a resume
// "because it reflects a member physically sitting in a car", and the way that precedence is
// expressed here is that this command never asks whether the member is `waiting`: a `racing`
// member whose resume landed first is still accepted, because the car has them.
func (c commander) AcceptPickup(ctx context.Context, actor Actor, year types.YearSlug, ids []types.MemberID, unit types.Slug, driver types.UserID) ([]Change, error) {
	currents := make([]*SpejderStatus, 0, len(ids))
	for _, id := range ids {
		current, err := c.q.GetByMemberID(ctx, year, id)
		if err != nil {
			return nil, err
		}
		switch current.Status {
		case types.MemberStatusRegistered, types.MemberStatusSeated, types.MemberStatusNone:
			// The same single refusal AcceptIntoShelter makes, for the same reason: those
			// members are at home, so an acceptance for one of them would invent a child in
			// our care who is not on site.
			return nil, fmt.Errorf("%w: %s", ErrNotStarted, id)
		}
		currents = append(currents, current)
	}

	changes := []Change{}
	for _, current := range currents {
		// Skip anybody a car (or HQ) already has, and *only* those. Deliberately not
		// `InOurCare()`, which is true for `waiting` as well — a waiting scout is precisely
		// who this command is for, and using that helper here silently accepted nobody. Found
		// by the test that expected two changes and got none.
		switch current.Status {
		case types.MemberStatusTransit, types.MemberStatusSheltered,
			types.MemberStatusReunited, types.MemberStatusReleased:
			continue
		}
		change, err := c.change(ctx, year, current, types.MemberStatusTransit)
		if err != nil {
			return nil, err
		}
		body := &PickupAccepted{
			MemberID:     current.MemberID,
			TeamID:       current.CurrentTeamID,
			SectionSlug:  unit,
			DriverUserID: driver,
			Actor:        actor,
		}
		if err := c.publish(actor, year, current.MemberID, "pickup.accepted", body); err != nil {
			// Returned rather than swallowed, and whatever was published stays published — the
			// log is the record. The caller must be able to tell the operator the pickup was
			// only half recorded, because the difference matters to whoever is standing at the
			// roadside.
			return nil, err
		}
		changes = append(changes, *change)
	}
	return changes, nil
}

// CompleteHandover records that somebody else has taken charge of the member, which is what
// takes them out of the in-our-care count.
//
// `to` is the caller's to state and must be `released` (a guardian came for them) or
// `reunited` (their own patrol finished and took them back). Guessing from the hour was the
// alternative and would have been wrong in both directions; the two are different facts about
// who has the child, which is the one thing this whole lifecycle exists to keep straight.
//
// `finished` is refused with its own error rather than falling through to "not an ending":
// somebody reaching for it is reaching for the wrong idea, not making a typo. A scout who was
// driven in has not walked the route, however close to the end they dropped out — see
// types.MemberStatusFinished.
func (c commander) CompleteHandover(ctx context.Context, actor Actor, year types.YearSlug, id types.MemberID, to types.MemberStatus) (*Change, error) {
	if to == types.MemberStatusFinished {
		return nil, ErrCannotFinish
	}
	if to != types.MemberStatusReleased && to != types.MemberStatusReunited {
		return nil, ErrNotAnEnding
	}
	current, err := c.q.GetByMemberID(ctx, year, id)
	if err != nil {
		return nil, err
	}
	if current.Status == to {
		return nil, nil
	}
	// Deliberately not requiring `sheltered` first. A guardian can collect a scout from the
	// roadside or out of the car, and a command that insisted on an arrival at HQ would refuse
	// to record a handover that actually happened — leaving a child counted as ours all night.
	// The timeline shows the route they took through the lifecycle, which is the honest record.
	change, err := c.change(ctx, year, current, to)
	if err != nil {
		return nil, err
	}
	body := &HandoverCompleted{MemberID: id, TeamID: current.CurrentTeamID, To: to, Actor: actor}
	if err := c.publish(actor, year, id, "handover.completed", body); err != nil {
		return nil, err
	}
	return change, nil
}

// change assembles the Change for a status transition, including the team's
// resulting strength.
func (c commander) change(ctx context.Context, year types.YearSlug, current *SpejderStatus, to types.MemberStatus) (*Change, error) {
	strength, err := c.strengthAfter(ctx, year, current.CurrentTeamID, current.MemberID, to)
	if err != nil {
		return nil, err
	}
	return &Change{
		MemberID:     current.MemberID,
		TeamID:       current.CurrentTeamID,
		From:         current.Status,
		To:           to,
		TeamStrength: strength,
	}, nil
}

// strengthAfter counts a team's racing members as they will be once the pending
// change is applied.
//
// It reads the team and substitutes the one member's new status in memory rather
// than trusting patrulje.activeMemberCount, which still holds the pre-event value
// at this point. MemberStatusNone is how "this member is leaving the team
// altogether" is expressed, since it is not racing and not a real status either.
//
// Counting in Go rather than in SQL keeps the arithmetic next to the reason for it
// and avoids a query that would have to express "…as if this row said something
// else".
func (c commander) strengthAfter(ctx context.Context, year types.YearSlug, teamID types.TeamID, member types.MemberID, to types.MemberStatus) (int, error) {
	if teamID == "" {
		return 0, nil
	}
	members, err := c.q.GetByTeam(ctx, Filter{YearSlug: year, TeamID: teamID})
	if err != nil {
		return 0, err
	}
	strength := 0
	seen := false
	for _, m := range members {
		status := m.Status
		if m.MemberID == member {
			seen = true
			status = to
		}
		if status == types.MemberStatusRacing {
			strength++
		}
	}
	// A member being moved *into* this team is not in it yet, so they were not in
	// the rows above.
	if !seen && to == types.MemberStatusRacing {
		strength++
	}
	return strength, nil
}

func (c commander) publish(actor Actor, year types.YearSlug, id types.MemberID, event string, body any) error {
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK.%s.spejder.%s.%s", year, id, event)))
	if err := msg.SetBody(body); err != nil {
		return err
	}
	if err := msg.SetMeta(&messages.Metadata{UserID: actor.UserID}); err != nil {
		return err
	}
	return c.p.Publish(msg)
}
