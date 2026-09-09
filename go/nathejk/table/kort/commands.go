package kort

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// MaxNameLength caps a set's or a sheet's name.
//
// 199 characters, matching the column. A name is a label an operator reads off a printed sheet
// ("Kort 3 — Post 4 til Post 6"), so the cap exists to protect the column rather than to express
// a rule.
const MaxNameLength = 199

var (
	// ErrEmptyName is returned rather than creating an unnamed set or sheet. The name is how an
	// operator tells one of fifteen sheets from another, and a blank row in that list is worse
	// than a refused save.
	ErrEmptyName = errors.New("name is required")

	// ErrNameTooLong is returned for a name over MaxNameLength.
	ErrNameTooLong = errors.New("name is too long")

	// ErrInvalidTeamType is returned for a team type outside shared-go's vocabulary.
	//
	// Checked rather than stored blindly, because the value's whole job is to be matched on by
	// another service reading the stream: a typo would produce a set that looks marked on screen and
	// is invisible to every consumer.
	ErrInvalidTeamType = errors.New("invalid team type")

	// ErrInvalidFormat is returned for a format outside a4/a3/skitse/andet.
	ErrInvalidFormat = errors.New("invalid format")

	// ErrSetNotEmpty is returned when deleting a set that still holds sheets.
	//
	// Its own error, and not a generic refusal, because the operator needs to be told what to do
	// about it: move or delete the sheets first. Cascading instead would mean a mis-click on a
	// set costs a season of map definitions, and an event stream offers no undo.
	ErrSetNotEmpty = errors.New("set still holds maps")
)

// SetCommands is the write surface for map sets.
//
// The Actor is passed in by the caller rather than read from the request context, matching every
// other table package: it keeps this package free of nathejk.dk/internal/requestctx, which is
// also one of the things that has to stay true for the eventual lift to shared-go.
type SetCommands interface {
	CreateSet(ctx context.Context, actor Actor, year types.YearSlug, name string, teamType *types.TeamType) (KortsaetID, error)
	UpdateSet(ctx context.Context, actor Actor, year types.YearSlug, id KortsaetID, name string, teamType *types.TeamType) error
	DeleteSet(ctx context.Context, actor Actor, year types.YearSlug, id KortsaetID) error
	SortSets(ctx context.Context, actor Actor, year types.YearSlug, ids []KortsaetID) error
}

// Actor is who made the change.
//
// Defined here rather than borrowed from another table package so this one keeps no dependency it
// does not need. Empty in practice until HQ has login (PRD 001 §6); recorded anyway, so nothing
// changes here the day identity arrives.
type Actor struct {
	UserID types.UserID `json:"userId,omitempty"`
	Name   string       `json:"name,omitempty"`
}

type commander struct {
	p stream.Publisher
	q Queries
}

// CreateSet adds a set of sheets and returns its id.
//
// The id is minted here rather than accepted from the client, as every other id in this codebase
// is: a client cannot collide with an id it has not seen.
//
// No duplicate-name check, and none wanted: two sets called "Patruljer" is a mistake an operator
// can see and fix in the list, whereas a refusal on a name would be maddening in the year they
// genuinely want "Patruljer nord" and "Patruljer syd" and type the first one twice on the way
// there. The thing that must not be ambiguous is the team type marking, and that is deliberately
// not unique either — see Kortsaet.TeamType.
func (c commander) CreateSet(ctx context.Context, actor Actor, year types.YearSlug, name string, teamType *types.TeamType) (KortsaetID, error) {
	name = trimName(name)
	if err := validateName(name); err != nil {
		return "", err
	}
	if err := validateTeamType(teamType); err != nil {
		return "", err
	}
	id := NewKortsaetID()
	body := &SetCreated{KortsaetID: id, Name: name, TeamType: normalizeTeamType(teamType)}
	if err := c.publish(actor, year, "kortsaet", string(id), "created", body); err != nil {
		return "", err
	}
	return id, nil
}

