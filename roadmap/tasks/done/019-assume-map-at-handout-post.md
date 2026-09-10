# 019 — Assume the map when the scanner mans a handout post

**Status:** done
**Priority:** medium
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

**Resolved:** HQ copied in `checkpoint` and `checkpersonnel`, which turned out to be enough
— no `checkgroup` projection is needed, because `checkpoint.checkgroupId` is the only part
of a checkgroup this feature has to know. (`kort.Maps()` still cannot run, so
`data.KortReader` stays.)

## Description

HQ: if a scanner is active as checkpersonnel on a checkpoint belonging to a checkgroup
where maps are handed out, **assume that map**. Otherwise ask.

This removes the picker from the common case. A post that hands out sheet 3 hands out
sheet 3 all night, so making the crew member choose it from a list every time is friction
that invites the wrong choice — and the system already knows the answer.

The pieces on the `kort` side are ready: `kort.handoutCheckgroupId` is exactly "the
checkgroup whose post gives this sheet to the team", and `""` on that column already means
"handed over at the QR scan" rather than at a post.

## What is needed — exactly three projections

The chain skan has to walk is `scanner → checkpoint → checkgroup → sheet`:

```
login.User.ID (types.UserID)
   │  checkpersonnel: userId ↔ checkpointId (+ timeRange)
   ▼
checkpoint.checkgroupId
   │
   ▼
kort.handoutCheckgroupId  →  the sheet handed out at that post
```

### 1. `checkpersonnel`

The missing link, and the only one skan cannot fake. Needed to answer "which post is this
scanner manning".

| | |
|---|---|
| Subject | `NATHEJK.{year}.checkpersonnel.{id}.added` — **3 on the stream** (1×2026, 2×2025) |
| Bodies | `messages.NathejkCheckpersonnelAdded{UserID, CheckpointID, TimeRange}`, plus `…Removed{UserID, CheckpointID}` and `…TimeSpecified{Start, End}`, which exist as types but have **no events on the stream yet** |
| Must expose | given a `types.UserID`, the checkpoint(s) they are assigned to |

`UserID` lines up with `login.User.ID` for crew, since crew come from the `personnel`
projection — so no identity mapping is needed.

### 2. `checkpoint`

| | |
|---|---|
| Subjects | `NATHEJK.{year}.checkpoint.{id}.created` / `.updated` — **13 each** |
| Bodies | `NathejkCheckpointCreated{CheckpointID, CheckgroupID}`, `NathejkCheckpointUpdated{…}` |
| Must expose | a checkpoint's `checkgroupId` |

**The table must be named `checkpoint` with `id` and `year` columns.** `kort`'s querier
already runs `SELECT id FROM checkpoint WHERE (year = ? OR ? = '')`, so getting these names
right is what makes `kort.Maps()` work here.

### 3. `checkgroup`

| | |
|---|---|
| Subjects | `NATHEJK.{year}.checkgroup.{id}.created` / `.updated` — **8 each**, plus `NATHEJK.{year}.checkgroups.sorted` |
| Must expose | nothing beyond existing; skan only needs the id to compare against `kort.handoutCheckgroupId` |

**Table named `checkgroup` with `id` and `year`** — same reason: `kort` queries
`SELECT id FROM checkgroup WHERE (year = ? OR ? = '')`.

### Things to expect when they land

The last three copied packages each brought an integration snag, so worth checking up front:

- **A `table.sql` with more than one `CREATE TABLE`** needs `multiStatements=true` — already
  set in `DB_DSN` since `spejderstatus`.
- **Unused helpers in a copied test file** trip `staticcheck` U1000, which fails the prod
  build; the fix is a per-package `staticcheck.conf` with `checks = ["inherit", "-U1000"]`,
  not editing the file.
- **A newer `shared-go`** may be required; the last bump silently dropped a field skan
  depended on.
- Construct all three with a **nil publisher**: skan reads them, hq owns them.

### Once they are in

`data.KortReader` exists only because `kort.Maps()` could not run without these tables. It
should be reconsidered — probably deleted in favour of `kort.Maps()` plus `kort.Sets()`.

## Why this is blocked

The events exist on the stream — confirmed by listing subjects:

| Subject | Count |
|---|---|
| `checkpersonnel.added` | present |
| `checkpoint.created` / `checkpoint.updated` | present |
| `checkgroup.created` / `checkgroup.updated` | present |
| `NATHEJK.{year}.checkgroups.sorted` | present |

But skan has **no projection for any of them**, so it cannot answer "which checkpoint is
this scanner manning, and which checkgroup does it belong to". Three projections are
needed, and they belong to hq: the same situation as `kort` and `photo`, which HQ copied
in when skan needed them.

