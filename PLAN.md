# Demystify UI

A customizable UI library and design-system generator that targets **the UI library you already use**.

> **Status: pre-implementation.** This document is the specification and build plan. No code has been written yet; the bootstrap sequence in [Setup](#setup) has not been run.

---

## What this is

The wedge is not "another shadcn." It's:

> **Point us at whatever UI library you already use, and we generate a coherent, contrast-verified design system for it — with an honest report of what got lost in translation.**

On top of that sits a shadcn-style component library of our own, customizable in the browser before you install it, plus a dev-guide and code-style-guide so teams have something to follow after the export.

### Why it's non-generative

This project is part of [nongenerativeai.com](https://nongenerativeai.com) — automating what doesn't need generative AI,
on the belief that deterministic methods and classical ML are better suited to many tasks people now reach for an LLM to solve.

Every transformation here is deterministic and explainable:

| Job                                | Method                                           |
| ---------------------------------- | ------------------------------------------------ |
| Color ramps                        | OKLCH interpolation with lightness-curve fitting |
| Contrast targeting                 | APCA solved via Newton-Raphson on lightness      |
| "You have 14 near-duplicate greys" | ΔE-OK clustering                                 |
| Palette from a screenshot          | k-means / median-cut                             |
| Per-component customization        | AST codemods                                     |

Same input, same output, every time — with a printed reason. That claim only has teeth if the algorithm is **named in the UI**, so this is treated as a product requirement, not a nicety. An LLM competitor cannot show its arithmetic, because it didn't do any.

---

## Stack

| Concern                                   | Choice                                   | Why                                                                                |
| ----------------------------------------- | ---------------------------------------- | ---------------------------------------------------------------------------------- |
| Token engine                              | **TypeScript**                           | Runs per-keystroke in the browser and inside the CLI                               |
| AST codemods, theme ingestion             | **TypeScript**                           | No Go library parses or prints idiomatic TSX                                       |
| Palette extraction, bulk audit, API, auth | **Go**                                   | Genuinely CPU-bound and server-shaped                                              |
| Persistence                               | **Postgres** (pgx + sqlc + goose)        | —                                                                                  |
| Local environment                         | **Docker Compose**                       | Postgres, Go services and test runners in containers; TS dev loop stays native     |
| Images                                    | distroless (Go), Next.js `standalone`    | Same Dockerfiles dev → CI → prod                                                   |
| Web                                       | Next.js App Router, Tailwind v4          | v4's `@theme` emits CSS custom properties natively                                 |
| Docs                                      | Fumadocs                                 | —                                                                                  |
| Components                                | Ark UI / Zag.js                          | Framework-agnostic state machines; a second framework is an adapter, not a rewrite |
| Color math                                | culori (OKLCH/P3), apca-w3               | —                                                                                  |
| Monorepo                                  | pnpm workspaces + Turborepo + Changesets | —                                                                                  |

---

## Layout

```
demystify/
├── go.work                     # points at ./services
├── pnpm-workspace.yaml
├── turbo.json                  # drives BOTH toolchains
├── compose.yaml                # dev topology
├── compose.ci.yaml             # CI overrides
├── .dockerignore               # ESSENTIAL — see Images
├── .env.example
├── LICENSES.md
│
├── packages/
│   ├── tokens/                 # canonical model, DTCG in/out, zod schemas
│   ├── engine/                 # ramps, APCA, ΔE-OK, projection runtime
│   ├── adapters/               # declarative specs + escape-hatch code
│   ├── ingest/                 # parse tailwind.config.js / theme.ts → tokens
│   ├── codemod/                # ts-morph transforms for component export
│   ├── cli/                    # npx demystify
│   └── ui/                     # Ark-based components
│
├── apps/
│   ├── web/                    # generator, live preview, export
│   │   ├── Dockerfile          # next standalone, prod
│   │   └── public/r/           # static registry JSON (shadcn-conformant)
│   └── docs/                   # dev-guide + code-style-guide
│       └── Dockerfile
│
├── services/                   # ONE Go module, multiple cmds
│   ├── Dockerfile              # multi-stage → distroless
│   ├── cmd/{api,palette,audit}/
│   ├── internal/{db,auth,figma,color,httpx}/
│   └── migrations/
│
├── docker/
│   ├── postgres/init.sql
│   └── goose/Dockerfile        # migration runner
│
├── fixtures/
│   ├── canonical/              # input token sets
│   └── expected/               # golden output per adapter@version
│
└── tests/visual/               # Playwright per-adapter render + screenshot
    └── Dockerfile           # digest-pinned playwright
```

---

## Setup

Requires Docker + Compose v2, Node 22+, pnpm 10+, Go 1.26+.

Dependencies run in containers; the TypeScript edit loop runs natively. Containers make the
environment reproducible where reproducibility matters (Postgres, Go services, screenshot
rendering) without paying bind-mount HMR latency on the loop you touch every minute.

```bash
# Once per machine — everything below fails without it
sudo usermod -aG docker $USER && newgrp docker
docker info >/dev/null && echo "daemon reachable"

mkdir -p demystify && cd demystify
git init && pnpm init

# JS workspace
printf 'packages:\n  - "packages/*"\n  - "apps/*"\n' > pnpm-workspace.yaml
pnpm add -Dw turbo @changesets/cli typescript vitest prettier
pnpm changeset init

# Go module + workspace
mkdir -p services/cmd/{api,palette,audit} services/internal services/migrations
cd services && go mod init github.com/<you>/demystify/services && cd ..
go work init ./services

# Apps
pnpm create next-app@latest apps/web --ts --app --tailwind --eslint --src-dir
pnpm create fumadocs-app@latest apps/docs
```

Daily loop, once the repo exists:

```bash
cp .env.example .env
docker compose up -d                       # postgres, api, palette, audit
docker compose run --rm migrate up         # goose
pnpm install && pnpm dev                   # native, hot reload
```

### Turborepo across two toolchains

Go tasks need explicit `inputs`/`outputs` or caching silently does nothing:

```jsonc
{
  "tasks": {
    "build": { "dependsOn": ["^build"], "outputs": ["dist/**", ".next/**"] },
    "go:build": {
      "inputs": ["services/**/*.go", "services/go.{mod,sum}"],
      "outputs": ["services/bin/**"],
    },
    "test": { "dependsOn": ["build"] },
    "test:golden": { "inputs": ["fixtures/**", "packages/adapters/**"] },
  },
}
```

### Container topology

`compose.yaml` defines six services. One Go image serves three of them via different
`command:` entries — consistent with the one-module/many-`cmd` decision above.

| Service    | Source                         | Notes                                                                                                                                                   |
| ---------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `postgres` | `postgres:<pin current patch>` | Named volume. **Healthcheck required** so dependents can use `condition: service_healthy`. Publishes `5432` to the host — the native dev loop needs it. |
| `api`      | `services/Dockerfile`          | `depends_on: postgres (healthy)`                                                                                                                        |
| `palette`  | same image, `cmd: palette`     | k-means workload                                                                                                                                        |
| `audit`    | same image, `cmd: audit`       | ΔE clustering                                                                                                                                           |
| `migrate`  | `docker/goose/Dockerfile`      | Profile-gated — `run`, never `up`                                                                                                                       |
| `visual`   | `tests/visual/Dockerfile`      | Profile-gated. Digest-pinned Playwright                                                                                                                 |

**Pin every image by digest, not tag.** For `visual` this is not hygiene, it is the whole
point: `mcr.microsoft.com/playwright:v1.xx-noble` is rebuilt in place when its base OS ships
font updates. Font metrics shift, every golden fails at once, and the team learns to ignore
visual failures — which destroys the value of Track D. Resolve once and commit:

```bash
# resolve the digest for whatever tag is current at setup time, then commit it
docker buildx imagetools inspect mcr.microsoft.com/playwright:<current-tag> \
  --format '{{.Manifest.Digest}}'
```

Bumping that digest is a deliberate change that regenerates all goldens **in the same PR**.

#### The two-URL footgun

`DATABASE_URL` is not the same string in both contexts, and this bites everyone exactly once:

| Consumer                                        | Value                                   |
| ----------------------------------------------- | --------------------------------------- |
| Go services (inside the compose network)        | `postgres://…@postgres:5432/demystify`  |
| Web app, CLI, host-side `sqlc`/`goose` (native) | `postgres://…@localhost:5432/demystify` |

`.env.example` carries the **host** form. `compose.yaml` overrides it per-service with the
in-network form.

### Images

**`services/Dockerfile`** — multi-stage, `CGO_ENABLED=0`, final stage
`gcr.io/distroless/static-debian12:nonroot`. ~15MB. Use BuildKit cache mounts or every
rebuild re-downloads the module graph:

```dockerfile
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags='-s -w' -o /out/api ./cmd/api
```

**`apps/web/Dockerfile`** — requires `output: 'standalone'` in `next.config.ts`. Two gotchas
that each cost an afternoon if undocumented:

- standalone does **not** bundle `.next/static` or `public` — copy both explicitly
- in a Turborepo monorepo, use `turbo prune --docker` to produce a minimal build context, or
  every package rebuilds on any change

Run as non-root.

**`.dockerignore`** — must exclude `node_modules`, `.next`, `.git`, `.turbo`. Without it the
build context is hundreds of megabytes and every build is needlessly slow.

> Podman 5.x runs this compose file rootless if you prefer it; nothing here depends on the
> Docker daemon specifically. Docker is the default because Compose v2 ships with it.

### Auth

**Don't build it.** Use a hosted provider (Clerk or WorkOS) and have the Go API verify JWTs. Rolling sessions, password reset, and email verification is two weeks that buys nothing unique.

---

## Core data model

Three artifacts define the whole system. Get these right and the rest is mechanical.

### 1. Canonical token model

A **superset** of every target's capability — a continuous perceptual ramp plus semantic anchors — because every adapter is a lossy projection off it.

```ts
type Ramp = {
  space: 'oklch'
  seed: { l: number; c: number; h: number }
  curve: { lightness: BezierControlPoints; chroma: ChromaCurve }
  // continuous — sample at ANY cardinality:
  //   4 (MUI) · 10 (Mantine) · 11 (Tailwind) · 12 (Radix)
  sample(n: number): OklchColor[]
  anchors: Record<SemanticRole, number> // 'solid-bg' → 0.62
}
```

Storing a **curve** rather than a fixed array of shades is the single most important decision in this repo. Store 10 hex values and Radix's 12 assigned-meaning steps become guesswork forever.

### 2. Adapter spec — data, not code

An adapter is a declarative file the engine interprets. This is a prerequisite, not polish: it's the only way 12 libraries × 2 live majors = 24 surfaces stays maintainable, and the only way a contributor adds a library by PR without touching the engine.

```yaml
id: mantine
version: '7'
cardinality: { type: fixed, count: 10, index: 0-based }
sampling: perceptual-even
mapping:
  colors.$name: 'theme.colors.{name}'
  radius.md: 'theme.defaultRadius'
declaresLoss:
  - { what: 'ramp resampled 12→10', severity: minor }
  - { what: 'chroma curve flattened at extremes', severity: minor }
escapeHatch: null # Tier 2/3 only
```

Only Ant Design (algorithmic seeds) and SCSS-map targets need an `escapeHatch` code module.

### 3. Loss report — a first-class typed output

Every projection returns `{ output, loss: LossReport[] }`. Not a footnote, not a console warning — a rendered UI panel. **Nobody else does this**, and it turns the weakest part of the product (translation is lossy) into the most trustworthy part.

---

## Adapter tiers

| Tier  | Libraries                                              | Difficulty                                                              |
| ----- | ------------------------------------------------------ | ----------------------------------------------------------------------- |
| **0** | shadcn, Tailwind v4, DaisyUI, Radix Themes, Open Props | Nearly free — all CSS custom properties, ~one formatter covers the tier |
| **1** | MUI, Mantine, Chakra v3, HeroUI, PrimeReact            | Mechanical but real — object themes with cardinality mismatches         |
| **2** | Ant Design                                             | Fight the algorithm — its seed tokens discard your ramp                 |
| **3** | Bootstrap SCSS maps, Vuetify                           | Compile-time; needs the user's build step. Do last, or never            |

Ant Design needs a **user-facing toggle**: _seeds-only_ (hand it `colorPrimary`, accept its math) vs. _full override_ (fight its math, break on minor versions). Both are legitimate; the UI must say which one you're getting.

Adapters are versioned as `mui@6`, `chakra@3` from the first commit. Chakra v2→v3 and Tailwind v3→v4 were both total theming rewrites, and old adapters must stay alive.

---

## Roadmap

Four parallel tracks. Track A is the critical path.

**M0 — Foundations** (week 1)

- Repo scaffold, CI green on both toolchains, first migration.
- Exit: `docker compose up -d` brings the stack healthy, `migrate up` applies, `docker compose run --rm visual` produces a baseline, and CI builds every image.
- Keep the schema thin: `users`, `themes (id, user_id, token_json JSONB, created_at)`.
- Token payloads go in JSONB — the canonical model will move for months.

### Track A — Engine & adapters

- **M1** (wk 2–4) · Canonical model, OKLCH ramps, APCA solver, ΔE-OK clustering, projection runtime, **Tier 0 adapters**, golden fixtures.
- **M3** (wk 6–9) · **Tier 1 adapters** + loss-report UI. This is where cardinality mismatch gets real.
- **M5** (wk 9–12) · **Ingestion & audit.** Parse an existing `tailwind.config.js` / `theme.ts` back into tokens; critique it; fix; re-export. Flips the product from _generator_ to _migration and audit tool_ — which works on projects that already exist, i.e. almost all of them.

### Track B — Web app & live theming

- **M2** (wk 4–6) · **First shippable.** Generator UI, live preview, export.
- **M6** (wk 12–16) · ~15 Ark-based components served as `registry-item.json`. shadcn's registry schema is open, so `npx shadcn add https://demystify.dev/r/button.json` works with no CLI of our own.
- **M7** (wk 16–20) · AST-based per-component export. String templating collapses under its own weight — parse TSX, codemod, print with recast, format with Prettier, all in a Web Worker.
- **M8** · Figma **plugin** (not the REST API — see below).

### Track C — Docs & guides (continuous from wk 2)

Ships alongside M2, not after. Dev-guide, code-style-guide, and the **algorithm explanations** — _"merged these 3 greys, ΔE-OK 1.2 < 2.0 threshold"_, _"raised L to 0.71 to reach Lc 78 against your surface."_

### Track D — Test infrastructure (from M1, non-negotiable)

1. **Golden-file snapshot tests** — committed expected output per adapter per version. `fixtures/` is public;
   it doubles as credibility and as the contribution guide.

2. **Visual regression** — render a fixture page in each library with the generated theme, screenshot, diff.
   Without this you will not find out you broke Mantine 7 until a user tells you.

---

## Non-negotiables

**No component may contain a hardcoded color, radius, or spacing value.** Every visual property resolves through a CSS custom property on `:root`. Hold that line and the entire site becomes a live preview for free — no iframe sync, no rebuild, no sandbox. Break it once and you'll be building a postMessage bridge for the rest of the project. Enforce with a lint rule from M0.

**Accessibility is a computed guarantee, not a badge.** Every theme ships an APCA matrix — every semantic foreground against every semantic background. Export **fails** if a pairing declared as body text drops below `Lc 75`.

**Theme state is URL-encodable.** Sharing works with the database down.

**The Figma path is the plugin, not the REST API.** The Variables REST API is Enterprise-plan-gated; the plugin API has no plan restriction and reaches far more users.

**License audit from week one.** Emitting theme objects shaped for MUI/Ant/Chakra is config data and fine. Shipping component source derived from shadcn, Radix, or Ant (all MIT) requires retained notices — and "convert my shadcn button to CSS modules" distributes derivative MIT code. `LICENSES.md` costs an hour now and a rewrite later.

---

## Deployment

The images CI builds are the images that ship — no separate production build path, which is
what makes dev/prod parity real rather than aspirational.

| Piece      | Artifact                          | Target                                                      |
| ---------- | --------------------------------- | ----------------------------------------------------------- |
| `apps/web` | Next.js standalone image (~120MB) | Fly.io or Railway                                           |
| `services` | distroless image (~15MB)          | same                                                        |
| Postgres   | **managed**                       | the compose Postgres is a dev convenience, never production |

Images are tagged by commit SHA. Promotion between environments is a **tag change, never a
rebuild** — a rebuild is a different artifact and voids the parity above.

---

## What actually kills this project

Not the build — the **maintenance surface**. 12 libraries × 2 live majors = 24 adapters, each breaking on someone else's release schedule. That's why the declarative adapter format and golden files are _prerequisites for M1_, not cleanup. The goal is to reach twelve libraries without personally maintaining twelve libraries.

**Scope honesty:** this describes roughly four products. M1 + M2 + Track C is the only combination that stands alone as a shippable thing.
If the timeline compresses, cut M6/M7/M8 — not Track D.

---

## Verification

```bash
pnpm test                                # ramp math, APCA solver, ΔE clustering — native
pnpm test:golden                         # text fixtures, byte-exact — native
docker compose run --rm api go test ./...
docker compose run --rm visual           # screenshot diff — MUST be containerized
docker compose run --rm visual --update  # regenerate goldens
```

Golden **text** fixtures are deterministic across platforms and run natively — keep them fast.
Only **screenshots** need the container, because font rasterization is what varies.

**Adapter correctness.** Golden files prove _stability_; only this proves _correctness_ — export a theme, drop it into a scratch Vite app using the real library, confirm it compiles and renders with the right colors. Repeat per Tier 1 adapter.

**Live theming (M2 exit).** Drag the hue slider and verify in devtools that **only `:root` custom properties mutate**. Any hardcoded hex in the rendered CSS blocks the milestone.

**Round-trip (M5 exit).** Ingest a real repo's Tailwind config → audit → apply fixes → export back to `tailwind@4` → diff against the original. Differences must be exactly the reported fixes, nothing else.

**Registry (M6 exit).** In a throwaway Next.js app, `npx shadcn add http://localhost:3000/r/button.json` must install, compile, and pick up the theme's CSS variables with no manual wiring.
