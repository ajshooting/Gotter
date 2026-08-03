# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-bookworm AS build

WORKDIR /src

RUN apt-get update \
  && apt-get install -y --no-install-recommends gcc libc6-dev \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=1 GOOS=linux go build -o /out/gotter ./cmd/gotter

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd --system --gid 10001 gotter \
  && useradd --system --uid 10001 --gid gotter --home-dir /nonexistent --shell /usr/sbin/nologin gotter

WORKDIR /app

COPY --from=build /out/gotter /app/gotter

RUN install -d -o gotter -g gotter -m 0700 /app/data \
  && chown gotter:gotter /app/gotter

USER gotter

ENV PORT=8080
ENV DATABASE_PATH=/app/data/gotter.db

EXPOSE 8080

ENTRYPOINT ["/app/gotter"]
