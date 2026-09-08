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

### Open question — teams with no photograph

If a patrol has neither a cover nor any photograph, there is nothing to confirm
against. **Decide before implementing** (needs HQ):

- refuse the registration and send them to HQ, or
- allow it with a clear warning that no photo could be checked, or
- fall back to confirming the team name and member count only.

Do not silently fall back to the stock image — that reproduces exactly the
meaningless confirmation this task removes.

### Other notes

- Registration is published via `a.commands.QR.Register`; the confirmation gate
  belongs in the handler before that call, never in the template.
- Image URLs are `<foto-base-url>/photos/<ref>`, with the base URL from an env var
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
- [ ] The no-photograph case behaves per HQ's decision, recorded in the progress log
- [ ] The stock image is not used as a stand-in for a missing photograph
- [ ] Thumbnail renditions are used where a full display image is not needed
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. HQ: every scan should show the corresponding cover
  photo, and it is crucial that the scanner confirms the group in front of them is
  the one in the cover photo before a new qr/team connection is created. Noted that
  the existing *Bekræft* step already exists but confirms a stock image, i.e. it is
  currently theatre.
