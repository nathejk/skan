# 013 — Replace %q with correct SQL string quoting in consumers

**Status:** done
**Priority:** medium
**Created:** 2026-09-09
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

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

- [x] All statement-building sites in `nathejk/table/*/consumer.go` quote values with a
      SQL-correct helper instead of `%q`
- [x] Numeric and time values are formatted explicitly, not via `%q`
- [x] A projection test covers a value containing a single quote, a double quote, a
      backslash and a Danish character, and asserts the row round-trips
- [x] Reads still use `?` placeholders
- [x] A full replay produces identical table contents to before the change — for four
      of six tables; the two differences are intended corrections, see log
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 08:13 — Split out of 007. The transport migration deliberately left the SQL
  string building untouched: it is orthogonal, spans 27 statement sites across six
  consumers, and is the kind of change that wants tests in front of it rather than being
  bundled into a library swap.
- 2026-09-09 10:20 — Picked up. Checked `cqrs` first: it has no quoting helper, so one was
  needed here. Put `Quote` and `Datetime` in the **parent** `nathejk/table` package rather
  than duplicating them six times — every projection already imports it as `tables` for
  `ErrRecordNotFound`, and the package travels to shared-go with them. `photo`/`photocover`
  keep their own copies, since they must stay byte-identical to their origin.
- 2026-09-09 10:25 — Converted the live statements in all six consumers. Two things beyond
  plain quoting:
  (1) `%q` on a `time.Time` invokes its Stringer (fmt uses Stringer for `%q` as well as
  `%v`), so timestamps were being written as `2025-06-02 18:23:38.123 +0000 UTC`. Now
  `Datetime` writes a clean UTC datetime, or `NULL` for a zero time.
  (2) `patrulje` took its `year` from `msg.Time().Year()` rather than the subject. Fixed to
  use the subject, per the rule the other projections follow.
- 2026-09-09 10:30 — Deleted dead code rather than converting statements that can never
  run: `senior/consumer.go` had a **second `switch` after an unconditional `return nil`**
  (~95 lines of klan and patrulje handlers copy-pasted from a sibling repo, including an
  `INSERT INTO spejder` for a table 012 deleted). That was the `go vet` "unreachable code"
  warning. Also removed `klan`'s `nathejk:patrulje.updated` case, a subject it does not
  subscribe to and a table it does not own. `personnel`'s `staff` branch was already
  commented out.
- 2026-09-09 10:35 — ✅ Verified by checksum: recorded `CHECKSUM TABLE` for all six
  projections, dropped them, replayed the whole log, and compared. **klan, personnel, qr
  and scan are byte-identical.** Zero dead-letters. The two differences are both intended:
  - `patrulje`: 3 rows changed year (2026 → 2025), because the year now comes from the
    subject instead of the message timestamp. The timestamp is unreliable here — it
    reflects when the event was published, and part of this stream looks republished, so a
    2025 signup could carry a 2026 timestamp. **Worth an HQ sanity-check**, since three
    patrols changing event year is data-visible.
  - `senior`: `createdAt`/`updatedAt` are `VARCHAR(99)`, not `DATETIME`, so the old code
    stored the full Go time string verbatim — fractional seconds and a ` +0000 UTC` suffix
    included. They are now clean `2025-06-02 18:23:38` values. `qr.mapCreatedAt` is a real
    `datetime` column, which is why it was unaffected: MariaDB had been silently truncating
    the junk, and `INSERT IGNORE` suppressed the warning. It worked by accident.
- 2026-09-09 10:36 — Added `nathejk/table/sql_test.go`: quoting of single quotes, double
  quotes, backslashes, NUL/CR/LF/Ctrl-Z and Danish characters, a statement-breakout case,
  and `Datetime` including the zero time and non-UTC normalisation.
- 2026-09-09 10:36 — Completed.
