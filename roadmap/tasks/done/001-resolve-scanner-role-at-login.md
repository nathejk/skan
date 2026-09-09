# 001 — Resolve scanner role from phone number at login

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-09
**Completed:** 2026-09-09

## Description

A scanner has exactly one role and it is derived from their phone number. Today
`userByPhone` in `go/internal/login/user.go` checks the `personnel` projection,
returns on the first hit, and only then falls back to `senior` — so it never
notices a number registered in both, and the resolved role is never carried
anywhere useful.

Decided rules (see `.rules` → *Authentication and roles*):

| Phone found in | Result |
|---|---|
| `personnel` only | log in as **crew** |
| `senior` only | log in as **bandit** |
| **both** | **refuse** — data error upstream, tell the user to contact HQ |
| neither | refuse — unknown number |

*senior* and *bandit* are synonyms: presence in the `senior` projection is the
entire bandit test. There is no per-person flag and no subset of seniors that
behaves differently.

There is no role switcher and must never be one again — the pre-port PHP app let
people choose between several functions per session, and that concept is gone.

**Both refusal paths are currently a 500 error.** `LoginHandler` marshals a `nil`
user into the cookie as the JSON literal `null` and redirects; the next request
decodes it to a zero `User`, trips the `empty user cookie found` check, and
`Authenticate` maps that non-`ErrNoCookie` error to `500 server error`. A scanner
in a field at 2am gets a blank server error. No cookie should be written unless a
user was actually resolved.

User-facing text is Danish. Wording **approved by HQ**:

- unknown number — "Vi kender ikke det telefonnummer. Tjek at du har skrevet det
  rigtigt, eller kontakt HQ."
- registered twice — "Dit telefonnummer er registreret både som crew og som
  senior. Ring til HQ, så de kan fjerne den ene registrering."

This task blocks 003 (role-split scan page) and 002's crew-grade reasoning.

## Acceptance Criteria

- [x] `userByPhone` (or its replacement) queries **both** projections and reports
      three distinct outcomes: resolved user + role, conflict, not found
- [x] A number present in both `personnel` and `senior` is refused — no cookie
      written, no guessing, no preferring one table
- [x] An unknown number is refused with a Danish message, not a 500
- [x] No cookie is ever written for a `nil`/unresolved user
- [x] The login page re-renders with the relevant Danish message, preserving the
      `redir` target so the scanner lands where they were going
- [x] The resolved role is available to handlers (e.g. on `login.User`) so 003 can
      branch on it without re-querying
- [x] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Derived from a documentation pass over the repo;
  decision confirmed by HQ (roles gone, phone number is the only identity, both →
  refuse and ring HQ).
- 2026-09-08 04:30 — HQ approved the drafted Danish wording as written above.
- 2026-09-09 09:30 — Picked up. Found a prerequisite the task had not anticipated:
  **`GetByPhone` did not filter by year** in either projection. People sign up again
  every event, so a number matches rows from several years. Checked the live data before
  deciding: 3+ numbers appear in both 2025 and 2026 `personnel`, and there are **12
  crew/senior overlaps — all of them cross-year** (8 crew in 2025 + senior in 2026, 4 the
  other way), and **zero within a single year**. So implementing the conflict rule
  without a year filter would have refused 12 real people on race night while the actual
  rule caught nobody. Both queriers now take the year, and their swallowed `Scan` errors
  are handled properly.
- 2026-09-09 09:35 — Implemented. `login.User` gains a `Role` (`crew`/`bandit`) plus
  `IsBandit()`; `userByPhone` consults **both** projections and returns one of three
  outcomes; `ErrUnknownPhone`/`ErrAmbiguousRole` map to the approved Danish text.
  `LoginHandler` writes no cookie unless a user was resolved and re-renders the login
  page with the message, preserving `redir`.
- 2026-09-09 09:36 — Restructured how the login page is drawn rather than bolting an
  error path onto the old shape: the `login` package now takes a `Renderer`, so
  `Authenticate(next)` and `LoginHandler` both render through the same seam and the
  package stays free of template wiring. `Authenticate` also treats *any* unusable cookie
  as "not logged in" — clearing it — instead of returning 500, which is the other half of
  the bug: the old code wrote `null` and then 500'd on the next request.
- 2026-09-09 09:40 — Added `internal/login/user_test.go`, the repo's first behavioural
  test: 8 role-resolution cases (including both cross-year directions, which are the ones
  that actually occur), plus tests that a refused login writes no cookie, that a
  successful one stores the resolved role, and that the three historically fatal cookie
  values (`null`, non-base64, empty id) render the login page rather than a 500.
- 2026-09-09 09:45 — ✅ Verified live through Traefik as well: unknown and empty numbers
  render the Danish "vi kender ikke det telefonnummer" message with **200** and no
  cookie; a 2026 crew number, a 2026 senior number and a cross-year overlap all get
  **303**. Then temporarily inserted a same-year crew row for a senior's phone — the
  login was refused with "registreret både som crew og som senior … ring til HQ" — and
  removed it again, after which that number logs in normally.
- 2026-09-09 09:45 — Completed. `scanHandler` still hardcodes `isBandit: true`; the role
  is now available for 003 to use, which owns that page.
