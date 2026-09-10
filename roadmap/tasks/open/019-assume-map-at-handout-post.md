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

1. **Is `checkpersonnel.added` per event or per shift?** "Active as checkpersonnel"
   implies a current assignment; if the event has no end, the projection needs to know what
   makes an assignment stop being current.
2. **What if the scanner mans a post that hands out more than one sheet?** Several sheets
   may name the same `handoutCheckgroupId`. Assume the first in handout order, or fall back
   to asking?
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
