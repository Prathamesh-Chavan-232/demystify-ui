# syntax=docker/dockerfile:1
# Build context is the REPO ROOT, not apps/web — turbo prune needs the workspace.
#   docker build -f apps/docs/Dockerfile .

FROM node:22-trixie-slim@sha256:db8a96a63e5264607ada2d206758876ebbed6a12be2ada7517793cbfb0c2a29c AS base
ENV PNPM_HOME=/pnpm PATH=/pnpm:$PATH
RUN corepack enable

# 1. Prune the monorepo to just what @demystify/docs needs. Without this every
#    package rebuilds on any change.
FROM base AS pruner
WORKDIR /repo
COPY . .
RUN pnpm dlx turbo@2.10.11 prune @demystify/docs --docker

# 2. Install from the pruned lockfile — this layer caches on manifests alone.
FROM base AS deps
WORKDIR /repo
COPY --from=pruner /repo/out/json/ ./
RUN --mount=type=cache,target=/pnpm/store pnpm install --frozen-lockfile

# 3. Build.
FROM base AS builder
WORKDIR /repo
COPY --from=deps /repo/ ./
COPY --from=pruner /repo/out/full/ ./
RUN pnpm turbo run build --filter=@demystify/docs

# 4. Runtime. standalone does NOT bundle .next/static or public — copy both.
FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production PORT=3000 HOSTNAME=0.0.0.0
COPY --from=builder --chown=node:node /repo/apps/docs/.next/standalone ./
COPY --from=builder --chown=node:node /repo/apps/docs/.next/static ./apps/docs/.next/static
COPY --from=builder --chown=node:node /repo/apps/docs/public ./apps/docs/public
USER node
EXPOSE 3000
CMD ["node", "apps/docs/server.js"]
