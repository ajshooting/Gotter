# Gotter

Gotter is a small, team-limited microblogging app built with Go and SQLite.

It keeps the first version intentionally simple: esa OAuth login, text-only posts, and a timeline visible only to members of the configured esa team.

## Features

- esa OAuth authentication
- Team-based access control
- Text posts up to 200 characters
- Timeline view
- User profile pages
- Like posts
- Delete your own posts

## Stack

- Go
- SQLite
- Docker Compose

## Configuration

Copy `.env.example` to `.env` and set these values:

```text
APP_NAME=Gotter
APP_BASE_URL=https://gotter.example.com
BIND_HOST=127.0.0.1
HOST_PORT=8080
PORT=8080
DATABASE_PATH=/app/data/gotter.db
ESA_CLIENT_ID=...
ESA_CLIENT_SECRET=...
ESA_ALLOWED_TEAM=s-union
COOKIE_SECURE=true
SESSION_LIFETIME=24h
SESSION_IDLE_TIMEOUT=8h
```

`APP_NAME` is used for the page title, header brand, and login heading.

`APP_BASE_URL` must be the public origin that users access. Gotter derives the esa OAuth callback URL from it:

```text
https://gotter.example.com/auth/esa/callback
```

`COOKIE_SECURE` must match the `APP_BASE_URL` scheme: use `true` for HTTPS and `false` for local HTTP. Gotter refuses to start with an insecure mismatch.

Sessions expire after `SESSION_LIFETIME` and, unless `SESSION_IDLE_TIMEOUT=0`, after the configured period of inactivity. Session tokens are hashed before they are stored in SQLite.

## esa OAuth Setup

1. Open `https://[team].esa.io/user/applications`.
2. Create an OAuth application.
3. Set the redirect URI to `${APP_BASE_URL}/auth/esa/callback`.
4. Grant the application `read` scope.
5. Copy the generated client ID and client secret into `.env` as `ESA_CLIENT_ID` and `ESA_CLIENT_SECRET`.
6. Set `ESA_ALLOWED_TEAM` to the esa team name, for example `s-union`.

The application uses esa OAuth only during login. It exchanges the authorization code, checks that the signed-in user belongs to `ESA_ALLOWED_TEAM`, fetches the user's esa screen name and avatar, and stores neither the esa access token nor the member's name or email address.

## Deployment

Build and start with Docker Compose:

```sh
docker compose up -d --build
```

The SQLite database is stored in the `gotter-data` Docker volume mounted at `/app/data`. This avoids host directory ownership issues when the container runs as a non-root user.

By default, Compose publishes the application only on `127.0.0.1`, so a reverse proxy on the same VPS can reach it without exposing the application port directly to the internet. Only set `BIND_HOST=0.0.0.0` when direct external access is intentional and separately protected.

Check logs after deployment:

```sh
docker compose logs -f gotter
```

Back up SQLite while the application is stopped so the database and any WAL state are copied consistently:

```sh
mkdir -p backups/gotter-data
docker compose stop gotter
docker compose cp gotter:/app/data/. ./backups/gotter-data/
docker compose start gotter
```

Route HTTPS traffic from the reverse proxy to `127.0.0.1:HOST_PORT`, and keep `APP_BASE_URL` exactly aligned with the public URL registered in esa. Apply request-rate limits at the reverse proxy, especially to `/auth/esa/start`.

When upgrading an existing Docker volume, restrict the legacy data directory before starting the new image:

```sh
docker compose run --rm --user root gotter chmod 0700 /app/data
```

Database, WAL, and SHM files are also restricted to mode `0600` automatically at startup. When upgrading from a version that stored unhashed session tokens, all existing sessions become invalid and users must sign in again. Migration 004 permanently removes previously stored esa email addresses.

## Status

Work in progress.
