# dev-guide

A builder's guide to this repository — how it works, and how to learn the parts of it
you have not met before.

> **Who this is for.** You, building demystify-ui by hand. You are fluent in
> TypeScript, new to Go, and new to designing a multi-language monorepo. The scaffold
> in this repo was generated quickly; this guide exists so that none of it stays a
> black box.
>
> **This is not the product's dev-guide.** [PLAN.md](../PLAN.md)'s Track C reserves
> that name for a doc shipped to _users_ of demystify, which will live in
> `apps/docs/content/`. Different audience, same word. This one never ships.

---

## How to read this

Every chapter has the same three layers. Use whichever you need today.

| Layer | Heading                   | Use it when                                            |
| ----- | ------------------------- | ------------------------------------------------------ |
| 1     | **Do this**               | You want the commands and nothing else. ~2 minutes.    |
| 2     | **Why it works this way** | Something broke, or you are about to change it.        |
| 3     | **Exercise**              | You want the knowledge to stick. Answers are included. |

Read straight through to learn. Skip to **Do this** to move.

## Division of labour with PLAN.md

[PLAN.md](../PLAN.md) is the decision record: _what_ was chosen and _why_, plus scope
and roadmap. It is the source of truth for decisions and this guide never overrides
it.

This guide is the operating manual: _how_ the decisions are wired together, and how
to work on them. Where they would overlap, this guide links.

---

## Chapters

| #   | Chapter                            | What you get                          |
| --- | ---------------------------------- | ------------------------------------- |
| —   | you are here                       | orientation, glossary, progress       |
| 01  | [The scaffold](01-the-scaffold.md) | what every file in the repo is for    |
| 02  | [The monorepo](02-monorepo.md)     | pnpm workspaces, Turborepo, `go.work` |
| 03  | [Containers](03-containers.md)     | compose, images, goose migrations     |
| 04  | [Go from zero](04-go-from-zero.md) | Go for a TypeScript developer         |
| 05  | [Finishing M0](05-finishing-m0.md) | the gaps that are still open          |
| 06  | [M1: the engine](06-m1-engine.md)  | ramps, APCA, ΔE-OK, adapters          |

### Suggested order

Chapters 01–03 are orientation and can be read in an afternoon. **Chapter 05 is the
first thing you should actually do** — it is small, it is real, and it closes M0.
Chapter 04 can wait until you touch Go, which chapter 05 does not require. Chapter 06
is the critical path and the longest by far.

```
01 ─→ 02 ─→ 03 ─→ 05 (do the work) ─→ 06 (build M1)
                 ↘ 04 (when you touch Go)
```

---

## Progress checklist

### M0 — Foundations

- [x] Repo scaffold, 12 workspace projects
- [x] `docker compose up -d` brings the stack healthy
- [x] `migrate up` applies the first migration
- [x] `docker compose run --rm visual` produces a baseline
- [x] Thin schema: `users`, `themes`
- [x] All six images build; web and docs both serve
- [ ] **CI builds every image** — docs is missing → [ch 05](05-finishing-m0.md)
- [ ] **CI green on both toolchains** — never run → [ch 05](05-finishing-m0.md)
- [ ] **Hardcoded-value lint rule** — not written → [ch 05](05-finishing-m0.md)

### M1 — Engine & Tier 0 adapters

- [ ] Canonical token model (`packages/tokens`)
- [ ] OKLCH ramps with lightness-curve fitting (`packages/engine`)
- [ ] APCA solver via Newton-Raphson (`packages/engine`)
- [ ] ΔE-OK clustering (`packages/engine`)
- [ ] Projection runtime + `LossReport` (`packages/engine`)
- [ ] Tier 0 adapters (`packages/adapters`)
- [ ] Golden fixtures, with `test:golden` caching proven correct

---

## Glossary

Terms that recur everywhere and are never defined in PLAN.md.

**Workspace** — the whole monorepo as pnpm sees it. Defined by
`pnpm-workspace.yaml`. Currently 12 projects.

**Package** — one member of the workspace: a directory with its own `package.json`.
`packages/engine`, `apps/web`, and `services` are all packages, even though the last
one is Go.

**Task** — a named script (`build`, `test`) that Turborepo can run. Defined in
`turbo.json`, implemented as a script in each package's `package.json`. A task only
runs in packages that actually define that script.

**Task graph** — the dependency ordering Turborepo derives from `dependsOn`. `^build`
means "the `build` of my dependencies first"; plain `build` means "my own `build`
first".

**Hash** — the fingerprint Turborepo computes per task from its inputs, its
dependencies' hashes, and declared env vars. Same hash ⇒ cache hit ⇒ the task does
not run. **A wrong `inputs` glob produces a hash that never changes**, which is the
subtlest bug in this repo's tooling. See [ch 02](02-monorepo.md).

**Ramp** — a continuous perceptual curve through colour space, sampled at whatever
cardinality a target needs. _Not_ a fixed array of shades. Storing the curve rather
than the samples is, per PLAN.md, the single most important decision in the repo.

**Cardinality** — how many steps a target library's colour scale has. MUI wants 4,
Mantine 10, Tailwind 11, Radix 12. One ramp serves all of them.

**Projection** — taking the canonical token model and emitting it for one specific
target library. Lossy by nature, which is why it is called a projection.

**Loss report** — the typed record of what a projection could not preserve. Every
projection returns `{ output, loss }`. Rendered as UI, never hidden in a console
warning.

**Adapter** — a declarative spec describing one target library: its cardinality,
sampling strategy, token name mapping, and declared losses. Data, not code, so
contributors can add libraries without touching the engine.

**Golden file** — committed expected output for a given input. The test asserts
byte-exactness. Proves _stability_: that today's build produces what yesterday's did.

**Tier** — how hard a target library is to support. Tier 0 (shadcn, Tailwind, …) is
nearly free; Tier 3 (Bootstrap SCSS, Vuetify) needs the user's build step. See
PLAN.md's Adapter tiers table.

---

## Getting unstuck

**Turbo did nothing and exited 0.** No package defines that task's script. Check with
`pnpm turbo run <task> --dry=json`. See [ch 02](02-monorepo.md).

**A change did not trigger a rebuild.** Your `inputs` glob probably matches nothing.
Hashes are in `--dry=json`; if editing a file does not move the hash, the glob is
wrong. See [ch 02](02-monorepo.md).

**`docker` says permission denied.** Your login session predates your `docker` group
membership. Log out and back in, or prefix with `sg docker -c '…'`.

**A container writes root-owned files into the repo.** It is missing a `user:` entry
in `compose.yaml`. See [ch 03](03-containers.md).

**Postgres will not start.** Check the volume mount — Postgres 18 requires a single
mount at `/var/lib/postgresql`, not `/var/lib/postgresql/data`.

**`go` commands fail outside `services/`.** Go's module root is `services/`, not the
repo root. `go.work` at the root points at it. See [ch 04](04-go-from-zero.md).
