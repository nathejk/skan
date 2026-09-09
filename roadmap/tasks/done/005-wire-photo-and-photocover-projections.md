# 005 — Wire up the photo and photocover projections

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

**Blocks:** 006 (showing the cover photo, and confirming against it)
**Depends on:** 007 — HQ decided on a **complete switch** to `cqrs`/`stream`, so the
legacy plumbing is retired first and these are wired onto the new mux only.

## Description

`go/nathejk/table/photo/` and `go/nathejk/table/photocover/` have been copied into
this repo but are wired to nothing: not on the mux in `main.go`, not on
`data.Models`, and not reachable from any handler.

- `photo` is a **verbatim copy of the `foto` service's package**, bound for
  `shared-go`. Its own package doc says it must not diverge here. Treat it as
  read-only: adapt around it, do not edit it.
- `photocover` owns which photograph represents a patrulje, keyed `(year, teamId)`,
  and consumes `NATHEJK.<year>.patrulje.<teamId>.photocoverselected`.

Both are written against **`github.com/jrgensen/cqrs`**, which is the library that
replaces this repo's `pkg/tablerow` + `pkg/sqlpersister` (with
`github.com/jrgensen/stream` replacing `superfluids`). So these two packages are
not foreign code to be adapted to the old shape — **they are the target shape**,
and everything else in `nathejk/table/` is what will eventually move to meet them.
All projections are destined for `shared-go` once stabilised.

Both libraries are **public** on the module proxy, so `go get` needs no auth.

### Wiring happens after the switch, not alongside it

HQ decided on a complete switch (007): `superfluids` and `pkg/tablerow` are
deprecated and removed, not run in parallel. So there is **one** mux, the `cqrs`
one, and these two projections register on it like every other. Do not build a
second mux or an adapter to bridge the old shape — an earlier revision of this task
suggested running both side by side, and that option is withdrawn.

Do **not** port `photo` backwards to `tablerow`/`streaminterface`: it must stay
byte-identical to its origin.

### Image bytes — resolved

`photo` stores content-hash refs, never bytes. The bytes are served by the `foto`
service:

```
<foto-base-url>/photos/<ref>
https://foto.local.nathejk.dk/photos/e3875accb257a8717b17e72353020e8c50fa131ee8978bea61177ed7f8a7a567
```

The base URL **must come from an environment variable**, `FOTO_BASE_URL` (confirmed
by HQ) — never hardcode the host.
Skan therefore needs **no** blob store, no credentials and no byte-serving route;
`photo.Servable` is not needed here. Add the variable to `docker-compose.yml` with
the `foto.local.nathejk.dk` dev value and document it in `.rules` and `README.md`.

### Other things to get right

- **Read-only wiring.** `photocover` has a `Select` write command; choosing a cover
  is `hq`'s job, not a scanner's. Construct it with a **nil publisher** so this
  service cannot publish cover selections.
- `photocover.New` **panics** if schema creation fails, while `photo.New` returns an
  error. Handle both at boot deliberately.
- **The year stops being cosmetic.** Every photo/photocover read is keyed by
  `year`, and skan hardcodes `"2025"` in `commands.NewQR` with nothing else carrying
  it. Wiring these makes that literal load-bearing — a wrong year silently means "no
  photo", and once 006 gates registration on the photo, that means "nobody can
  register a map". Thread a real year value through rather than adding a second
  `"2025"`.
- `photo` consumes `photographed` and `purged`. **`photographed` is confirmed to
  exist** on the stream; `purged` may not yet, which is harmless — it simply never
  arrives. Whether `photocover`'s `photocoverselected` is published by anything yet
  is **unconfirmed**, so assume most or all teams have **no chosen cover** and that
  the fallback (newest photograph) is the normal path, not the exception.
- Skan only needs `photocover.Ref` (single team), not the whole-year `Covers`.
- Reads must reach handlers through a `data.Models` interface; depend on
  `photocover.Queries` and photo's read methods, not the concrete `*Table`.

## Acceptance Criteria

