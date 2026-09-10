# 024 — Only offer map sheets that are handed out at a QR scan

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: the map selector should only show sheets that are **handed out by scanning a QR code**.

`kort.handoutCheckgroupId` already records exactly this, and its own column comment uses the
same words: it holds "the id of the checkgroup whose post gives it to the team, or `""` for
*at the QR scan*". The picker was listing the whole spejder set regardless, so sheets given
out by a specific post appeared as options for a scanner binding a code.

### Why the two must be the same list

Whatever the picker omits, `POST /map/{id}/{cs}` must refuse. The form can be posted
directly, so a restriction applied only to the rendered options is decoration. Both reads now
share the same pair of filters.

### The skitse cross-check

2026's spejder set has five sheets. The two carrying a `handoutCheckgroupId` are exactly the
two whose `format` is `skitse` — and a skitse, per `kort`'s own schema, is "a hand-drawn slip
with **no QR code**". So the new rule removes precisely the sheets that could never have been
the one whose code is being scanned. Two independent facts agreeing is a good sign the rule
is the intended one.

## Acceptance criteria

- [x] The picker lists only sheets with no handout checkgroup.
- [x] Handout order is preserved.
- [x] `POST` refuses an excluded sheet.
- [x] A carried-over sheet on a re-bind is still accepted, even if it would now be excluded.
- [x] The handout-post suggestion cannot name a sheet that is not on offer.

## Progress Log

- 2026-09-10 15:50 — Checked the live data before writing the filter, rather than assuming
  which column meant this. 2026's spejder set: `Deltagerkort 1/2/3` with
  `handoutCheckgroupId = ''`, and `Skitse CP2` / `Målskitse` with one set — the same two rows
  that have `format = 'skitse'`.
- 2026-09-10 15:55 — Added `qrHandoutFilter` next to `spejderSetFilter` in
  `internal/data/kort.go` and applied it to **both** `SpejderSheets` and `IsSpejderSheet`, so
  the validated set is the offered set. Left the carried-sheet path in `doMapHandler`
  untouched: a re-bind accepts the sheet the scouts already hold without re-validating it,
  which is what stops a hand-over breaking when a sheet's configuration changes.
- 2026-09-10 16:00 — **This makes task 019's suggestion unreachable.**
  `SheetForScanner` resolves a sheet *by its handout post*, so every sheet it can return has a
  non-empty `handoutCheckgroupId` — exactly what the picker now excludes. Left the code in
  place but guarded it with a new `offeredSheet` helper: a suggestion is used only if it is
  actually among the options. Naming a sheet that is not in the list would send a scanner
  hunting for an option that does not exist.
  Not deleted, because that is HQ's call and the two features may simply need reconciling:
  either a handout post's sheet *is* scanned when handed over (in which case the filter needs
  to be narrower than "no handout checkgroup"), or 019 was built on a premise that does not
  hold and should go. Raised with HQ; noted here so the next reader does not "fix" the guard.
- 2026-09-10 16:05 — Added `TestOfferedSheetKeepsASuggestionHonest`. The helper is pure, so it
  is testable without a database, unlike the queries around it.
- 2026-09-10 16:10 — ✅ Verified live. `/map/6/165347856?number=2` now lists only
  `Deltagerkort 1`, `Deltagerkort 2`, `Deltagerkort 3`, still in handout order — the two
  skitser are gone. `POST` with `Skitse CP2`'s id → `424` *"Det valgte kort hører ikke til
  spejdernes kortsæt"*; `POST` with `Deltagerkort 1` → `303` and the binding recorded
  (`qr` id 6 → team 2, `mapId = kort-6d016679…`). `go build`, `go test`, `staticcheck` green.
