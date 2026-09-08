# 005 — Wire up the photo and photocover projections

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

**Blocks:** 006 (showing the cover photo, and confirming against it)

## Description

`go/nathejk/table/photo/` and `go/nathejk/table/photocover/` have been copied into
this repo but are wired to nothing: not in `xstream.Mux` in `main.go`, not on
`data.Models`, and not reachable from any handler.

- `photo` is a **verbatim copy of the `foto` service's package**, bound for
  `shared-go`. Its own package doc says it must not diverge here. Treat it as
  read-only: adapt around it, do not edit it.
- `photocover` owns which photograph represents a patrulje, keyed `(year, teamId)`,
  and consumes `NATHEJK.<year>.patrulje.<teamId>.photocoverselected`.

### Blocker 1 — they don't compile in this repo

Both packages import **`github.com/jrgensen/cqrs`** (`cqrs.Publisher`, `Writer`,
`Reader`, `Subject`, `Message`), which is **not in `go.mod` or `go.sum`**. This
repo's projections use a different abstraction entirely: `streaminterface.Consumer`
+ `pkg/tablerow.Consumer` + a raw `*sql.DB`, registered on `xstream.Mux`.

So `xstream.Mux.AddConsumer` cannot accept either package today. Two options:

1. **Add the `cqrs` dependency and write an adapter** between
   `streaminterface.Message`/`Subject` and their `cqrs` equivalents, plus
   `tablerow.Consumer`→`cqrs.Writer` and `*sql.DB`→`cqrs.Reader` shims. Keeps
   `photo` byte-identical to its origin, which its package doc requires.
2. Port both packages to `streaminterface`. Cheaper now, but breaks the "verbatim
   copy" constraint and guarantees divergence from `foto`/`shared-go`.

**Option 1 is the intended route** unless HQ says otherwise. The adapter is the
deliverable; the copied packages stay untouched.

### Blocker 2 — there is no source for the image bytes

`photo` stores **content-hash refs** (`ref`, `thumbRef`, `renditions`), not bytes.
It deliberately never exposes the original. Skan has no blob store configuration
and no byte-serving route, so a ref cannot currently become an `<img src>`.

This must be resolved before 006 can display anything. Either:

- the `foto` service exposes a URL per ref that skan can link to directly (needs
  the URL pattern and whether it is public or token-guarded), **or**
- skan grows its own serving route backed by the blob store, gated on
  `photo.Servable(ctx, ref)` — which exists precisely as the access rule for such a
  route, and refuses refs that are only originals.

Linking out is far less work and keeps the bytes in one place. Serving locally
means blob credentials, caching, and a new public surface in a service that is
about to have a token-guarded export (002).

### Other things to get right

- **Read-only wiring.** `photocover` has a `Select` write command; choosing a cover
  is `hq`'s job, not a scanner's. Construct it with a **nil publisher** so this
  service cannot publish cover selections.
- `photocover.New` **panics** if schema creation fails, while `photo.New` returns an
  error. Handle both at boot deliberately.
- **The year stops being cosmetic.** Every photo/photocover read is keyed by
  `year`, and skan hardcodes `"2025"` in `commands.NewQR` with nothing else
  carrying it. Wiring these projections makes that hardcoded literal load-bearing —
  a wrong year means "no photo", silently. Thread a real year value through rather
  than sprinkling another `"2025"`.
- `photo` consumes `photographed`/`purged` events belonging to the `foto` service.
  Confirm those subjects actually exist on the JetStream `NATHEJK` stream this
  service reads, or the tables stay empty and 006 has nothing to show.
- `photocover.Covers()` uses window functions (MariaDB 10.2+). Compose runs 10.8, so
  this is fine — but note skan only needs the single-team read (`Ref`), not the
  whole-year `Covers`.
- Reads must reach handlers through a `data.Models` interface, per house style.
- `photocover.Queries` and the reads on `photo` are the interfaces to depend on —
  not the concrete `*Table`.

## Acceptance Criteria

- [ ] Decision recorded in the progress log: adapter vs port, and byte source
- [ ] `go build ./...` succeeds with both packages compiled in
- [ ] Both projections registered on `xstream.Mux` in `main.go`
- [ ] `photocover` constructed with a nil publisher; this service publishes no
      `photocoverselected` events
- [ ] Schema-creation failure at boot is handled for both (`panic` vs `error`)
- [ ] Both tables are created and populated after a replay against a stream that
      carries photo events
- [ ] A team's cover ref, and a usable image URL for it, are reachable from a
      handler via a `data.Models` interface
- [ ] The year used for photo reads comes from configuration, not a second
      hardcoded `"2025"`
- [ ] The copied `photo` package is unmodified
- [ ] `go test ./...` (including the packages' own `table_test.go`), `staticcheck`
      and `govulncheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ copied both packages in and asked for them to
  be wired up. Inspection found two blockers before any wiring is possible: the
  `github.com/jrgensen/cqrs` dependency is absent from `go.mod` so neither package
  compiles, and neither provides image bytes — only content-hash refs — so there is
  no way to render a photo until a byte source is decided.
