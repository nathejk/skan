package kort

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/stream"
	"github.com/nathejk/shared-go/types"
)

// --- fakes ---

type recordingPublisher struct {
	subjects []string
	bodies   []any
}

func (p *recordingPublisher) MessageFunc() stream.MessageFunc {
	return func(s stream.Subject) stream.MutableMessage {
		return &recordedMessage{p: p, subject: s}
	}
}

func (p *recordingPublisher) Publish(msg stream.Message) error {
	m := msg.(*recordedMessage)
	p.subjects = append(p.subjects, m.subject.Subject())
	p.bodies = append(p.bodies, m.body)
	return nil
}

type recordedMessage struct {
	p       *recordingPublisher
	subject stream.Subject
	body    any
}

func (m *recordedMessage) Subject() stream.Subject     { return m.subject }
func (m *recordedMessage) SetBody(b any) error         { m.body = b; return nil }
func (m *recordedMessage) SetMeta(any) error           { return nil }
func (m *recordedMessage) Body(any) error              { return nil }
func (m *recordedMessage) Meta(any) error              { return nil }
func (m *recordedMessage) RawBody() any                { return m.body }
func (m *recordedMessage) RawMeta() any                { return nil }
func (m *recordedMessage) Time() time.Time             { return time.Time{} }
func (m *recordedMessage) Sequence() uint64            { return 0 }
func (m *recordedMessage) SetSubject(s stream.Subject) { m.subject = s }
func (m *recordedMessage) SetTime(time.Time) error     { return nil }

// stubQueries serves one set and a sheet count, which is all the set commands read.
type stubQueries struct {
	set      *Kortsaet
	mapCount int
	err      error
}

func (s stubQueries) Maps(context.Context, types.YearSlug) ([]Kort, error) { return nil, nil }

func (s stubQueries) GetByID(context.Context, types.YearSlug, KortID) (*Kort, error) {
	return nil, ErrRecordNotFound
}

func (s stubQueries) Sets(context.Context, types.YearSlug) ([]Kortsaet, error) { return nil, nil }

func (s stubQueries) GetSetByID(context.Context, types.YearSlug, KortsaetID) (*Kortsaet, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.set == nil {
		return nil, ErrRecordNotFound
	}
	return s.set, nil
}

func (s stubQueries) CountMapsInSet(context.Context, types.YearSlug, KortsaetID) (int, error) {
	return s.mapCount, nil
}

func newCommander(q stubQueries) (commander, *recordingPublisher) {
	p := &recordingPublisher{}
	return commander{p: p, q: q}, p
}

func teamType(t types.TeamType) *types.TeamType { return &t }

// --- tests ---

func TestCreateSetPublishesOnTheSetsSubject(t *testing.T) {
	c, p := newCommander(stubQueries{})

	id, err := c.CreateSet(context.Background(), Actor{}, "2026", "  Patruljer  ", teamType(types.TeamTypePatrulje))
	if err != nil {
		t.Fatalf("CreateSet: %v", err)
	}
	if id == "" {
		t.Fatal("want a minted id")
	}
	if len(p.subjects) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(p.subjects), p.subjects)
	}
	if want := "NATHEJK.2026.kortsaet." + string(id) + ".created"; p.subjects[0] != want {
		t.Errorf("subject = %q, want %q", p.subjects[0], want)
	}
	body := p.bodies[0].(*SetCreated)
	// Trimmed, because a name with a trailing space is the same set and would sort oddly.
	if body.Name != "Patruljer" {
		t.Errorf("name = %q, want it trimmed", body.Name)
	}
}

// The crew set is unmarked, and that is the ordinary case rather than missing data.
func TestCreateSetAcceptsNoTeamType(t *testing.T) {
	c, p := newCommander(stubQueries{})

	if _, err := c.CreateSet(context.Background(), Actor{}, "2026", "Crew", nil); err != nil {
		t.Fatalf("CreateSet: %v", err)
	}
	if body := p.bodies[0].(*SetCreated); body.TeamType != nil {
		t.Errorf("teamType = %v, want nil", *body.TeamType)
	}
}

// `"teamType": ""` from a form that submits an empty select means the general set, not a set whose
// team type is the empty string — which would otherwise become a matchable value in the column.
func TestCreateSetNormalizesEmptyTeamTypeToNil(t *testing.T) {
	c, p := newCommander(stubQueries{})

	if _, err := c.CreateSet(context.Background(), Actor{}, "2026", "Crew", teamType("")); err != nil {
		t.Fatalf("CreateSet: %v", err)
	}
	if body := p.bodies[0].(*SetCreated); body.TeamType != nil {
		t.Errorf("teamType = %q, want nil", *body.TeamType)
	}
}

