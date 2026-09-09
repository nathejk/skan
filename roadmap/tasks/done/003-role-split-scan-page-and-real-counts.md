# 003 — Split the scan-result page by role and compute real counts

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

**Depends on:** 001 (the role must be resolved at login first)

## Description

`scanHandler` in `go/routes.go` hardcodes the entire scan result:

```go
"scanCount":  10,
"catchCount": 1,
"isBandit":   true,
```

So every scanner — crew included — gets the restricted bandit page, and both
counts are fiction.

**Rule of thumb: bandits may only learn about bandits. It has to be a fair game.**
A bandit is a player, so anything they learn about a patrol's progress through the
race is an unfair advantage. Crew are not players — they may be checkpoint
personnel, a guide walking with the scouts, or a samarit taping up blisters — and
crew may see everything.

| | Bandit | Crew |
|---|---|---|
| Patrol identity, name, photo | yes | yes |
| `catchCount` — times caught by bandits | yes | yes |
| Total scan count (all scanners) | **no** | yes |
| Checkpoint / crew scan activity, positions | **no** | yes |
| Free-text remark about the patrol | yes | yes |

Definitions:

- **`catchCount`** — the number of scans of that patrol whose scanner is a senior
  (= a bandit). A plain count of scans, not of distinct scanners — see 004 for the
  accidental-rescan guard.
- **scan count** — total scans by anyone, crew included. Crew-only.

Do **not** send crew-only values to a bandit's browser and hide them in the
markup. The pre-port PHP page did exactly that, and `templates/coordinates.html`
still carries the inherited `<div style="display:hidden">` wrapper around the
counts. Client-side hiding of data already delivered to a player's phone is not a
boundary — the handler must not put the value in the template data at all.

Also fix while here: `coordinates.html` references `.remark`, which no handler
supplies. Comparing a missing map key against `""` is a **template execution
error**, so the page truncates mid-render. `remark` is a free-text note about the
patrol (the PHP app's `noticeText`), visible to both roles. Either supply it or
remove the markup — do not leave it half-wired.

New reads are needed on the `scan` projection; the querier currently has only
`GetAll` and a `GetByID` that selects `mapCreatedBy`/`mapCreatedAt` from the `scan`
table, columns which do not exist there (it is broken and uncalled — fix or delete
it as part of this work).

## Acceptance Criteria

- [x] `catchCount` is computed from real data: scans of this patrol whose scanner is
      a senior
- [x] Total scan count is computed from real data
- [x] `isBandit` (or equivalent) comes from the role resolved in 001, never hardcoded
- [x] A bandit's response contains **no** crew-only values — verified by inspecting
      the rendered HTML, not just the visible text
- [x] A crew member sees both counts
- [x] The `display:hidden` wrapper is removed; visibility is decided in the handler
- [x] `.remark` is either supplied by the handler or removed from the template; the
      page renders without a template execution error in both cases
- [x] `scan` querier reads use `?` placeholders
- [x] The broken `scan.GetByID` is fixed or deleted
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Visibility rule and `catchCount` definition
  confirmed by HQ during a documentation pass.
- 2026-09-09 09:47 — Picked up. Added two reads to the `scan` projection:
  `CountCatchesByTeam` (scans whose scanner is a senior) and `CountByTeam` (all scans).
  Both key on `teamId`, which is already year-unique because a team gets a fresh id each
  event, so no year filter is needed. Used `EXISTS` rather than a join to `senior`: that
  projection is keyed `(year, memberId)`, so the same memberId can appear in several
  years and a join would multiply one scan into several catches.
- 2026-09-09 09:48 — Deleted the broken `scan.GetByID`, which selected
  `mapCreatedBy`/`mapCreatedAt` — columns that only exist on `qr`. Nothing called it.
- 2026-09-09 09:48 — Also filled the `scan.year` column from the message subject while
  here. It existed but was never written, which left `scan.Filter{YearSlug}` working only
  through its empty-string bypass. Dropped the table so a replay backfilled it: 3587 rows,
  all labelled 2025, zero dead-letters.
- 2026-09-09 09:50 — Extracted `scanResultData` from the handler so the fair-game rule is
  stated and tested in one place. The crew-only count is **omitted from the template data
  entirely** for a bandit rather than passed and hidden — the inherited
  `style="display:hidden"` wrapper is gone, since hiding data already sent to a player's
  phone is not a boundary. The handler also skips the crew-only query when the scanner is
  a bandit, so it is not even read.
- 2026-09-09 09:51 — Removed `.remark` from `coordinates.html` rather than inventing a
  source: nothing in any projection holds a per-patrol note, so supplying it would have
  meant faking a field. If HQ wants remarks back (the PHP app's `noticeText`, shown in red
  to both roles) it needs somewhere to live first — flagging rather than guessing.
- 2026-09-09 09:52 — Decided the counts are "as of before this scan", and worded the page
  to match ("fanget N gange før", BINGO at zero). The page renders first and
  `PUT /register` records the scan afterwards, so a first catch would otherwise have read
  "fanget 0 gange". The alternative — having `/register` return fresh counts for the page
  to display — is cleaner and worth doing when 004 gives that endpoint a richer response.
- 2026-09-09 09:53 — ✅ Verified. Query semantics against real 2025 data: the busiest team
  has 36 scans of which 30 are bandit catches, so crew scans are correctly excluded rather
  than the two counts being the same number. End to end through Traefik with forged
  role cookies: a bandit gets "BINGO, det er første gang denne patrulje bliver fanget!"
  and a crew member gets "fanget 0 gange af banditter, og scannet 0 gange i alt". No
  `no value` in either page, confirming the `.remark` execution error is gone.
- 2026-09-09 09:54 — Added `routes_test.go`: `scanResultData` omits `scanCount` for a
  bandit and includes it for crew, and the real template is rendered for all three cases
  and asserted not to contain the crew-only figure or any `no value`. That last assertion
  is what would have caught the `.remark` bug.
- 2026-09-09 09:54 — Completed.
