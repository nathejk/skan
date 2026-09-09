# 006 — Show the cover photo, and confirm identity against it before registering

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

**Depends on:** 005 (projections wired and a usable image URL for a ref)

## Description

Two related requirements, both about the patrol's cover photograph.

### 1. Every scan shows the patrol's cover photo

`scanHandler` currently passes a hardcoded `"photo": "/groupphoto.jpg"` — a stock
image of climbers, identical for every patrol. It should be the cover photograph of
the patrol the scanned code belongs to: `photocover.Ref(year, teamID)`, falling back
to the team's newest photograph when no cover has been chosen (which is the normal
state — most teams never have one).

Showing the photo to **both** roles is correct and already agreed: patrol identity,
name and photo are visible to bandits and crew alike. Nothing else about the photo
changes the fair-game rules.

### 2. Registration requires confirming the photo matches

**This is the critical one.** Binding a QR code to a team is the moment an error
becomes permanent: every later scan of that map is attributed to whichever patrol
was named here, and the code lives for the rest of the race. So before a new
qr↔team connection is created, the scanner must have confirmed that **the group
standing in front of them is the group in the cover photo**.

`mapHandler`/`doMapHandler` already have a two-step shape — type a team number,
then press *Bekræft* — but the confirmation is meaningless today: it shows the same
stock photo for every team, so a scanner confirming it is confirming nothing. The
photo has to be the real one, the question has to be about the photo, and the
Danish wording must make the check explicit rather than implying "press to
continue".

A mistyped team number is the failure this catches: numbers are typed by hand, in
the dark, from a scout's arm.

### Teams with no photograph — decided

**A patrol cannot start the race without a photograph.** The race begins with the
scouts receiving their first map, and that does not happen until they have been
photographed. So by the time any QR code is registered, a photograph exists — the
first registration *is* the start of that patrol's race.

That makes "no photograph" not a normal case to design a fallback for, but an
**error state**: either the patrol has not actually started, or something upstream
failed. Behaviour:

- **Refuse the registration** and tell the scanner (in Danish) to contact HQ.
- Do **not** fall back to the stock image, and do not fall back to confirming the
  team name only — both would let a mistyped team number through at the one moment
  the error becomes permanent.

Note the ordering consequence: this makes the photograph a hard dependency of
registration, so if the photo projection is empty or the year is wrong, **no map can
be registered at all**. That is the intended strictness, but it means 005 must be
demonstrably working — correct year, events arriving — before this ships.

### A chosen cover is the exception, not the rule

`photocover` holds an organizer's explicit choice, and nothing may be publishing
`photocoverselected` yet. Assume most teams have **no** chosen cover, so the
fallback — the team's newest photograph — is the path that actually runs in
practice. It must be the well-tested one.

### Other notes

- Registration is published via `a.commands.QR.Register`; the confirmation gate
  belongs in the handler before that call, never in the template.
- Image URLs are `<FOTO_BASE_URL>/photos/<ref>`, with the base URL from the env var
  wired up in 005. Never hardcode the host.
- `doMapHandler` currently accepts any `confirmed` form value that parses to a team
  number, with no evidence a photo was shown. Whatever gate is added must not be
  bypassable by posting the form directly.
- `templates/map.html` and `templates/coordinates.html` both take `.photo`; keep the
  fallback chain in the handler so the templates stay dumb.
- Keep the pages fast on a bad field connection: serve the smallest rendition
  (`thumbRef`) where a thumbnail suffices, not the display image.

## Acceptance Criteria

- [ ] `scanHandler` shows the scanned patrol's cover photo, falling back to its
      newest photograph
- [ ] `mapHandler` shows the candidate patrol's cover photo on the confirmation step
- [ ] The stock `/groupphoto.jpg` default is gone from both handlers
- [ ] The confirmation wording (Danish) explicitly asks whether the group present is
      the group in the photo
- [ ] Registration cannot be completed without that confirmation, including by
      posting the form directly
- [ ] A patrol with no photograph cannot have a QR code registered; the scanner gets
      a Danish message telling them to contact HQ
- [ ] The stock image is not used as a stand-in for a missing photograph
- [ ] The newest-photograph fallback is exercised by tests, since a chosen cover is
      expected to be rare
- [ ] Thumbnail renditions are used where a full display image is not needed
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ: every scan should show the corresponding cover
  photo, and it is crucial that the scanner confirms the group in front of them is
  the one in the cover photo before a new qr/team connection is created. Noted that
  the existing *Bekræft* step already exists but confirms a stock image, i.e. it is
  currently theatre.
- 2026-09-08 02:00 — No-photograph case decided by HQ: a team is not allowed to start
  without a photograph, and the race starts by handing out the first map — so a
  photograph always exists by registration time. Missing means error, not
  "unphotographed team": refuse the registration and send them to HQ. Recorded the
  consequence that registration now depends hard on the photo projection being
  correct. Also noted that `photocoverselected` may not be published by anything yet,
  so the newest-photograph fallback is the common path.
