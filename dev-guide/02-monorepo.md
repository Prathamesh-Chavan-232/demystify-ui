# 02 · The monorepo

pnpm workspaces, Turborepo, and `go.work` — three tools that each solve a different
problem and are easy to confuse.

← [01 The scaffold](01-the-scaffold.md) · next → [03 Containers](03-containers.md)

---

## Do this

```bash
pnpm install                      # link every workspace package
pnpm turbo run build              # build everything, both toolchains
pnpm turbo run build              # again — should say FULL TURBO, ~15ms

pnpm turbo run test --filter=@demystify/engine     # one package
pnpm turbo run build --dry=json                    # what WOULD run, with hashes
```

Adding a dependency:

```bash
pnpm add culori --filter @demystify/engine     # to one package
pnpm add -Dw prettier                          # to the workspace root
```

`-w` means "the workspace root". It fails if `pnpm-workspace.yaml` does not exist yet
— which is why the order of the two commands matters when bootstrapping.

---

## Three tools, three jobs

They are not alternatives to each other. Each does one thing:

| Tool                | Job                                                                      | Config                |
| ------------------- | ------------------------------------------------------------------------ | --------------------- |
| **pnpm workspaces** | Links packages together so they can import each other without publishing | `pnpm-workspace.yaml` |
| **Turborepo**       | Decides what to run, in what order, and what can be skipped              | `turbo.json`          |
| **go.work**         | Tells Go tooling where the Go module lives                               | `go.work`             |

pnpm never decides _when_ to build. Turborepo never installs anything. If you
remember only that split, most confusion goes away.

---

## Why one repo at all

PLAN.md gives the deciding reason: **an adapter change and its golden files must land
in one commit, or CI cannot tell a fix from a regression.**

Consider the alternative. Adapters in one repo, fixtures in another. You change how
Mantine's ramp is resampled; the output legitimately changes. Now the fixtures repo
is red. Did you fix something or break something? The only way to know is to correlate
two commit histories by timestamp — and reviewers will not, so red becomes normal, and
a real regression sails through.

One repo, one commit, one reviewable diff: _"here is the behaviour change, and here is
exactly what it did to every golden file."_

The cost is that you now need tooling to avoid rebuilding everything on every change.
That tooling is Turborepo.

---

## pnpm workspaces

### What linking actually does

When `packages/engine` declares `"@demystify/tokens": "workspace:*"`, pnpm creates a
symlink from `packages/engine/node_modules/@demystify/tokens` to `packages/tokens`.
Imports resolve to your local source. Edit `tokens`, and `engine` sees it immediately
— no publish, no build step in between.

That symlink is also the reason the Next.js `standalone` Docker build needed
`node-linker=hoisted`: Next traces the real files behind symlinks, and pnpm's default
layout confused it. See [ch 03](03-containers.md).

### The catalog

```yaml
catalog:
  typescript: 7.0.2
  vitest: 4.1.11
```

A package writes `"typescript": "catalog:"`. Every package gets 7.0.2. To bump, edit
one line here. Without a catalog, seven packages drift to seven TypeScript versions
and you eventually debug a type error that only reproduces in one of them.

### Why `services` is a member

`services` is Go. It has no npm dependencies. It is in the workspace anyway, and this
is the least obvious decision in the repo.

Turborepo only runs a task in packages that define that task's script. `services` had
no `package.json`, so it was not a package, so **no package owned the Go build**.
`turbo run build` skipped it entirely and exited 0 — a green build that built no Go.

Giving `services` a small `package.json` fixes that:

```json
{
  "name": "@demystify/services",
  "private": true,
  "scripts": {
    "build": "go build -o bin/ ./cmd/...",
    "test": "go test ./...",
    "lint": "go vet ./..."
  }
}
```

It is not idiomatic Go, and a Go developer will raise an eyebrow. The payoff is that
one command builds and caches both toolchains, and CI has one entry point.

---

## Turborepo

### Tasks and where they live

`turbo.json` **declares** tasks. Packages **implement** them. A task with no
implementing package silently does nothing.

```jsonc
"build": {
  "dependsOn": ["^build"],
  "outputs": ["dist/**", ".next/**", "!.next/cache/**", "bin/**"]
}
```

- `^build` (with caret) — build my _dependencies_ first.
- `build` (no caret) — my own `build` first. Used by `test`, so tests run against
  fresh output.
- `outputs` — what to save and restore on a cache hit. Miss one and a cache hit
  restores an incomplete build.
- `!.next/cache/**` — an exclusion. Without it you cache Next's entire internal build
  cache, and the artifact balloons.

### Root-only tasks need `//#`

A script in the _root_ `package.json` is not reachable as a plain task name. It needs
the `//#` prefix:

```jsonc
"//#lint:root": {}
```

