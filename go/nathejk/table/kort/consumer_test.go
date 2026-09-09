package kort

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/types"
)

// --- fakes ---

// recordingWriter captures the statements the projection produces. Testing the emitted SQL rather
// than a database is what the rest of the repo does, and here it pins the two things most worth
// pinning: that a partial update touches only the columns the event mentioned, and that an empty
// list is written as `[]` rather than `null`.
type recordingWriter struct {
	stmts []string
}

func (w *recordingWriter) Consume(stmt string) error {
	w.stmts = append(w.stmts, stmt)
	return nil
}

type fakeMessage struct {
	subject stream.Subject
	body    any
	seq     uint64
	at      time.Time
}

func (m fakeMessage) Subject() stream.Subject { return m.subject }
func (m fakeMessage) Time() time.Time         { return m.at }
func (m fakeMessage) Sequence() uint64        { return m.seq }
func (m fakeMessage) Body(dst any) error {
	b, err := json.Marshal(m.body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
func (m fakeMessage) Meta(any) error { return nil }
func (m fakeMessage) RawBody() any   { return m.body }
func (m fakeMessage) RawMeta() any   { return nil }

func msg(subj string, seq uint64, body any) stream.Message {
	return fakeMessage{
		subject: subject.FromStr(subj),
		body:    body,
		seq:     seq,
		at:      time.Date(2026, 9, 3, 20, 30, 0, 0, time.UTC),
	}
}

func handle(t *testing.T, c *consumer, m stream.Message) {
	t.Helper()
	if err := c.HandleMessage(m); err != nil {
		t.Fatalf("HandleMessage(%s): %v", m.Subject().Subject(), err)
	}
}

func ptr[T any](v T) *T { return &v }

// --- tests ---

func TestCreatedUpsertsWithEmptyJSONArrays(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.created", 1,
		Created{KortID: "kort-1", KortsaetID: "kortsaet-1", Name: "Kort 1"}))

	if len(w.stmts) != 1 {
		t.Fatalf("got %d statements, want 1: %v", len(w.stmts), w.stmts)
	}
	stmt := w.stmts[0]
	for _, want := range []string{"INSERT", "`kort`", "'[]'", "ON DUPLICATE KEY UPDATE"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %q: %s", want, stmt)
		}
	}
	// The row must be well-formed JSON from the moment it exists, so a reader never meets an
	// empty string where an array belongs.
	if strings.Count(stmt, "'[]'") != 2 {
		t.Errorf("want both checkpointIds and extents seeded with '[]': %s", stmt)
	}
}

// A replayed create must not undo later edits. This is the whole reason the upsert names only
// kortsaetId and name in its update list, and it breaks the moment somebody "helpfully" adds the
// JSON columns to it.
func TestCreatedReplayDoesNotResetCheckpointsOrExtents(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.created", 1,
		Created{KortID: "kort-1", KortsaetID: "kortsaet-1", Name: "Kort 1"}))

	update := w.stmts[0][strings.Index(w.stmts[0], "ON DUPLICATE KEY UPDATE"):]
	for _, col := range []string{"checkpointIds", "extents", "format", "note", "sortOrder"} {
		if strings.Contains(update, col) {
			t.Errorf("replayed create must not overwrite %s: %s", col, update)
		}
	}
}

func TestUpdatedOnlyTouchesMentionedColumns(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.updated", 2,
		Updated{KortID: "kort-1", Name: ptr("Kort 1 — Start til Post 2")}))

	stmt := w.stmts[0]
	if !strings.Contains(stmt, "UPDATE") || !strings.Contains(stmt, "name") {
		t.Fatalf("want an UPDATE of name: %s", stmt)
	}
	for _, col := range []string{"format", "note", "checkpointIds", "extents", "kortsaetId"} {
		if strings.Contains(stmt, col) {
			t.Errorf("update must not mention %s when the event did not: %s", col, stmt)
		}
	}
	// Year-scoped as well as keyed, so a wrong-year request cannot reach another year's sheet.
	if !strings.Contains(stmt, "'2026'") {
		t.Errorf("update must be scoped to the subject's year: %s", stmt)
	}
}

// Clearing a sheet's checkpoints is a real edit. A plain nil slice could not express it — it
// would be indistinguishable from "this event does not mention checkpoints" — which is why the
// field is a pointer to a slice.
func TestUpdatedCanClearCheckpoints(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.updated", 2,
		Updated{KortID: "kort-1", CheckpointIDs: &[]types.CheckpointID{}}))

	stmt := w.stmts[0]
	if !strings.Contains(stmt, "checkpointIds") || !strings.Contains(stmt, "[]") {
		t.Fatalf("want checkpointIds set to an empty array: %s", stmt)
	}
}

