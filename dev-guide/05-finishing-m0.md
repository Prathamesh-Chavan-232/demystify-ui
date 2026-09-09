# 05 · Finishing M0

Three open gaps, and one case study of a gap that was closed. **This is the chapter to
actually do**, not just read.

← [04 Go from zero](04-go-from-zero.md) · next → [06 M1: the engine](06-m1-engine.md)

---

## Do this

```
Gap 1  apps/docs/Dockerfile broken .................. FIXED — read as a case study
Gap 2  docs image missing from CI .................. yours
Gap 3  CI has never run ............................ yours
Gap 4  hardcoded-value lint rule not written ....... yours, and the interesting one
```

Work them in order. Gap 3 will surface problems in gap 2, and gap 4 is easier once CI
is proving itself.

---

## Gap 1 (fixed) — a case study in generated code

`apps/docs/Dockerfile` was produced by copying `apps/web/Dockerfile` and substituting
`web` → `docs`. It had never been built. It carried three defects:

**1. A stale comment.** Line 2 still said "not apps/web". Harmless, but a reliable
signal that a file was mechanically derived and not read afterwards.

**2. A `COPY` with no source.**

```dockerfile
COPY --from=builder /repo/apps/docs/public ./apps/docs/public
```

`create-next-app` generates `apps/web/public/`. `create-fumadocs-app` does **not**
generate `apps/docs/public/`. Docker fails hard when a `COPY` source does not exist,
so the build could never have succeeded.

The fix was to create `apps/docs/public/.gitkeep` rather than delete the `COPY` line.
Deleting it would work today and silently break the day someone adds a logo — a 404 in
production with no error anywhere in the build. Creating the directory keeps the two
app Dockerfiles symmetrical, which matters precisely because one was generated from
the other.

**3. A missing fix that its source had gained.** `apps/web/Dockerfile` was patched with
`--config.node-linker=hoisted` _after_ the docs copy was taken. So docs would have
built and then crashed at runtime on `MODULE_NOT_FOUND` for `@swc/helpers` — see
[ch 03](03-containers.md).

> **The lesson worth keeping.** Defect 3 is the dangerous class: a file cloned from
> another, which then diverges. There is no link between them, so the fix does not
> propagate and nothing reports the drift. When you copy a file, either factor out the
> shared part or write down that the copy exists.

Verify it now works:

```bash
docker build -f apps/docs/Dockerfile -t demystify/docs:test .
docker run --rm -p 3101:3000 demystify/docs:test &
curl -o /dev/null -w '%{http_code}\n' http://localhost:3101/docs    # 200
```

---

## Gap 2 — CI must build every image

`.github/workflows/ci.yml` builds services, web, and migrate. M0's exit criterion says
_every_ image. Docs is missing, and so is visual.

**Your task:** add the docs image to the `images` job.

<details>
<summary>Answer</summary>

In the `images` job, after the web step:

```yaml
- name: Build docs image
  uses: docker/build-push-action@v6
  with:
    context: .
    file: apps/docs/Dockerfile
    tags: demystify/docs:${{ github.sha }}
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

`context: .` is the repo root, not `apps/docs` — `turbo prune` needs the whole
workspace. Getting this wrong produces a confusing "package not found" during prune.

**Consider whether `visual` belongs here too.** It is built by the `stack` job's
compose invocation only when that job runs it. Building it in `images` costs a slow
3.4 GB pull for a check that the `stack` job already performs. Reasonable either way —
but decide deliberately rather than by omission, and if you leave it out, say so in a
comment. PLAN.md's rule is _no silent caps_.
</details>

---

## Gap 3 — CI has never run

There are no commits and no remote. `ci.yml` is a plausible file, not a passing build.
Until it runs, "CI green on both toolchains" is an assertion, not a fact.

**Your task:** commit, push, watch it, fix what breaks.

```bash
git add -A
git commit -m "M0: monorepo scaffold, both toolchains, compose stack"
gh repo create demystify-ui --private --source=. --push
gh run watch
```

**Expect it to fail the first time.** That is the point of the exercise. Likely
causes, in rough order of probability:

- **`pnpm install --frozen-lockfile` fails.** The lockfile must be committed and match
  every `package.json` exactly. If you edited a manifest by hand without re-running
  `pnpm install`, this is where you find out.
- **`services/go.sum` does not exist.** `cache-dependency-path: services/go.sum` in
  the Go job refers to a file that only appears once you have a real dependency. Today
  the module has none.
- **`pnpm format:check` fails.** Prettier is stricter about files you have edited by
  hand than about the ones it generated.
- **The `stack` job times out** waiting for health. Runners are slower than your
  laptop; `start_period` may need raising.

<details>
<summary>On the go.sum problem specifically</summary>

`actions/setup-go`'s `cache-dependency-path` fails when the file is absent. Options:

1. Make the path tolerant — `cache-dependency-path: services/go.sum` only once a
   dependency exists; until then, drop the `cache` config from the Go job.
2. Add the first real dependency now (you will need `pgx` shortly anyway).

Option 1 is honest for today's state. Note that whichever you pick, this is a
_sequencing_ artefact — the workflow was written for the repo as it will be, not as it
is. Worth checking the rest of `ci.yml` for the same assumption.
</details>

---

## Gap 4 — the hardcoded-value lint rule

The one that takes thought. PLAN.md is unambiguous:

> **No component may contain a hardcoded color, radius, or spacing value.** Every
> visual property resolves through a CSS custom property on `:root`. Hold that line
> and the entire site becomes a live preview for free — no iframe sync, no rebuild, no
> sandbox. Break it once and you'll be building a postMessage bridge for the rest of
> the project. **Enforce with a lint rule from M0.**

There is currently a Playwright assertion in `tests/visual/specs/swatches.spec.ts`
that checks one fixture page. That is not the same guarantee: it covers a single file
and only catches violations that reach a rendered stylesheet.

### Why this rule earns its keep

The entire live-preview architecture rests on it. If every visual property resolves
through `:root`, then changing a theme is one `style.setProperty` call and the whole
page re-themes instantly. One hardcoded `#3b82f6` in one component and that stops
being true — so you build an iframe, then a postMessage bridge to sync it, and the
architecture is now permanently more complex because of a single hex code nobody
noticed in review.

