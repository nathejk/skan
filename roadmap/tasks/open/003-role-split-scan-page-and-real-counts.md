# 003 — Split the scan-result page by role and compute real counts

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `catchCount` is computed from real data: scans of this patrol whose scanner is
      a senior
- [ ] Total scan count is computed from real data
- [ ] `isBandit` (or equivalent) comes from the role resolved in 001, never hardcoded
- [ ] A bandit's response contains **no** crew-only values — verified by inspecting
      the rendered HTML, not just the visible text
- [ ] A crew member sees both counts
- [ ] The `display:hidden` wrapper is removed; visibility is decided in the handler
- [ ] `.remark` is either supplied by the handler or removed from the template; the
      page renders without a template execution error in both cases
- [ ] `scan` querier reads use `?` placeholders
- [ ] The broken `scan.GetByID` is fixed or deleted
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Visibility rule and `catchCount` definition
  confirmed by HQ during a documentation pass.
