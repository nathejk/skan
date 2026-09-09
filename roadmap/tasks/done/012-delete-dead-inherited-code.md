# 012 — Delete dead inherited code

**Status:** done
**Priority:** medium
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-08
**Completed:** 2026-09-08

## Description

A large amount of code was inherited from sibling Nathejk repos (`hq`, tilmelding) and
never used here. It cost real confusion: task 008 spent effort de-hardcoding a year
inside `commands/team.go`, which nothing calls.

HQ: delete it.

## What was deleted

Projections and packages:

- `nathejk/table/spejder/`, `nathejk/table/signup/`, `nathejk/table/payment/`
- `nathejk/table/scanner/` — a byte-identical copy of `personnel`, still declaring
  `package personnel`
- Loose files in `nathejk/table/`: `confirm.go`, `patruljestatus.{go,sql}`,
  `pincode.{go,sql}`, `registrant.{go,sql}`, `signup.go`, `spejder.{go,sql}`,
  `spejderstatus.{go,sql}`

`nathejk/table/errors.go` was **kept** — `tables.ErrRecordNotFound` is used by every
live projection's queries.

Write side:

- `nathejk/commands/team.go` and `nathejk/commands/types.go` (the `Patrulje`, `Contact`,
  `Spejder`, `Klan`, `Senior`, `StartPatruljeMember` types existed only for it)
- The `Team` interface and its wiring in `commands.go`; `commands.New` no longer takes
  `data.Models`, since nothing else used it

Read side:

- `internal/data/{team,member,users,tokens,permissions,signup,filters}.go`
- The `Teams`, `Members`, `Permissions`, `Tokens`, `Users`, `Signup`, `Payment` and
  `Spejder` members of `data.Models`, plus the unused `NewModels` constructor

`data.Models` is now exactly the six projections this service reads: `Klan`, `Senior`,
`Patrulje`, `Personnel`, `QR`, `Scan`.

## Acceptance Criteria

- [x] Every listed file deleted
- [x] `data.Models` contains only projections that are actually read
- [x] `nathejk/table/errors.go` retained (still referenced)
- [x] Build, `go vet` and `staticcheck` pass on all packages that compile today
- [x] No behaviour change to any live route

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 04:10 — Created and picked up. Verified deadness before deleting rather
  than trusting the earlier documentation pass: grepped every candidate for external
  imports, confirmed `NewModels` is never called, confirmed `data.Filters`/`Metadata`
  are used only by the interfaces being removed, and confirmed the `commands` types are
  referenced only by `team.go`. `nathejk/table/errors.go` turned out to be live and was
  kept.
- 2026-09-08 04:20 — Deleted via `git rm`, then rewrote `internal/data/models.go` and
  `nathejk/commands/commands.go` around the gaps and dropped the now-unused `models`
  parameter from `commands.New` (updating the call in `main.go`).
- 2026-09-08 04:25 — ✅ Verified in the container: `go build`, `go vet` and
  `staticcheck` clean across `.`, `./internal/...`, `./nathejk/commands/...` and all six
  live projections. `./...` still fails only on the pre-existing `jrgensen/cqrs` gap
  (007).
- 2026-09-08 04:25 — Completed. Removed 25 files.
