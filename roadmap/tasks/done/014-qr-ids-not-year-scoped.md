# 014 — QR ids are not year-scoped

**Status:** done
**Priority:** high
**Created:** 2026-09-09
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

## Description

The `qr` projection has **no `year` column**, and its primary key is `id` alone:

```sql
CREATE TABLE IF NOT EXISTS qr (
  id INT(10) UNSIGNED NOT NULL,
  teamNumber INT(10) unsigned DEFAULT NULL,
  ...
  PRIMARY KEY (`id`)
)
```

QR ids are plain incrementing integers handed out by `/qr?n=N`, starting at 1 every
time. So the sticker numbered 1 printed for 2026 collides with the sticker numbered 1
printed for 2025. Combined with the consumer's `INSERT IGNORE`, **the older binding
wins and the new one is silently discarded**.

Observed live: `qr` id 1 holds `teamNumber=33, mapCreatedAt=2025-09-19`. Publishing
`NATHEJK.2026.qr.1.registered` for a different patrol changed nothing — the projection
ignored it without error. Every subsequent scan of that 2026 sticker would be
attributed to **2025's team 33**.

This is the same class of bug as the one just fixed in `patrulje.GetByNumber` (which
resolved arm numbers without a year filter and returned the previous year's patrol),
and it is the more dangerous half, because it silently misattributes scans rather than
just failing to find a photograph.

## Options

1. **Add `year` to the table and to the key** — `PRIMARY KEY (year, id)` — and take the
   year from the message subject (not from `YEAR`, so replays of older years stay
   correctly labelled). `qr.GetByID` then needs the year too, like
   `patrulje.GetByNumber` now does.
2. Make the id itself year-unique when stickers are generated (e.g. prefix or offset
   per year). Cheaper, but leaves a schema that lies about its own key, and every
   printed sticker for the current year is already numbered from 1.

**Recommendation: (1).**

Note `INSERT IGNORE` also means a *correction* to a binding is silently dropped. Decide
deliberately whether re-registering a code should be possible; if the binding is meant
to be permanent, an ignored duplicate should at least be visible rather than silent.

## Acceptance Criteria

- [x] A QR id from a previous year cannot resolve to that year's patrulje
- [x] The `qr` projection is year-scoped, with the year taken from the message subject
- [x] `qr.GetByID` (and its `data.Models` interface) is year-aware
- [x] Registering a code in the current year works even if the same id exists for an
      earlier year
- [x] A re-registration attempt within the same year behaves per an explicit decision,
      recorded in the progress log
- [x] Existing dev databases can be rebuilt from the log (drop the table; the read model
      is disposable)
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 08:40 — Found while verifying 006's registration gate: a POST with the
  correct photo ref published `registered` and returned 303, but the `qr` row still
  showed 2025's `teamNumber=33`, because `INSERT IGNORE` on a year-less primary key kept
  the old binding. Filed rather than fixed inside 006, since it needs a schema and key
  change plus a year-aware read, and it deserves its own verification.
- 2026-09-09 08:55 — HQ confirmed and added the decisive detail: **the printed stickers
  may be reused from year to year.** That rules out option (2) (making ids year-unique at
  generation time) — a reused sticker keeps its number, so the year has to be part of the
  key. Took option (1).
- 2026-09-09 09:00 — Implemented: `year` column added with `PRIMARY KEY (year, id)`, the
  consumer takes the year from the **message subject** (not `YEAR`, so replaying an older
  year keeps its own label), and `GetByID` plus `data.QrInterface` now require the year.
  `scanHandler` passes `a.config.year`.
- 2026-09-09 09:00 — Kept `INSERT IGNORE`, deliberately: within a year the first binding
  wins, so a later scanner cannot silently re-point a map that is already in play.
  Correcting a mis-registration stays an HQ job. This is safer than last-write-wins now
  that 006 makes a wrong binding much harder to create in the first place, but it does
  mean a genuine mistake needs intervention — flagging in case HQ wants the opposite.
- 2026-09-09 09:05 — Hit a self-inflicted confusion worth recording: 680 statements
  dead-lettered with `Unknown column 'year'`. Cause was the edit landing before the table
  was dropped — `air` restarted the new binary against the old schema, and
  `CREATE TABLE IF NOT EXISTS` never alters an existing table. Dropping `qr` and
  restarting gave a clean replay with **0** dead-letters. Also noted that the boot log
  line overstates things: `deadletter.Count()` counts every unresolved row ever, not just
  this replay's.
- 2026-09-09 09:10 — ✅ Verified end to end against real data. Sticker id 1 is bound to
  team 33 in 2025; `GET /qr/1/<cs>` under `YEAR=2026` now redirects to the registration
  page and reports "ikke været scannet før", and registering it to 2026's team 1 leaves
  **both** rows in place: `(2025, 1, 33)` and `(2026, 1, 1)`. Before this change the 2026
  registration was silently discarded and every scan of that reused sticker would have
  been credited to a patrol from the previous race.
- 2026-09-09 09:10 — Correction to an earlier claim in 006: the POST verification there
  used `wget --max-redirect=0`, which BusyBox wget does not support, so that command had
  errored rather than succeeded and I misread the empty output as a pass. Re-run properly
  here; the refusal paths (424) were genuinely verified, but the success path was not
  until now.
- 2026-09-09 09:12 — Completed. One side finding filed as 015: the post-registration
  redirect races the projection and can bounce the scanner back to the registration page.
