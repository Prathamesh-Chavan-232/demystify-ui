# 06 · M1 — the engine

Ramps, APCA, ΔE-OK clustering, projection, and the first adapters. This is Track A,
the critical path, and the longest chapter here.

← [05 Finishing M0](05-finishing-m0.md) · [index](dev-guide.md)

---

## Do this

Build in this order. Each step is testable before the next exists.

```
1. tokens    canonical model + zod schemas          ← no maths yet
2. engine    OKLCH ramp sampling                    ← first real output
3. engine    APCA + Newton-Raphson solver
4. engine    ΔE-OK clustering
5. engine    projection runtime + LossReport
6. adapters  Tier 0 (all five share one formatter)
7. fixtures  golden files, and fix test:golden caching
```

Dependencies you will need:

```bash
pnpm add culori apca-w3 --filter @demystify/engine
pnpm add zod --filter @demystify/tokens
```

> Check exact export names against the installed versions — `culori` and `apca-w3`
> have both moved things between majors. The shapes below are the current common
> usage, not a promise.

---

## The one decision that matters

PLAN.md calls it the single most important decision in the repo, and it is worth
understanding before writing a line:

> Storing a **curve** rather than a fixed array of shades.

Store ten hex values and you have thrown away the information needed to produce nine,
or twelve, or four. Worse, Radix's 12 steps have **assigned meanings** — step 9 is
"solid background", step 11 is "low-contrast text" — and recovering those from ten
arbitrary swatches is guesswork forever.

Store the curve and every target samples what it needs:

```ts
type Ramp = {
  space: 'oklch'
  seed: { l: number; c: number; h: number }
  curve: { lightness: BezierControlPoints; chroma: ChromaCurve }
  sample(n: number): OklchColor[] // 4 (MUI) · 10 (Mantine) · 11 (TW) · 12 (Radix)
  anchors: Record<SemanticRole, number> // 'solid-bg' → 0.62
}
```

`anchors` maps a semantic role to a **position on the curve** (0–1), not to an index.
That is what survives resampling: "solid-bg lives at 0.62 along the ramp" is true at
any cardinality; "solid-bg is index 9" is only true for Radix.

**Test this before building on it.** `sample(12)[9]` and `sample(10)[7]` should land
at nearly the same colour, because they are the same point on the same curve. If they
do not, the sampling strategy is wrong and everything downstream inherits it.

---

## OKLCH, briefly

Three components:

|       | Range  | Meaning                                |
| ----- | ------ | -------------------------------------- |
| **L** | 0–1    | perceptual lightness. 0 black, 1 white |
| **C** | 0–~0.4 | chroma — colourfulness. 0 is grey      |
| **H** | 0–360  | hue angle                              |

**Why not HSL.** HSL's lightness is not perceptual. `hsl(60 100% 50%)` (yellow) and
`hsl(240 100% 50%)` (blue) claim identical lightness and are wildly different to the
eye — yellow is far brighter. Build a ramp by stepping HSL lightness evenly and the
result looks lumpy, with muddy midtones and a jarring yellow.

OKLCH is built on OKLab, designed so that equal numeric steps look like equal
perceptual steps. Step L evenly and the ramp _looks_ even. That property is the entire
reason this project can claim its output is principled rather than tasteful.

**Hue is not perfectly stable across lightness.** Holding H constant while varying L
can drift slightly in appearance (the Abney effect). Worth knowing before you chase a
bug that is actually human vision.

### culori

```ts
import { converter, formatCss, clampChroma, inGamut } from 'culori'

const toOklch = converter('oklch')
const toRgb = converter('rgb')

const c = toOklch('#3b82f6') // { mode: 'oklch', l, c, h }
formatCss(c) // 'oklch(0.62 0.19 259)'
```

### The gamut problem

OKLCH can express colours sRGB cannot display. High chroma at extreme lightness is
the usual offender — there is no such thing as a very light, very saturated blue in
sRGB.

Ignore this and `formatCss` happily emits a colour that browsers clip unpredictably,
so your ramp's ends go flat and your APCA calculations describe a colour nobody sees.

```ts
const safe = clampChroma(color, 'oklch') // reduces C until it fits
```

**Clamping is a loss.** It belongs in the `LossReport`, not swallowed silently. This is
the first place the loss-report discipline pays off, and a good first entry to
implement.

---

## APCA

APCA (Accessible Perceptual Contrast Algorithm) reports **Lc**, roughly −108 to +106.
PLAN.md gates export at **Lc 75** for body text.

Two differences from WCAG 2's contrast ratio that matter:

**It is polarity-sensitive.** Dark-on-light and light-on-dark are _different
calculations_, and swapping foreground and background gives a different magnitude —
which matches how vision actually works. WCAG 2's ratio is symmetric, which is part of
why it misjudges dark themes.

**Argument order is therefore load-bearing.** `APCAcontrast(text, background)`, not
the reverse. Getting it backwards produces plausible numbers that are wrong, and no
error.

