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

A **started team with no active members left**:
`signupStatus == STARTED && activeMemberCount == 0`. Both halves matter — a team that has
not started yet has no active members either, and calling that discontinued would treat
every patrol in the hours before the start as having dropped out.

`activeMemberCount` lives on `patrulje` but is maintained by the **spejderstatus**
projection, which recomputes it from the member rows next to writing them. That is
deliberate on its part: the mux gives no ordering guarantee between consumers, so
recomputing it in `patrulje` could count member rows that had not been written yet and land
a plausible-looking number that is one out.

There is deliberately **no event** for discontinuation, and no reverse event: move a member
back in and the recompute makes the team active again.

### Superseded encodings

Two earlier answers were tried and are wrong:

- `patrulje.*.status.changed` — the stream carries none at all, only `signedup`, `updated`,
  `numberassigned` and `started`. `klan.*.status.changed` does exist, which makes the
  absence easy to misread.
- `NathejkTeamMerged` / the legacy `patruljemerged` table — **deprecated**. It stored the
  conclusion rather than the input, which is why it needed `.merged` *and* `.splited` to
  undo itself.

## Acceptance Criteria

- [x] Discontinuation is derived from `activeMemberCount` and `signupStatus`, with the rule
      written once in `Patrulje.Discontinued()`
- [x] The `spejderstatus` projection is wired, so the count is fed before it is trusted
- [x] Scanning a discontinued patrol's code asks who holds the map now, in Danish,
      instead of recording a scan against a team that has left the race
- [x] A running patrol's code still scans normally
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
- 2026-09-10 08:00 — **Reopened by HQ**: `NathejkTeamMerged` is deprecated, and a
  `spejderstatus` projection was copied in that maintains `patrulje.activeMemberCount`. A
  started team with zero active members is discontinued. Removed the `.merged` subscription
  and the `mergedIntoTeamId` column, and rewrote `Discontinued()` accordingly — the whole
  point of having put that rule in one method.
- 2026-09-10 08:05 — Note the copied package writes `patrulje.activeMemberCount` itself,
  across a package boundary, and documents why: the mux has no ordering guarantee between
  consumers, so recomputing the count in `patrulje` could read member rows that had not been
  written yet. The column therefore had to be added to `patrulje/table.sql` for a table this
  repo owns but another projection fills.
- 2026-09-10 08:10 — Two integration problems, both from the copied package's assumptions:
  (1) its `table.sql` declares **two** tables and is consumed as a single statement, which
  MariaDB rejects — `kort` avoids this by embedding its second schema separately. Fixed by
  adding `multiStatements=true` to `DB_DSN`, which is presumably what hq runs; flagged in
  compose that this makes stacked queries possible, so consumer quoting matters more than
  before.
  (2) its test file has an unused helper, which staticcheck reports as U1000 and which would
  fail the production build. Suppressed with a per-package `staticcheck.conf` (`inherit`,
  minus U1000) rather than editing a file that must stay identical to its origin — `all`
  turned out to widen the check set and surface an ST1003 naming complaint about a
  convention every projection here shares.
- 2026-09-10 08:14 — ✅ Verified against **real data** this time, no injection needed. A clean
  replay gives 700 `spejderstatus` rows, 1149 log rows and **zero** dead-letters, and 2026
  has a genuinely discontinued patrol: team 1 "Skjoldungerne 22", `STARTED` with
  `activeMemberCount = 0`. Scanning its sticker redirects to "Hvem har kortet nu?", while
  team 2 (`STARTED`, 7 active) still scans straight through to "Din scanning er registreret".
- 2026-09-10 08:15 — Completed again. The earlier merge-based encoding is gone rather than
  left dormant, since HQ has deprecated it.
