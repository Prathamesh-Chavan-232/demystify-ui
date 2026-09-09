# 01 · The scaffold

What is in this repo, and what each piece is for.

← [index](dev-guide.md) · next → [02 The monorepo](02-monorepo.md)

---

## Do this

Get oriented in three commands:

```bash
# What does pnpm think exists?
pnpm list --depth -1 --recursive

# What does Turborepo think it can run?
pnpm turbo run build --dry=json | head -40

# What is actually running?
docker compose ps
```

The repo has **12 workspace projects**: 7 stub packages, 2 Next.js apps, the Go
services module, the visual-test harness, and the root.

---

## The tree

```
demystify-ui/
├── PLAN.md                  ← decisions. read this first, and often
├── dev-guide/               ← you are here
│
├── package.json             ← root. name: demystify-monorepo, private
├── pnpm-workspace.yaml      ← which directories are workspace members
├── turbo.json               ← the task graph for BOTH toolchains
├── tsconfig.base.json       ← TS options every package extends
├── go.work                  ← points Go at ./services
│
├── compose.yaml             ← dev topology, 6 services
├── compose.ci.yaml          ← CI overrides
├── .dockerignore            ← essential; see ch 03
├── .env.example             ← HOST-form DATABASE_URL
├── LICENSES.md              ← MIT notice obligations, tracked from week one
│
├── packages/                ← the library. all 7 are stubs today
│   ├── tokens/              canonical model, DTCG in/out, zod schemas
│   ├── engine/              ramps, APCA, ΔE-OK, projection runtime
│   ├── adapters/            declarative specs + escape-hatch code
│   ├── ingest/              parse tailwind.config.js / theme.ts → tokens
│   ├── codemod/             ts-morph transforms for component export
│   ├── ui/                  Ark-based components
│   └── cli/                 npx demystify  (package name: "demystify")
│
├── apps/
│   ├── web/                 generator, live preview, export
│   └── docs/                fumadocs; dev-guide + code-style-guide ship here
│
├── services/                ONE Go module, three binaries
│   ├── go.mod               module github.com/…/demystify-ui/services
│   ├── package.json         ← so Turborepo owns the Go tasks. see ch 02
│   ├── turbo.json           ← package-relative inputs/outputs for Go
│   ├── cmd/{api,palette,audit}/
│   ├── internal/httpx/      shared server + self-healthcheck
│   └── migrations/          goose .sql files
│
├── docker/
│   ├── postgres/init.sql    runs once, on first volume init
│   └── goose/Dockerfile     migration runner
│
├── fixtures/                golden files. public; doubles as contribution guide
│   ├── canonical/           input token sets
│   └── expected/            expected output per adapter@version
│
└── tests/visual/            Playwright render + screenshot diff
    ├── Dockerfile           digest-pinned. see ch 03
    ├── fixtures/            HTML pages driven purely by :root custom properties
    └── specs/
```

---

## Why it works this way

### Why the 7 packages exist before they contain anything

They are stubs — each is a `package.json`, a `tsconfig.json`, and a one-line
`src/index.ts`. That is deliberate. Creating the boundary early forces you to decide
_where code goes_ before you have code, and the alternative — one big package you
split later — means every import path changes on the day you split it.

The split follows the data flow:

```
ingest ──→ tokens ──→ engine ──→ adapters ──→ output
(parse)   (model)    (math)     (project)
                         │
              codemod ───┴──→ ui, cli
```

`tokens` is the shared vocabulary and depends on nothing. `engine` does the maths on
that vocabulary. `adapters` projects the result outward. Nothing upstream ever
imports something downstream — if you find yourself wanting `tokens` to import
`adapters`, the model is wrong.

### Why `services` is one Go module and not three

Three `go.mod` files would need to stay in version-sync with each other forever, and
buy nothing at this size. One module, three `cmd/` directories, three binaries out of
one image. PLAN.md fixes this; ch 04 covers what a module actually is.

### Why `services` has a `package.json`

It is Go, so this looks wrong. It exists so Turborepo treats `services` as a
first-class package and can therefore cache the Go build correctly. Full reasoning in
[ch 02](02-monorepo.md) — this is the single least obvious file in the repo.

### Why `fixtures/` sits at the root rather than inside a package

Golden files are consumed by `packages/adapters` but are also **public
documentation** — PLAN.md wants them to double as the contribution guide. A
contributor adding a library reads `fixtures/expected/` to learn the shape of the
job.

That placement has a cost: a root-level directory is outside every package, so
Turborepo cannot see it through normal `inputs`. That is exactly the trap described
in [ch 02](02-monorepo.md), and why `test:golden` currently runs with `cache: false`.

### Why the root package is named `demystify-monorepo`

Because `packages/cli` is named `demystify` — it ships to npm as `npx demystify`, so
it gets the good name. Two packages with the same name make Turborepo conflate them
in its task graph, which produced a genuinely confusing failure before it was renamed.

---

## Reading the four config files as a set

They are easier to understand together than separately, because each one answers a
question the previous one raises.

**`pnpm-workspace.yaml`** — _what are the members?_

```yaml
packages:
  - 'packages/*'
  - 'apps/*'
  - 'services' # ← the Go module, deliberately included
  - 'tests/visual'

catalog: # one place to pin shared tool versions
  typescript: 7.0.2
  vitest: 4.1.11
  '@playwright/test': 1.58.0
```

`catalog:` means a package writes `"typescript": "catalog:"` and gets the pinned
version. Versions can never drift between packages, and bumping one is a one-line
change here.

**`turbo.json`** — _what can be run, and in what order?_

Task definitions only. It does not implement anything; each package supplies the
actual script.

**`tsconfig.base.json`** — _what TypeScript rules apply?_

Every package's `tsconfig.json` extends it and adds only `outDir`/`rootDir`. Strict
mode plus `noUncheckedIndexedAccess`, which matters when you are indexing colour ramps
by step and a typo would otherwise hand you `undefined` at runtime.

**`go.work`** — _where is the Go code?_

```
go 1.26.5
use ./services
```

Two lines. It tells Go tooling that the module lives in `services/`, so editors and
`go` commands work from the repo root.

---

## Exercise

**Predict, then verify.** You want to add `packages/registry` (it will serve
shadcn-conformant `registry-item.json` in M6).

Before running anything, write down: which files must change, and which are created
automatically?

<details>
<summary>Answer</summary>

**Nothing needs to change.** `pnpm-workspace.yaml` already globs `packages/*`, so the
new directory is picked up on the next install. You create three files inside it:

```bash
mkdir -p packages/registry/src
# package.json, tsconfig.json, src/index.ts — copy the shape of packages/tokens
```

Then:

```bash
pnpm install                              # links it; "13 workspace projects"
pnpm turbo run build --dry=json | grep registry
```

The dry run should show `@demystify/registry#build`. If it does not, the package is
missing a `build` script — Turborepo only runs tasks that a package actually defines,
which is the single most common source of "turbo did nothing".

**The trap:** if you name it `"registry"` rather than `"@demystify/registry"`, it
still works, but you have burned an unscoped npm name. Every package here is scoped
except `cli`, which is unscoped on purpose.
</details>

---

← [index](dev-guide.md) · next → [02 The monorepo](02-monorepo.md)
