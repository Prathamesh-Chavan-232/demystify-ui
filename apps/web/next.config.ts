import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  // Required by apps/web/Dockerfile. Note that standalone does NOT bundle
  // .next/static or public/ — the Dockerfile copies both explicitly.
  output: 'standalone',
  // In a monorepo, tell Next where the workspace root is so file tracing
  // picks up linked packages instead of guessing.
  outputFileTracingRoot: new URL('../../', import.meta.url).pathname,
}

export default nextConfig
