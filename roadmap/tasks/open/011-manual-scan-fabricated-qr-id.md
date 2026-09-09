# 011 — Manual scan publishes a fabricated QR id ("x")

**Status:** open
**Priority:** medium
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

## Description

`doIndexHandler` (`POST /`, the manual-entry page where a scanner types a team number
instead of scanning a sticker) publishes the scan with a **literal QR id of `"x"`**:

```go
a.commands.QR.Scan("x", *team, *user, r.FormValue("latitude"), r.FormValue("longitude"))
```

HQ has confirmed this is a bug. Two concrete consequences:

1. The event subject becomes `NATHEJK.<year>.qr.x.scanned`, and the `scan` row claims a
   QR code that does not exist.
2. The `scan` table's primary key is `(qrId, uts)` with `INSERT IGNORE`, so **every
   manual scan across the whole race shares one key space**. Two manual scans in the
   same second — different patrols, different scanners — silently discard one of them.

### Why this is not just a one-line fix

The manual page exists precisely because there is no readable sticker (damaged, dark,
no camera). The scan fact is about *(team, scanner, time, location)*; the QR id is
incidental. But the only event available, `NathejkQrScanned`, is qr-keyed. So a real
fix has to choose:

- **(a) Use the patrol's most recently registered QR code.** No schema change, keeps
  every scan attributable, and the counts that matter (`catchCount`, the 004 rescan
  guard) key on team + scanner anyway, so the substitution is harmless to them. It
  does record a map the scanner never saw, and it has no answer for a patrol with no
  registered code yet — which is exactly the situation manual entry exists for.
- **(b) A non-qr-keyed event**, e.g. `NATHEJK.<year>.patrulje.<teamId>.scanned`. The
  truthful model, and it makes `scan.qrId` legitimately empty. Needs a new message type
  in `github.com/nathejk/shared-go`, and the `scan` projection's primary key has to
  change, since `(qrId, uts)` cannot key rows with no qr.
- **(c) Empty `qrId` on the existing event** plus a primary-key change. Cheaper than
  (b) and equally requires touching the key.

**Recommendation: (b)**, accepting the shared-go change, because it is the only option
that does not either invent data or overload a key. (c) is the acceptable compromise if
a shared-go change is unwelcome right now. This was deliberately **not** implemented
without a decision, since guessing writes wrong facts into race data.

Note the primary-key question overlaps with 004, which has to deal with the same
`(qrId, uts)` + `INSERT IGNORE` silent-drop behaviour. Whoever gets there first should
settle the key.

### Also worth fixing in the same handler

- An unknown team number renders the receipt page with `found: false` and records
  nothing, but returns `200`. Fine as behaviour; make sure the Danish text is explicit
  that nothing was recorded.
- The scan is published before the template is parsed, so a template failure returns
  500 after the scan was already recorded. Harmless but confusing in logs.

## Acceptance Criteria

- [ ] Decision recorded in the progress log: (a), (b) or (c)
- [ ] No fabricated QR id is ever published
- [ ] Two manual scans of different patrols in the same second are both recorded
- [ ] A manual scan is attributable to the right team, scanner, time and position
- [ ] The unknown-team-number path clearly says nothing was recorded, in Danish
- [ ] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 04:05 — Task created. HQ confirmed the `"x"` literal is a bug. Not fixed
  in place, because every available fix either invents a QR id or requires changing the
  `scan` primary key / a shared-go message type — a decision with consequences for race
  data. Options and a recommendation are written up above.
