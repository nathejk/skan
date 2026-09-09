package kort

import (
	"context"
	"errors"
	"testing"

	"github.com/nathejk/shared-go/types"
)

// mapStub serves one sheet, which is all the map commands read for their dirty-check.
type mapStub struct {
	sheet *Kort
}

func (s mapStub) Maps(context.Context, types.YearSlug) ([]Kort, error) { return nil, nil }

func (s mapStub) GetByID(context.Context, types.YearSlug, KortID) (*Kort, error) {
	if s.sheet == nil {
		return nil, ErrRecordNotFound
	}
	return s.sheet, nil
}

func (s mapStub) Sets(context.Context, types.YearSlug) ([]Kortsaet, error) { return nil, nil }

func (s mapStub) GetSetByID(context.Context, types.YearSlug, KortsaetID) (*Kortsaet, error) {
	return nil, ErrRecordNotFound
}

func (s mapStub) CountMapsInSet(context.Context, types.YearSlug, KortsaetID) (int, error) {
	return 0, nil
}

func newMapCommander(sheet *Kort) (commander, *recordingPublisher) {
	p := &recordingPublisher{}
	return commander{p: p, q: mapStub{sheet: sheet}}, p
}

func pos(lat, lng float64) types.Position {
	return types.Position{Latitude: types.Latitude(lat), Longitude: types.Longitude(lng)}
}

// --- create ---

func TestCreateCarriesOnlySetAndName(t *testing.T) {
	c, p := newMapCommander(nil)

	id, err := c.Create(context.Background(), Actor{}, "2026", "kortsaet-1", "  Kort 1  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if want := "NATHEJK.2026.kort." + string(id) + ".created"; p.subjects[0] != want {
		t.Errorf("subject = %q, want %q", p.subjects[0], want)
	}
	body := p.bodies[0].(*Created)
	if body.Name != "Kort 1" || body.KortsaetID != "kortsaet-1" {
		t.Errorf("body = %+v, want a trimmed name and the set", body)
	}
}

// A sheet may be created before its set exists, because replay materialises events in stream order
// and a command that refused an unknown set would enforce a constraint the projection does not hold.
func TestCreateDoesNotRequireTheSetToExist(t *testing.T) {
	c, p := newMapCommander(nil)

	if _, err := c.Create(context.Background(), Actor{}, "2026", "kortsaet-unknown", "Kort 1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("want the create published, got %v", p.subjects)
	}
}

// --- update ---

func TestUpdatePublishesOnlyChangedFields(t *testing.T) {
	current := &Kort{KortID: "kort-1", KortsaetID: "kortsaet-1", Name: "Kort 1", Format: FormatA4, Note: "n"}
	c, p := newMapCommander(current)

	newName := "Kort 1 — Start til Post 2"
	sameFormat := FormatA4
	err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{
		Name:   &newName,
		Format: &sameFormat,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(p.subjects), p.subjects)
	}
	body := p.bodies[0].(*Updated)
	if body.Name == nil || *body.Name != newName {
		t.Errorf("want the new name carried, got %+v", body)
	}
	// The unchanged format must not ride along: an event carrying it would make a replay look
	// like an edit and would invalidate clients that only watch the format.
	if body.Format != nil {
		t.Errorf("want format omitted when unchanged, got %q", *body.Format)
	}
}

func TestUpdateWithNoRealChangePublishesNothing(t *testing.T) {
	current := &Kort{KortID: "kort-1", Name: "Kort 1", Format: FormatA4}
	c, p := newMapCommander(current)

	sameName := "Kort 1"
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Name: &sameName}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Fatalf("an unchanged update must publish nothing, got %v", p.subjects)
	}
}

func TestUpdateRefusesUnknownFormat(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1"})

	bogus := Format("A4 dobbeltsidet")
	err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Format: &bogus})
	if !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("err = %v, want ErrInvalidFormat", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("want nothing published, got %v", p.subjects)
	}
}

// --- handout ---

func TestUpdateSetsHandoutCheckgroup(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", Name: "Kort 1"})

	group := types.CheckgroupID("cg-3")
	err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{HandoutCheckgroupID: &group})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	body := p.bodies[0].(*Updated)
	if body.HandoutCheckgroupID == nil || *body.HandoutCheckgroupID != "cg-3" {
		t.Errorf("body = %+v, want the handout checkgroup carried", body)
	}
}

// Empty means "at the scan of this sheet's QR code", which is a real choice an operator can make
// after picking a checkgroup — so it must publish, not be treated as "no value given".
func TestUpdateCanSwitchHandoutBackToQRScan(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", HandoutCheckgroupID: "cg-3"})

	qr := types.CheckgroupID("")
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{HandoutCheckgroupID: &qr}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.bodies) != 1 {
		t.Fatalf("want the change published, got %v", p.subjects)
	}
	body := p.bodies[0].(*Updated)
	if body.HandoutCheckgroupID == nil || *body.HandoutCheckgroupID != "" {
		t.Errorf("body = %+v, want the handout cleared to the QR rule", body)
	}
}

func TestUpdateWithUnchangedHandoutPublishesNothing(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", HandoutCheckgroupID: "cg-3"})

	same := types.CheckgroupID("cg-3")
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{HandoutCheckgroupID: &same}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Fatalf("an unchanged handout must publish nothing, got %v", p.subjects)
	}
}

// --- extents ---

