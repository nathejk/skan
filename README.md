# Nathejk Skan

QR scanning for the Nathejk night race.

Nathejk is a Danish scouting race: patrols (*patruljer*) navigate from checkpoint
to checkpoint through the night while *banditter* hunt them. Every patrol carries
a paper map with a unique QR sticker, and they get a fresh map — with a fresh
sticker — several times during the race.

**Skan is what your phone opens when you scan one of those stickers.** Checkpoint
crew scan a patrol when it arrives; bandits scan a patrol when they catch it. Each
scan is stored with who scanned, when, and where.

### Vocabulary

The domain is Danish and stays Danish in the code and the UI:

| Term | Meaning |
|---|---|
| *patrulje* | a patrol — the scout team being hunted |
| *spejder* | a scout, a member of a patrulje |
| *klan* | a clan — a group of seniors |
| *senior* / *bandit* | **the same person.** A senior signed up to a klan *is* a bandit; `senior` is the signup word, `bandit` is the race word |
| *holdnummer* | team number — the big number on the scouts' arms |
| *post* | a checkpoint |
| *lok* | a camp location a klan operates from |

---

## How it works

### 1. Codes are generated before the race

`GET /qr?n=500` returns a CSV of `id,url` — one line per sticker:

```
1,https://skan.nathejk.dk/qr/1/2394859
2,https://skan.nathejk.dk/qr/2/1029384
```

The trailing number is a checksum of the id plus a shared secret, so the URLs
can't be guessed by counting upwards. The CSV goes to whoever prints the
stickers.

Because this endpoint hands out working URLs for codes nobody has seen yet, it is
guarded by a secret token in the query string rather than by a login.

> Changing the `SECRET` environment variable invalidates every sticker already
> printed.

### 2. The first scan binds a code to a patrol

A freshly printed code means nothing to the system. The first scanner is asked
for the patrol's **team number** — the large number on the scouts' arms, not the
small number printed beside the QR code. Confirming it ties that sticker to that
patrol permanently.

### 3. Every later scan is just a scan

Once a code is known, scanning it shows the patrol (name, arm number, photo) and
records the scan. The page asks the browser for GPS coordinates; if that's
refused or unavailable, the scanner is prompted to type the map coordinate by
hand instead.

### 4. Logging in

Scanning is not anonymous. A visitor without a session is asked for their phone
number, which is matched against event personnel and clan seniors already
registered in the Nathejk system. That lookup is also what decides whether the
scanner is **crew** (found among personnel) or a **bandit** (found among the
seniors — a senior signed up to a klan *is* a bandit) — the two roles are intended
to see different things after a scan. There is no role picker: your phone number
*is* your role.

If the same phone number is registered both as crew and as a senior, the app
can't tell which one you are — so it refuses the login and asks you to contact HQ
to get one of the registrations removed.

> Neither the role-specific views nor the duplicate-number check are built yet:
> today every scanner sees the bandit variant, and a number in both tables
> silently logs in as crew. See *Known gaps*.

### 5. Bandits and crew see different things

**Bandits may only learn about bandits — it has to be a fair game.** A bandit is a
player, so anything they learn about how a patrol is doing in the race is an
unfair advantage. A bandit's scan result is limited to what their own side
produced: how many times that patrol has been caught by bandits.

Crew are not players — they might be checkpoint personnel, a guide walking with
the scouts, or a samarit taping up blisters. **Crew see everything.**

| After scanning a patrol | Bandit | Crew |
|---|---|---|
| Who the patrol is (name, arm number, photo) | yes | yes |
| How many times bandits have caught them | yes | yes |
| Total number of scans, checkpoints included | no | yes |
| Checkpoint activity and positions | no | yes |

### 6. Catching the same patrol twice

Bandits do catch the same patrol more than once during a night, so **a rescan
counts**. To stop accidental double scans from inflating the tally, there's one
guard: if the patrol's most recent scan was by *you*, less than *30 minutes* ago,
you're asked to confirm that this really is a new catch.

So two scans half an hour apart just count, and so do two of your scans with
another scanner's in between — only an immediate repeat by the same person asks a
question.

> Not implemented yet — every rescan currently counts without asking.

```mermaid
flowchart TD
    A[Scan sticker] --> B{Logged in?}
    B -- no --> C[Enter phone number]
    C --> B
    B -- yes --> D{Code known?}
    D -- no --> E[Enter team number]
    E --> F[Code bound to patrol]
    F --> D
    D -- yes --> G[Show patrol, get position]
    G --> H[Scan recorded]
```

---

## Where Skan sits

Skan is one service in a larger Nathejk setup. Services talk to each other over
NATS JetStream rather than by calling each other.

```mermaid
flowchart LR
    HQ[hq / signup admin] -->|patrols, clans, crew| JS[JetStream]
    SKAN[skan] -->|qr found / registered / scanned| JS
    JS -->|replay| SKAN
    JS --> OTHER[reporting and other services]
```

Skan **publishes** three facts: a code was *found*, a code was *registered* to a
patrol, a code was *scanned*. Everything Skan knows about patrols, clans, seniors
and crew it **learns by listening** — those records are created elsewhere.

A local MariaDB holds only a cache of that event history, rebuilt from scratch on
every start. It is safe to delete.

Scan data is also available to other tooling. Both of these are **machine
endpoints guarded by a secret token in the query string**, not by the phone login:

- `GET /qr?n=N` — the sticker-printing CSV described above
- `GET /geo` — all scans that have coordinates, as JSON, for map/GIS export