// UpdateSet changes a set's name and team type.
//
// Both are carried whether or not both changed, because SetUpdated is a whole-record event — see
// its doc comment for why a patch cannot express "clear the team type".
//
// Dirty-checked: an unchanged submit publishes nothing, so it emits no live signal and does not
// make every other open session refetch. That matters here more than it looks, because the set
// editor is a two-field form an operator will open, read and close without touching.
func (c commander) UpdateSet(ctx context.Context, actor Actor, year types.YearSlug, id KortsaetID, name string, teamType *types.TeamType) error {
	name = trimName(name)
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateTeamType(teamType); err != nil {
		return err
	}
	current, err := c.q.GetSetByID(ctx, year, id)
	if err != nil {
		return err
	}
	teamType = normalizeTeamType(teamType)
	if current.Name == name && sameTeamType(current.TeamType, teamType) {
		return nil
	}
	body := &SetUpdated{KortsaetID: id, Name: name, TeamType: teamType}
	return c.publish(actor, year, "kortsaet", string(id), "updated", body)
}

// DeleteSet removes an empty set.
//
// Refused while the set still holds sheets. The alternative — cascade — would delete a season's
// map definitions from one mis-click in a list, and there is no undo: the projection rebuilds
// from the stream, and the stream would contain the deletion.
func (c commander) DeleteSet(ctx context.Context, actor Actor, year types.YearSlug, id KortsaetID) error {
	if id == "" {
		return ErrRecordNotFound
	}
	if _, err := c.q.GetSetByID(ctx, year, id); err != nil {
		return err
	}
	count, err := c.q.CountMapsInSet(ctx, year, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: %d", ErrSetNotEmpty, count)
	}
	return c.publish(actor, year, "kortsaet", string(id), "deleted", &SetDeleted{KortsaetID: id})
}

// SortSets records a new order for the year's sets.
//
// One event for the whole collection, on NATHEJK.{year}.kortsaet.sorted, following
// checkgroups.sorted. No dirty-check against the current order: comparing two lists to suppress a
// no-op drag would cost a read on every reorder to save an event nobody minds, and a drag that
// ends where it started is rare enough not to matter.
func (c commander) SortSets(ctx context.Context, actor Actor, year types.YearSlug, ids []KortsaetID) error {
	if len(ids) == 0 {
		return nil
	}
	return c.publishCollection(actor, year, "kortsaet", "sorted", &SetsSorted{KortsaetIDs: ids})
}

// trimName trims a name, so that a trailing space cannot make two identical labels sort apart.
func trimName(name string) string { return strings.TrimSpace(name) }

func validateName(name string) error {
	if name == "" {
		return ErrEmptyName
	}
	if len([]rune(name)) > MaxNameLength {
		return ErrNameTooLong
	}
	return nil
}

// validateTeamType accepts nil — the ordinary crew set — and otherwise requires a team type
// shared-go knows.
//
// Checked against types.TeamTypes rather than a switch of our own, so the vocabulary has one
// definition: a team type added there becomes markable here without an edit, and one removed stops
// being accepted. patrulje and klan are the two that mean anything for maps, but a set marked for
// gøglere is not this package's business to forbid.
func validateTeamType(t *types.TeamType) error {
	if t == nil || *t == "" {
		return nil
	}
	if !types.TeamTypes.Exists(*t) {
		return ErrInvalidTeamType
	}
	return nil
}

// normalizeTeamType collapses a pointer to an empty string into nil.
//
// A JSON body that sends `"teamType": ""` means the same thing as omitting it — the general set —
// and letting the empty string through would put a matchable non-NULL value in the column.
func normalizeTeamType(t *types.TeamType) *types.TeamType {
	if t == nil || *t == "" {
		return nil
	}
	return t
}

func sameTeamType(a, b *types.TeamType) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// publish sends an entity event on NATHEJK.{year}.{entity}.{id}.{event}.
func (c commander) publish(actor Actor, year types.YearSlug, entity, id, event string, body any) error {
	return c.send(actor, fmt.Sprintf("NATHEJK.%s.%s.%s.%s", year, entity, id, event), body)
}

// publishCollection sends a collection event on NATHEJK.{year}.{entity}.{event}.
//
// No id, following checkgroups.sorted: a reorder is about the collection, and live.Signal renders
// an id-less subject as "something of this type changed" — which is exactly what a client needs
// to know when every row's order may have moved.
func (c commander) publishCollection(actor Actor, year types.YearSlug, entity, event string, body any) error {
	return c.send(actor, fmt.Sprintf("NATHEJK.%s.%s.%s", year, entity, event), body)
}

func (c commander) send(actor Actor, subj string, body any) error {
	msg := c.p.MessageFunc()(subject.FromStr(subj))
	if err := msg.SetBody(body); err != nil {
		return err
	}
	if err := msg.SetMeta(&messages.Metadata{UserID: actor.UserID}); err != nil {
		return err
	}
	return c.p.Publish(msg)
}
