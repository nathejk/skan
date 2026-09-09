package table

import (
	"strings"
	"testing"
	"time"
)

func TestQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "Ulvene", want: `'Ulvene'`},
		{name: "empty", in: "", want: `''`},
		{
			// The case %q gets away with, and the one that matters most in practice:
			// Danish characters must pass through as themselves.
			name: "danish characters", in: "Bålsø Ægir Ål",
			want: `'Bålsø Ægir Ål'`,
		},
		{
			// The case %q gets wrong in a way that changes the value.
			name: "single quote", in: "O'Brien",
			want: `'O\'Brien'`,
		},
		{name: "double quote", in: `say "hi"`, want: `'say \"hi\"'`},
		{name: "backslash", in: `a\b`, want: `'a\\b'`},
		{name: "newline and carriage return", in: "a\nb\rc", want: `'a\nb\rc'`},
		{name: "nul and ctrl-z", in: "a\x00b\x1ac", want: `'a\0b\Zc'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Quote(tt.in); got != tt.want {
				t.Fatalf("Quote(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// TestQuotePreventsStatementBreakout is the point of the exercise: a value must not be
// able to end the literal it sits in.
func TestQuotePreventsStatementBreakout(t *testing.T) {
	evil := `'; DROP TABLE scan; --`
	stmt := "UPDATE patrulje SET name=" + Quote(evil) + " WHERE teamId='x'"

	// One opening and one closing quote around the value, and no unescaped quote in
	// between, so the statement still has exactly one string literal for the name.
	if strings.Count(stmt, "DROP TABLE") != 1 {
		t.Fatalf("unexpected statement: %s", stmt)
	}
	if strings.Contains(stmt, `name=''; DROP`) {
		t.Fatalf("value escaped its literal: %s", stmt)
	}
	if !strings.Contains(stmt, `\'`) {
		t.Fatalf("quote was not escaped: %s", stmt)
	}
}

func TestDatetime(t *testing.T) {
	got := Datetime(time.Date(2026, 9, 12, 23, 30, 15, 500, time.UTC))
	if want := `'2026-09-12 23:30:15'`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	// A zero time is NULL, not '0000-00-00', which MariaDB rejects in strict mode.
	if got := Datetime(time.Time{}); got != "NULL" {
		t.Fatalf("got %s, want NULL", got)
	}

	// Non-UTC input is normalised, so a replay does not shift timestamps with the
	// server's zone.
	oslo := time.FixedZone("CEST", 2*60*60)
	if got := Datetime(time.Date(2026, 9, 13, 1, 30, 15, 0, oslo)); got != `'2026-09-12 23:30:15'` {
		t.Fatalf("got %s, want the UTC equivalent", got)
	}
}
