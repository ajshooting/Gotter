# Gotter

Gotter is a small, team-limited microblogging app built with Go and SQLite.

It keeps the first version intentionally simple: esa OAuth login, text-only posts, and a timeline visible only to members of the configured esa team.

## Features

- esa OAuth authentication
- Team-based access control
- Text posts up to 200 characters
- Timeline view
- User profile pages
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
HOST_PORT=8080
PORT=8080
DATABASE_PATH=/app/data/gotter.db
ESA_CLIENT_ID=...
ESA_CLIENT_SECRET=...
ESA_ALLOWED_TEAM=s-union
COOKIE_SECURE=true
```

`APP_NAME` is used for the page title, header brand, and login heading.

`APP_BASE_URL` must be the public origin that users access. Gotter derives the esa OAuth callback URL from it:

```text
https://gotter.example.com/auth/esa/callback
```

Use `COOKIE_SECURE=true` in production when the app is served over HTTPS.

## esa OAuth Setup

1. Open `https://[team].esa.io/user/applications`.
2. Create an OAuth application.
3. Set the redirect URI to `${APP_BASE_URL}/auth/esa/callback`.
4. Grant the application `read` scope.
5. Copy the generated client ID and client secret into `.env` as `ESA_CLIENT_ID` and `ESA_CLIENT_SECRET`.
6. Set `ESA_ALLOWED_TEAM` to the esa team name, for example `s-union`.

The application uses esa OAuth only during login. It exchanges the authorization code, checks that the signed-in user belongs to `ESA_ALLOWED_TEAM`, fetches the user's esa profile, and does not store the esa access token.

## Deployment

Build and start with Docker Compose:

```sh
docker compose up -d --build
```

The SQLite database is stored in the `gotter-data` Docker volume mounted at `/app/data`. This avoids host directory ownership issues when the container runs as a non-root user.

Check logs after deployment:

```sh
docker compose logs -f gotter
```

If you run Gotter behind a reverse proxy, route HTTPS traffic to `HOST_PORT` and keep `APP_BASE_URL` exactly aligned with the public URL registered in esa.

## Status

Work in progress.
