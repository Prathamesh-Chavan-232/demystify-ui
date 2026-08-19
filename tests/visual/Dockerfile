# syntax=docker/dockerfile:1
# Digest-pinned deliberately. The :v1.xx-noble tag is rebuilt in place when the
# base OS ships font updates; font metrics shift, every golden fails at once,
# and the team learns to ignore visual failures. Bumping this digest is a
# deliberate change that regenerates all goldens IN THE SAME PR.
FROM mcr.microsoft.com/playwright:v1.58.0-noble@sha256:35c7d48b4ccaf3aca5018f5f1bf7f50c7da7d61d176c530741f4f2e9ca336c34
WORKDIR /repo
ENV CI=true
RUN corepack enable
# Playwright directly, NOT `turbo run` — the turbo graph pulls in the Go build,
# and this image has no Go toolchain. Build orchestration is the host's job.
CMD ["pnpm", "--filter", "@demystify/visual", "test:visual"]
