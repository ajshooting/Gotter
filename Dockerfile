FROM golang:1.26-bookworm AS build

WORKDIR /src

RUN apt-get update \
  && apt-get install -y --no-install-recommends gcc libc6-dev \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/gotter ./cmd/gotter

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --uid 10001 --gid 0 gotter

WORKDIR /app

COPY --from=build /out/gotter /app/gotter

RUN mkdir -p /app/data \
  && chown -R gotter:0 /app

USER gotter

ENV PORT=8080
ENV DATABASE_PATH=/app/data/gotter.db

EXPOSE 8080

ENTRYPOINT ["/app/gotter"]
