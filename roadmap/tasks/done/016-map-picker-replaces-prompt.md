# 016 — Replace the typed-coordinate prompt with a map picker

**Status:** done
**Priority:** medium
**Created:** 2026-09-09
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

## Resolution

HQ: **drop the prompt.** When the browser will not give a position, show a map, let the
scanner zoom in and place a marker, and record that the position was set manually.

That resolves the original problem rather than working around it. A marker yields real
coordinates, so they go in the existing `location.lat`/`location.lon` — no parsing, no
guessing, nothing discarded. The only genuinely new fact is the *provenance*, and that
is now recorded explicitly.

## Original description

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

- [x] The typed-coordinate prompt is gone
- [x] A map lets the scanner place and adjust a marker, and submit it
- [x] A manually placed position is recorded as such, distinguishable from a GPS fix
- [x] The event shape change is additive and does not break other consumers
- [x] The `scan` projection carries the provenance, and `/geo` exports it
- [x] Scans recorded before this existed are not misreported as GPS
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 10:05 — Found while implementing 004's confirmation flow, which touches the
  same handler and the same client-side code path. Filed rather than fixed because every
  honest fix needs a decision about the event shape, which lives in shared-go.
- 2026-09-09 11:10 — HQ decided: drop the prompt, show a map with a draggable marker, and
  record the position as manual. Picked up.
- 2026-09-09 11:15 — The event still needed one new field, since `NathejkQrScanned` has
  only `lat`/`lon`. Added `nathejk/event` with a `QrScanned` that **embeds** the shared
  body and adds `locationSource` alongside it. Embedding rather than copying, so the
  shared fields stay defined in one place; additive rather than modified, so consumers
  decoding into the shared struct ignore it and skan reading older events just sees an
  empty source. The package doc says to collapse it into shared-go once the field is
  upstreamed — **worth doing, so hq and the map tooling can rely on it too.**
- 2026-09-09 11:20 — `commands.QR.Scan` now takes an `event.Position` rather than two
  loose strings, so provenance cannot be forgotten at a call site. `scan` gained a
  `locationSource` column, and `/geo` exports it as `position`.
- 2026-09-09 11:25 — Client side: `prompt('Indtast dit kortkoordinat:')` is replaced by a
  Leaflet map (CDN, no build step, consistent with Bootstrap/jQuery already being loaded
  that way). Tapping places a marker, tapping again moves it, and it is draggable; the
  submit button stays disabled until a marker exists. The map opens on the patrol's **last
  known position** where there is one — reusing the `LatestByTeam` read added for 004 — so
  nobody has to pan across Denmark in the dark, and on a wide view otherwise.
- 2026-09-09 11:30 — ✅ Verified server-side end to end: a `PUT` with `manual:true` stores
  `locationSource=manual`, one without stores `gps`, and the 3408 replayed historical scans
  store `""` — not misreported as GPS. `/geo` shows `"position":"manual"` and
  `"position":"gps"` for the two new rows. The scan page serves the Leaflet assets and the
  picker markup, and contains no `prompt(` or "kortkoordinat" anywhere.
- 2026-09-09 11:30 — Completed, with one honest limitation: the marker interaction itself
  could not be exercised from here — there is no browser in this environment, and the
  fallback only triggers when geolocation is refused. The wiring either side of it is
  verified; **someone should tap through it on a phone** before race night, ideally with
  location permission denied.