func TestCreateSetRefusesEmptyNameAndUnknownTeamType(t *testing.T) {
	c, p := newCommander(stubQueries{})

	if _, err := c.CreateSet(context.Background(), Actor{}, "2026", "   ", nil); !errors.Is(err, ErrEmptyName) {
		t.Errorf("blank name: err = %v, want ErrEmptyName", err)
	}
	if _, err := c.CreateSet(context.Background(), Actor{}, "2026", "X", teamType("spejder")); !errors.Is(err, ErrInvalidTeamType) {
		// "spejder" is the domain's word for a person, not a team type. Accepting it would
		// produce a set that looks marked on screen and is invisible to every consumer.
		t.Errorf("bogus team type: err = %v, want ErrInvalidTeamType", err)
	}
	if _, err := c.CreateSet(context.Background(), Actor{}, "2026", strings.Repeat("æ", MaxNameLength+1), nil); !errors.Is(err, ErrNameTooLong) {
		// Counted in runes: a Danish name must not be refused because of its æ's.
		t.Errorf("long name: err = %v, want ErrNameTooLong", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("a refused command must publish nothing, got %v", p.subjects)
	}
}

// An operator opening the two-field editor and closing it again must not make every other session
// refetch.
func TestUpdateSetIsDirtyChecked(t *testing.T) {
	current := &Kortsaet{KortsaetID: "kortsaet-1", Name: "Patruljer", TeamType: teamType(types.TeamTypePatrulje)}
	c, p := newCommander(stubQueries{set: current})

	err := c.UpdateSet(context.Background(), Actor{}, "2026", "kortsaet-1", "Patruljer", teamType(types.TeamTypePatrulje))
	if err != nil {
		t.Fatalf("UpdateSet: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Fatalf("an unchanged update must publish nothing, got %v", p.subjects)
	}
}

// Un-marking the spejder set is a real edit, and the one a patch-shaped event could not express:
// with pointer-means-absent semantics, "clear it" and "leave it" are the same nil.
func TestUpdateSetCanClearTheTeamType(t *testing.T) {
	current := &Kortsaet{KortsaetID: "kortsaet-1", Name: "Patruljer", TeamType: teamType(types.TeamTypePatrulje)}
	c, p := newCommander(stubQueries{set: current})

	if err := c.UpdateSet(context.Background(), Actor{}, "2026", "kortsaet-1", "Patruljer", nil); err != nil {
		t.Fatalf("UpdateSet: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("clearing the team type must publish: got %v", p.subjects)
	}
	body := p.bodies[0].(*SetUpdated)
	if body.TeamType != nil {
		t.Errorf("teamType = %v, want nil", *body.TeamType)
	}
	// The name rides along whether or not it changed, because this event is the whole record.
	if body.Name != "Patruljer" {
		t.Errorf("name = %q, want the whole record carried", body.Name)
	}
}

func TestUpdateSetRefusesUnknownSet(t *testing.T) {
	c, p := newCommander(stubQueries{})

	err := c.UpdateSet(context.Background(), Actor{}, "2026", "kortsaet-nope", "X", nil)
	if !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("err = %v, want ErrRecordNotFound", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("want nothing published, got %v", p.subjects)
	}
}

// Refused rather than cascaded: a mis-click in a list must not cost a season of map definitions,
// and a projection that replays the stream offers no undo.
func TestDeleteSetRefusedWhileItHoldsMaps(t *testing.T) {
	current := &Kortsaet{KortsaetID: "kortsaet-1", Name: "Patruljer"}
	c, p := newCommander(stubQueries{set: current, mapCount: 4})

	err := c.DeleteSet(context.Background(), Actor{}, "2026", "kortsaet-1")
	if !errors.Is(err, ErrSetNotEmpty) {
		t.Fatalf("err = %v, want ErrSetNotEmpty", err)
	}
	// The count travels with the error so the operator can be told what is in the way.
	if !strings.Contains(err.Error(), "4") {
		t.Errorf("err = %q, want it to name the number of maps", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("want nothing published, got %v", p.subjects)
	}
}

func TestDeleteSetPublishesWhenEmpty(t *testing.T) {
	current := &Kortsaet{KortsaetID: "kortsaet-1", Name: "Gamle kort"}
	c, p := newCommander(stubQueries{set: current, mapCount: 0})

	if err := c.DeleteSet(context.Background(), Actor{}, "2026", "kortsaet-1"); err != nil {
		t.Fatalf("DeleteSet: %v", err)
	}
	if want := "NATHEJK.2026.kortsaet.kortsaet-1.deleted"; p.subjects[0] != want {
		t.Errorf("subject = %q, want %q", p.subjects[0], want)
	}
}

// One event for the whole collection, on an id-less subject — which live.Signal renders as
// "something of this type changed", the right meaning when every row's order may have moved.
func TestSortSetsPublishesOneCollectionEvent(t *testing.T) {
	c, p := newCommander(stubQueries{})

	ids := []KortsaetID{"kortsaet-2", "kortsaet-1"}
	if err := c.SortSets(context.Background(), Actor{}, "2026", ids); err != nil {
		t.Fatalf("SortSets: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(p.subjects), p.subjects)
	}
	if want := "NATHEJK.2026.kortsaet.sorted"; p.subjects[0] != want {
		t.Errorf("subject = %q, want %q", p.subjects[0], want)
	}
	if body := p.bodies[0].(*SetsSorted); len(body.KortsaetIDs) != 2 || body.KortsaetIDs[0] != "kortsaet-2" {
		t.Errorf("body = %v, want the ids in the given order", body.KortsaetIDs)
	}
}

func TestSortSetsWithNoIdsPublishesNothing(t *testing.T) {
	c, p := newCommander(stubQueries{})

	if err := c.SortSets(context.Background(), Actor{}, "2026", nil); err != nil {
		t.Fatalf("SortSets: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("want nothing published, got %v", p.subjects)
	}
}

// The nil-means-general rule is written once, on the type, so no caller reimplements it as
// `*s.TeamType == t` and panics on the crew set.
func TestForTeamType(t *testing.T) {
	patrol := Kortsaet{TeamType: teamType(types.TeamTypePatrulje)}
	crew := Kortsaet{}

	if !patrol.ForTeamType(types.TeamTypePatrulje) {
		t.Error("a marked set must match its team type")
	}
	if patrol.ForTeamType(types.TeamTypeKlan) {
		t.Error("a marked set must not match another team type")
	}
	// Klaner draw from the unmarked crew set, so this returning false is exactly the trap
	// documented on the field: a caller must fall back to the unmarked set, not conclude there
	// are no maps.
	if crew.ForTeamType(types.TeamTypeKlan) {
		t.Error("an unmarked set is not specifically for any team type")
	}
}
