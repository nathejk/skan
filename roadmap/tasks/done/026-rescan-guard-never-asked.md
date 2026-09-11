# 026 — Three scans in a row, no question asked

**Status:** done
**Priority:** high
**Created:** 2026-09-11
**Picked up by:** Zed agent
**Started:** 2026-09-11
**Completed:** 2026-09-11

## Description

HQ scanned `/qr/7/165872145` three times in a row and was never asked whether the repeats
counted. The 30-minute guard from task 004 did not fire.

The `scan` table shows five scans of code 7 inside three minutes — two by one scanner, three by
another — of which **four should have been questioned**:

| time (UTC) | scanner | expected |
|---|---|---|
| 09:16:26 | 35e4bfb5 | record (first) |
| 09:16:50 | 35e4bfb5 | **ask** |
| 09:18:34 | 07fb8f00 | record (other scanner in between) |
| 09:18:46 | 07fb8f00 | **ask** |
| 09:19:04 | 07fb8f00 | **ask** |

Every one was recorded.

## Investigation

- The server-side rule itself is fine. Replaying the exact sequence with `curl` and a forged
  cookie carrying the real scanner's id — page load, PUT, 12 s gap, three times over — gives
  `201, 409, 409`. So `needsRescanConfirmation` and `LatestByTeam` both work.
- The identity is not mismatched: `commands.QR.Scan` writes `string(scanner.ID)` as
  `scannerId`, and the guard compares against `string(user.ID)`. Same field, so a forged cookie
  is a faithful stand-in.
- No read error: an error from `LatestByTeam` logs, and the log across 09:16–09:25 has no such
  line. The database connection was not the problem.
- So the server returned `201`, meaning the guard genuinely decided "no question" — and it left
  **no trace of why**. That absence is itself a defect, and is fixed here.

Two mechanisms can produce this, and both are real defects, so both are fixed rather than
guessing between them.

### 1. The question was a native dialog

`window.confirm()`. Browsers may suppress those — after a previous dialog, in some in-app
browsers, or under their own throttling — and **a suppressed dialog still returns a value**.
Whichever way it goes, it is silent:

- returns true → the page re-sends with `confirm` set and the duplicate is recorded, nobody
  asked. *This matches what HQ saw exactly: scans recorded, no question.*
- returns false → the scan is dropped, nobody asked. Worse, and equally invisible.

A decision that changes what gets counted cannot depend on a browser's dialog policy.

### 2. The guard read a projection that can lag

The history came only from the `scan` projection, written asynchronously by a JetStream
consumer. While that read does not yet include the scan published seconds earlier, the guard
finds no history and asks nothing. A lagging projection therefore switches the guard **off**,
silently. This is the same hazard as task 015's registration redirect, from the other side:
there the handler can wait for the projection, here it cannot — nobody stands in a field while
a consumer catches up.

## Acceptance criteria

- [x] The question is asked in the page, not by a browser dialog.
- [x] Both answers are reachable, and cancelling does not read like a failure.
- [x] Answering twice cannot record two scans.
- [x] The guard does not depend on the projection having caught up.
- [x] Another scanner in between still clears the question — the rule is unchanged.
- [x] A guard decision leaves evidence in the log.

## Progress Log

- 2026-09-11 09:25 — Reproduced the *working* path first, to find out what was actually broken:
  `curl` gets `409` for the same scanner, team and window that went unasked in the browser. That
  moved the search from the rule to its two dependencies — the dialog and the projection.
- 2026-09-11 09:40 — Replaced `window.confirm` with an in-page state (`asking`): the server's
  message, a *"Ja, tæl det som en ny scanning"* button and a *"Nej, det var et uheld"* button,
  plus a line saying nothing is recorded until they answer — which is true, since the server
  answered `409` instead of writing.
  An `answered` flag makes the answer once-only: without it a double-tap on a phone sends two
  confirmed scans, which is the very thing being guarded against.
  Cancelling now has its own state rather than reusing the error one. "Din fangst er ikke
  registreret" is alarming when it is exactly what the scanner asked for; the two must not look
  the same.
- 2026-09-11 09:50 — Closed the projection hole with `recentScans`: the handler remembers what
  it published, and the guard uses whichever of the projection's answer and its own record is
  **newer**.
  Newer-wins rather than memory-wins is the important part: if another scanner has scanned
  since, the projection knows about a scan this process never published, and that scan is what
  the rule is about. So the rule is preserved rather than tightened.
  In-process state is sound because skan is a single instance, and losing it on restart is
  harmless — the projection is then the only source, which is where this started.
- 2026-09-11 09:55 — Added one log line per unconfirmed scan, naming what the projection said,
  what this process remembered, and which was used. The failure mode here is invisible after the
  fact: "I was not asked" cannot be told apart from "the rule said record" without knowing what
  the guard looked at. That is why this investigation needed a re-enactment rather than evidence.
- 2026-09-11 10:00 — ✅ Verified live. Three PUTs back-to-back as one scanner: `201, 409, 409`,
  with the log showing `own=race-test@… (seen=true)` on the second — so the guard held even
  though the projection had barely a moment. A confirmed retry then records (two rows, not
  three). The full sequence with page loads and 12 s gaps also gives `201, 409, 409`.
  Tests: `TestMostRecentScanSurvivesALaggingProjection` (five cases, including "another scanner
  in between" to pin the rule down) and `TestRescanQuestionIsAskedInThePage`, which fails if
  `window.confirm` ever comes back. Gate green.
- 2026-09-11 10:02 — **Honest limitation:** I could not reproduce the original `201`, so I
  cannot say which of the two mechanisms produced it. Both are genuine defects and both are
  closed; the new log line means a recurrence will be diagnosable instead of a re-enactment.
  If it happens again, `docker compose logs api | grep "rescan guard"` around the scan says
  immediately which input was wrong.