// Whichever two corners were clicked, the stored pair is a true north-west/south-east one, so no
// reader has to guess which way round the rectangle was drawn.
func TestUpdateNormalizesCornersWhicheverWayRound(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1"})

	// Clicked bottom-right first, then top-left.
	backwards := []Extent{{NorthWest: pos(55.8, 9.6), SouthEast: pos(56.1, 9.1)}}
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Extents: &backwards}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := (*p.bodies[0].(*Updated).Extents)[0]
	if got.NorthWest.Latitude != 56.1 || got.NorthWest.Longitude != 9.1 {
		t.Errorf("north-west = %+v, want the northern, western corner", got.NorthWest)
	}
	if got.SouthEast.Latitude != 55.8 || got.SouthEast.Longitude != 9.6 {
		t.Errorf("south-east = %+v, want the southern, eastern corner", got.SouthEast)
	}
}

// A double-sided sheet is one map with two areas; a third would be a bug in the caller.
func TestUpdateRefusesMoreThanTwoExtents(t *testing.T) {
	c, _ := newMapCommander(&Kort{KortID: "kort-1"})

	three := []Extent{
		{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.0, 9.2)},
		{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.0, 9.2)},
		{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.0, 9.2)},
	}
	err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Extents: &three})
	if !errors.Is(err, ErrTooManyExtents) {
		t.Errorf("err = %v, want ErrTooManyExtents", err)
	}
}

// Two clicks on the same spot draw nothing, and a saved invisible extent looks to an operator like
// a save that failed.
func TestUpdateRefusesDegenerateExtent(t *testing.T) {
	c, _ := newMapCommander(&Kort{KortID: "kort-1"})

	flat := []Extent{{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.1, 9.4)}}
	err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Extents: &flat})
	if !errors.Is(err, ErrDegenerateExtent) {
		t.Errorf("err = %v, want ErrDegenerateExtent", err)
	}
}

// Deciding a sheet is really a skitse means clearing its extents — a real edit, and the reason
// Extents is a pointer to a slice.
func TestUpdateCanClearExtents(t *testing.T) {
	current := &Kort{KortID: "kort-1", Extents: []Extent{{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.0, 9.2)}}}
	c, p := newMapCommander(current)

	none := []Extent{}
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Extents: &none}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("clearing extents must publish, got %v", p.subjects)
	}
	if got := *p.bodies[0].(*Updated).Extents; len(got) != 0 {
		t.Errorf("extents = %v, want empty", got)
	}
}

func TestUpdateWithUnchangedExtentsPublishesNothing(t *testing.T) {
	extent := Extent{NorthWest: pos(56.1, 9.1), SouthEast: pos(56.0, 9.2)}
	c, p := newMapCommander(&Kort{KortID: "kort-1", Extents: []Extent{extent}})

	// Sent back with the corners swapped: after normalisation this is the same rectangle, so it
	// must not count as an edit.
	swapped := []Extent{{NorthWest: pos(56.0, 9.2), SouthEast: pos(56.1, 9.1)}}
	if err := c.Update(context.Background(), Actor{}, "2026", "kort-1", UpdateRequest{Extents: &swapped}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Fatalf("want nothing published for the same rectangle, got %v", p.subjects)
	}
}

// --- checkpoints ---

func TestSetCheckpointsReplacesAndDedupes(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1"}})

	err := c.SetCheckpoints(context.Background(), Actor{}, "2026", "kort-1",
		[]types.CheckpointID{"cp-1", "cp-2", "cp-2", "", "cp-3"})
	if err != nil {
		t.Fatalf("SetCheckpoints: %v", err)
	}
	got := *p.bodies[0].(*Updated).CheckpointIDs
	want := []types.CheckpointID{"cp-1", "cp-2", "cp-3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSetCheckpointsUnchangedPublishesNothing(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1", "cp-2"}})

	err := c.SetCheckpoints(context.Background(), Actor{}, "2026", "kort-1",
		[]types.CheckpointID{"cp-1", "cp-2"})
	if err != nil {
		t.Fatalf("SetCheckpoints: %v", err)
	}
	if len(p.subjects) != 0 {
		t.Fatalf("re-saving the same selection must publish nothing, got %v", p.subjects)
	}
}

// Emptying a sheet's checkpoints is a legitimate edit — an overview map for drivers has none.
func TestSetCheckpointsCanEmptyASheet(t *testing.T) {
	c, p := newMapCommander(&Kort{KortID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1"}})

	if err := c.SetCheckpoints(context.Background(), Actor{}, "2026", "kort-1", nil); err != nil {
		t.Fatalf("SetCheckpoints: %v", err)
	}
	if len(p.subjects) != 1 {
		t.Fatalf("want an event, got %v", p.subjects)
	}
	got := p.bodies[0].(*Updated).CheckpointIDs
	if got == nil {
		t.Fatal("want a present-but-empty list, not nil — nil means \"not mentioned\"")
	}
	if len(*got) != 0 {
		t.Errorf("got %v, want empty", *got)
	}
}

// --- delete and sort ---

func TestDeleteRequiresTheSheetToExist(t *testing.T) {
	c, p := newMapCommander(nil)

	if err := c.Delete(context.Background(), Actor{}, "2026", "kort-nope"); !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("err = %v, want ErrRecordNotFound", err)
	}
	if len(p.subjects) != 0 {
		t.Errorf("want nothing published, got %v", p.subjects)
	}
}

func TestSortMapsPublishesOneCollectionEvent(t *testing.T) {
	c, p := newMapCommander(nil)

	err := c.SortMaps(context.Background(), Actor{}, "2026", []KortID{"kort-3", "kort-1"})
	if err != nil {
		t.Fatalf("SortMaps: %v", err)
	}
	if want := "NATHEJK.2026.kort.sorted"; p.subjects[0] != want {
		t.Errorf("subject = %q, want %q", p.subjects[0], want)
	}
	if body := p.bodies[0].(*Sorted); body.KortIDs[0] != "kort-3" {
		t.Errorf("body = %v, want the given order", body.KortIDs)
	}
}
