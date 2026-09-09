# 03 · Containers

Compose, images, and database migrations.

← [02 The monorepo](02-monorepo.md) · next → [04 Go from zero](04-go-from-zero.md)

---

## Do this

```bash
cp .env.example .env          # once
docker compose up -d --wait   # postgres, api, palette, audit — waits for healthy
pnpm db:up                    # apply migrations
pnpm install && pnpm dev      # the TypeScript loop, NATIVE — not in a container

docker compose ps             # what is running, and is it healthy
docker compose logs -f api    # follow one service
docker compose down           # stop.  add -v to also delete the database
```

Migrations and screenshots are one-shot tasks, not services, so they are gated behind
a profile and never start with `up`:

```bash
pnpm db:up      # docker compose --profile tools run --rm migrate up
pnpm db:down    # roll back one migration
docker compose --profile tools run --rm migrate status
docker compose --profile tools run --rm visual          # screenshot diff
```

> **If `docker` says permission denied:** your login session predates your `docker`
> group membership. Log out and back in. To work around it right now, prefix with
> `sg docker -c '…'`.

---

## What runs in a container, and what does not

| Runs in a container    | Runs natively                 |
| ---------------------- | ----------------------------- |
| Postgres               | the TypeScript edit loop      |
| Go services            | `vitest`, `tsc`               |
| goose migrations       | golden **text** fixture tests |
| Playwright screenshots |                               |

The rule is: **containerise where reproducibility matters, stay native where latency
matters.** You touch the TypeScript loop every minute; paying bind-mount and rebuild
overhead there would be miserable. You touch Postgres's version approximately never,
and when it differs between two machines you lose an afternoon.

Screenshots are the sharpest case. Font rasterisation differs across operating
systems, so the _same_ CSS produces different pixels on your laptop and in CI. That
is not a bug you can fix; it is a reason to pin the renderer. Golden **text** fixtures
have no such problem and run natively, fast.

---

## compose.yaml, service by service

### postgres

```yaml
volumes:
  - pgdata:/var/lib/postgresql # NOT /data — see below
  - ./docker/postgres/init.sql:/docker-entrypoint-initdb.d/init.sql:ro
healthcheck:
  test: ['CMD-SHELL', 'pg_isready -U ${POSTGRES_USER:-demystify} …']
```

**The mount path matters.** Postgres 18 stores data in a major-version subdirectory so
that `pg_upgrade --link` works across a version bump. It wants **one** mount at
`/var/lib/postgresql`. Mount `/var/lib/postgresql/data` directly — as was standard for
Postgres 17 and earlier, and as most tutorials still show — and 18 refuses to start
with a long explanatory error. This crashed the stack the first time it was brought up.

**The healthcheck is not optional.** Without it, `depends_on` only means "the
container started", not "Postgres is accepting connections". Your API then races the
database on every boot and fails perhaps one time in five. With it, dependents can use
`condition: service_healthy` and the race disappears.

`init.sql` runs **once**, on first initialisation of the volume — not on every start.
It is for database-level setup that must exist before the first migration (here, the
`pgcrypto` extension for `gen_random_uuid()`). Schema is goose's job, not this file's.

### api, palette, audit

Three services, one image, differing only by `command:`:

```yaml
x-go-service: &go-service
  build: { context: ./services, dockerfile: Dockerfile }
  depends_on:
    postgres: { condition: service_healthy }

api:
  <<: *go-service
  command: ['/api']
```

`&go-service` defines a YAML anchor; `<<: *go-service` merges it. One place to change
shared config.

The image's Dockerfile ends with `CMD ["/api"]` rather than `ENTRYPOINT`, precisely so
that compose's `command:` can override it. Had it been `ENTRYPOINT`, `command:` would
be appended as _arguments_ to `/api` instead of replacing it.

### Healthchecks without a shell

The runtime image is distroless: no shell, no `curl`, no `wget`. Nothing to run a
conventional healthcheck with.

The answer is that the binary checks itself:

```yaml
healthcheck:
  test: ['CMD', '/api', '-health']
```

`CMD` (not `CMD-SHELL`) executes directly with no shell. `services/cmd/api/main.go`
intercepts the `-health` argument, performs an HTTP GET against its own `/healthz`,
and exits 0 or 1. See `services/internal/httpx/health.go`.

This is the standard distroless pattern and worth recognising — you will meet it
again.

### migrate and visual

Both are `profiles: ["tools"]`, so `docker compose up` skips them. They are one-shot
tasks: run, do a thing, exit. Starting them as services would be meaningless.

`visual` also carries:

```yaml
user: '${UID:-1000}:${GID:-1000}'
```

The Playwright image runs as root. Without this line, generated screenshot baselines
land in your repo owned by root, and you cannot regenerate or delete them without
`sudo`. This actually happened during setup and had to be cleaned up.

> **General rule:** any container that writes into a bind-mounted repo needs a `user:`
> mapping. Containers that only read do not.

---

## The two-URL footgun

`DATABASE_URL` is **not the same string** in both contexts, and this bites everyone
exactly once:

| Who is connecting                       | Host        | Value                                   |
| --------------------------------------- | ----------- | --------------------------------------- |
| Go services, inside the compose network | `postgres`  | `postgres://…@postgres:5432/demystify`  |
| Web app, CLI, host-side `sqlc`/`goose`  | `localhost` | `postgres://…@localhost:5432/demystify` |

Inside the compose network, `postgres` is a DNS name that resolves to the container.
From your laptop, that name means nothing — you reach the same database through the
published port on `localhost`.

