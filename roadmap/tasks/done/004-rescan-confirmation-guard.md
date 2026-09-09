# 004 — Confirmation guard against accidental rescans

**Status:** done
**Priority:** medium
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

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

- [x] `scan` projection exposes a "latest scan for this team" read, via a
      `data.Models` interface, using `?` placeholders
- [x] `PUT /register` returns a distinct, machine-readable "needs confirmation"
      response when the patrol's most recent scan is by the same scanner within 30
      minutes
- [x] The scan page asks the scanner in Danish and re-submits with a confirm flag
- [x] A confirmed rescan is recorded and counts toward `catchCount`
- [x] A declined confirmation records nothing, and the scanner is told so
- [x] All four rows of the table above behave as specified, covered by tests
- [x] The `(qrId, uts)` / `INSERT IGNORE` same-second drop is fixed or documented as
      accepted, with reasoning in the progress log
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Rule proposed and confirmed by HQ: rescans count,
  but same-scanner-within-30-minutes-in-a-row needs an explicit confirmation.
- 2026-09-09 09:58 — Picked up. Added `scan.LatestByTeam` (keyed on the patrulje, not the
  QR code — a patrol carries a new map with a new code several times, so a code-based
  check would miss the exact double-scan being guarded against). Extracted the rule into
  `needsRescanConfirmation` so it is testable without HTTP.
- 2026-09-09 09:59 — `PUT /register` answers **409** with
  `{"status":"confirm","message":"…"}` when the guard trips, and the page re-sends once
  with `confirm:true` if the scanner says yes. Declining records nothing and shows the
  "ikke registreret" state. Nothing is ever dropped silently, and the decision stays
  server-side — the template only relays the question.
- 2026-09-09 10:00 — Two deliberate details: a failure to *read* the scan history logs and
  records the scan anyway (losing a catch is worse than recording a possible duplicate),
  and clock skew that puts the last scan in the future counts as "recent" rather than
  "ancient", so skew cannot switch the guard off.
- 2026-09-09 10:01 — Settled the same-second drop by widening the primary key to
  `(qrId, uts, scannerId)`. Two *different* scanners scanning one code in the same second
  are now both recorded — ordinary play at a checkpoint — while the same scanner twice in
  one second still collapses, which is a double-submit rather than two catches. Evidence
  it was real: replaying the log now yields **3591** scans where the old key produced
  3587, so four historical scans had been silently discarded. A confirmed rescan can never
  hit this, because a human has to answer a prompt first.
- 2026-09-09 10:02 — ✅ Verified live against the running service, all four rule cases:
  first scan → 201; same scanner immediately → **409** with "Du har lige scannet denne
  patrulje. Skal dette tælle som en ny scanning?"; same scanner with `confirm:true` → 201;
  a different scanner immediately → 201 with no question. Then scanned with a **real**
  senior's memberId and confirmed the counts: 4 total scans, 1 catch — the forged
  non-senior ids are correctly not counted as catches.
- 2026-09-09 10:03 — Fixed Danish grammar while verifying: the page said "fanget 1 gange".
  Both roles now use "én gang" in the singular, covered by a test. Also made the template
  assertions whitespace-insensitive, since the pluralisation split the sentence across
  lines.
- 2026-09-09 10:05 — Completed. One adjacent bug found in the same handler and filed as
  016: typed map coordinates (the geolocation fallback) are decoded into `input.Prompt`
  and then discarded, so the scan is stored with no position at all.