- [x] `github.com/jrgensen/cqrs` and `github.com/jrgensen/stream` added to `go.mod`,
      `go.sum` updated
- [x] Both projections registered on the single `cqrs` mux — no second mux, no
      adapter to the legacy shape
- [x] `go build ./...` succeeds with both packages compiled in
- [x] Both projections consume from JetStream and their tables are created and
      populated after a replay against a stream carrying photo events
- [x] `photocover` constructed with a nil publisher; this service publishes no
      `photocoverselected` events
- [x] Schema-creation failure at boot is handled for both (`panic` vs `error`)
- [x] A team's cover ref is reachable from a handler via a `data.Models` interface
- [x] A ref can be turned into an image URL using a base URL read from an env var,
      with no hardcoded host anywhere
- [x] Env var `FOTO_BASE_URL` added to `docker-compose.yml` and documented in `.rules` + `README.md`
- [x] The year used for photo reads comes from configuration, not a second
      hardcoded `"2025"`
- [x] The copied `photo` package is unmodified
- [x] `go test ./...` (including the packages' own `table_test.go`), `staticcheck`
      and `govulncheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ copied both packages in and asked for them to
  be wired up. Inspection found two blockers before any wiring is possible: the
  `github.com/jrgensen/cqrs` dependency is absent from `go.mod` so neither package
  compiles, and neither provides image bytes — only content-hash refs — so there is
  no way to render a photo until a byte source is decided.
- 2026-09-08 00:30 — Both blockers clarified by HQ, and the framing in the original
  description was wrong. (1) `cqrs` is not a foreign abstraction to adapt to: it
  replaces `pkg/tablerow`/`sqlpersister`, and `jrgensen/stream` replaces
  `superfluids`. These two packages are the *target* shape; the rest of
  `nathejk/table/` migrates to meet them (task 007 created). The earlier
  "write an adapter, or port photo to streaminterface" recommendation is withdrawn —
  porting `photo` backwards is explicitly wrong. (2) Bytes come from the `foto`
  service at `<base>/photos/<ref>` with the base URL in an env var, so skan needs no
  blob store and no serving route. Acceptance criteria rewritten accordingly.
- 2026-09-08 02:00 — HQ answered the open questions. Both modules are **public**, so
  no proxy auth is needed. A **complete switch** is wanted (007) rather than running
  the new mux alongside the old, so this task now depends on 007 and the "alongside"
  option is withdrawn. `photographed` is confirmed present on the stream; `purged`
  and `photocoverselected` are not confirmed, so the newest-photograph fallback must
  be treated as the normal path rather than an edge case.
- 2026-09-09 08:15 — Picked up now that 007 has landed. Wired both projections onto the
  single `xstream` mux in `main.go`, each constructed with a **nil publisher** so this
  service can only read them, and exposed narrow interfaces on `data.Models`:
  `PhotoInterface.ByTeam` and `PhotoCoverInterface.Ref` only. `Servable` and
  `OriginalRefs` are deliberately left off — skan never serves bytes and must never
  touch originals.
- 2026-09-09 08:16 — Added `FOTO_BASE_URL` (required, no default, trailing slash
  trimmed) and `go/photos.go`, which resolves a team's photograph: chosen cover first,
  newest photograph otherwise, `""` when there is none. Prefers `thumbRef` for the
  field-connection reason, and never uses the original ref.
- 2026-09-09 08:20 — ✅ Verified against the live stream: `photo` holds 3 photographs
  across 2 teams and `photocover` holds 1 selection, all keyed `year=2026`, matching the
  configured year. So `photographed` **and** `photocoverselected` are both being
  published — better than this task assumed.
- 2026-09-09 08:35 — ✅ End-to-end: the registration page renders
  `https://foto.local.nathejk.dk/photos/e3875acc…a567` for a team using the newest
  photograph, and the chosen cover's ref for the team that has one — confirming the
  precedence order works against real data.
- 2026-09-09 08:40 — Completed. Two bugs found while verifying are recorded in 006 and
  filed as 014.
