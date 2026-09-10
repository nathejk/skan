# 020 — Show the expected head count, and warn when strength has changed

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: when a patrol is scanned it is **very important** to show how many members are
expected, because the scanner has to confirm that the number actually in front of them
matches what is registered. If strength differs from the number the team started with,
show a small yellow marking saying *be aware*.

### Why strength moves in both directions

A patrol that drops below **three** may not continue on its own: its remaining members are
reassigned to other teams. So a team can end up **larger** than it started, not only
smaller — which is why "expected" has to be current strength (`activeMemberCount`) rather
than the start count.

This also explains something spotted while wiring `spejderstatus` in 018 and initially
mistaken for a bug: team 2 shows `memberCount 4` and `activeMemberCount 7`. That is a team
that took in scouts from a dissolved one.

### Why the warning matters more than it looks

The **armband the scouts wear encodes the number they started with** — `armNumber` is
`teamNumber-memberCount`. Once strength changes, the armband and reality disagree
permanently. A scanner counting heads against the armband would either think something is
wrong when it is not, or — worse — fail to notice that it is. The yellow note says which of
the two is happening.

### Visible to both roles

The head count is **not** race progress, so it is shown to bandits as well as crew. A
bandit is supposed to have caught the whole patrol, so they need it at least as much. This
is the same category as the patrol's name and photograph.

## Acceptance Criteria

- [x] The scan result shows the expected number of scouts prominently
- [x] "Expected" is current strength, not the number the team started with
- [x] A difference between the two shows a yellow *vær opmærksom* note
- [x] The note says whether the team grew or shrank, and that the armband is now out of date
- [x] Danish singular/plural is correct (`1 spejder`, `2 spejdere`)
- [x] Shown to bandits as well as crew, without leaking the crew-only scan count
- [x] Also shown on the registration screen, where the scanner is equally face to face
      with the patrol
- [x] `go test ./...` and `staticcheck` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-10 08:30 — Created and picked up. Both numbers were already on `Patrulje` —
  `MemberCount` from the `started` roster and `ActiveMemberCount` maintained by
  `spejderstatus` — so this is a presentation change rather than new data.
- 2026-09-10 08:35 — Put the count in `scanResultData` next to the fair-game split, with a
  `countChanged` flag rather than letting the template compare the two numbers itself.
  Keeping the comparison in Go means the rule is testable and stated once, and the template
  stays a template.
- 2026-09-10 08:36 — Placed the count **above** the confirmation text, in a panel of its own:
  it is the one thing on that page a scanner can act on, and burying it under "din scanning
  er registreret" would invite skipping it. The note names the direction, because "grew" and
  "shrank" have different causes and different follow-up.
- 2026-09-10 08:40 — Test-harness detail worth keeping: the assertions now strip HTML before
  matching, since `Der skal være <strong>7</strong> spejdere` is one sentence to a reader and
  three fragments to `strings.Contains`. Asserting on visible text rather than markup is also
  what makes these tests survive styling changes.
- 2026-09-10 08:42 — ✅ Verified live against real data as a **bandit**, on team 2, which
  genuinely started with 4 and now has 7: "Der skal være 7 spejdere. Tæl dem, og kontakt HQ
  hvis tallet ikke passer" followed by "Vær opmærksom: holdet startede med 4 spejdere, men er
  nu 7." Tests cover unchanged, grown, shrunk, the singular, and that a bandit gets the count
  while still not getting the crew-only scan total.
- 2026-09-10 08:42 — Completed. Not done, and worth a thought: nothing records the scanner's
  answer. The page asks them to count and to contact HQ if it does not match, but if the
  discrepancy is real, the only trace is a phone call — no event says "counted 6, expected 7".
