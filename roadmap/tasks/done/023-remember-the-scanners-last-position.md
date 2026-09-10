# 023 — Remember the scanner's own last position, and stop leaking the patrol's

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: every time a position is reported — by GPS or by placing a marker — store it in a
cookie, and centre the manual map on it next time one is needed.

The reasoning is sound and worth writing down: **a scanner barely moves all night.** A
checkpoint stays where it is, and a bandit works a lok. So the last place *this* scanner
reported from is a far better guess at where they are than anything else available, and it
saves panning across Denmark in the dark on a phone.

### The leak this replaces

The map was previously centred on **the patrol's** last known position, read from the `scan`
projection — and it was sent to **bandits** as well as crew.

That breaks the fair-game rule outright. The patrol's last recorded position is race
progress, very likely produced by a checkpoint minutes earlier, and *"checkpoint activity
and positions"* is crew-only. A bandit who declined geolocation was handed a map already
pointed at where the patrol had just been seen — a hunting hint, delivered by the app.

It hid well because it is not displayed anywhere: it is two numbers in a `setView` call that
only run on the fallback path. That is exactly the shape the rules warn about — a crew-only
figure must not merely be hidden from the markup, it must not be sent, and not even queried.

The scanner's own position has no such problem: it is information they produced themselves,
so it is safe for either role, and it is the better centre anyway.

## Acceptance criteria

- [x] A reported position — GPS *or* manual — is stored on the device.
- [x] The manual map opens on it when present.
- [x] Precedence and fallbacks: own last position → patrol's last position (crew only) →
      wide view of Denmark.
- [x] The scanner is told why the map is where it is.
- [x] The patrol's last position is not sent to a bandit, and not queried for one.
- [x] A tampered or nonsense cookie cannot break the map.

## Progress Log

- 2026-09-10 15:05 — Found the leak while reading `scanHandler` for where to add the cookie:
  `lastLatitude`/`lastLongitude` came from `Scan.LatestByTeam` and went into the template
  regardless of role. Not a regression from 016 so much as something never considered when
  the fallback map was introduced — the value is invisible on the page, which is precisely
  why it survived the role split in 003.
- 2026-09-10 15:12 — Fixed by wrapping the read in `if !user.IsBandit()`, so a bandit's page
  never carries it and the query never runs for them. Left it in place for crew: it is
  legitimate for them and still useful on the first scan of a night, before the cookie
  exists.
- 2026-09-10 15:20 — Cookie `lastPosition=lat,lng`, `path=/`, `max-age=86400`,
  `samesite=lax`. One race night: longer would risk centring on a previous event, and the
  first scan of the night refreshes it anyway. Written in both report paths — `getLocation`
  for a fix and the *Gem position* handler for a marker.
  Deliberately **not** `Secure`: dev is reachable over plain HTTP and the existing `user`
  cookie is not marked either. It carries no more than the position the same device is about
  to publish anyway.
- 2026-09-10 15:24 — `rememberedPosition()` validates rather than trusts: a cookie is
  editable, and a nonsense pair would leave Leaflet with a broken view and no obvious cause.
  Rejects non-numbers and out-of-range degrees.
- 2026-09-10 15:26 — Zoom 16 for the scanner's own position against 14 for the patrol's,
  because a position they reported themselves is one they are probably standing within a few
  hundred metres of. Added a line saying the map is centred on their last reported position:
  a view the scanner cannot account for is worse than a wide one, the same argument as the
  preselected sheet in 019.
- 2026-09-10 15:30 — ✅ Verified the role split live on `/qr/1/162726411`, which has real
  positions behind it: crew renders `parseFloat('55.6895844830955')`, a bandit renders
  `parseFloat('')`.
  ✅ Exercised the cookie parser under `node` (the two functions verbatim, with a stubbed
  `document`): round-trips a GPS fix, finds the value among other cookies, and returns null
  for junk, an impossible latitude, an empty value, and a cookie whose name merely *ends*
  with `lastPosition`. Confirmed separately that the same-case suffix `mylastPosition=1,2`
  does not match either, so that case passes for the right reason.
  `go build`, `go test`, `staticcheck` green.
- 2026-09-10 15:32 — **Not machine-verified:** that the cookie is actually written by a real
  browser and picked up on the next scan. It needs a browser, and there is no JS harness
  here. To confirm by hand: scan, allow geolocation, then scan again and decline — the map
  should open tight on the first position with the explanatory line showing.
