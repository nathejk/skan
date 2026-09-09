# 015 — Registration redirect races the projection

**Status:** done
**Priority:** medium
**Created:** 2026-09-09
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

## Description

`doMapHandler` publishes `qr.<id>.registered` and immediately redirects to
`/qr/{id}/{cs}`. That is a read-after-write against an **eventually consistent** read
model: the projection may not have folded the event yet, so `scanHandler` finds no
`qr` row, treats the code as unknown, publishes another `qr.<id>.found`, and redirects
back to `/map/{id}/{cs}`.

Observed while verifying 014: a single registration POST produced a chain of five
consecutive `303 See Other` responses before settling. The registration itself
succeeded, so this is a UX and stream-hygiene problem rather than data loss:

- The scanner may briefly be bounced back to the "type a team number" page they just
  completed, which invites them to register the same code twice.
- Each bounce appends a spurious `found` event to the log, permanently.

## Options

1. **Have the command wait for its own projection** before redirecting — e.g. poll
   `QR.GetByID` briefly, or use the stream's catch-up signal (`stream/caughtup`
   exists for this) so the redirect happens once the write is visible.
2. **Render the confirmation page directly** from `doMapHandler` instead of
   redirecting, using the data already in hand. Removes the race entirely, at the cost
   of a non-POST-redirect-GET flow (a refresh would re-POST).
3. **Make `scanHandler` tolerant**: if a code is unknown but was registered moments ago
   by this scanner, show the scan page rather than restarting registration. Needs a
   marker to recognise "moments ago", so it is the fiddliest.

**Recommendation: (1)**, scoped narrowly — the redirect is the right shape, it just
needs to wait for the fact it created to become readable.

Note the duplicate `found` events are harmless to the read model (`found` writes
nothing) but they are noise in the log forever, and the same race exists anywhere a
handler redirects to a page that reads what it just published.

## Acceptance Criteria

- [x] A successful registration lands on the scan page without bouncing back to the
      registration page
- [x] One registration produces exactly one `found`/`registered` pair, no repeats
- [x] The fix does not block the request indefinitely if the projection never catches up
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 09:10 — Found while verifying 014: one registration POST produced five
  chained 303s. Filed separately because the fix is a decision about how handlers
  synchronise with their own projections, which affects more than this one route.
- 2026-09-09 10:45 — Picked up. Checked option (1) properly first: `stream/caughtup`
  turned out to be a **boot-time sentinel** ("replay finished"), not a per-write signal, so
  there is nothing to subscribe to for one event. That rules out an event-driven wait and
  leaves a bounded poll as the honest implementation.
- 2026-09-09 10:50 — Implemented `waitForRegistration`: after `Register` succeeds,
  `doMapHandler` polls `QR.GetByID` every 25ms for up to 2s until the binding is readable,
  then redirects. Chose this over option (2) (rendering the page directly) because the
  POST-redirect-GET shape is worth keeping — a refresh on a rendered POST response would
  re-submit the registration.
- 2026-09-09 10:51 — The wait deliberately never fails the request: the registration is
  already durable in the stream before the wait starts, so a timeout logs and redirects
  anyway. Holding a scanner's request open would be worse than the bounce this is meant to
  avoid.
- 2026-09-09 10:55 — ✅ Verified. Registering sticker 3 to team 2 now produces exactly
  **one 303 followed by 200**, where the same flow produced five chained 303s before, and
  the `qr` row is present immediately. No "not yet visible" warnings, so the projection
  caught up inside the budget every time. Tests cover the catch-up path (three reads, two
  misses then a hit), that the happy path does not sleep at all, and that a projection
  which never catches up returns rather than hanging.
- 2026-09-09 10:55 — Completed.
