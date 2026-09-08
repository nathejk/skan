# 005 — Wire up the photo and photocover projections

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

**Blocks:** 006 (showing the cover photo, and confirming against it)
**Related:** 007 (migrating the legacy plumbing these packages already assume)

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

Neither library is in `go.mod`/`go.sum` yet, so **nothing here compiles today**.
That is the first thing to fix.

### How to wire them without doing the whole migration first

The existing projections (`qr`, `scan`, `patrulje`, `klan`, `senior`, `personnel`)
are still on `streaminterface` + `tablerow` and registered on `xstream.Mux`, which
cannot accept a `cqrs.Consumer`. Options, in preference order:

1. **Run the new plumbing alongside the old** — add `jrgensen/stream` +
   `jrgensen/cqrs`, stand up their mux for the two photo projections over the same
   NATS connection, and leave `xstream.Mux` serving the legacy ones until 007
   retires it. Unblocks 006 without a repo-wide rewrite.
2. Do 007 first and wire these onto the new mux afterwards. Cleaner end state, but
   006 waits on a much larger change.

Do **not** port `photo` backwards to `tablerow`/`streaminterface` to make it fit:
it must stay byte-identical to its origin, and that would deepen the dependency the
org is removing.

### Image bytes — resolved

`photo` stores content-hash refs, never bytes. The bytes are served by the `foto`
service:

```
<foto-base-url>/photos/<ref>
https://foto.local.nathejk.dk/photos/e3875accb257a8717b17e72353020e8c50fa131ee8978bea61177ed7f8a7a567
```

The base URL **must come from an environment variable** — never hardcode the host.
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
- `photo` consumes `photographed`/`purged` events belonging to the `foto` service.
  Confirm those subjects exist on the JetStream `NATHEJK` stream this service reads,
  or the tables stay empty and 006 has nothing to show.
- Skan only needs `photocover.Ref` (single team), not the whole-year `Covers`.
- Reads must reach handlers through a `data.Models` interface; depend on
  `photocover.Queries` and photo's read methods, not the concrete `*Table`.

## Acceptance Criteria

- [ ] `github.com/jrgensen/cqrs` and `github.com/jrgensen/stream` added to `go.mod`,
      `go.sum` updated
- [ ] Decision recorded in the progress log: alongside-the-old vs migrate-first
- [ ] `go build ./...` succeeds with both packages compiled in
- [ ] Both projections consume from JetStream and their tables are created and
      populated after a replay against a stream carrying photo events
- [ ] `photocover` constructed with a nil publisher; this service publishes no
      `photocoverselected` events
- [ ] Schema-creation failure at boot is handled for both (`panic` vs `error`)
- [ ] A team's cover ref is reachable from a handler via a `data.Models` interface
- [ ] A ref can be turned into an image URL using a base URL read from an env var,
      with no hardcoded host anywhere
- [ ] Env var added to `docker-compose.yml` and documented in `.rules` + `README.md`
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
- 2026-09-08 00:30 — Both blockers clarified by HQ, and the framing in the original
  description was wrong. (1) `cqrs` is not a foreign abstraction to adapt to: it
  replaces `pkg/tablerow`/`sqlpersister`, and `jrgensen/stream` replaces
  `superfluids`. These two packages are the *target* shape; the rest of
  `nathejk/table/` migrates to meet them (task 007 created). The earlier
  "write an adapter, or port photo to streaminterface" recommendation is withdrawn —
  porting `photo` backwards is explicitly wrong. (2) Bytes come from the `foto`
  service at `<base>/photos/<ref>` with the base URL in an env var, so skan needs no
  blob store and no serving route. Acceptance criteria rewritten accordingly.
