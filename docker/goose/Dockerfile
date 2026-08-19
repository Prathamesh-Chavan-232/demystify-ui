# syntax=docker/dockerfile:1
FROM golang:1.26-trixie@sha256:b75d466dd608587fd66cca705a307ba65b889827d06ad61d6a75f0482b51b7c7 AS build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=build /go/bin/goose /goose
USER nonroot:nonroot
WORKDIR /migrations
# Driver and DSN come from GOOSE_DRIVER / GOOSE_DBSTRING (set in compose),
# so they must NOT also be passed positionally.
ENTRYPOINT ["/goose", "-dir", "/migrations"]
