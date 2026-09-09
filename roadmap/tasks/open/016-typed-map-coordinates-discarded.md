# 016 — Typed map coordinates are discarded

**Status:** open
**Priority:** medium
**Created:** 2026-09-09
**Picked up by:**
**Started:**
**Completed:**

## Description

The scan page's fallback for when the browser will not give up a position asks the
scanner to type a map coordinate:

```js
var coor = prompt('Indtast dit kortkoordinat:');
if (coor) { register({prompt: coor}) }
```

`registerHandler` decodes that into `input.Prompt` — and then **never uses it**. The
scan is published with empty latitude and longitude, so:

- The position the scanner took the trouble to type is lost entirely.
- `geoHandler` skips scans with no coordinates, so the scan is invisible on the map
  export.
- The scan itself still counts (`catchCount` is unaffected), so this fails quietly:
  the scanner sees "Din scanning er registreret" and nothing is wrong on screen.

This matters most exactly where it fails: geolocation is refused or unavailable in
poor conditions, which is when a hand-typed coordinate is the only position there is.

## Why it was not fixed in passing

`commands.QR.Scan` takes `latitude, longitude string`, and the event body
(`messages.NathejkQrScanned`) has a `Location` with only those two fields. A typed
map coordinate is not a latitude/longitude — it is a grid reference from a paper map
— so there is nowhere truthful to put it without either:

1. **Adding a field to the event** in `github.com/nathejk/shared-go` (e.g.
   `Location.Prompt` or `Location.MapReference`), plus a column on the `scan`
   projection. The honest option.
2. Parsing the typed value into coordinates. Only possible if the map grid has a known
   transform, and wrong the moment someone types something unexpected.
3. Dropping the fallback and requiring geolocation. Simplest, but it removes the only
   option a scanner has when their phone refuses — and refusing to record a catch is
   worse than recording it without a position.

**Recommendation: (1).** Until then the fallback should at least not pretend to have
worked \u2014 consider telling the scanner their position could not be recorded.

## Acceptance Criteria

- [ ] A typed map coordinate is either stored or explicitly reported as not stored
- [ ] The event shape change (if any) is agreed and lands in `shared-go` first
- [ ] The `scan` projection carries the value if the event does
- [ ] A scan with a typed coordinate is distinguishable from one with GPS
- [ ] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 10:05 — Found while implementing 004's confirmation flow, which touches the
  same handler and the same client-side code path. Filed rather than fixed because every
  honest fix needs a decision about the event shape, which lives in shared-go.
