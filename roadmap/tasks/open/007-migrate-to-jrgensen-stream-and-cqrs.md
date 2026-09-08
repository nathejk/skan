# 007 — Migrate superfluids and tablerow to jrgensen/stream and jrgensen/cqrs

**Status:** open
**Priority:** medium
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

**Related:** 005 (the photo projections already assume the new libraries)

## Description

The in-repo streaming and projection plumbing has been superseded:

| Legacy, in-repo | Replacement |
|---|---|
| `go/superfluids/` — `streaminterface`, `jetstream`, `xstream` | `github.com/jrgensen/stream` |
| `go/pkg/tablerow/`, `go/pkg/sqlpersister/`, and more | `github.com/jrgensen/cqrs` |

Six projections are still on the old shape and registered on `xstream.Mux` in
`main.go`: `qr`, `scan`, `patrulje`, `klan`, `senior`, `personnel`. The write side
(`nathejk/commands/qr.go`) publishes through `streaminterface.Publisher`.

`nathejk/table/photo` and `nathejk/table/photocover` are already written against
the new libraries and are the reference for the target shape (see `.rules` →
*Coding conventions*). All projections are ultimately destined for
`github.com/nathejk/shared-go`, which is the reason to converge on one shape rather
than maintain two.

This is a mechanical but wide change, so it is worth doing per projection rather
than in one commit. 005 may stand the new mux up alongside the old one first; this
task finishes the job and deletes the legacy packages.

### Things to watch

- **Subject spelling.** `streaminterface.SubjectFromStr` replaces the *first* `:`
  with `.`, so `NATHEJK:*.klan.*.updated` and `NATHEJK.*.klan.*.updated` are the
  same subject today. Both spellings appear across the consumers. Confirm
  `jrgensen/stream` treats them identically before assuming the patterns still
  match, or events will silently stop arriving.
- **`log.Fatalf` on SQL errors.** Every legacy consumer kills the process on a bad
  row. Migration is the moment to return the error instead.
- **Write quoting changes.** Legacy consumers build SQL with
  `fmt.Sprintf("… SET x=%q", …)`, which is not correct quoting for SQL string
  literals. `cqrs.Writer.Consume` also takes a finished statement, so the values
  still need quoting — copy `photocover`'s `quote()` helper rather than carrying
  `%q` across.
- **The `year` column.** `qr` and `scan` never populate it, so `scan.Filter.YearSlug`
  only works via its empty-string bypass. Fix while touching these, since the photo
  projections are keyed by year and the inconsistency will start to bite.
- **Replay is the safety net.** The read model is rebuilt from the log on every
  start, so a migrated projection can be validated by dropping its table and
  comparing the result against the old one's output.
- Keep migrated projections free of skan-specific imports so they can be lifted to
  `shared-go` unchanged.

## Acceptance Criteria

- [ ] `github.com/jrgensen/stream` and `github.com/jrgensen/cqrs` in `go.mod`
- [ ] All six legacy projections consume via `cqrs`/`stream`
- [ ] `nathejk/commands` publishes via the new library
- [ ] One mux, not two
- [ ] `go/superfluids/`, `go/pkg/tablerow/`, `go/pkg/sqlpersister/` deleted
- [ ] Consumers return SQL errors instead of `log.Fatalf`
- [ ] Writes use correct SQL string quoting, not `%q`
- [ ] Subject patterns verified to match under the new library — events still arrive
      for every projection
- [ ] Each projection's table content after a full replay matches the pre-migration
      result
- [ ] `go test ./...`, `staticcheck` and `govulncheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ confirmed `superfluids` → `jrgensen/stream`
  and `tablerow` (plus more) → `jrgensen/cqrs`, with all projections eventually
  lifted to `shared-go`. Split out of 005 so wiring the photo projections is not
  blocked on a repo-wide rewrite.
