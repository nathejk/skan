package kort

import (
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"
)

// The cascade is the price of storing checkpoints as a JSON array rather than a join table, so it
// is the part of this package most worth pinning down (PRD 010 §8).

func TestCheckpointDeletedPrunesTheIdFromEverySheet(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.checkpoint.cp-7.deleted", 9, nil))

	if len(w.stmts) != 1 {
		t.Fatalf("got %d statements, want 1: %v", len(w.stmts), w.stmts)
	}
	stmt := w.stmts[0]
	for _, want := range []string{"UPDATE", "`kort`", "JSON_REMOVE", "JSON_SEARCH", "'cp-7'"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %q: %s", want, stmt)
		}
	}
	// Guarded, so a deleted checkpoint only writes the sheets that actually showed it. Without
	// this every checkpoint deletion would bump every row in the table.
	if !strings.Contains(stmt, "IS NOT NULL") {
		t.Errorf("want a guard so untouched sheets are not written: %s", stmt)
	}
	if !strings.Contains(stmt, "'2026'") {
		t.Errorf("want the prune scoped to the year: %s", stmt)
	}
}

// The id comes from the subject, which is what makes this independent of whether the checkpoint
// projection has already deleted the row: the two consumers see the same event.
func TestCheckpointDeletedTakesTheIdFromTheSubject(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.checkpoint.cp-real.deleted", 9,
		map[string]string{"checkpointId": "cp-imposter"}))

	if !strings.Contains(w.stmts[0], "'cp-real'") || strings.Contains(w.stmts[0], "imposter") {
		t.Fatalf("want the subject's id: %s", w.stmts[0])
	}
}

// A checkgroup delete is deliberately *not* subscribed to. It names only the group, and its
// members cannot be cascaded out of a JSON array safely — see consumer.pruneCheckpoint. This test
// exists so that adding the subscription without also solving that problem fails loudly rather
// than shipping a cascade that corrupts rows.
func TestCheckgroupDeletedIsNotConsumed(t *testing.T) {
	c := &consumer{}
	for _, s := range c.Consumes() {
		if strings.Contains(s.Subject(), "checkgroup") {
			t.Fatalf("kort must not consume %q: a checkgroup's members cannot be pruned from the "+
				"JSON array safely — the stale ids are filtered on read instead", s.Subject())
		}
	}
}

// Stale ids never reach a client, which is where the checkgroup case is actually handled.
func TestFilterKnownDropsUnresolvableIds(t *testing.T) {
	known := map[types.CheckpointID]bool{"cp-1": true, "cp-3": true}

	got := filterKnown([]types.CheckpointID{"cp-1", "cp-2", "cp-3"}, known)

	if len(got) != 2 || got[0] != "cp-1" || got[1] != "cp-3" {
		t.Fatalf("got %v, want the surviving ids in order", got)
	}
}

// `[]`, never `null`: clients parse this, and a sheet whose whole checkgroup was deleted is
// an ordinary sheet with nothing on it.
func TestFilterKnownReturnsEmptyNotNil(t *testing.T) {
	got := filterKnown([]types.CheckpointID{"cp-9"}, map[types.CheckpointID]bool{})

	if got == nil {
		t.Fatal("want an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want nothing kept", got)
	}
}