```ts
import { APCAcontrast, sRGBtoY } from 'apca-w3'

const lc = APCAcontrast(sRGBtoY(textRgb), sRGBtoY(bgRgb))
```

A negative Lc means light text on a dark background. Compare `Math.abs(lc)` against
your threshold, but keep the sign — it tells you the polarity, which the loss report
should state.

### The accessibility matrix

PLAN.md requires every theme to ship an APCA matrix: every semantic foreground against
every semantic background, with export **failing** below Lc 75 for anything declared as
body text.

Note what that means architecturally: contrast is a **computed property of the theme**,
not a checkbox. The matrix is generated, not authored, and the failure is mechanical.
That is the difference between "accessible" as a claim and as a guarantee.

---

## Newton-Raphson: solving for lightness

The core question: _given this background and this target Lc, what lightness must my
foreground have?_

There is no closed-form inverse of APCA. So solve numerically.

Newton-Raphson iterates toward a root of `f(x) = 0`:

```
x₁ = x₀ − f(x₀) / f′(x₀)
```

Here `f(L) = APCA(colorAtLightness(L), bg) − targetLc`. When `f(L) = 0`, that L hits
the target exactly.

APCA has no analytic derivative, so approximate it:

```
f′(L) ≈ (f(L + h) − f(L − h)) / 2h        with h ≈ 1e-4
```

A sketch:

```ts
function solveLightnessForLc(
  base: Oklch,
  bg: Oklch,
  targetLc: number,
  { tolerance = 0.1, maxIterations = 20 } = {},
): { l: number; achievedLc: number; converged: boolean } {
  let l = base.l
  for (let i = 0; i < maxIterations; i++) {
    const err = lcAt(l) - targetLc
    if (Math.abs(err) < tolerance) return { l, achievedLc: lcAt(l), converged: true }

    const h = 1e-4
    const slope = (lcAt(l + h) - lcAt(l - h)) / (2 * h)
    if (Math.abs(slope) < 1e-6) break // flat — Newton cannot proceed

    l = Math.min(1, Math.max(0, l - err / slope))
  }
  return { l, achievedLc: lcAt(l), converged: false }
}
```

### Where it fails, and why you must handle it

Newton-Raphson is fast — usually 3–5 iterations — and **not unconditionally
convergent**. Three real failure modes:

1. **Flat regions.** APCA soft-clips at low contrast, so the derivative approaches
   zero and the step explodes. Guard on `|slope|`.
2. **Clamping at the boundary.** The target may be unreachable: no lightness gives
   Lc 90 against a mid-grey background. You clamp to 0 or 1 and still miss.
3. **Oscillation.** Rare here, but possible.

**Never return a non-converged result as though it succeeded.** Return
`{ converged: false, achievedLc }` and let the caller record a `LossReport` entry:
_"requested Lc 90 against surface, achieved Lc 78 at maximum lightness."_

That sentence is the product. An LLM competitor cannot produce it, because it did not
do any arithmetic.

**A robust alternative:** bisection is slower but cannot diverge, given a bracketing
interval. A good production shape is Newton with a bisection fallback — and since
`lcAt` is monotonic in L for a fixed background over most of the range, bracketing is
usually easy.

---

## ΔE-OK clustering

This powers _"you have 14 near-duplicate greys"_ — a measurement, not an opinion.

**ΔE-OK** is Euclidean distance in OKLab. Because OKLab is perceptually uniform, that
distance approximates perceived difference:

```
ΔE = √( (L₁−L₂)² + (a₁−a₂)² + (b₁−b₂)² )
```

PLAN.md quotes _"ΔE-OK 1.2 < 2.0 threshold"_, so values are scaled ×100 from OKLab's
native 0–1 range. **Pick one convention and put it in the type name**, because a
threshold of 2.0 in raw units would merge your entire palette into one colour.

```ts
import { differenceEuclidean } from 'culori'
const rawDiff = differenceEuclidean('oklab')
const deltaEOk = (a: Oklch, b: Oklch) => rawDiff(a, b) * 100
```

### Which algorithm

For _deduplication_ — "these are the same colour wearing different hex codes" —
**agglomerative clustering with a distance threshold** is the right tool. You do not
know how many clusters there are, which is exactly what k-means requires you to
specify.

```
1. every colour starts as its own cluster
2. find the two closest clusters
3. if their distance < threshold, merge; else stop
4. repeat
```

k-means belongs in the _other_ job PLAN.md lists — extracting a palette from a
screenshot, where you genuinely do want "give me 6 colours". Different problem, and it
lives in Go (`services/cmd/palette`) because it is CPU-bound.

**The threshold is a product decision, not a constant.** ΔE-OK 2.0 is "most people
cannot tell these apart side by side". Merging is destructive, so surface the merge in
the report with its measured distance and let the user override.

---

## Projection and the loss report

Every projection returns both halves:

