# 017 — Pick a map from the spejder set when linking a QR code

**Status:** done
**Priority:** high
**Created:** 2026-09-09
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

## Description

HQ copied in a new projection, `nathejk/table/kort`: the map sheets printed and handed
out during the race, grouped into sets (`kortsaet`). Requirements:

- When a new map is linked to a patrulje, the scanner must **pick a map** from the set
  marked as the spejder set.
- The chosen **MapID must follow the registered event**.

### Which set is "the spejder set"

The set marked for the **patrulje** team type. There is no `spejder` team type —
`types.TeamTypes` is `patrulje`, `klan`, `staff`, `gøgler` — so the two names describe the
same thing from opposite ends: *spejder* is who carries the map, *patrulje* is the team
they form. `kortsaet`'s own schema comment asks "which set is the spejder set?" and
answers it with `teamType`, so this reading is the package's own.

Confirmed against live data: the year has two sets, `Crew` (unmarked) and `Deltagerkort`
(marked `patrulje`) holding five sheets.

An **unmarked set is the crew set and is deliberately not a fallback here.** Handing a
patrol a crew sheet would show the scouts checkpoints they are not meant to see yet —
the same fair-game concern that keeps checkpoint activity off a bandit's screen. Note
this is the opposite of the advice in `Kortsaet.TeamType`, which is about reading maps
*for* a team type in general; when the answer decides what scouts may see, guessing is
worse than refusing.

## Acceptance Criteria

- [x] `kort` is wired onto the mux, with a nil publisher (hq owns the sheets)
- [x] The registration page lists the spejder set's sheets in handout order
- [x] Sheets from the crew set are never offered
- [x] Registration refuses without a chosen sheet
- [x] A submitted sheet is validated server-side, so posting the form directly cannot
      name a crew sheet
- [x] A year with no patrol maps refuses registration and says so
- [x] The map id travels on the `registered` event, additively
- [x] The `qr` projection records it; codes registered before it read back as unknown
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 16:40 — Task created and picked up.
- 2026-09-09 16:45 — **Two dependency problems first.** `kort` needs
  `github.com/doug-martin/goqu/v9`, and it needs `types.CheckpointID`/`types.CheckgroupID`,
  which the pinned `shared-go` did not have — so `shared-go` had to be upgraded to
  `v0.0.0-20260907212133-4445c9538f2e`.
- 2026-09-09 16:50 — That upgrade broke `senior`: `messages.NathejkSeniorUpdated` **lost its
  `TeamID` field** upstream. Checked the stream before working around it — 1429 of 1430
  senior rows have a non-empty `teamId`, so publishers never stopped sending it; only the
  Go struct stopped describing it. Decoding into the new struct would have silently dropped
  a value that is present in the JSON, quietly emptying the klan link that gives a bandit
  its LOK label in `/geo`. Added `event.SeniorUpdated`, which embeds the shared body and
  restores the field. **This should be fixed upstream in shared-go** rather than carried
  here.
- 2026-09-09 16:55 — Wired `kort` onto the mux with a **nil publisher**: sheets and sets are
  drawn up in hq, and a scanner may only choose among them.
- 2026-09-09 17:05 — Hit the substantive design problem: `kort.Maps()` resolves each
  sheet's checkpoint list and handout post against the `checkpoint` and `checkgroup`
  projections, which **this service does not run** — it fails with
  `Table 'skan.checkpoint' doesn't exist`. Rejected two tempting fixes: creating those
  tables empty would let `Maps()` succeed while reporting every sheet as having no
  checkpoints, a lie the read model would then repeat; and editing `kort` is out, since like
  `photo` it must stay identical to its origin. Added `data.KortReader` instead, which asks
  the narrower question skan actually has — "which sheets may a patrulje be handed, and what
  are they called" — over the tables the projection maintains. `photocover` reading `photo`
  is the precedent.
- 2026-09-09 17:10 — `commands.QR.Register` now takes the map id, and publishes
  `event.QrRegistered` — the shared body plus `mapId`, additively, so consumers decoding
  into `messages.NathejkQrRegistered` are unaffected and older events read back as "unknown
  sheet" rather than "no sheet". The `qr` projection gained a `mapId` column.
- 2026-09-09 17:15 — ✅ Verified live against the real data. The picker lists exactly the
  five `Deltagerkort` sheets in handout order, with formats shown (`skitse` marked as such).
  A POST with no `mapId` → **424**; with a sheet id outside the spejder set → **424**; with
  a valid sheet → **303**, and `qr` row 4 now reads `mapId = kort-3b84b28b…` joining to
  *Deltagerkort 2*. Zero dead-letters through a full replay.
- 2026-09-09 17:18 — Added `nathejk/event/qr_test.go`: the map id is serialised as `mapId`
  at the top level, the embedded shared fields stay at the top level so existing consumers
  still decode, an unset map id is omitted rather than sent as `""`, and `SeniorUpdated`
  recovers `teamId`.
- 2026-09-09 17:20 — Completed. Not done, and worth a decision: the sheet is only recorded
  on the event and in `qr` — nothing yet *shows* a patrol's current sheet back to a scanner,
  and nothing consumes `handoutCheckgroupId`, which is how a sheet's checkpoints are meant
  to become visible. Both belong to whoever builds the scouts' view.
