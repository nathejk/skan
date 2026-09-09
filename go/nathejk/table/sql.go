package table

import (
	"strings"
	"time"
)

// Quote renders s as a SQL string literal, escaped for MySQL/MariaDB.
//
// # Why this exists rather than fmt.Sprintf("%q", s)
//
// `%q` produces a **Go** string literal, which is not the same language. It happens to
// survive on MariaDB most of the time — double quotes delimit strings unless
// ANSI_QUOTES is set, and Go escapes an embedded `"` as `\"`, which MySQL also
// understands — but it is wrong in ways that surface later:
//
//   - Enabling ANSI_QUOTES, or moving engine, breaks every statement at once.
//   - `%q` escapes non-ASCII to `\uXXXX` when the value is not valid UTF-8, and Danish
//     names (ø, å, æ) are exactly the values at risk.
//   - Go's backslash rules are not MySQL's, so a value containing a backslash is
//     escaped by the wrong grammar.
//
// # Why quoting at all, rather than placeholders
//
// cqrs.Writer.Consume takes a finished statement, not a statement plus arguments, so a
// projection has to do its own quoting. Reads are different: they go through
// cqrs.Reader and must keep using `?` placeholders.
//
// The escape set matches the one in nathejk/table/photocover, which is the reference
// implementation for the target projection shape.
func Quote(s string) string {
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

// Datetime renders t as a quoted MySQL DATETIME literal, or NULL for the zero time.
//
// Needed because `%q` on a time.Time invokes its Stringer — time.Time implements it,
// and fmt uses Stringer for %q as well as %v — yielding
// "2025-09-19 22:06:57.123 +0000 UTC". MariaDB then silently truncates that to fit a
// DATETIME column, and because the projections use INSERT IGNORE the truncation
// warning is suppressed. It worked by accident; this makes it deliberate.
func Datetime(t time.Time) string {
	if t.IsZero() {
		return "NULL"
	}
	return Quote(t.UTC().Format("2006-01-02 15:04:05"))
}
