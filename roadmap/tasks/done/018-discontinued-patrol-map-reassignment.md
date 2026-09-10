# 018 — A discontinued patrol's map may have moved to another team

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: if a QR code is scanned but the patrol it belongs to is discontinued, the map may
have been handed on. One member quits, the other two are reassigned to a new team, and
they bring their old map along. In that case the scanner should be asked to enter the
**new team number**.

Asking rather than following the merge automatically is the right call: the remaining
scouts may have been split across more than one team, and only the person holding the map
can say who has it.

## What "discontinued" is, in this system

Not a signup status. The stream carries **no** `patrulje.*.status.changed` events at all —
only `signedup`, `updated`, `numberassigned` and `started` (verified by listing the
stream's subjects). `klan.*.status.changed` does exist, which is what makes the absence
easy to misread.

It is a **merge**: `NathejkTeamMerged{TeamID, ParentTeamID}`. The pre-012 code agreed —
`GetDiscontinuedTeamIDs` read a `patruljemerged` table. So `patrulje` gained a
`mergedIntoTeamId` column, set from `patrulje.*.merged`, and `Patrulje.Discontinued()` is
the one place that rule is written.

**No merge events exist on this stream yet**, so the projection side is correct but
dormant, and the flow could not be verified from real data. It was verified by injecting
the state instead — see the log.

## Acceptance Criteria

- [x] `patrulje` records a merge, and `Discontinued()` states the rule once
- [x] Scanning a discontinued patrol's code asks who holds the map now, in Danish,
      instead of recording a scan against a team that has left the race
- [x] The scanner enters the new team number and confirms against that team's photograph
- [x] The map sheet travels with the scouts rather than being chosen again
- [x] Re-binding a code actually takes effect in the read model
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-10 07:05 — Created and picked up, together with the senior `teamId` fix HQ
  explained alongside it.
- 2026-09-10 07:10 — **The senior fix turned out to be a live bug, not just a missing
  field.** HQ explained that only the first `senior.updated` carries `teamId` — that event
  is really "senior added" — and a senior may not change klan, so later events omit it. But
  the consumer folded it with `teamId=VALUES(teamId)`, so *any* later edit of a name or
  t-shirt size would have overwritten a real klan with `""`, silently costing every bandit
  in that klan its LOK label. Now `teamId=IF(VALUES(teamId) = '', teamId, VALUES(teamId))`:
  set once, never cleared. Only 1 of 1430 seniors has no klan, so this had not yet bitten
  widely.
- 2026-09-10 07:20 — Went looking for the discontinued signal and guessed wrong twice
  (`patrulje.*.status.changed`, then `patrulje.*.merged`), each time replaying and finding
  nothing. Stopped guessing and listed the stream's subjects with `nats stream subjects`,
  which settled it: 29,428 messages, no merge or patrol-status subjects at all. That should
  have been the first move — it would also have saved the earlier `kort.Maps()` detour.
- 2026-09-10 07:25 — Kept the `merged` subscription and the `mergedIntoTeamId` column even
  though nothing publishes them yet: it is the signal both shared-go and the pre-012 code
  agree on, and a dormant-but-correct fold costs nothing. Recorded as its own column rather
  than folded into `signupStatus`, which `.started` also writes — the two facts are
  independent, and a merged team that had already started would otherwise lose one.
- 2026-09-10 07:30 — **Reversed a decision from 014.** That task chose `INSERT IGNORE` for
  `qr` so the first binding within a year wins, reasoning that a later scanner should not be
  able to re-point a map already in play. This scenario shows that was incomplete: the same
  physical sheet genuinely changes hands, and refusing the second binding would leave every
  later scan credited to a team that has left the race. Now `ON DUPLICATE KEY UPDATE`, with
  the protection moved to where it can judge — the photo confirmation, and re-binding only
  being offered when the team is actually discontinued. An empty `mapId` does not erase a
  known one, the same rule as `senior.teamId`.
- 2026-09-10 07:32 — The re-bind reuses the registration flow rather than adding a second
  one: `scanHandler` redirects to `/map/{id}/{cs}?reassign=1`, which explains why in Danish,
  asks only for the new team number, and carries the existing sheet in a hidden field. A
  carried sheet is accepted without re-checking it against the current spejder set —
  otherwise a sheet since retired from the set would refuse a legitimate hand-over.
- 2026-09-10 07:35 — ✅ Verified by injecting the state the stream lacks: marked team 2 as
  merged, then scanned its sticker. The scan page redirected to **"Hvem har kortet nu?"**
  with "Patruljen der havde dette kort er udgået af løbet"; the re-bind page asked only for
  the team number and carried `mapId` in a hidden input with no picker; submitting moved
  sticker 4 from team 2 to team 1 keeping the same sheet, with a fresh `mapCreatedAt`.
- 2026-09-10 07:40 — Worth recording, because it surprised me: a replay **upserts** and does
  not truncate, so my injected row survived a restart and only disappeared once the table was
  dropped. The read model is disposable, but only if you actually drop it.
- 2026-09-10 07:40 — Completed. Zero dead-letters, 719 patruljer, and the re-binding persists
  from the real event rather than from my edit.
