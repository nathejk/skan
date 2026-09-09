# 009 — Route every web service directly through Traefik, fix JetStream wiring

**Status:** done
**Priority:** high
**Created:** 2026-09-08
**Picked up by:** Zed agent
**Started:** 2026-09-08
**Completed:** 2026-09-08

## Description

The compose stack deviated from the org standard in three ways:

1. A `gw` gateway container (`jrgensen/gateway`) with `PROXY_MAPPINGS` fronted every
   web service behind a single Traefik router, instead of each service declaring its
   own labels and joining the `traefik` network.
2. `api` published a port to the host (`ports: - 80`), which the org rules forbid.
3. `JETSTREAM_DSN` pointed at `nats://dev.nathejk.dk:4222`, so local runs read and
   wrote the **shared dev stream**. It should be `nats://jetstream:4222` over the
   external `jetstream` network.

HQ: the gateway is out; every service offering a web interface connects directly to
Traefik.

## Acceptance Criteria

- [x] `gw` service removed, along with every `depends_on: gw`
- [x] `api`, `adminer` and `redis-commander` each join `traefik` and declare their own
      repo-scoped routers
- [x] `api` no longer publishes a port
- [x] `JETSTREAM_DSN` is `nats://jetstream:4222`, over the `jetstream` network
- [x] `docker compose config` validates and the stack starts
- [x] Traefik reports every skan router as enabled, with no errors

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-08 03:00 — Created and picked up. Plan: delete `gw`, give each web-exposed
  service its own labels per the org pattern and the `docker-dev-stack` skill, drop
  the published port, and repoint JetStream at the local broker.
- 2026-09-08 03:10 — Chose the **HTTPS-redirect** pattern for `api` rather than plain
  HTTP. The scan pages call `navigator.geolocation`, which browsers only expose in a
  secure context, and `*.local.nathejk.dk` is not treated as `localhost`. Plain HTTP
  would break the core feature of the app. Internal tools (adminer, redis-commander)
  stay on plain HTTP.
- 2026-09-08 03:20 — Removed the `prod` service from compose entirely. It made a bare
  `docker compose up` build the production image, which runs the full
  test/lint/vuln gate with no source mount — and that currently fails (see below), so
  `up` was broken before any app code ran. The prod image is now built explicitly:
  `docker build -f docker/Dockerfile --target prod -t skan:local .`
- 2026-09-08 03:30 — Traefik rejected the first attempt:
  `middleware "redirect-to-https@docker" does not exist`. The org rules describe a
  shared `redirect-to-https` middleware, but no such middleware exists on the running
  Traefik — sibling repos declare their own (`hq-redirect-to-https@docker` is
  visible). Declared a repo-scoped `skan-redirect-to-https` via labels instead.
  `tilmelding@docker` is currently disabled with this exact error, so it has the same
  latent bug.
- 2026-09-08 03:35 — Second error: `the service "skan@docker" does not exist`. The
  routers named a service that was never declared; Traefik's implicit service for the
  container is `api-skan`. Declared `traefik.http.services.skan.loadbalancer.server.port: 80`
  so both routers point at one explicitly-named, repo-scoped service, consistent with
  `skan-sql` and `skan-redis`.
- 2026-09-08 03:40 — ✅ Verified against the running Traefik API: `skan@docker`
  (web, redirect), `skan-secure@docker` (websecure, desec cert), `skan-sql@docker` and
  `skan-redis@docker` all `status: enabled`, no errors.
- 2026-09-08 03:45 — ✅ JetStream verified from inside the `api` container: `jetstream`
  resolves to 172.26.0.2 and port 4222 answers (`-err 'Unknown Protocol Operation'`
  is NATS rejecting an HTTP request, i.e. the broker is listening). Note the NATS
  container runs with `-js` only, so the monitoring port 8222 is **not** enabled,
  contrary to the org rules' claim.
- 2026-09-08 03:50 — Completed, with one limitation: end-to-end HTTP through Traefik
  to the app could not be verified, because the `api` container's dev loop cannot
  build (`photo`/`photocover` import `github.com/jrgensen/cqrs`, absent from
  `go.mod`), so the Go binary never starts. Routing and networking are verified at the
  Traefik/NATS level; the last hop needs task 007. `*.local.nathejk.dk` also does not
  resolve from this agent's sandbox, so browser-level checks are the user's to make.
