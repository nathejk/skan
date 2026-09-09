# 013 — Replace %q with correct SQL string quoting in consumers

**Status:** open
**Priority:** medium
**Created:** 2026-09-09
**Picked up by:**
**Started:**
**Completed:**

**Split out of:** 007

## Description

The six migrated projections build their statements with Go's `%q` verb:

```go
sql := "INSERT IGNORE INTO qr SET id=%q, teamNumber=%q, mapCreatedBy=%q, mapCreatedAt=%q"
c.w.Consume(fmt.Sprintf(sql, body.QrID, body.TeamNumber, body.ScannerID, msg.Time()))
```

`%q` produces a **Go** string literal, not a SQL one. It happens to work most of the
time on MariaDB — double quotes are accepted as string delimiters unless `ANSI_QUOTES`
is set, and Go escapes an embedded `"` as `\"`, which MySQL also understands — but it is
wrong in ways that will eventually bite:

- Turning on `ANSI_QUOTES` (or moving to another engine) breaks every statement at once.
- Go escapes non-ASCII to `\uXXXX` for some verbs and passes bytes through for others;
  Danish names (`ø`, `å`) and free-text remarks are exactly the values at risk.
- A value containing a backslash is escaped by Go's rules, which are not MySQL's.

`cqrs.Writer.Consume` takes a finished statement, so the values must be quoted by the
projection; the fix is not "use placeholders" but "quote correctly".
`nathejk/table/photocover/table.go` already carries the right helper (`quote()`, which
escapes `'`, `"`, `\`, NUL, newline, CR and Ctrl-Z), and `photo/sql.go` documents why
`%q` is not it. Copy that, do not reinvent it.

Reads are unaffected: queriers already use `?` placeholders through `cqrs.Reader`, and
must keep doing so.

## Acceptance Criteria

- [ ] All statement-building sites in `nathejk/table/*/consumer.go` quote values with a
      SQL-correct helper instead of `%q`
- [ ] Numeric and time values are formatted explicitly, not via `%q`
- [ ] A projection test covers a value containing a single quote, a double quote, a
      backslash and a Danish character, and asserts the row round-trips
- [ ] Reads still use `?` placeholders
- [ ] A full replay produces identical table contents to before the change
- [ ] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 08:13 — Split out of 007. The transport migration deliberately left the SQL
  string building untouched: it is orthogonal, spans 27 statement sites across six
  consumers, and is the kind of change that wants tests in front of it rather than being
  bundled into a library swap.
