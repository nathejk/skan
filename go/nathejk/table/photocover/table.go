// Package photocover owns which of a patrulje's photographs represents it.
//
// # Why this is not part of the photo entity
//
// `nathejk/table/photo` is a verbatim copy of the package foto owns (see its
// package doc: it is bound for shared-go). foto decides what a photograph *is*;
// hq decides which one an organizer wants to look at first. Those are different
// facts with different owners, so the cover lives in its own entity and the copied
// package stays byte-identical to its origin.
//
// # The event
//
//	NATHEJK.<year>.patrulje.<teamId>.photocoverselected
//
// On the patrulje subject, like the photograph itself, so the choice is a fact
// about the team and a per-team purge erases it along with the pictures. It also
// means the live signal derived from the subject is `patrulje:<teamId>` — the same
// token a patrol page already depends on, so nothing on the client needs a new
// dependency to see a cover change.
package photocover

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

//go:embed table.sql
var tableSchema string

const (
	streamDomain = "NATHEJK"
	entityToken  = "patrulje"
	selectedVerb = "photocoverselected"
)

// PatruljePhotoCoverSelected says which photograph represents a patrulje.
//
// An empty Ref is meaningful: it is how a choice is withdrawn, leaving the page to
// fall back to the newest photograph. That is why the field is not omitempty —
// "absent" and "deliberately none" must not serialise the same.
type PatruljePhotoCoverSelected struct {
	TeamID     string    `json:"teamId"`
	Year       string    `json:"year"`
	Ref        string    `json:"ref"`
	SelectedAt time.Time `json:"selectedAt"`
}

// Table is the entity: a projection over the choice, the command that makes one,
// and the reads.
type Table struct {
	p cqrs.Publisher
	w cqrs.Writer
	r cqrs.Reader
}

// New creates the schema and returns the entity.
func New(p cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) *Table {
	if w != nil {
		if err := w.Consume(tableSchema); err != nil {
			// Consistent with the other hq tables, which also cannot continue without
			// their schema. photo.New returns an error instead because it runs in a
			// service whose job is to answer a webhook truthfully; here a missing
			// schema at boot is a deployment fault with nobody to report it to.
			panic(fmt.Sprintf("photocover: create table: %v", err))
		}
	}
	return &Table{p: p, w: w, r: r}
}

func (t *Table) CreateTableSql() string { return tableSchema }

func (t *Table) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr(fmt.Sprintf("%s:*.%s.*.%s", streamDomain, entityToken, selectedVerb)),
	}
}

func (t *Table) HandleMessage(msg cqrs.Message) error {
	if !msg.Subject().Match(fmt.Sprintf("%s.*.%s.*.%s", streamDomain, entityToken, selectedVerb)) {
		// Not an error: the mux may hand over a subject this projection does not care
		// about, and failing would dead-letter somebody else's event.
		return nil
	}

	var body PatruljePhotoCoverSelected
	if err := msg.Body(&body); err != nil {
		return err
	}

	// The subject is the fallback for both identifiers, and the more trustworthy
	// source: the broker matched on it.
	parts := msg.Subject().Parts()
	year, teamID := body.Year, body.TeamID
	if year == "" && len(parts) > 1 {
		year = parts[1]
	}
	if teamID == "" && len(parts) > 3 {
		teamID = parts[3]
	}
	if year == "" || teamID == "" {
		return fmt.Errorf("photocover: selected with no year or teamId")
	}

	// A ref that is not a content hash is written as "" rather than dead-lettered:
	// the worst outcome is "no cover chosen", which the page already handles by
	// falling back to the newest photograph. Storing the malformed value would leave
	// a row insisting on a cover no object can satisfy.
	ref := body.Ref
	if ref != "" && !validRef(ref) {
		ref = ""
	}

	return t.w.Consume(fmt.Sprintf(
		"INSERT INTO photocover SET year=%s, teamId=%s, ref=%s, selectedAt=%s "+
			"ON DUPLICATE KEY UPDATE ref=VALUES(ref), selectedAt=VALUES(selectedAt)",
		quote(year), quote(teamID), quote(ref), datetime(body.SelectedAt),
	))
}

// Commands is the write side of this entity, as a handler sees it.
type Commands interface {
	Select(year, teamID, ref string) error
}

// Select records that ref is the patrulje's cover photograph.
//
// Dirty-checked against the read model: re-selecting the photograph that is
// already the cover publishes nothing, so it emits no live signal and does not
// grow the log with restatements of a fact. An empty ref clears the choice.
//
// Without a reader there is nothing to compare against and the event is published
// unconditionally. That is the honest degradation — a duplicate statement of the
// same fact is harmless, whereas skipping the publish would lose a real change.
func (t *Table) Select(year, teamID, ref string) error {
	if t.p == nil {
		return fmt.Errorf("photocover: no publisher configured")
	}
	if err := validToken(year, "year"); err != nil {
		return err
	}
	if err := validToken(teamID, "team id"); err != nil {
		return err
	}
	if ref != "" && !validRef(ref) {
		return fmt.Errorf("photocover: ref %q is not a content hash", ref)
	}

	if t.r != nil {
		current, err := t.Ref(year, teamID)
		if err != nil {
			return err
		}
		if current == ref {
			return nil
		}
	}

	subject := cqrs.SubjectFromStr(fmt.Sprintf("%s.%s.%s.%s.%s",
		streamDomain, year, entityToken, teamID, selectedVerb))
	msg := t.p.MessageFunc()(subject)
	if msg == nil {
		return fmt.Errorf("photocover: no event transport")
	}
	if err := msg.SetBody(PatruljePhotoCoverSelected{
		TeamID:     teamID,
		Year:       year,
		Ref:        ref,
		SelectedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("photocover: set event body: %w", err)
	}
	return t.p.Publish(msg)
}

// validToken rejects anything that would not survive as one NATS subject token.
// An id containing a dot would publish successfully and quietly stop matching the
// per-team purge pattern — see photo/subject.go, which explains this at length.
func validToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("photocover: %s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("photocover: %s %q is not a valid subject token", what, s)
	}
	return nil
}

// validRef mirrors photo's check: 64 lowercase hex characters. Duplicated rather
// than shared because photo is a copy of another repo's package and exports no
// such helper; adding one there would diverge the copy.
func validRef(ref string) bool {
	if len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// quote renders s as a SQL string literal. cqrs.Writer.Consume takes a finished
// statement, not a statement plus arguments, so this package must do its own
// quoting; see photo/sql.go for why this is not fmt.Sprintf("%q").
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case 0:
			b.WriteString(`\0`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 26:
			b.WriteString(`\Z`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func datetime(t time.Time) string {
	if t.IsZero() {
		return "NULL"
	}
	return quote(t.UTC().Format("2006-01-02 15:04:05"))
}
