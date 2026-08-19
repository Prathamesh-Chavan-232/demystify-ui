import { test, expect } from '@playwright/test'
import { pathToFileURL } from 'node:url'
import { resolve } from 'node:path'

const fixture = pathToFileURL(resolve(import.meta.dirname, '../fixtures/swatches.html')).href

test('swatch fixture renders from :root custom properties', async ({ page }) => {
  await page.goto(fixture)
  await expect(page).toHaveScreenshot('swatches.png')
})

test('no hardcoded colors leak into computed styles', async ({ page }) => {
  await page.goto(fixture)
  // The non-negotiable from PLAN.md: every visual property resolves through a
  // custom property, so the theme can be swapped without touching components.
  const declared = await page.evaluate(() => {
    const sheet = Array.from(document.styleSheets)[0] as CSSStyleSheet
    return Array.from(sheet.cssRules)
      .filter((r): r is CSSStyleRule => r instanceof CSSStyleRule)
      .filter((r) => r.selectorText !== ':root')
      .flatMap((r) =>
        ['color', 'background-color', 'border-radius', 'background', 'border']
          .map((p) => r.style.getPropertyValue(p))
          .filter(Boolean),
      )
  })
  const offenders = declared.filter(
    (v) => /#[0-9a-f]{3,8}\b/i.test(v) || /\b(rgb|hsl|oklch|lab)\(/i.test(v),
  )
  expect(offenders).toEqual([])
})
