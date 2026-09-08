# 002 — Protect /qr and /geo with a secret token

**Status:** open
**Priority:** high
**Created:** 2026-09-08
**Picked up by:**
**Started:**
**Completed:**

## Description

`GET /qr` and `GET /geo` are wired in `go/routes.go` without `user.Authenticate`
and without any other check, so both are **fully public** today:

- `/geo` returns every scan that has coordinates, with patrol names, scanner names
  and LOK labels — a live map of the entire race. Any bandit with the URL gets a
  decisive, unfair advantage, which breaks the fair-game rule outright.
- `/qr?n=N` mints valid checksummed sticker URLs for ids `1..N`, which is enough
  to register or scan codes that have not been handed out yet.

These are machine endpoints, not scanner pages: `/qr` feeds sticker printing and
`/geo` feeds map/GIS export. They are therefore **not** behind the phone login —
they are protected by a **secret token passed as a GET parameter** (decided; see
`.rules` → *Protecting `/qr` and `/geo`*).

Implementation requirements:

- Use a **separate environment variable**, *not* `SECRET`. `SECRET` seeds the QR
  checksum, and a query-string token leaks into browser history, proxy logs and
  `Referer` headers. Leaking `SECRET` would let anyone compute valid URLs for every
  sticker, printed or not, recoverable only by reprinting the whole run. Keep the
  two secrets unrelated.
- Compare with `crypto/subtle.ConstantTimeCompare`.
- Refuse when the supplied token is empty **or** the env var is unset — never fail
  open.
- Respond `404` (not `403`) so the endpoints are not advertised.
- Check the token before any DB or template work runs.
- Add the variable to `docker-compose.yml` with a dev value, and document it in
  `.rules` and `README.md` (both currently say "not yet added").

These endpoints remain crew-grade information: the token must not be handed to
bandits.

## Acceptance Criteria

- [ ] A new env var (name to be chosen, e.g. `EXPORT_TOKEN`) is read at startup and
      threaded to the handlers — not read ad hoc via `os.Getenv` inside them
- [ ] `/qr` and `/geo` return `404` when the token is missing, empty or wrong
- [ ] Both endpoints work with a correct token
- [ ] Comparison uses `crypto/subtle.ConstantTimeCompare`
- [ ] An unset/empty env var refuses all requests rather than allowing them
- [ ] The token check runs before any query or response body is written
- [ ] `SECRET` is not reused for this purpose
- [ ] Dev value added to `docker-compose.yml`; env tables in `.rules` and
      `README.md` updated
- [ ] `go test ./...` and `staticcheck ./...` pass in the container

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 00:00 — Task created. Found during a documentation pass: both routes
  are wired without `user.Authenticate`. HQ decided on a GET-parameter token rather
  than a login, since these feed printing and GIS tooling.
