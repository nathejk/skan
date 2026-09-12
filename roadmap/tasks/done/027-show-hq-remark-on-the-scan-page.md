# 027 — Show HQ's remark on the scan page

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** Zed agent
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

HQ can attach an operational note to a patrulje ("Info til banditter og postmandskab"). The
write side lives in `hq` and publishes `NATHEJK.{year}.patrulje.{teamId}.remark.set`; skan
ignored it, so the note reached nobody. A note that only HQ can see is not a note — the person
it is written for is the one standing in front of the scouts with a phone.

Two severities, and they must not look alike:

- **`information`** — worth knowing, changes nothing. A visible but subtle yellow box on the
  scan result.
- **`stop`** ("fuld stop") — the patrol must not be sent on. A red, in-your-face box above
  everything else on the page.

An **empty remark is disabled**, whatever the severity says. `inactive` is likewise off: HQ
stood the note down and kept the text so it can be reinstated without retyping.

Very few patrols will ever carry a note, so both severities are out of the ordinary and the
page must render exactly as before when there is none.

## What was done

- Copied hq's remark handling into `nathejk/table/patrulje`:
  - `messages.go` — the `RemarkSet` body and the three severity constants, so the decision is
    made against named values rather than string literals in a template. Skan only reads, so
    nothing here validates or publishes.
  - `consumer.go` — subscribes to `patrulje.*.remark.set` and overwrites both columns. The
    event restates the whole note, so there is no guard against an empty value: clearing a
    note is deliberate (hq's `ClearRemark`) and must not be ignored. **Quoted with
    `tables.Quote`, not hq's `%q`** — see `.rules`.
  - The case is matched *before* the four-part patterns, per the ordering hazard the
    `spejderstatus` consumer already ran into.
  - `table.sql` gains `remark`/`remarkSeverity`, plus a `schemaMigrations` list on the table:
    `CREATE TABLE IF NOT EXISTS` is a no-op on an existing database, and without the `ALTER`
    the consumer's `UPDATE` would fail with "Unknown column" on every note. That is not
    hypothetical — it is how hq learned the same lesson.
- `Patrulje.RemarkInForce()` / `RemarkStops()` hold the on/off rule, so it is written once and
  not re-derived in the handler and again in the template.
- `scanResultData` sets `remark` and `remarkStops` **only when the note is in force**, so
  nothing renders in the ordinary case. Shown to **both roles**: this is an instruction from
  HQ, not race progress, and a bandit needs "fuld stop" more than crew do — they are the ones
  who would otherwise send the patrol running again.
- `coordinates.html` renders the stop box above the head count and the information box below
  it, and `GetByID` selects the two columns.
- **The registration page carries it too** (`mapHandler`, `map.html`). Handing over a map is
  the other moment a scanner stands in front of the patrol, and a stop order reaching only the
  scan page means a patrol under one can be given their next map by someone who was never
  told. `addRemark` is shared by both handlers so the on/off rule is applied once.
  - The stop box sits **above the heading, outside every branch**: it is true whether the page
    is asking for a number, confirming a photograph, refusing a discontinued team or reporting
    a missing photograph. The information box stays inside the confirmation branch, where
    there is a patrol to say it about.
  - It deliberately **does not block the registration**. The sheet is already in the scouts'
    hands; refusing would leave its code unattributed as well, which is strictly worse. The
    box says "udlever kortet, men kontakt HQ med det samme".

## Verification

- `go build`, `go test`, `staticcheck`, `gofmt` clean.
- Two teams carry `remark.set` events on the dev stream. After a restart the projection folds
  them: one shows `inactive`/"mikkel har hjemve", the other is cleared to `""` (hq's
  `ClearRemark`), which is correct.
- Rendered `/qr/1/162726411` (team 2) as a bandit with the row forced to each severity in turn:
  `stop` → the red FULD STOP box with the text; `information` → "Info fra HQ:" in the yellow
  box; `inactive` → the text appears nowhere in the response. Same three states checked on
  `/map/1/162726411?number=2`, where the stop box renders above the heading and the
  "tilknyt kortet" button is still offered. Row restored afterwards.

## Notes for later

- The severity constants and `RemarkSet` are duplicated between hq and skan. They belong in
  `shared-go` with the rest of the projections; until then, a severity renamed in hq is a note
  silently not shown here.
