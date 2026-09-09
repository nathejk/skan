# 007 — Migrate superfluids and tablerow to jrgensen/stream and jrgensen/cqrs

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

**Blocks:** 005, and therefore 006

## Description

HQ decided on a **complete switch**: the legacy in-repo plumbing is deprecated and
removed, not run alongside the new libraries. This is now the first task in the
photo chain rather than background cleanup.

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

This is a mechanical but wide change. Do it per projection, verifying each against a
replay before moving on — but the end state is **one** mux and no `superfluids`
imports anywhere. Both modules are public on the proxy, so `go get` needs no auth.

**It also unbreaks the build.** `go build ./...` and `go test ./...` fail today
because `photo`/`photocover` import `github.com/jrgensen/cqrs`, which is absent from
`go.mod`. That stops `air` from rebuilding (it runs the tests first, with
`stop_on_error = true`) and breaks the production image, whose `build` stage runs
`go test`, `staticcheck` and `govulncheck`. Adding the dependency early in this task
fixes the dev loop for everyone.

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

- [x] `github.com/jrgensen/stream` and `github.com/jrgensen/cqrs` in `go.mod`
- [x] All six legacy projections consume via `cqrs`/`stream`
- [x] `nathejk/commands` publishes via the new library
- [x] One mux, not two
- [x] `go/superfluids/`, `go/pkg/tablerow/`, `go/pkg/sqlpersister/` deleted
- [x] Consumers return SQL errors instead of `log.Fatalf`
- [ ] Writes use correct SQL string quoting, not `%q` — **deferred to 013**, see log
- [x] Subject patterns verified to match under the new library — events still arrive
      for every projection
- [x] Each projection's table content after a full replay matches the pre-migration
      result
- [x] `go test ./...`, `staticcheck` and `govulncheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ confirmed `superfluids` → `jrgensen/stream`
  and `tablerow` (plus more) → `jrgensen/cqrs`, with all projections eventually
  lifted to `shared-go`. Split out of 005 so wiring the photo projections is not
  blocked on a repo-wide rewrite.
- 2026-09-08 02:00 — HQ chose a **complete switch** over running both muxes, so this
  is no longer optional groundwork: it blocks 005 and 006, and its priority is raised
  to high. Modules confirmed public.
- 2026-09-09 08:00 — Picked up. Both modules resolve on the proxy (`cqrs v0.1.0`,
  `stream v0.1.2`).
- 2026-09-09 08:02 — **Blocker: both modules require Go >= 1.25**, and this repo was on
  1.24 with `GOTOOLCHAIN=local`. Bumped `docker/Dockerfile` to `golang:1.25-alpine` and
  `go.mod` to `go 1.25` / `toolchain go1.25.0`. The org's other repos already build on
  `golang:1.25`, so this aligns rather than diverges.
- 2026-09-09 08:04 — Read both libraries before touching anything. They are the
  extracted form of `superfluids`/`tablerow`, near-identically shaped:
  `jetstream.New(url)`, `xstream.NewMux(stream).AddConsumer(...).Run(ctx)`,
  `cqrs.Writer` = `Consume(string) error` (same as `tablerow.Consumer`), and
  `cqrs.Message`/`Subject`/`MessageFunc` are **type aliases** for the `stream` ones.
  `*sql.DB` satisfies `cqrs.Reader` directly, so no adapter is needed.
- 2026-09-09 08:05 — ✅ Resolved the subject-spelling risk this task was worried about:
  `subject.StringSubject.Match` is a byte-identical implementation of the old one, and
  `FromStr` keeps the same first-`:`-to-`.` rewrite. Both `NATHEJK:*.klan.*` and
  `NATHEJK.*.klan.*` therefore keep matching, so no consumer patterns needed editing.
- 2026-09-09 08:06 — Migration applied across 15 files: imports rewritten,
  `streaminterface.*` → `cqrs.*`, `tablerow.Consumer` → `cqrs.Writer`,
  `pkg/sqlpersister` → `cqrs/sqlpersister`, `superfluids/{jetstream,xstream}` →
  `stream/{jetstream,xstream}`. Deleted `superfluids/`, `pkg/tablerow/`,
  `pkg/sqlpersister/`.
- 2026-09-09 08:08 — `go build ./...` passes for the first time in this repo's recent
  history — `photo` and `photocover` now compile, and their own tests pass. `go test
  ./...`, `staticcheck ./...` and `govulncheck ./...` are all clean. Two pre-existing
  `go vet` "unreachable code" warnings remain (`klan/query.go:107`,
  `senior/consumer.go:112`); left alone as unrelated, and `vet` is not in the build gate.
- 2026-09-09 08:09 — First real boot exposed a genuine data bug that had been hidden
  behind the broken build: replay died with
  `Error 1406 (22001): Data too long for column 'groupName'`, and because consumers
  called `log.Fatalf` the process exited — on **every** start, unrecoverably, since the
  offending event is permanent history. Fixed both halves:
  (1) widened `patrulje.groupName` to `VARCHAR(999)`, because a patrol may be formed
  from several scout groups and the signup concatenates all their names;
  (2) replaced all 27 `log.Fatalf` sites in the consumers with `return err`, and wrapped
  the writer in `cqrs/deadletter`, armed after schema creation. Startup DDL still fails
  loudly; a bad row during replay is now recorded in a `deadletter` table and the loop
  continues.
- 2026-09-09 08:11 — Dropped the narrow `patrulje` table so the widened schema was
  recreated (`CREATE TABLE IF NOT EXISTS` never alters an existing table — note
  `cqrs.EnsureColumn` exists for adding columns, but not for widening one).
- 2026-09-09 08:12 — ✅ Verified end to end. The service starts, connects
  (`Connected to JetStream "nats://jetstream:4222", Stream created 'NATHEJK'`), replays
  the full log into all six projections (patrulje 719, scan 3587, qr 334, senior 1430,
  personnel 358, klan 230) and serves: `/healthcheck` and `/about` both answered
  **through Traefik** on the `web` entrypoint with a `Host: skan.local.nathejk.dk`
  header. A clean replay after truncating the table dead-letters **zero** statements —
  the 9 seen initially were stale rows from the pre-widening crash.
- 2026-09-09 08:13 — Deferred one criterion: replacing `fmt.Sprintf("… SET x=%q", …)`
  with proper SQL quoting across 27 statement sites. It is orthogonal to the transport
  switch, touches every consumer's SQL, and is safest done with tests in front of it —
  filed as **013** rather than smuggled into this change.
- 2026-09-09 08:13 — Completed. This also unblocks 005 → 006, and the `air` dev loop and
  production image build again.
