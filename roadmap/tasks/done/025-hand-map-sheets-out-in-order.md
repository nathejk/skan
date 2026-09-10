# 025 — Hand map sheets out in order

**Status:** done
**Priority:** high
**Created:** 2026-09-10
**Picked up by:** Zed agent
**Started:** 2026-09-10
**Completed:** 2026-09-10

## Description

HQ: a patrol receives its maps **in order**. If they have no code registered for
Deltagerkort 1, they cannot be given Deltagerkort 2. Disable the sheets out of reach, and
default the dropdown to the first sheet they can actually be given.

This matters because each sheet reveals the next stretch of the route. Handing over sheet 3
early shows the scouts checkpoints they have not earned — the same class of mistake as giving
them a crew sheet (task 017), but from a direction the set filter cannot catch.

### The rule

A sheet is reachable if the patrol **already holds it**, or it is the **first one they do
not** hold. The default is that first missing sheet.

Held sheets stay selectable on purpose. A map gets torn, soaked or lost, and the replacement
carries a new sticker for the same sheet; refusing it would leave a scanner unable to record
a handover that really happened. It also keeps a gap recoverable: a patrol somehow holding 1
and 3 has {1, 2, 3} in reach with **2** as the default, rather than a dead end.

### Disabled, not hidden

Sheets out of reach stay in the list, greyed. A scanner looking for "Deltagerkort 3" needs to
see that it exists and is not due yet; a list that silently omits it looks broken, and the
scanner's next move is to phone HQ about a bug that isn't one.

## Acceptance criteria

- [x] The picker disables sheets beyond the patrol's next one.
- [x] The sheet that is due is preselected.
- [x] Already-handed-out sheets are selectable and marked as such.
- [x] `POST` refuses an out-of-order sheet and names the one that is due.
- [x] A patrol holding every sheet gets no default and an explanation.
- [x] A re-bind (a map moving with reassigned scouts) is unaffected.

## Progress Log

- 2026-09-10 16:20 — Needed a read that did not exist: which sheets a patrulje already holds.
  Added `MapIDsByTeamNumber` to the `qr` projection's querier — keyed on `teamNumber` because
  that is all `qr` stores; it never learns a team's id. Rows with an empty `mapId` are skipped:
  codes only ever *found*, and codes registered before the sheet was recorded, say nothing
  about what a patrol holds.
- 2026-09-10 16:30 — Put the rule in one pure function, `sheetsInReach`, returning both the
  options and the default. `sheetReachable` asks it about a single id for the submit check, so
  the page and the server cannot drift: a `disabled` attribute is a hint to a browser, and this
  form can be posted without one.
- 2026-09-10 16:35 — The refusal names the sheet that *is* due ("Patruljen mangler Deltagerkort
  2 først"). "Wrong sheet" without "this one instead" leaves a scanner guessing in the dark
  with scouts waiting.
- 2026-09-10 16:38 — A carried sheet on a re-bind skips the order check. The receiving team
  may well not hold the earlier sheets, and refusing would strand a map the scouts are already
  carrying — the very situation 018 exists to handle.
- 2026-09-10 16:40 — Made the sequence default win over task 019's handout-post suggestion,
  which is now shown only when the two agree. Two competing preselections on one form is how a
  scanner ends up recording the sheet the page chose rather than the one in their hand. (019 is
  already unreachable for a separate reason — see 024.)
- 2026-09-10 16:45 — Tests: `TestSheetsInReachFollowTheHandoutOrder` covers nothing-held,
  first-held, replacement, a gap in the sequence, and everything-held, and asserts in each case
  that `sheetReachable` agrees with the rendered list. Plus two template tests for the markup —
  held sheets marked `(udleveret)`, out-of-reach ones `disabled (ikke nået endnu)`.
- 2026-09-10 16:55 — ✅ Verified live against 2026 data, all three states:
  team 5 (holds nothing) → `Deltagerkort 1` selected, 2 and 3 `disabled`;
  team 2 (holds Deltagerkort 1) → 1 marked `(udleveret)`, **2 selected**, 3 `disabled`;
  team 2 after being given 2 and 3 → all three `(udleveret)`, nothing preselected, and the note
  "Patruljen har fået alle kortene. Vælg kun et kort her, hvis de har mistet et og får det
  igen."
  `POST` skipping ahead to Deltagerkort 3 → `424` "Patruljen får kortene i rækkefølge, og
  Deltagerkort 3 er ikke næste kort. Patruljen mangler Deltagerkort 2 først."; `POST` with
  Deltagerkort 2 → `303` and the binding recorded. Gate green.
- 2026-09-10 16:58 — Note for whoever reads the dev database next: verifying the
  everything-held state meant actually registering sheets 2 and 3 to team 2 (stickers 5 and 2).
  Those events are on the stream and will replay. Team 5 is the untouched patrol with a
  photograph and no sheets.
- 2026-09-10 17:00 — Chased one scare to ground: `qr` id 1 changed team between two of my
  queries and I had not posted to it. Read the message rather than guessing —
  `nats stream get NATHEJK --last-for=NATHEJK.2026.qr.1.registered` showed scanner phone
  `40733886` at 14:48, i.e. a real person testing in a browser, not a registration landing on
  the wrong code. Worth recording as the cheap way to settle that class of question.
