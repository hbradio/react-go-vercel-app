# React + Go Vercel Starter

A serverless starter app deployed on Vercel with Auth0 authentication, Stripe one-time purchase billing, and CockroachDB Cloud persistence.

**Production:** https://react-go-vercel-app.vercel.app

## Prerequisites

Install these before running locally:

- **Node.js** (v18+) and **npm** — https://nodejs.org
- **Go** (v1.21+) — https://go.dev/dl
- **Docker Desktop** — https://www.docker.com/products/docker-desktop
- **Git** — https://git-scm.com

Verify installations:
```bash
node --version && npm --version && go version && docker --version && git --version
```

## Setup

1. Clone and install dependencies:
```bash
git clone https://github.com/hbradio/react-go-vercel-app.git
cd react-go-vercel-app
npm install
```

2. Create a `.env` file from the template:
```bash
cp .env.example .env
```

3. Fill in `.env` with your credentials (see `.env.example` for required variables).

4. Pull the Docker images (one-time):
```bash
docker pull postgres:16-alpine
docker pull stripe/stripe-cli
```

## Running Locally

Start everything with one command:
```bash
npm run dev:all
```

This starts four services concurrently:
| Service | Port | Label |
|---------|------|-------|
| PostgreSQL (Docker) | 5433 | — |
| Go API server | 8089 | `[api]` |
| Vite dev server | 5179 | `[web]` |
| Stripe CLI (Docker) | — | `[stripe]` |

Open http://localhost:5179 in your browser.

### First-time Stripe CLI setup

The Stripe CLI prints a webhook signing secret on first run:
```
Ready! Your webhook signing secret is whsec_... (^C to quit)
```
Copy this `whsec_...` value into your `.env` as `STRIPE_WEBHOOK_SECRET`, then restart:
```bash
# Ctrl+C to stop, then:
npm run dev:all
```
This only needs to be done once — the secret persists across restarts.

### Running services individually

```bash
npm run dev:docker   # Start PostgreSQL container
npm run dev:api      # Go API on :8089
npm run dev          # Vite frontend on :5179
npm run dev:stripe   # Stripe webhook forwarding (requires STRIPE_SECRET_KEY in env)
```

## Stack

| Layer | Technology |
|-------|-----------|
| Frontend | React 19, TypeScript, Vite |
| Backend | Go serverless functions (Vercel) |
| Auth | Auth0 (Universal Login + JWT validation) |
| Payments | Stripe Checkout (one-time purchase) |
| Database | CockroachDB Cloud (prod) / PostgreSQL (local) |
| Hosting | Vercel |

## Project Structure

```
api/                  Go serverless functions (1 file = 1 endpoint)
pkg/                  Shared Go packages (db, auth, models)
src/                  React SPA (pages, components, lib)
cmd/local-server/     Local dev server mounting all API handlers
migrations/           SQL schema (auto-applied on first connection)
```

## Environment Variables

See `.env.example` for the full list. Key variables:

| Variable | Description |
|----------|-------------|
| `VITE_AUTH0_DOMAIN` | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Auth0 API audience identifier |
| `DATABASE_URL` | PostgreSQL/CockroachDB connection string |
| `STRIPE_SECRET_KEY` | Stripe secret key |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook signing secret |
| `STRIPE_PRICE_ID` | Stripe Price ID for the $9.99 purchase |

## Deployment

The app auto-deploys to Vercel on every push to `master`. Environment variables are configured in the Vercel dashboard.