Catching it costs a lint rule. Not catching it costs the rest of the project.

### Where violations actually appear

Three distinct surfaces, and they need different tools:

| Surface                          | Example                     | Tool      |
| -------------------------------- | --------------------------- | --------- |
| CSS / Tailwind `@theme`          | `color: #3b82f6`            | stylelint |
| Tailwind arbitrary values in JSX | `className="bg-[#3b82f6]"`  | ESLint    |
| Inline style objects             | `style={{ color: '#fff' }}` | ESLint    |

**Your task:** implement enough of this to fail on a deliberately planted hex, in both
a `.css` and a `.tsx` file.

<details>
<summary>A working starting point</summary>

**CSS — stylelint.** Add to the workspace and configure:

```bash
pnpm add -Dw stylelint stylelint-config-standard
```

`.stylelintrc.json`:

```jsonc
{
  "extends": ["stylelint-config-standard"],
  "rules": {
    "declaration-property-value-allowed-list": {
      "/^(color|background-color|border-color|fill|stroke|outline-color)$/": [
        "/^var\\(--/",
        "transparent",
        "currentColor",
        "inherit",
        "none",
      ],
      "/^(border-radius|gap|padding|margin)$/": ["/^var\\(--/", "0", "auto", "inherit"],
    },
  },
  "overrides": [
    {
      // The token definition itself is where real values are ALLOWED to live.
      "files": ["**/tokens.css", "**/*.tokens.css"],
      "rules": { "declaration-property-value-allowed-list": null },
    },
  ],
}
```

Custom properties (`--ds-bg: oklch(…)`) are exempt automatically: the property name is
`--ds-bg`, which matches none of the patterns above. So the `:root` block passes
without needing a special case, and only _consumers_ are constrained.

**JSX — ESLint.** A full custom rule is not needed for M0; `no-restricted-syntax`
covers it:

```js
// eslint.config.mjs
{
  files: ['packages/ui/**/*.tsx', 'apps/web/src/**/*.tsx'],
  rules: {
    'no-restricted-syntax': [
      'error',
      {
        selector: 'Literal[value=/#[0-9a-fA-F]{3,8}\\b/]',
        message:
          'Hardcoded color. Use a CSS custom property: var(--ds-*). See PLAN.md Non-negotiables.',
      },
      {
        selector: 'Literal[value=/\\b(rgb|rgba|hsl|hsla|oklch|lab)\\(/]',
        message: 'Hardcoded color function. Use var(--ds-*).',
      },
    ],
  },
}
```

**Wire it into the task graph** so CI runs it — add a `lint` script to the packages it
covers; `turbo.json` already declares the `lint` task.

**Verify it actually fails.** A lint rule nobody has seen fail is not known to work:

```bash
echo '.x { color: #ff0000; }' >> apps/web/src/app/globals.css
pnpm turbo run lint            # must fail
git checkout apps/web/src/app/globals.css
```

</details>

### The trade-off to get right

**A rule that is too strict gets disabled within a week, and that is worse than no
rule.** Once someone adds `/* stylelint-disable */` to unblock a release, the
disable-comment becomes precedent and the rule is decorative.

So aim for a rule that is _correct on the cases that matter_ and quiet elsewhere:

- Scope it to component code. Marketing pages, `og:image` routes, and email templates
  do not need to be live-themeable.
- Allow an escape hatch that is **greppable and explained** — an inline disable
  requiring a reason comment. Then `grep -r "stylelint-disable"` is your honest count
  of exceptions, and it can be reviewed rather than accumulating silently.
- Start with colours only. Radius and spacing produce far more noise per real bug;
  add them once colour is clean.

---

## M0 exit checklist

Re-check all of it, not only what you changed:

```bash
docker compose up -d --wait                              # all healthy
pnpm db:up                                               # migrations apply
docker compose --profile tools run --rm visual           # baseline passes
pnpm turbo run build typecheck test lint                 # all green
cd services && go vet ./... && go test ./...             # Go clean
gh run watch                                             # CI green on a real push
```

- [ ] `docker compose up -d` brings the stack healthy
- [ ] `migrate up` applies
- [ ] `visual` produces (and re-verifies) a baseline
- [ ] CI builds **every** image, docs included
- [ ] CI is green on both toolchains, on a real push
- [ ] The lint rule exists and has been **seen to fail** on a planted violation

When all six hold, M0 is genuinely done and [ch 06](06-m1-engine.md) is the critical
path.

---

← [04 Go from zero](04-go-from-zero.md) · next → [06 M1: the engine](06-m1-engine.md)
