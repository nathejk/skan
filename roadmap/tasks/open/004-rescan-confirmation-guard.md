# 004 — Confirmation guard against accidental rescans

**Status:** open
**Priority:** medium
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

**Related:** 003 (defines `catchCount`)

## Description

A rescan **counts**. Bandits legitimately catch the same patrol more than once
during a night, so `catchCount` is a plain count of scans, not of distinct
scanners. But an accidental double scan must not inflate the tally.

Decided guard (see `.rules` → *Counting catches: the rescan rule*):

> Look at the patrol's **single most recent scan**. If it was made by the *same*
> scanner *less than 30 minutes* ago, ask for an explicit confirmation
> ("skal dette tælle som en ny scanning?") before recording. Otherwise record it
> straight away.

Consequences, all intended:

| Situation | Result |
|---|---|
| Same scanner, same patrol, 5 min apart, nothing in between | confirmation required |
| Same scanner, same patrol, 45 min apart | recorded, no question |
| Same scanner twice, another scanner scanned in between | recorded, no question |
| A different scanner scans right after | recorded, no question |

Implementation constraints:

- The check keys on the **patrol**, not the QR code. A patrol picks up a new map
  with a new code several times during the race, so a code-based check would miss
  the exact double-scan being guarded against.
- The check is **server-side**, against the patrol's scan history. Not in the
  browser, not in the template.
- A confirmation must never *drop* a scan silently. Unconfirmed means "not recorded
  yet, ask the user", and saying yes must make it count.
- The existing flow is `GET /qr/{id}/{cs}` → page → `PUT /register` with the
  position. So `PUT /register` should answer "this needs confirming" and the page
  re-`PUT`s with an explicit confirm flag. Keep the decision out of the template.
- Needs a new read on the `scan` projection: latest scan for a team. The querier has
  only `GetAll` and a broken `GetByID` today.

**Beware an existing silent-drop bug in the same area:** the `scan` table's primary
key is `(qrId, uts)` and the consumer uses `INSERT IGNORE`
(`go/nathejk/table/scan/table.sql`, `consumer.go`). Two scans of the same code in
the same second are already discarded without trace, which will also swallow a
legitimate *confirmed* rescan that lands in the same second. Fix or explicitly
accept this as part of the task — do not build the confirmation flow on top of a
store that drops writes.

## Acceptance Criteria

- [ ] `scan` projection exposes a "latest scan for this team" read, via a
      `data.Models` interface, using `?` placeholders
- [ ] `PUT /register` returns a distinct, machine-readable "needs confirmation"
      response when the patrol's most recent scan is by the same scanner within 30
      minutes
- [ ] The scan page asks the scanner in Danish and re-submits with a confirm flag
- [ ] A confirmed rescan is recorded and counts toward `catchCount`
- [ ] A declined confirmation records nothing, and the scanner is told so
- [ ] All four rows of the table above behave as specified, covered by tests
- [ ] The `(qrId, uts)` / `INSERT IGNORE` same-second drop is fixed or documented as
      accepted, with reasoning in the progress log
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Rule proposed and confirmed by HQ: rescans count,
  but same-scanner-within-30-minutes-in-a-row needs an explicit confirmation.
