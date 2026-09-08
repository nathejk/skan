# 001 — Resolve scanner role from phone number at login

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

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

User-facing text is Danish. Suggested wording, to be corrected by HQ:

- unknown number — "Vi kender ikke det telefonnummer. Tjek at du har skrevet det
  rigtigt, eller kontakt HQ."
- registered twice — "Dit telefonnummer er registreret både som crew og som
  senior. Ring til HQ, så de kan fjerne den ene registrering."

This task blocks 003 (role-split scan page) and 002's crew-grade reasoning.

## Acceptance Criteria

- [ ] `userByPhone` (or its replacement) queries **both** projections and reports
      three distinct outcomes: resolved user + role, conflict, not found
- [ ] A number present in both `personnel` and `senior` is refused — no cookie
      written, no guessing, no preferring one table
- [ ] An unknown number is refused with a Danish message, not a 500
- [ ] No cookie is ever written for a `nil`/unresolved user
- [ ] The login page re-renders with the relevant Danish message, preserving the
      `redir` target so the scanner lands where they were going
- [ ] The resolved role is available to handlers (e.g. on `login.User`) so 003 can
      branch on it without re-querying
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Derived from a documentation pass over the repo;
  decision confirmed by HQ (roles gone, phone number is the only identity, both →
  refuse and ring HQ).