**So this needs `checkpoint`, `checkgroup` and `checkpersonnel` copied into
`nathejk/table/`** (or lifted into `shared-go`), after which the chain is short:

```
scanner (userId) --checkpersonnel--> checkpoint --> checkgroup
                                                      |
                        kort.handoutCheckgroupId <-----+
```

Do **not** work around it by inferring the post from something else. A wrong assumption
here binds a patrol's code to a sheet they were not given, which is worse than asking.

### Bonus: it also fixes an existing compromise

`data.KortReader` exists only because `kort.Maps()` needs the `checkpoint` and `checkgroup`
projections and fails with `Table 'skan.checkpoint' doesn't exist` (task 017). With those
projections present, `kort.Maps()` works and the narrow reader can probably go away.

## Open questions for HQ

1. **What makes a checkpersonnel assignment "current"?** `NathejkCheckpersonnelAdded`
   carries an optional `TimeRange`, and `Removed`/`TimeSpecified` exist as message types but
   have no events on the stream. If assignments in practice never end, "active as
   checkpersonnel" means "ever assigned", and a crew member who manned a post last year would
   still match — so the read must at least be year-scoped, and probably time-scoped too.
2. **What if the scanner mans a post that hands out more than one sheet?** Several sheets may
   name the same `handoutCheckgroupId`. Assume the first in handout order, or fall back to
   asking?
3. **Should an assumed sheet still be shown for confirmation**, or silently applied? The
   photo confirmation is already on that screen, so showing "Kort: Deltagerkort 2" beside it
   costs nothing and keeps the scanner able to catch a wrong assumption.

## Acceptance Criteria

- [x] `checkpoint` and `checkpersonnel` projections wired (no `checkgroup` needed)
- [x] A scanner manning a handout post gets that post's sheet preselected
- [x] A scanner not manning such a post is still asked
- [x] The assumed sheet is validated server-side exactly as a chosen one is
- [x] The assumption is visible to the scanner rather than silent
- [ ] `kort.Maps()` reconsidered — **still not possible**, it also needs a `checkgroup` table
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-10 07:45 — Filed rather than started. The `kort` half is ready and the events
  exist, but skan has no checkpoint, checkgroup or checkpersonnel projection, so there is no
  way to know which post a scanner is manning. Listed the stream's subjects to confirm the
  events are there before concluding this is a missing-projection problem rather than a
  missing-event one.
- 2026-09-10 08:50 — HQ offered to copy the projections in, so wrote up exactly which three
  and what they must expose, including the **table names `checkpoint` and `checkgroup` with
  `id` and `year` columns** that `kort`'s querier already assumes. Counted the events on the
  stream: 13 checkpoints, 8 checkgroups, and only **3** checkpersonnel assignments — enough
  to verify the feature, but thin enough that a bug would be easy to miss.
- 2026-09-10 08:52 — Sharpened question 1 after reading the message types:
  `CheckpersonnelAdded` carries an optional `TimeRange`, and `Removed`/`TimeSpecified` exist
  but have never been published. So "active as checkpersonnel" currently has no end condition
  in the data, which is the one thing that could make this feature assume a sheet for someone
  who is not at that post tonight.
- 2026-09-10 09:00 — Picked up: HQ copied in `checkpoint` and `checkpersonnel`. Two
  useful surprises. `checkpersonnel` has `startUts`/`endUts` columns, so the shift **is**
  expressible even though no event has yet set one; and no `checkgroup` projection is needed,
  because the only thing this feature wants from a checkgroup is its id, which
  `checkpoint.checkgroupId` already carries.
- 2026-09-10 09:02 — The copied packages needed `nathejk.dk/internal/requestctx`, an hq
  package that does not exist here — their command sides stamp `Metadata{UserID}`. Wrote a
  minimal one. It deliberately does **not** reuse `login.User`: `internal/login` →
  `internal/data` → projection packages → `requestctx` would be an import cycle.
- 2026-09-10 09:05 — Did the join in `data.KortReader` rather than walking the chain in Go.
  The copied queriers cannot answer the question anyway — `checkpersonnel.Filter` has no user
  field — so going through them would mean reading every assignment and filtering here, and
  the chain is three tables on a page a scanner is waiting for.
- 2026-09-10 09:06 — Answered the three open questions in the cautious direction, since every
  wrong answer binds a patrol's code to a map they were not given:
  **Q1** the shift is honoured when set (`0` means unbounded, which is how every assignment
  currently looks) and the read is year-scoped regardless, so last year's roster cannot match
  tonight;
  **Q2** more than one candidate sheet means no suggestion — `LIMIT 2` so "exactly one" can be
  told from "several" — and the scanner is asked as before;
  **Q3** the sheet is **preselected and labelled** ("Foreslået ud fra posten du står på")
  rather than applied silently. That is one tap saved, still visible, still correctable. A
  preselected value a scanner cannot account for would be worse than none.
