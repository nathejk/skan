package photocover

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

func ref(c byte) string { return strings.Repeat(string(c), 64) }

func newTable(t *testing.T) (*Table, *cqrstest.Writer, *cqrstest.Publisher) {
	t.Helper()
	w := &cqrstest.Writer{}
	p := &cqrstest.Publisher{}
	// No Reader: these tests exercise the projection and the publish path, neither of
	// which reads. See Select for what a nil Reader means there.
	table := New(p, w, nil)
	w.Reset() // the schema statement is not interesting below
	return table, w, p
}

func message(t *testing.T, subj string, body any) cqrs.Message {
	t.Helper()
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subj))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	return msg
}

func TestNewCreatesSchema(t *testing.T) {
	w := &cqrstest.Writer{}
	New(nil, w, nil)
	if len(w.Statements) != 1 || !strings.Contains(w.Statements[0], "CREATE TABLE IF NOT EXISTS photocover") {
		t.Fatalf("expected the schema to be created, got %v", w.Statements)
	}
}

// The subject this entity publishes must be matched by the pattern it subscribes
// to. Getting this wrong is a projection that silently never sees its own events.
func TestPublishedSubjectMatchesConsumed(t *testing.T) {
	table, _, p := newTable(t)
	if err := table.Select("2026", "team-abc", ref('a')); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(p.Messages) != 1 {
		t.Fatalf("expected one event, got %d", len(p.Messages))
	}

	subj := p.Messages[0].Subject().Subject()
	if want := "NATHEJK.2026.patrulje.team-abc.photocoverselected"; subj != want {
		t.Fatalf("got subject %q, want %q", subj, want)
	}

	// The live signal is derived from this subject, so the entity token must be
	// `patrulje` — that is what makes a patrol page see a cover change with no new
	// client dependency.
	if parts := strings.Split(subj, "."); parts[2] != "patrulje" || parts[3] != "team-abc" {
		t.Errorf("subject %q must carry entity `patrulje` and the team id", subj)
	}

	for _, consumed := range table.Consumes() {
		if p.Messages[0].Subject().Match(strings.Replace(consumed.Subject(), ":", ".", 1)) {
			return
		}
	}
	t.Errorf("published subject %q matches none of %v", subj, table.Consumes())
}

func TestSelectRejectsBadInput(t *testing.T) {
	table, _, p := newTable(t)
	for _, tc := range []struct{ name, year, teamID, ref string }{
		{"no year", "", "team-abc", ref('a')},
		{"no team", "2026", "", ref('a')},
		{"dotted team id", "2026", "team.abc", ref('a')},
		{"not a hash", "2026", "team-abc", "../../etc/passwd"},
	} {
		if err := table.Select(tc.year, tc.teamID, tc.ref); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
	if len(p.Messages) != 0 {
		t.Errorf("nothing should have been published, got %d", len(p.Messages))
	}
}

// An empty ref is a legitimate command: it is how an operator undoes a choice.
func TestSelectAcceptsEmptyRefToClear(t *testing.T) {
	table, _, p := newTable(t)
	if err := table.Select("2026", "team-abc", ""); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(p.Messages) != 1 {
		t.Fatalf("expected one event, got %d", len(p.Messages))
	}
}

func TestHandleSelectedWritesRow(t *testing.T) {
	table, w, _ := newTable(t)
	err := table.HandleMessage(message(t, "NATHEJK.2026.patrulje.team-abc.photocoverselected",
		PatruljePhotoCoverSelected{TeamID: "team-abc", Year: "2026", Ref: ref('a')}))
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(w.Statements) != 1 {
		t.Fatalf("expected one statement, got %v", w.Statements)
	}
	for _, want := range []string{"INSERT INTO photocover", "'team-abc'", "'2026'", "'" + ref('a') + "'", "ON DUPLICATE KEY UPDATE"} {
		if !strings.Contains(w.Statements[0], want) {
			t.Errorf("statement %q should contain %q", w.Statements[0], want)
		}
	}
}

// The subject is the more trustworthy source of both identifiers: the broker
// matched on it.
func TestHandleSelectedFallsBackToSubject(t *testing.T) {
	table, w, _ := newTable(t)
	err := table.HandleMessage(message(t, "NATHEJK.2026.patrulje.team-abc.photocoverselected",
		PatruljePhotoCoverSelected{Ref: ref('b')}))
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if !strings.Contains(w.Statements[0], "'team-abc'") || !strings.Contains(w.Statements[0], "'2026'") {
		t.Errorf("statement should carry the subject's tokens: %q", w.Statements[0])
	}
}

// A malformed ref costs the cover, not the event: the page falls back to the newest
// photograph, whereas storing the value would insist on a cover nothing satisfies.
func TestHandleSelectedBlanksMalformedRef(t *testing.T) {
	table, w, _ := newTable(t)
	err := table.HandleMessage(message(t, "NATHEJK.2026.patrulje.team-abc.photocoverselected",
		PatruljePhotoCoverSelected{TeamID: "team-abc", Year: "2026", Ref: "NOT-A-HASH"}))
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if strings.Contains(w.Statements[0], "NOT-A-HASH") {
		t.Errorf("a malformed ref must not be written: %q", w.Statements[0])
	}
	if !strings.Contains(w.Statements[0], "ref=''") {
		t.Errorf("expected the ref to be blanked: %q", w.Statements[0])
	}
}

func TestUnhandledSubjectIsIgnored(t *testing.T) {
	table, w, _ := newTable(t)
	if err := table.HandleMessage(message(t, "NATHEJK.2026.patrulje.team-abc.updated", map[string]string{})); err != nil {
		t.Fatalf("an unrelated subject must not fail: %v", err)
	}
	if len(w.Statements) != 0 {
		t.Errorf("nothing should have been written, got %v", w.Statements)
	}
}

// The one genuinely dangerous thing in this package: values from an event body are
// interpolated into a statement.
func TestQuoteEscapes(t *testing.T) {
	if got := quote(`a'b\c`); got != `'a\'b\\c'` {
		t.Errorf("quote: got %s", got)
	}
}
