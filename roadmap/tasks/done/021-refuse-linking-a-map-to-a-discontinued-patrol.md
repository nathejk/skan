# 021 — Refuse linking a QR code to a patrol that has left the race

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: when trying to connect a QR code to a team that is **discontinued**, the page should
say *"Patruljen er udgået"*.

Today it does not. Nothing stops it: `mapHandler` looks the number up, finds the team, and
offers the photograph and the confirm button exactly as it would for an active patrol. Bind
it and the sheet is recorded against a team that has nobody left in the race.

### Why this is a refusal, not a warning

A patrol is discontinued when it has **started and has no active members left**
(`Patrulje.Discontinued()` — `signupStatus == STARTED && activeMemberCount == 0`). There is
therefore nobody standing in front of the scanner to hand a map to. The number that was
typed is either a slip, or — much more likely — the number of the team the remaining scouts
*used* to be on, when what is needed is the team they joined. Both cases want the same
answer: stop, and say which number to use instead.

That is the same reasoning the scan page already applies from the other direction: scanning
a code whose patrulje is discontinued sends the scanner to *"Hvem har kortet nu?"* (018).
This closes the matching gap on the receiving side.

### Not to be confused with the message already on that page

`map.html` already prints *"Patruljen der havde dette kort er udgået af løbet"*. That is
about the code's **previous** team, on the reassign path. This task is about the team being
connected **to**. Two different sentences about two different teams, and the page must not
blur them — see the wording rule in `.rules`.

## Acceptance criteria

- [x] Entering the number of a discontinued patrol on `/map/{id}/{cs}` shows *Patruljen er
      udgået* and no confirm button.
- [x] The message says what to do instead: use the holdnummer of the team the scouts joined.
- [x] The scanner can try another number without losing `reassign`.
- [x] `POST /map/{id}/{cs}` refuses independently of the page, so a stale or forged form
      cannot bind the code anyway.
- [x] Works on both arrivals: an unused code and a reassign.
- [x] Tests cover the refusal and prove the two "udgået" sentences stay distinct.
- [x] Verified live against a genuinely discontinued 2026 team.

## Progress Log
- 2026-09-10 14:20 — Confirmed the gap before writing anything: `mapHandler` never consults
  `Patrulje.Discontinued()`, so a discontinued team's number produced a perfectly ordinary
  confirmation page — photograph, head count ("Der skal være 0 spejdere"), and a working
  "tilknyt kortet" button. The string HQ quoted appears nowhere in the repo, so this is a
  missing guard rather than a message gone wrong.
- 2026-09-10 14:25 — Guarded in both places. `mapHandler` sets `discontinued` and clears
  `confirm`; `doMapHandler` refuses with a `424` through `registrationRefused`. The server
  check is not redundant: the page could have been open while the team's last member was
  moved off, and the form is trivially editable.
  Placed the check **after** the photograph resolution so it takes precedence over
  `noPhoto` — "this team is out of the race" is the more useful answer, and it holds whether
  or not a photograph exists.
- 2026-09-10 14:28 — Wording kept deliberately separate from the reassign notice already on
  that page. `map.html` now has two "udgået" sentences about two different teams: the
  previous holder ("Patruljen der havde dette kort er udgået af løbet") and the one just
  typed ("Patrulje 1-6. Skjoldungerne 22 er udgået af løbet, så den kan ikke få et kort").
  The refusal names the patrol, because that is what lets a scanner see they mistyped, and
  it says what to use instead — the holdnummer of the team the scouts joined — since a
  refusal with no way forward just strands them at a handout post.
- 2026-09-10 14:30 — Added `TestMapPageRefusesADiscontinuedPatrol` (both arrivals): asserts
  the refusal, that it names the patrol and the number to use instead, that neither confirm
  button survives, that the previous-holder sentence is absent, and that "Prøv et andet
  nummer" keeps `reassign`. Extended `mapPageData` with the new key.
- 2026-09-10 14:40 — ✅ Verified live against 2026 team 1 "Skjoldungerne 22" (`STARTED`,
  6 members, 0 active — genuinely discontinued, not injected):
  `/map/6/165347856?number=1` → `<h1>Patruljen er udgået</h1>`, no button;
  `/map/4/164299278?reassign=1&number=1` → same, with `href="?reassign=1"`;
  `POST /map/6/165347856` with `confirmed=1` → `424` and the Danish refusal, so the page is
  not the only thing stopping it; `/map/4/164299278?reassign=1&number=2` (team 2, active)
  still reaches "flyt kortet" — no regression on the paths that should work.
- 2026-09-10 14:42 — Done. Gate green (`go build`, `go test`, `staticcheck`).
