# pi-portfolio-server

Go backend for [tyrellcurry.io](https://tyrellcurry.io), self-hosted on a Raspberry Pi 5.

Two services share a single SQLite database (`data/portfolio.db`):

- **webserver** (`cmd/webserver`) — serves the static frontend and logs every request to the database
- **metrics** (`cmd/metrics`) — exposes `/api/stats` and `/api/requests/weekly` for live traffic data

Both run as systemd services on the Pi, behind a Caddy reverse proxy that handles HTTPS.

## Development

```bash
make dev
```

## Deploy

Pushing to `main` triggers a GitHub Actions workflow that cross-compiles both binaries for `linux/arm64` and deploys them to the Pi over Tailscale SSH.