```ts
type Projection<T> = { output: T; loss: LossReport[] }

type LossReport = {
  what: string // 'ramp resampled 12→10'
  severity: 'minor' | 'major' | 'blocking'
  algorithm?: string // 'perceptual-even resampling'
  measured?: { requested: number; achieved: number; unit: string }
}
```

Two design points:

**Loss is a return value, not a side effect.** Not a console warning, not a log line —
a typed value the caller must handle. That is what makes it impossible to forget.

**`algorithm` and `measured` are the product.** PLAN.md is explicit that naming the
algorithm in the UI is a product requirement rather than a nicety: _"merged these 3
greys, ΔE-OK 1.2 < 2.0 threshold"_, _"raised L to 0.71 to reach Lc 78 against your
surface"_. Design the type so an empty explanation is _impossible to construct_ and
you will never ship one.

---

## Tier 0 adapters

Tier 0 is shadcn, Tailwind v4, DaisyUI, Radix Themes, and Open Props. All five emit
CSS custom properties, so **one formatter covers the tier** — the differences are
naming and cardinality, which is exactly what the declarative spec captures.

```yaml
id: tailwind
version: '4'
cardinality: { type: fixed, count: 11, index: 0-based }
sampling: perceptual-even
mapping:
  colors.$name: '--color-{name}-{step}'
declaresLoss:
  - { what: 'ramp resampled to 11 steps', severity: minor }
escapeHatch: null
```

The engine interprets this. It is data, not code — which is the only way 12 libraries ×
2 live majors stays maintainable, and the only way a contributor adds a library by pull
request without touching the engine.

**Version adapters from the first commit** — `tailwind@4`, `mui@6`. Chakra v2→v3 and
Tailwind v3→v4 were both total theming rewrites, and old adapters must keep working.

Start with Tailwind v4: its `@theme` directive emits CSS custom properties natively, so
the projection is nearly the identity function and you can validate the pipeline
end-to-end before adding a target that fights you.

---

## Golden fixtures

```
fixtures/canonical/    input token sets
fixtures/expected/     expected output per adapter@version
```

The test asserts **byte-exactness**. That sounds brittle and is the point: it proves
_stability_, so any behaviour change shows up as a reviewable diff rather than a
surprise.

Determinism is therefore a hard requirement:

- No `Date.now()`, no `Math.random()`, no map iteration order dependence.
- Round explicitly. Floating-point output must be formatted to a fixed precision, or
  the last decimal will differ across machines and every golden will fail somewhere
  else.
- Golden **text** fixtures run natively and fast. Only **screenshots** need the
  container — see [ch 03](03-containers.md).

### Fix the caching while you are here

`test:golden` currently runs with `cache: false`, because its `inputs` did not
reference `fixtures/` correctly and a wrong glob produces a permanent false cache hit.
See [ch 02](02-monorepo.md) for the mechanism.

Fix it, then **prove** it:

```bash
pnpm turbo run test:golden --dry=json | grep hash    # note it
# edit any file under fixtures/
pnpm turbo run test:golden --dry=json | grep hash    # MUST have changed
```

Only when the hash moves should you remove `cache: false`. Until then the honest
setting is the slow one.

> **Golden files prove stability, not correctness.** They only assert that output has
> not changed — a wrong output, committed once, stays wrong forever and green. For
> correctness, PLAN.md prescribes a different check: export a theme, drop it into a
> scratch Vite app using the real library, and confirm it compiles and renders with
> the right colours. Do that once per Tier 1 adapter. The two tests answer different
> questions and you need both.

---

## Exercises

### 1 — resampling invariance

Build a `Ramp` and assert that `sample(12)[9]` and `sample(10)[7]` land within
ΔE-OK 2.0 of each other.

<details>
<summary>Why this is the right first test</summary>

It is the executable form of the curve-not-array decision. Both indices are near 0.8
along the curve, so they must be near-identical colours. If they are not, your `sample`
is interpolating in the wrong space, or `anchors` is storing indices rather than
positions.

It also fails loudly if someone later "optimises" `Ramp` into a fixed array — which is
exactly the regression worth guarding against.
</details>

### 2 — a loss report you would show a user

Take a 12-step canonical ramp, project it to Mantine's 10, and produce a `LossReport`
naming the algorithm and the measurement.

<details>
<summary>What good output looks like</summary>

```
ramp resampled 12→10 (perceptual-even)        minor
  requested 12 steps, achieved 10
chroma clamped at steps 0, 11 to fit sRGB     minor
  requested C 0.21, achieved C 0.17
```

Not `"some precision was lost"`. The measurement is what makes it trustworthy, and
trustworthiness is the entire wedge: translation _is_ lossy, and saying so precisely
turns the weakest part of the product into the strongest.
</details>

### 3 — the solver's failure path

Ask `solveLightnessForLc` for Lc 90 against a mid-grey background, where no lightness
can reach it. Confirm it returns `converged: false` with the achieved value — and that
your projection turns that into a `blocking` loss entry rather than silently shipping
a failing contrast.

---

← [05 Finishing M0](05-finishing-m0.md) · [index](dev-guide.md)
