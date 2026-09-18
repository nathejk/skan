# 028 — Tell postmandskab whether a patrol is on time

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** zed agent
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

A scanner manning a post has to decide, right there in the dark, whether the patrol in
front of them is running to time. Nothing on the scan page says. The plan already holds
the answer — `checkpoint` carries both kinds of opening hours hq can configure — but skan
read that table only to guess a map sheet (task 019).

Two kinds of post, and they mean different things by "on time":

- **Static (fixed) hours** — `openFromUts`/`openUntilUts`. The post closes at a wall-clock
  time, so what the scanner needs is **minutes until close**.
- **Relative hours** — `openDuration`, stored in **minutes** (`checkpoint/consumer.go`
  divides `RelativeTimeDuration` by `time.Minute`). The allowance runs from the previous
  post, so what the scanner needs is **minutes since the last checkpoint**, against that
  allowance.

Both get a plain green or red marker, because that is the part a scanner reads at a glance.

Scope and constraints:

- Only for a scanner who is **postmandskab active at a checkpoint** — i.e. has a
  `checkpersonnel` row for this year whose shift covers now. There is no `postmandskab`
  userType to check (`personnel.userType` is `gøgler`/`friend` in real data); manning a
  post *is* the qualification, and it is the only thing that makes the question answerable
  at all, since the verdict is about *this* post.
- **Crew-only** by the fair-game rule: it is derived from checkpoint activity and from the
  patrol's previous checkpoint scan. A bandit must not be told it, and must not have it
  queried for them.
- No verdict is better than a guessed one. A post with neither kind of hours configured, a
  fixed range that does not make sense (real data has `Post 1A` with `openUntil` *before*
  `openFrom`), or a relative post the patrol has no previous checkpoint scan for → show
  nothing.

## Acceptance Criteria

- [x] A `data.CheckpointInterface` read: the post this scanner mans now, and the patrol's
      previous checkpoint scan. Narrow, year-scoped, and not through the copied package's
      own querier.
- [x] A pure, tested function turning (post, previous scan, now) into a verdict or nothing.
- [x] Fixed-hours post: minutes until close, green while open, red once closed.
- [x] Relative-hours post: minutes since the last checkpoint, green within the allowance,
      red beyond it.
- [x] Nothing rendered, and nothing queried, for a bandit.
- [x] Nothing rendered when the answer is unknown (no hours, nonsense range, no previous
      checkpoint scan).
- [x] `go build`, `go test`, `staticcheck`, `govulncheck` all pass.

## Progress Log

- 2026-09-15 10:00 — Task created. Plan: `internal/data/checkpoint.go` for the two reads,
  `checkpoints.go` in the main package for the verdict, wire into `scanHandler` and
  `coordinates.html`.
- 2026-09-15 10:25 — `internal/data/checkpoint.go` written. `PostForScanner` mirrors
  `KortReader.SheetForScanner` (same shift-bounds handling, LIMIT 2 so "exactly one" is
  distinguishable from "several"); `PreviousCheckpointScan` uses EXISTS rather than a JOIN,
  because a scanner rostered on several posts across the night would otherwise return the
  same scan once per roster row — the same trap `CountCatchesByTeam` documents for `senior`.
- 2026-09-15 10:40 — Decided the previous-scan clock is the previous *checkpoint* scan, with
  this post excluded. A bandit catching the patrol en route does not restart their allowance,
  and without the exclusion the colleague at the same post (or a confirmed rescan) would make
  every patrol arrive in 0 minutes.
- 2026-09-15 10:55 — `Post.HasFixedHours()` requires the range to make *sense*, not just to be
  present: real data has `Post 1A` with `openUntil` before `openFrom`, which would have shown
  a confident red marker to everyone on that post all night.
- 2026-09-15 11:10 — ✅ Criteria 2–4, 6: `postTimeliness` in `checkpoints.go` plus a table test.
  Fixed hours win over relative when both are set (the wall-clock deadline is what the field is
  held to); before opening is on time; minutes are always a magnitude with the sign in `OnTime`;
  rounded, not truncated, so "0 minutter for sent" cannot appear next to a red marker.
- 2026-09-15 11:25 — Template data carries **one** key holding the struct, after `{{ if ne
  .onTime nil }}` turned out to be unwritable (comparing a bool to nil is a template error).
  A single absent key is the same shape as the remark, and `{{ with .timeliness }}` works
  because a missing map key is nil and any struct value is truthy.
- 2026-09-15 11:35 — ✅ Criterion 5: `scanTimeliness` refuses before either read for a bandit,
  and the stub in the test fails if it is queried at all — task 023's lesson. It also skips
  the previous-scan read entirely for a fixed-hours post, which nothing would use.
- 2026-09-15 11:50 — ✅ Criterion 7: `gofmt`, `go build`, `go test`, `staticcheck`,
  `govulncheck` all clean in the api container.
- 2026-09-15 12:15 — ✅ Criterion 1, verified live against the running service and real data.
  Temporarily made the one 2026 `checkpersonnel` shift unbounded and reshaped `Afgang`'s hours;
  scanning `/qr/7/165872145` as crew rendered, in turn, `panel-success` "TIL TIDEN — Posten
  lukker om 42 min.", `panel-danger` "OVERTID — Posten lukkede for 12 min. siden", and
  `panel-danger` "4266 min. siden sidste post — der er afsat 30 min." (the patrol's real
  previous checkpoint scan, four days old). The same URL as a bandit rendered no marker and no
  wording. All edited rows restored to their original values afterwards.
- 2026-09-15 12:20 — All criteria met. Moving to done.
- 2026-09-15 12:35 — Scope confirmed with the requester: **the scan page only.** I had raised
  the registration page (`map.html`) as a possible extension, since a handout post is often
  also a timing post — rejected. Handing over a map is not an arrival to be judged, and the
  registration screen already has to carry the sheet picker, the photo confirmation and HQ's
  remark; a fourth thing competing for the same glance in the dark makes the ones that block
  the handover easier to miss. Do not add the marker there. No code change: `addTimeliness` is
  called from `scanHandler` alone, and the markup exists only in `coordinates.html`.
