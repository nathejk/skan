package kort

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"
)

func TestNestGroupsSheetsUnderTheirSets(t *testing.T) {
	sets := []Kortsaet{
		{KortsaetID: "s-patrulje", Name: "Patruljer", TeamType: teamType(types.TeamTypePatrulje)},
		{KortsaetID: "s-crew", Name: "Crew"},
	}
	maps := []Kort{
		{KortID: "k-1", KortsaetID: "s-patrulje"},
		{KortID: "k-2", KortsaetID: "s-patrulje"},
		{KortID: "k-3", KortsaetID: "s-crew"},
	}

	nested, orphans := Nest(sets, maps)

	if len(nested) != 2 {
		t.Fatalf("got %d sets, want 2", len(nested))
	}
	if len(nested[0].Maps) != 2 || len(nested[1].Maps) != 1 {
		t.Fatalf("sheets landed in the wrong sets: %d and %d", len(nested[0].Maps), len(nested[1].Maps))
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %v, want none", orphans)
	}
}

// A sheet whose set is unknown is possible during replay — events arrive in stream order, so a
// sheet may precede its set — and after a bad edit. Dropping it would make a map invisible in the
// one screen that exists to find such mistakes.
func TestNestReturnsSheetsWithAnUnknownSet(t *testing.T) {
	sets := []Kortsaet{{KortsaetID: "s-1", Name: "Patruljer"}}
	maps := []Kort{
		{KortID: "k-1", KortsaetID: "s-1"},
		{KortID: "k-lost", KortsaetID: "s-deleted"},
	}

	nested, orphans := Nest(sets, maps)

	if len(nested[0].Maps) != 1 {
		t.Errorf("known set got %d sheets, want 1", len(nested[0].Maps))
	}
	if len(orphans) != 1 || orphans[0].KortID != "k-lost" {
		t.Fatalf("orphans = %v, want the sheet whose set is gone", orphans)
	}
}

// A set an operator has just created has no sheets, and that must serialize as `[]`.
func TestNestGivesEmptySetsAnEmptyArray(t *testing.T) {
	nested, orphans := Nest([]Kortsaet{{KortsaetID: "s-1"}}, nil)

	if nested[0].Maps == nil {
		t.Error("want an empty slice for a set with no sheets, not nil")
	}
	if orphans == nil {
		t.Error("want an empty orphan slice, not nil")
	}

	// The JSON is the part that matters: the SPA parses this, and a `kort` key that comes and
	// goes would make every client handle absence as well as emptiness.
	encoded, err := json.Marshal(nested[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"kort":[]`) {
		t.Errorf("want an empty kort array in the payload: %s", encoded)
	}
}

// teamType must survive nesting: it is how a consumer finds the patrol sheets, and the whole
// reason sets are an entity rather than a string on each sheet.
func TestNestPreservesTheTeamTypeMarking(t *testing.T) {
	sets := []Kortsaet{
		{KortsaetID: "s-1", Name: "Patruljer", TeamType: teamType(types.TeamTypePatrulje)},
		{KortsaetID: "s-2", Name: "Crew"},
	}

	nested, _ := Nest(sets, []Kort{{KortID: "k-1", KortsaetID: "s-1"}})

	if !nested[0].ForTeamType(types.TeamTypePatrulje) {
		t.Error("want the patrol set still marked after nesting")
	}
	if nested[1].TeamType != nil {
		t.Error("want the crew set left unmarked")
	}
}

// A sheet's own arrays are never null either — an overview map with no checkpoints and a skitse
// with no extent are both ordinary, and both must read as `[]`.
func TestSheetArraysSerializeAsEmptyNotNull(t *testing.T) {
	encoded, err := json.Marshal(Kort{KortID: "k-1", CheckpointIDs: []types.CheckpointID{}, Extents: []Extent{}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"checkpointIds":[]`, `"extents":[]`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("want %s in %s", want, encoded)
		}
	}
}

// An unmarked set serializes teamType as null rather than omitting it, so a client can tell "the
// general set" from "a field this API version does not send".
func TestUnmarkedSetSerializesTeamTypeAsNull(t *testing.T) {
	encoded, err := json.Marshal(Kortsaet{KortsaetID: "s-1", Name: "Crew", Maps: []Kort{}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"teamType":null`) {
		t.Errorf("want an explicit null teamType: %s", encoded)
	}
}