The token is a different secret from the QR checksum secret, and it must not be
handed to bandits — `/geo` is a live map of the whole race.

> Neither endpoint checks a token yet; both are currently public. See *Known gaps*.

- `GET /healthcheck` — liveness probe, open by design

---

## Running it locally

Everything runs in Docker. You do not need Go installed.

**Prerequisites:** the shared Nathejk infrastructure (Traefik and NATS
JetStream) must be running, providing the external `traefik` and `jetstream`
Docker networks.

```sh
docker compose up
```

| What | Where |
|---|---|
| The app | https://skan.local.nathejk.dk |
| Database admin (Adminer) | http://mysql.skan.local.nathejk.dk |
| Redis admin | http://redis.skan.local.nathejk.dk |

The Go process rebuilds and restarts itself on every file change under `go/`
(via [air](https://github.com/air-verse/air)), and runs the test suite as part of
each rebuild.

Run a one-off command in the container:

```sh
docker compose run --rm api go test ./...
```

> Heads up: by default the dev stack connects to the **shared** JetStream at
> `dev.nathejk.dk`, so local scans are written to shared dev data. Override
> `JETSTREAM_DSN` in `docker-compose.override.yml` if you want isolation.

---

## Layout

```
skan/
├── docker-compose.yml     dev stack
├── docker/Dockerfile      dev / build / prod stages
├── webroot/               static files
└── go/                    the entire application
    ├── main.go            wiring
    ├── routes.go          routes and handlers
    ├── templates/         the HTML pages
    ├── nathejk/commands/  publishing events (the write side)
    ├── nathejk/table/     turning events into SQL tables (the read side)
    ├── internal/login/    phone-number login
    └── superfluids/       JetStream plumbing
```

Pages are rendered server-side with Go's `html/template`. There is no frontend
build step and no SPA — deliberately. A scan has to load instantly on an unknown
phone over a weak signal in a field at 2am, and the forms work even if JavaScript
never runs.

The streaming and projection plumbing is mid-migration: the in-repo `superfluids/`
and `pkg/tablerow/` packages are being replaced by `github.com/jrgensen/stream` and
`github.com/jrgensen/cqrs`, and projections are lifted to
`github.com/nathejk/shared-go` once they stabilise. `nathejk/table/photo` and
`photocover` are already written in the new shape and are the reference for new
work; the other six projections still use the old one (task 007).

Patrol photographs are not stored here. The projections hold content-hash refs, and
the bytes come from the `foto` service at `<foto-base-url>/photos/<ref>`.

Some directories are inherited from sibling Nathejk repos and unused here (for
example `go/nathejk/table/spejder`, `go/nathejk/table/scanner`). See `.rules` for
the full list. This app was a PHP/Twig application before the port to Go; those
original templates have been deleted.

---

## Configuration

| Variable | Purpose |
|---|---|
| `SECRET` | Seeds the QR URL checksum. **Required.** Changing it breaks printed stickers, so never expose it in a URL. |
| *(token var, not yet added)* | The secret token for `/qr` and `/geo`, passed as a query parameter. Deliberately a different secret from `SECRET`. |
| *(foto base URL, not yet added)* | Base URL of the `foto` service, e.g. `https://foto.local.nathejk.dk`. Patrol photos live at `<base>/photos/<ref>`. |
| `JETSTREAM_DSN` | NATS JetStream connection |
| `DB_DSN` | MariaDB connection |
| `WEBROOT` | Static file directory |

---

## Known gaps

Short, honest list — details and more items in `.rules`.

- **Bandit and crew see the same page.** The role follows from the phone number,
  but the scan result page hardcodes the bandit variant, along with placeholder
  catch/scan counts — so crew currently get *less* than they should, and both get
  invented numbers.
- **`/geo` and `/qr` are wide open** — no login and no token — which breaks the
  fair-game rule: `/geo` is a live map of every scan in the race, and `/qr?n=N`
  hands out working sticker URLs for codes that haven't been distributed yet.
- **Every patrol shows the same stock photo.** Both the scan page and the
  registration confirmation hardcode `/groupphoto.jpg`, so the "is this the right
  patrol?" confirmation currently confirms nothing. The `photo` and `photocover`
  projections that would fix it are copied in but not wired up.
- **Nothing guards against accidental rescans**; the 30-minute confirmation isn't
  built.
- **A phone number registered as both crew and senior logs in as crew.** It
  should be refused with a "contact HQ" message instead.
- **An unknown phone number returns a 500** rather than a "we don't know that
  number" message.
- **The login cookie is unsigned**, so a scanner's identity can be forged. Fine
  for a scouting race, not fine for anything sensitive.
- **The production image doesn't ship `webroot/`**, so static assets (including
  the fallback group photo) are missing there.
- **The event year is hardcoded** to `2025` in the code.
- The dev stack still uses an old gateway container instead of the standard
  per-service Traefik routing used elsewhere in the org.

---

## Contributing

See `.rules` (this repo) and `.agents/rules/rules.md` (org-wide) for conventions:
branch naming, commit format, and the rule that handlers publish events rather
than writing to the database directly.

Work is tracked on a file-based board in `roadmap/tasks/` — `open/`, `doing/` and
`done/` folders holding one Markdown file per task, with the conventions in
`roadmap/tasks/TASKS.md`. The known gaps listed above are tasks 001–007.

## Credits

Default patrol placeholder photo:
https://img.freepik.com/premium-vector/male-climbers-help-each-other-mountains-vector-silhouette-conceptual-business-scene-teamwork_556258-4616.jpg?w=2000
