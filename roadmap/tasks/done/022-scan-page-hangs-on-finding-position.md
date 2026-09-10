# 022 — The scan page can hang forever on "Finder position"

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: `/qr/{id}/{cs}` sometimes sits on *Finder position* and never moves. **The console is
empty.**

The empty console is the diagnosis, not a missing clue. `coordinates.html` called

```js
navigator.geolocation.getCurrentPosition(getLocation, pickOnMap)
```

with **no options**. `timeout` then defaults to `Infinity`, and — worse — the spec stops
that clock entirely while the browser's permission prompt is unanswered. So a scanner who
neither allows nor denies, or a phone that simply never gets a fix, leaves *both* callbacks
unfired. Nothing failed, so nothing was logged; the page waits forever by design.

At a checkpoint at 2am that is a scan lost, and lost quietly: the spinner reads as "working
on it", so nobody reloads.

### Not fixable with the geolocation timeout alone

Passing `timeout` is necessary but not sufficient, precisely because of the permission-prompt
carve-out. The page needs its **own** watchdog, which is the only clock that is certain to
run.

### Two neighbouring dead ends, same shape

Found while reading the flow, both leaving the spinner up forever:

- a `fetch` that rejects (signal dropped mid-`PUT`) was uncaught, so the page kept spinning
  after a scan that never reached the server;
- `pickOnMap` had no guard against a second call, and Leaflet throws when asked to
  initialise the same container twice — which the new watchdog would have made reachable.

## Acceptance criteria

- [x] The page always leaves the spinner: a fix, an error, or the map picker.
- [x] A watchdog independent of the geolocation timeout.
- [x] The scanner can reach the picker immediately rather than waiting out the budget.
- [x] A failed `PUT` says the scan was not recorded instead of spinning.
- [x] No scan can be recorded twice by two racing paths.

## Progress Log

- 2026-09-10 15:05 — Reproduced by reading rather than clicking: with no options object the
  default `timeout` is `Infinity`. Confirmed against the spec that the timeout does not start
  until permission is settled, which explains a hang with an empty console — the browser does
  not consider an unanswered prompt an error.
- 2026-09-10 15:15 — Added a single `claimed` flag rather than one guard per hazard. The
  hazards are all the same shape: the GPS fix, the watchdog and the scanner's own tap race to
  finish one scan, and exactly one may win. It also closes the double-init crash in
  `pickOnMap`.
- 2026-09-10 15:20 — `positionBudget = 15000`, used for both the watchdog and the geolocation
  `timeout`, with a comment saying why the watchdog cannot be replaced by the option. Also set
  `enableHighAccuracy: true` (the accuracy is recorded, and a coarse fix is little use for
  finding a patrol again) and `maximumAge: 15000` — reusing a fix a few seconds old removes
  most of the waiting, and a few seconds' drift on foot is nothing beside the accuracy radius.
- 2026-09-10 15:25 — Added a *"Det tager for lang tid – sæt markør på kort i stedet"* link in
  the spinner state, revealed by the script so it never appears without JS, where there is no
  picker to reach. Fifteen seconds is a long time to stand in a field wondering whether the
  phone is broken.
- 2026-09-10 15:28 — Wrapped the `fetch` in `try`/`catch`: on a dropped signal the page now
  shows *"Din fangst er ikke registreret"*. Leaving the spinner implied the scan might still
  land, and the scanner would walk away believing it had.
- 2026-09-10 15:30 — **Trade-off, deliberate:** once the picker is open the scan is claimed,
  so a GPS fix arriving late is ignored rather than replacing the manual marker. It loses a
  real fix in the narrow case where permission is granted after 15 s, but the alternative —
  accepting a late fix — needs a second flag to avoid recording the scan twice when it lands
  just after the scanner taps *Gem position*. A duplicate catch is worse than a manual
  position, and the manual one is labelled as such, so the data is not misleading.
- 2026-09-10 15:35 — ✅ Verified the page renders the watchdog, the options and the skip link
  (`docker run … curl … /qr/1/162726411`), and `go build` + `go test .` pass. **The timing
  behaviour itself is not machine-verified** — it needs a browser, and this repo has no
  JS test harness. Worth someone confirming by hand: deny permission → picker appears;
  leave the prompt untouched for 15 s → picker appears; tap the link → picker appears at once.
