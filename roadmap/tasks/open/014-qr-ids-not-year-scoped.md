# 014 — QR ids are not year-scoped

**Status:** open
**Priority:** high
**Created:** 2026-09-09
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A QR id from a previous year cannot resolve to that year's patrulje
- [ ] The `qr` projection is year-scoped, with the year taken from the message subject
- [ ] `qr.GetByID` (and its `data.Models` interface) is year-aware
- [ ] Registering a code in the current year works even if the same id exists for an
      earlier year
- [ ] A re-registration attempt within the same year behaves per an explicit decision,
      recorded in the progress log
- [ ] Existing dev databases can be rebuilt from the log (drop the table; the read model
      is disposable)
- [ ] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-09 08:40 — Found while verifying 006's registration gate: a POST with the
  correct photo ref published `registered` and returned 303, but the `qr` row still
  showed 2025's `teamNumber=33`, because `INSERT IGNORE` on a year-less primary key kept
  the old binding. Filed rather than fixed inside 006, since it needs a schema and key
  change plus a year-aware read, and it deserves its own verification.