The convention here: **`.env.example` carries the host form**, and `compose.yaml`
overrides it per-service with the in-network form via the `x-db-url` anchor. So the
file you copy is correct for native tools, and containers get corrected automatically.

Symptom when you get it wrong: `dial tcp: lookup postgres: no such host` (you used the
container form on the host), or connection refused (the reverse).

---

## Images

### services — multi-stage to distroless

```dockerfile
FROM golang:1.26-trixie@sha256:… AS build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot@sha256:…
COPY --from=build /out/ /
```

- **Multi-stage** — the compiler and module cache stay in the build stage. Only the
  binaries are copied forward.
- **`CGO_ENABLED=0`** — a fully static binary with no libc dependency, which is what
  lets the runtime image contain essentially nothing.
- **Cache mounts** — without them every rebuild re-downloads the entire module graph.
- **Distroless** — no shell, no package manager, no busybox. Nothing to exploit, and
  nothing to debug with either. That trade is deliberate.
- **`-ldflags='-s -w'`** — strips debug symbols.

The result is ~35 MB carrying all three binaries.

### web and docs — Next.js standalone

Two gotchas, each of which costs an afternoon undocumented:

**`output: 'standalone'` does not bundle `.next/static` or `public/`.** The Dockerfile
must copy both explicitly. Miss them and the app boots fine and serves an unstyled
page with broken images.

**`turbo prune --docker`** produces a minimal build context containing only the target
package and its dependencies. Without it, every package rebuilds on any change and the
Docker layer cache is useless.

**And one that is specific to pnpm:** the deps stage installs with
`--config.node-linker=hoisted`. Next's file tracing follows pnpm's symlinked
`node_modules/.pnpm` layout imperfectly and misses transitive dependencies like
`@swc/helpers`. The image builds successfully and then dies at startup on
`MODULE_NOT_FOUND`. A flat `node_modules` traces reliably. Both app Dockerfiles need
this; `apps/docs/Dockerfile` was missing it initially and had exactly that failure.

### Digest pinning

Every base image is pinned by `sha256:` digest, not tag:

```dockerfile
FROM mcr.microsoft.com/playwright:v1.58.0-noble@sha256:35c7d48b…
```

A tag is a _moving pointer_. `postgres:18.2-trixie` today and next month can be
different images — the maintainers rebuild in place when the base OS ships security
patches.

For most images that is mildly annoying. **For Playwright it is the whole point.**
When the base OS ships a font update, font metrics shift by a fraction of a pixel,
_every_ visual golden fails at once, and the team learns within a week to ignore
visual failures. At that moment the entire value of Track D is gone.

Bumping that digest is therefore a deliberate act that regenerates all goldens **in
the same pull request**. To resolve a new one:

```bash
docker buildx imagetools inspect mcr.microsoft.com/playwright:v1.58.0-noble \
  --format '{{.Manifest.Digest}}'
```

### .dockerignore

Excludes `node_modules`, `.next`, `.git`, `.turbo`. Without it the build context is
hundreds of megabytes, shipped to the daemon on **every** build. This is the single
cheapest build-speed fix in any Docker project.

---

## goose — database migrations

goose applies numbered `.sql` files in order and records what it applied in a
`goose_db_version` table, so nothing runs twice.

### The file format

`services/migrations/00001_init.sql`:

```sql
-- +goose Up
CREATE TABLE users (…);

-- +goose Down
DROP TABLE users;
```

Those `-- +goose` comments are the entire API. `Up` runs on `goose up`, `Down` on
`goose down`. Plain SQL otherwise — no DSL, no ORM, no generated schema file.

### Why the schema is deliberately thin

```sql
themes (id, user_id, token_json jsonb, created_at)
```

Token payloads go in JSONB because the canonical model will change for months.
Promoting fields into real columns is easy later; migrating a schema you designed
around a model that then moved is not.

### Configuration

goose reads `GOOSE_DRIVER` and `GOOSE_DBSTRING` from the environment, supplied by
compose. That is why `docker/goose/Dockerfile`'s entrypoint passes only `-dir`:

```dockerfile
ENTRYPOINT ["/goose", "-dir", "/migrations"]
```

Adding `postgres` positionally as well — which the goose docs show for the
non-environment form — makes goose parse it as the _command_ and fail with
`no such command`.

---

## Exercise

Write a migration, apply it, and roll it back, watching goose's bookkeeping at each
step.

```bash
docker compose --profile tools run --rm migrate status
```

Create `services/migrations/00002_theme_name.sql` adding a `name text` column to
`themes`, with a `Down` that removes it. Then apply, inspect, roll back, and inspect
again.

<details>
<summary>Answer</summary>

```sql
-- +goose Up
ALTER TABLE themes ADD COLUMN name text NOT NULL DEFAULT 'untitled';

-- +goose Down
ALTER TABLE themes DROP COLUMN name;
```

```bash
pnpm db:up
docker compose exec -T postgres psql -U demystify -d demystify -c '\d themes'
docker compose --profile tools run --rm migrate status      # 00002 applied

pnpm db:down                                                # roll back one
docker compose exec -T postgres psql -U demystify -d demystify -c '\d themes'
```

**Why `DEFAULT 'untitled'` matters:** `NOT NULL` with no default fails on a table that
already has rows, because existing rows would violate the constraint. On an empty
table it happens to succeed — which is how this bug reaches production, having passed
every test on a fresh database.

**Leave the migration in place** if you want the column, or delete the file _and_ roll
back first. Deleting an applied migration without rolling it back leaves
`goose_db_version` claiming a version whose file no longer exists.
</details>

---

← [02 The monorepo](02-monorepo.md) · next → [04 Go from zero](04-go-from-zero.md)