This is a real trap: put `go:build` in the root `package.json`, declare `"go:build"`
in `turbo.json`, run it, and turbo reports success having done nothing at all.

### Package configs

A package can carry its own `turbo.json` that extends the root. `services/turbo.json`
does this:

```json
{
  "extends": ["//"],
  "tasks": {
    "build": { "inputs": ["**/*.go", "go.{mod,sum}"], "outputs": ["bin/**"] }
  }
}
```

Because the task now lives _in_ the package, the globs are package-relative and
correct.

---

## The most important thing in this chapter

### Hashing, and how it silently fails

Before running a task, Turborepo hashes: files matching `inputs`, the hashes of the
package's dependencies, declared env vars, and global dependencies. Same hash ⇒ skip
and restore from cache.

`inputs` globs are **relative to the package's `package.json`**. This is the whole
problem.

Consider a task in `packages/adapters` declaring:

```jsonc
"test:golden": { "inputs": ["fixtures/**"] }
```

`fixtures/` is at the repo _root_. Relative to `packages/adapters`, that glob resolves
to `packages/adapters/fixtures/**` — which does not exist. It matches **zero files**.

A hash computed over zero files is a perfectly valid, perfectly constant hash. So:

1. You edit a golden fixture.
2. The hash does not move.
3. Turborepo reports a cache hit and replays the previous **pass**.
4. Your golden tests never run again, and never tell you.

This is worse than no caching. It is a test suite that reports green while testing
nothing — and Track D, the thing PLAN.md says never to cut, quietly dies.

### The two fixes

**`$TURBO_ROOT$`** makes a glob repo-root-relative:

```jsonc
"test:golden": {
  "inputs": ["$TURBO_DEFAULT$", "$TURBO_ROOT$/fixtures/**"]
}
```

`$TURBO_DEFAULT$` means "keep the normal behaviour too" — otherwise you replace the
defaults instead of adding to them.

**Or make `fixtures/` a real package** that `adapters` depends on. Its hash then flows
through the dependency graph automatically, with no globs to get wrong. Better
long-term, and it fits the plan to publish fixtures as documentation.

### Why `test:golden` currently has `cache: false`

Because none of the above has been _proven_ yet. `cache: false` is the honest
position until you can demonstrate that editing a fixture moves the hash. Turn caching
on only after the exercise below passes for `test:golden`.

> **Rule of thumb.** Caching a test is a claim that you know exactly what the test
> depends on. If you are not sure, `cache: false`. A slow correct test beats a fast
> lie.

---

## go.work

```
go 1.26.5
use ./services
```

Go's unit of dependency is the **module**, defined by a `go.mod`. Ours is at
`services/`, not the repo root, so from the root `go build ./...` finds nothing.

`go.work` is the fix. It tells Go tooling "the modules in this tree are here", so
editors and language servers work from the root. With one module it is close to
ceremonial; it becomes load-bearing the day a second module appears.

Practical consequence: **run `go` commands from `services/`**, or pass paths. See
[ch 04](04-go-from-zero.md).

---

## Exercise

### Part 1 — watch a hash respond

```bash
pnpm turbo run build --dry=json | grep -A1 services
```

Note the hash for `@demystify/services#build`. Then:

```bash
echo "// scratch" >> services/internal/httpx/httpx.go
pnpm turbo run build --dry=json | grep -A1 services     # hash MOVED
sed -i '$ d' services/internal/httpx/httpx.go
pnpm turbo run build --dry=json | grep -A1 services     # hash RESTORED
```

Confirm that `@demystify/web#build` did **not** change at any point.

<details>
<summary>What you just proved</summary>

Two things. First, the Go build is genuinely wired into Turborepo's hashing — a `.go`
edit invalidates it. Second, invalidation is _scoped_: web's hash is untouched, so
editing Go does not force a web rebuild.

This is the check to run whenever you add a task. If the hash does not move when you
edit a file the task depends on, the task's caching is broken.
</details>

### Part 2 — break it on purpose

Add a deliberately wrong task to `services/turbo.json`:

```jsonc
"broken": { "inputs": ["services/**/*.go"], "outputs": [] }
```

Add `"broken": "echo ran"` to `services/package.json` scripts. Now edit a `.go` file
and check whether `@demystify/services#broken` changes hash.

<details>
<summary>Answer</summary>

It does not. The task lives in `services/`, so `services/**/*.go` resolves to
`services/services/**/*.go` — zero matches, constant hash, permanent cache hit.

Correct it to `**/*.go` and the hash starts responding.

**Remember to remove the `broken` task and script afterwards.** This is precisely the
bug that was in this repo's original `turbo.json`, and the reason golden tests are
uncached until proven otherwise.
</details>

---

← [01 The scaffold](01-the-scaffold.md) · next → [03 Containers](03-containers.md)
