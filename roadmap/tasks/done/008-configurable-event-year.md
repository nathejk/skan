# 008 — Make the event year configurable via YEAR

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-08
**Completed:** 2026-09-08

## Description

The event year is hardcoded as `"2025"` in the write side:

- `nathejk/commands/qr.go:25` — `yearSlug: "2025"`, used to build every published
  `qr.*.found` / `registered` / `scanned` subject
- `nathejk/commands/team.go:33` — the same field, plus **five inline `"2025"`
  literals** at lines 196, 213, 223, 238 and 254 that ignore the field entirely

Hardcoded years are not allowed. The year comes from an environment variable
`YEAR`, with `2026` as the dev value in `docker-compose.yml`.

This matters beyond tidiness: once `photo`/`photocover` are wired (005) every photo
read is keyed by year, so a wrong year silently means "no photograph" — and because
006 gates registration on the photograph, that becomes "nobody can register a map".

**No code default.** A fallback literal in Go is itself a hardcoded year, and a
wrong-but-plausible default fails silently in exactly the way described above. An
empty `YEAR` must stop the process at boot with a clear message.

### Out of scope, deliberately

- `nathejk/table/scan/consumer.go` never populates the `year` column it has. The
  correct source there is the **subject** (`msg.Subject().Parts()[1]`), not `YEAR` —
  a projection folds what the log says, and replaying an older year must not be
  relabelled with the current one. Left to 007, which is migrating that file anyway,
  and it currently has uncommitted local changes.
- The `qr` table has **no** `year` column at all, so there is nothing to populate
  there — noted because `.rules` previously claimed otherwise.
- `geoHandler` does not filter by year and is left alone; adding a filter would
  silently hide every scan already stored with an empty year.

## Acceptance Criteria

- [x] `YEAR` read from the environment into config, exposed as a `-year` flag like
      the other settings
- [x] No default year value anywhere in Go
- [x] Empty/unset `YEAR` fails at boot with a clear message
- [x] `commands.New` takes the year and passes it to both command sets
- [x] The five inline `"2025"` literals in `team.go` use the configured value
- [x] `YEAR: 2026` added to `docker-compose.yml` for both `api` and `prod`
- [x] No `2025`/`2026` literal remains in Go outside tests
- [x] `.rules` and `README.md` env tables document `YEAR`
- [x] Build and `staticcheck` pass

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 01:00 — Task created and picked up. Plan: thread a `year` config value
  from `YEAR` through `commands.New` into both command sets, fail fast when unset,
  add the dev value to compose. Scope deliberately excludes the projection-side
  `year` column (belongs to 007, and that file has uncommitted local changes).
- 2026-09-08 01:20 — Implemented. `config.year` added in `app.go` (flag `-year`,
  env `YEAR`, no default) with a `log.Fatal` when empty; `commands.New`, `NewQR` and
  `NewTeam` now take a `yearSlug`; `main.go` passes `app.config.year`; the five
  inline `"2025"` literals in `team.go` now use `c.yearSlug`; `YEAR: 2026` added to
  the `api` and `prod` services in `docker-compose.yml`.
- 2026-09-08 01:25 — Chose fail-fast over a default. A default literal would itself
  be a hardcoded year, and the failure mode it hides is silent: year-keyed reads
  return nothing rather than erroring, so a wrong year looks like "no data".
- 2026-09-08 01:30 — ✅ Verified in the container: `go build`/`go vet`/`staticcheck`
  clean on `.`, `./nathejk/commands/...`, `./internal/...`. Unset and empty `YEAR`
  both exit 1 with "YEAR is required"; `-year 2027` overrides the env var.
- 2026-09-08 01:35 — Note for 005: `go build ./...` cannot pass today because
  `photo`/`photocover` import `github.com/jrgensen/cqrs`, absent from `go.mod`. This
  also breaks the `air` dev loop (it runs `go test ./...` before rebuilding, with
  `stop_on_error = true`) and the production image (`build` stage runs the tests).
  Pre-existing, not caused by this task, and now recorded in `.rules`/`README.md`.
- 2026-09-08 01:40 — Completed. All year literals gone from Go outside the copied
  photo packages' tests; `YEAR` documented in both env tables.