func TestUpdatedWritesHandoutCheckgroup(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.updated", 2,
		Updated{KortID: "kort-1", HandoutCheckgroupID: ptr(types.CheckgroupID("cg-3"))}))

	if stmt := w.stmts[0]; !strings.Contains(stmt, "handoutCheckgroupId") || !strings.Contains(stmt, "'cg-3'") {
		t.Fatalf("want handoutCheckgroupId set to the group: %s", stmt)
	}
}

// The empty string is a *value* here — "revealed at the scan of this sheet's QR code" — so switching
// a sheet back from a checkgroup to the QR rule must produce a write, not be mistaken for an absent
// field and silently dropped.
func TestUpdatedCanClearHandoutCheckgroupBackToQR(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.updated", 2,
		Updated{KortID: "kort-1", HandoutCheckgroupID: ptr(types.CheckgroupID(""))}))

	if len(w.stmts) != 1 {
		t.Fatalf("want one statement, got %v", w.stmts)
	}
	if stmt := w.stmts[0]; !strings.Contains(stmt, "handoutCheckgroupId") || !strings.Contains(stmt, "''") {
		t.Fatalf("want handoutCheckgroupId cleared: %s", stmt)
	}
}

func TestUpdatedWithNothingSetWritesNothing(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.updated", 2, Updated{KortID: "kort-1"}))

	// An UPDATE with an empty SET is a syntax error, so the consumer must refuse rather than
	// hand it to the writer.
	if len(w.stmts) != 0 {
		t.Fatalf("want no statement for an empty update, got %v", w.stmts)
	}
}

func TestUpdatedEncodesExtents(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-5.updated", 3, Updated{
		KortID: "kort-5",
		Extents: &[]Extent{
			{NorthWest: types.Position{Latitude: 56.1, Longitude: 9.1},
				SouthEast: types.Position{Latitude: 56.0, Longitude: 9.3}},
			{NorthWest: types.Position{Latitude: 55.9, Longitude: 9.4},
				SouthEast: types.Position{Latitude: 55.8, Longitude: 9.6}},
		},
	}))

	stmt := w.stmts[0]
	// Two rectangles on one row: a double-sided sheet is one map, not two.
	if !strings.Contains(stmt, "northWest") || strings.Count(stmt, "southEast") != 2 {
		t.Fatalf("want two extents encoded on the row: %s", stmt)
	}
}

func TestDeletedRemovesOnlyTheSheet(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-1.deleted", 4, Deleted{KortID: "kort-1"}))

	stmt := w.stmts[0]
	if !strings.HasPrefix(stmt, "DELETE") || !strings.Contains(stmt, "'kort-1'") {
		t.Fatalf("want a keyed DELETE: %s", stmt)
	}
	if !strings.Contains(stmt, "'2026'") {
		t.Errorf("delete must be year-scoped: %s", stmt)
	}
}

// The id comes from the subject, never the body, so the two cannot disagree with what the stream
// routed on. A body claiming a different id must not reach another sheet.
func TestSubjectWinsOverBodyID(t *testing.T) {
	w := &recordingWriter{}
	c := &consumer{w: w}

	handle(t, c, msg("NATHEJK.2026.kort.kort-real.updated", 5,
		Updated{KortID: "kort-imposter", Name: ptr("x")}))

	if !strings.Contains(w.stmts[0], "'kort-real'") || strings.Contains(w.stmts[0], "imposter") {
		t.Fatalf("statement must key on the subject's id: %s", w.stmts[0])
	}
}

func TestContainsAllIsExistentialNotPartitioning(t *testing.T) {
	sheet := Kort{CheckpointIDs: []types.CheckpointID{"cp-1", "cp-2", "cp-3"}}

	if !sheet.ContainsAll([]types.CheckpointID{"cp-1", "cp-3"}) {
		t.Error("a sheet holding both checkpoints must contain them")
	}
	if sheet.ContainsAll([]types.CheckpointID{"cp-1", "cp-9"}) {
		t.Error("a missing checkpoint must not count as contained")
	}
	// A checkgroup with no checkpoints yet is an unfinished course, not a coverage failure.
	if !sheet.ContainsAll(nil) {
		t.Error("an empty list is contained by every sheet")
	}
}
