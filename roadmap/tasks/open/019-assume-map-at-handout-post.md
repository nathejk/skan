# 019 — Assume the map when the scanner mans a handout post

**Status:** open
**Priority:** medium
**Created:** 2026-09-10
**Picked up by:**
**Started:**
**Completed:**

**Blocked on:** three projections this service does not have — see below.

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

- [ ] `checkpoint`, `checkgroup` and `checkpersonnel` projections available and wired
- [ ] A scanner manning a handout post gets that post's sheet assumed, not a picker
- [ ] A scanner not manning such a post is still asked
- [ ] The assumed sheet is validated server-side exactly as a chosen one is
- [ ] The assumption is visible to the scanner rather than silent (pending Q3)
- [ ] `kort.Maps()` reconsidered now that its dependencies exist
- [ ] `go test ./...` and `staticcheck` pass in the container

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
