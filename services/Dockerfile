# syntax=docker/dockerfile:1
# One image, three binaries. compose picks which via `command:` — consistent
# with the one-module / many-cmd decision in PLAN.md.

FROM golang:1.26-trixie@sha256:b75d466dd608587fd66cca705a307ba65b889827d06ad61d6a75f0482b51b7c7 AS build
WORKDIR /src
# go.su[m] so the COPY still succeeds before any dependency exists.
COPY go.mod go.su[m] ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
# Cache mounts, or every rebuild re-downloads the module graph.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags='-s -w' -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=build /out/ /
USER nonroot:nonroot
# CMD not ENTRYPOINT: compose overrides it per service.
CMD ["/api"]