- 2026-09-10 09:10 — ✅ Verified. With real data there is correctly **no** suggestion: the one
  2026 assignment is at "Afgang", whose checkgroup no sheet hands out, and its shift is 19
  September. Injected an assignment putting the test crew user on "Post 1A", whose checkgroup
  hands out *Skitse CP2*: the picker came back with that sheet `selected` and the reason shown.
  Then both negative paths, each returning to a plain picker: a shift outside the current time,
  and a second sheet sharing the post's checkgroup.
- 2026-09-10 09:12 — **Found a defect in the copied packages, left unfixed by policy.** Both
  consumers use plain `INSERT`, so every replay re-inserts rows that already exist and
  dead-letters them: **16 per boot** here (13 checkpoints + 3 assignments). The data stays
  correct — the first insert won and the values do not change — but it breaks the
  `cqrs.Consumer` idempotency contract, which exists precisely because projections are
  rebuilt by replay on every start. The fix upstream is `ON DUPLICATE KEY UPDATE` (or
  `INSERT IGNORE` if a created event never restates anything). They also build SQL with `%q`,
  which task 013 removed everywhere else in this repo. **Reported rather than patched**, since
  a copied package must stay identical to its origin — but note it costs the "zero
  dead-letters" health signal every other projection here upholds.
- 2026-09-10 09:12 — Completed. Cleaned up the injected rows and confirmed by replay that the
  remaining state is stream-derived.
- 2026-09-10 09:15 — HQ: the projections may be edited after all, so the defect above is now
  fixed rather than only reported. Both create/add handlers are upserts
  (`ON DUPLICATE KEY UPDATE`, built with goqu, which also retired their `%q` quoting — the
  thing task 013 removed everywhere else). Two details worth keeping:
  — the update list on `checkpoint.created` is only the columns that event carries, because
  `.updated` owns name, address, position and the open times and arrives after it on replay;
  restating them would undo it.
  — `checkpersonnel.added` only writes `startUts`/`endUts` when the event actually carries a
  range, since `.timespecified` sets them separately and `0` means "unbounded" to every
  reader.
  Also changed two `return nil`-on-error slips to `return err`: a statement the database
  refuses is exactly what the dead-letter writer exists to record.
- 2026-09-10 09:18 — ✅ Verified the fix the only way that means anything: booted twice
  **without** dropping the tables. First boot 0 dead-letters, second boot — the one that used
  to produce 16 — also **0**, with 13 checkpoints and 3 assignments intact. Re-checked the
  suggestion still works afterwards, and cleaned up the injected assignment.
- 2026-09-10 09:18 — Noted in `.rules` that these two packages now **diverge from hq's
  copies**, so re-copying them would silently revert the fix, and added idempotency to the
  projection conventions. The fix should go upstream.
- 2026-09-10 11:45 — HQ: the two arrivals on the registration page need different
  descriptions. They were sharing them, and worse than sharing — **contradicting**: a code
  from a discontinued patrulje was introduced as "udgået af løbet" and then, two lines later,
  as one that "har ikke været scannet før". Both sentences were on screen at once, because
  the reassign notice was printed above a number-entry branch written for unused codes.
- 2026-09-10 11:46 — Split them properly. An unused code: "Denne QR-kode er ikke tilknyttet
  en patrulje endnu" — which also drops the older claim that it "har ikke været scannet før",
  untrue on its face since scanning it is what brought the scanner to the page. A
  discontinued one: "Patruljen der havde dette kort er udgået af løbet", asking for the
  **holdnummer** of whoever has it now rather than a patruljenummer, and confirming with
  "Ja, de har kortet nu – flyt kortet" instead of "tilknyt kortet" — it is a hand-over, not a
  first registration.
- 2026-09-10 11:47 — Found a second bug while in there: the "det er en anden patrulje" and
  "prøv et andet patruljenummer" links pointed at bare `?`, dropping `reassign=1`. Going back
  therefore turned a hand-over into a first-time registration — losing the explanation *and*
  the sheet the scouts already carry, so the scanner would have been asked to pick a sheet
  again. Both links and the number form now carry it.
- 2026-09-10 11:50 — ✅ Verified live, both codes side by side: sticker 6 (never registered)
  gives "Tilknyt patrulje" → "ikke tilknyttet en patrulje endnu" → "Indtast
  patruljenummeret"; sticker 4 (bound to team 1, `STARTED` with 0 active) gives "Hvem har
  kortet nu?" → "udgået af løbet" → "Indtast holdnummeret". With a team chosen, the reassign
  path carries `mapId`, says "flyt kortet", and its back-link keeps `?reassign=1`. Tests
  assert each description is absent from the other page.
