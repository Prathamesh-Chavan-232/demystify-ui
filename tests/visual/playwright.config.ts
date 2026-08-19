import { defineConfig } from '@playwright/test'

// Screenshots are the ONLY thing that needs the container: font rasterization
// is what varies across platforms. Golden *text* fixtures run natively.
export default defineConfig({
  testDir: './specs',
  snapshotDir: './__screenshots__',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? 'line' : 'list',
  expect: {
    // Zero tolerance: a diff here means the theme changed, which is the point.
    toHaveScreenshot: { maxDiffPixelRatio: 0 },
  },
  use: { colorScheme: 'light' },
})
