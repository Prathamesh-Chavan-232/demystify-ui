import { createMDX } from 'fumadocs-mdx/next'

const withMDX = createMDX()

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  // Required by apps/docs/Dockerfile. standalone does NOT bundle .next/static
  // or public — the Dockerfile copies both explicitly.
  output: 'standalone',
  outputFileTracingRoot: new URL('../../', import.meta.url).pathname,
}

export default withMDX(config)
