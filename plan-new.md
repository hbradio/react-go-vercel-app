# Plan: React + Go Vercel Serverless Starter App

> **Read `implementation-tips.md` before starting.** It contains critical lessons (Windows bash quirks, Auth0 gotchas, Stripe webhook fixes) that will prevent hours of debugging.

---

## Goal

Build a lean, human-maintainable serverless starter app on Vercel: Go API backend, React SPA frontend (TypeScript), CockroachDB Cloud for production persistence, Docker PostgreSQL for local dev, Auth0 for authentication, Stripe for one-time purchase billing.

The integration pattern: user signs up via Auth0 → record stored in PostgreSQL/CockroachDB → user purchases via Stripe Checkout → webhook marks user as purchased → premium content unlocked.

---

## Architecture

```
react-go-vercel-app/
├── api/                        # Go serverless functions (1 file = 1 endpoint)
│   ├── health.go               # GET  /api/health
│   ├── user.go                 # GET  /api/user (protected)
│   ├── create-checkout.go      # POST /api/create-checkout (protected)
│   └── webhook.go              # POST /api/webhook (Stripe signature verified)
├── pkg/                        # Shared Go packages
│   ├── db/db.go                # PostgreSQL connection pool + auto-migration
│   ├── auth/auth.go            # Auth0 JWT validation + /userinfo helper
│   └── models/user.go          # User model & queries
├── src/                        # React SPA (Vite + TypeScript)
│   ├── main.tsx                # Auth0Provider + BrowserRouter wrapper
│   ├── App.tsx                 # Router + layout + nav
│   ├── pages/
│   │   ├── Home.tsx            # Public landing page
│   │   ├── Dashboard.tsx       # Protected — user info + purchase status
│   │   └── Premium.tsx         # Protected + purchase-gated content
│   ├── components/
│   │   ├── AuthButtons.tsx     # Login/Logout in nav
│   │   ├── ProtectedRoute.tsx  # Auth gate wrapper
│   │   └── PurchaseGate.tsx    # Purchase gate wrapper
│   └── lib/
│       └── api.ts              # useApi() hook — fetch with Auth0 token injection
├── cmd/
│   └── local-server/main.go    # Local dev HTTP server (mounts all handlers)
├── migrations/
│   └── 001_init.sql            # Reference schema (auto-applied by db.go)
├── go.mod / go.sum
├── package.json                # npm scripts: dev, dev:api, dev:stripe
├── vite.config.ts              # Port 5179, proxy /api → localhost:8089
├── vercel.json                 # SPA rewrites
├── .env / .env.example
└── .gitignore
```

### Key Design Decisions

1. **One Go file per endpoint** in `/api/`. Each exports a uniquely-named handler (e.g., `HealthHandler`, `UserHandler`). Vercel auto-detects the exported `http.HandlerFunc`. The local dev server imports them by name.

2. **Auth0 Universal Login** (redirect-based) via `@auth0/auth0-react`. No custom login forms.

3. **Auth0 JWT validation in Go** using `auth0/go-jwt-middleware/v2` validator (used directly, not as HTTP middleware — works in serverless). The `/userinfo` endpoint is called once per user to get their email on first login.

4. **Docker PostgreSQL for local dev** (port 5433). CockroachDB Cloud for production. Same SQL, same `pgx` driver, zero code branching. CockroachDB Cloud's port 26257 is often blocked by corporate firewalls.

5. **Stripe Checkout** for one-time $9.99 purchase. Webhook handled locally via Stripe CLI Docker container forwarding events to `host.docker.internal`.

6. **Auto-migration** in `pkg/db/db.go` using `sync.Once` — runs `CREATE TABLE IF NOT EXISTS` on first DB connection. No separate migration step needed.

7. **Non-standard local ports** to avoid conflicts: frontend on 5179, API on 8089, PostgreSQL on 5433.

---

## Implementation Phases

### Phase 0: Verify Cloud API Access

Use Python `urllib.request` for all API calls (curl breaks on Windows bash — see tips).

Verify all four services using credentials from `creds.md`:
- **Auth0**: POST to `/oauth/token` for management token, then GET `/api/v2/clients`
- **CockroachDB**: GET `/api/v1/clusters` with Bearer token
- **Stripe**: GET `/v1/products` with Basic auth (secret key as username, empty password)
- **Vercel**: GET `/v2/user` with Bearer token

Save the Auth0 management token to `C:/tmp/auth0_token.txt` for reuse.

### Phase 1: Cloud Service Setup

All via management APIs. List existing resources before creating to avoid duplicates.

**Auth0:**
1. Update the existing SPA app (client ID: `jHVpPOHGr52UYguebuNMHnxJ20aWrku7`) via PATCH `/api/v2/clients/{id}`:
   - `callbacks`: `["http://localhost:5179"]` (add Vercel domain later)
   - `allowed_logout_urls`, `web_origins`, `allowed_origins`: same
   - `token_endpoint_auth_method`: `"none"` (SPA = public client)
   - `is_first_party`: `true` ← **critical for skipping consent screen**
2. Create API resource server via POST `/api/v2/resource-servers`:
   - `identifier` (audience): `https://api.react-go-starter.vercel.app`
   - `signing_alg`: `RS256`
   - `skip_consent_for_verifiable_first_party_clients`: `true` ← **critical**
3. Verify Username-Password-Authentication connection is enabled for the SPA client:
   - GET `/api/v2/clients/{client_id}/connections` — it's usually enabled by default

**CockroachDB Cloud:**
1. Create serverless cluster: POST `/api/v1/clusters` (provider: AWS, region: us-east-2)
2. Poll until `state: CREATED` (usually instant)
3. Create SQL user: POST `/api/v1/clusters/{id}/sql-users`
4. Create database: POST `/api/v1/clusters/{id}/databases`
5. Save connection string: `postgresql://USER:PASS@HOST:26257/DB?sslmode=verify-full`

**Stripe:**
1. Create product: POST `/v1/products` (form-encoded, not JSON)
2. Create price: POST `/v1/prices` with `unit_amount=999&currency=usd&product={id}`
3. Save the `price_id` for checkout session creation

### Phase 2: Project Scaffolding

**Critical: `git init` and commit existing files BEFORE scaffolding.** The Vite `--overwrite` flag deletes all non-Vite files.

```bash
git init && git add . && git commit -m "initial: reference docs"
npm config set prefix "C:\Development\Repositories\Experiments"
npm create vite@latest . -- --template react-ts --overwrite
npm install @auth0/auth0-react react-router-dom
go mod init react-go-vercel-app
```

Create config files:
- `vercel.json`: `{"rewrites": [{"source": "/(.*)", "destination": "/index.html"}]}`
- `vite.config.ts`: port 5179, proxy `/api` → `http://localhost:8089`
- `.env.example`: template with all env var names
- `.env`: actual values (gitignored)
- `.gitignore`: add `.env`, `*.db`, `*.exe`, Go build artifacts

Update `package.json` scripts:
```json
"dev": "vite",
"dev:api": "go run ./cmd/local-server",
"dev:stripe": "docker run --rm --name stripe-cli stripe/stripe-cli listen --api-key SK_KEY --forward-to http://host.docker.internal:8089/api/webhook -s"
```

### Phase 3: Database Layer

Start Docker PostgreSQL on port 5433:
```bash
docker run -d --name starter-postgres \
  -e POSTGRES_USER=app_user -e POSTGRES_PASSWORD=localdev \
  -e POSTGRES_DB=starter_app -p 5433:5432 postgres:16-alpine
```

Set `DATABASE_URL=postgresql://app_user:localdev@localhost:5433/starter_app?sslmode=disable` in `.env`.

Implement `pkg/db/db.go`:
- `sync.Once` initialization with `sql.Open("pgx", DATABASE_URL)`
- Auto-migration: `CREATE TABLE IF NOT EXISTS users (...)` on first connection
- Schema: `id UUID, auth0_id TEXT UNIQUE, email TEXT, stripe_customer_id TEXT, has_purchased BOOLEAN, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ`

Implement `pkg/models/user.go`:
- `GetOrCreateUser(db, auth0ID, email)` — SELECT then INSERT with RETURNING
- `SetStripeCustomer(db, auth0ID, customerID)` — UPDATE stripe_customer_id
- `MarkPurchased(db, stripeCustomerID)` — UPDATE has_purchased = true

### Phase 4: Auth Layer (Go)

Implement `pkg/auth/auth.go`:
- Use `auth0/go-jwt-middleware/v2` validator directly (not as middleware)
- `sync.Once` JWKS provider initialization from `AUTH0_DOMAIN`
- `ValidateRequest(r)` → returns `(sub, accessToken, error)`
- `FetchUserEmail(accessToken)` → calls Auth0 `/userinfo` endpoint

Go dependencies:
```bash
go get github.com/jackc/pgx/v5/stdlib \
  github.com/auth0/go-jwt-middleware/v2 \
  github.com/stripe/stripe-go/v82 \
  github.com/rs/cors \
  github.com/joho/godotenv
go mod tidy  # MUST run after go get — resolves transitive deps
```

### Phase 5: Go API Endpoints

All files in `/api/` use `package handler` with uniquely-named functions.

- `api/health.go` → `HealthHandler`: returns `{"status":"ok"}`
- `api/user.go` → `UserHandler`: validates JWT, calls FetchUserEmail + GetOrCreateUser, returns user JSON. Add CORS headers.
- `api/create-checkout.go` → `CreateCheckoutHandler`: validates JWT, creates/reuses Stripe customer, creates Checkout Session (mode: payment), returns `{"url": "..."}`. Add CORS headers.
- `api/webhook.go` → `WebhookHandler`: reads body, verifies Stripe signature using `webhook.ConstructEventWithOptions` with `IgnoreAPIVersionMismatch: true` (critical — see tips), handles `checkout.session.completed` by calling `MarkPurchased`.

### Phase 6: React Frontend

- `src/main.tsx`: Auth0Provider with `domain`, `clientId`, `audience`, `redirect_uri`, scope `openid profile email`. Wrap with BrowserRouter.
- `src/lib/api.ts`: `useApi()` hook returning `fetchWithAuth(url, options)` that injects Bearer token via `getAccessTokenSilently()`.
- `src/App.tsx`: Routes for `/`, `/dashboard`, `/premium`. Nav with AuthButtons. Show loading state while Auth0 initializes.
- `src/pages/Home.tsx`: Public landing, login CTA.
- `src/pages/Dashboard.tsx`: Protected. Calls `/api/user`, shows profile + purchase status. Shows PurchaseGate if not purchased, or link to premium if purchased.
- `src/pages/Premium.tsx`: Protected. Calls `/api/user`, wraps content in PurchaseGate.
- `src/components/AuthButtons.tsx`: Login/Logout using useAuth0 hooks. Shows email when logged in.
- `src/components/ProtectedRoute.tsx`: Calls `loginWithRedirect()` if not authenticated.
- `src/components/PurchaseGate.tsx`: If not purchased, shows buy button that POSTs to `/api/create-checkout` and redirects to Stripe.
- CSS: minimal/raw, just enough to show the pattern.

### Phase 7: Local Dev Server + Stripe CLI

`cmd/local-server/main.go`:
- Loads `.env` via `godotenv`
- Mounts all handlers on an `http.ServeMux`
- CORS via `rs/cors` allowing `http://localhost:5179`
- Default port 8089

Stripe CLI webhook forwarding via Docker:
1. Run `npm run dev:stripe` — starts Stripe CLI listener
2. CLI prints `whsec_...` signing secret
3. Put that secret in `.env` as `STRIPE_WEBHOOK_SECRET`
4. Start/restart the API server: `npm run dev:api`

### Phase 8: Verify Locally

Run all three services:
1. `npm run dev:stripe` → copy whsec to .env
2. `npm run dev:api` → Go server on :8089
3. `npm run dev` → Vite on :5179

Test flow using Chrome DevTools MCP:
1. Landing page loads at `http://localhost:5179`
2. Click "Log in" → redirects to Auth0 Universal Login
3. Sign up with test credentials → consent screen (localhost-only, won't happen on HTTPS)
4. Dashboard shows user profile, Status: Free, Buy Access button
5. Click "Buy Access" → redirected to Stripe Checkout
6. Fill test card: `4242424242424242`, any future exp, any CVC, any name/ZIP
7. Uncheck "Save my information" first to avoid phone number requirement
8. Pay → redirected back to app
9. Stripe CLI forwards `checkout.session.completed` webhook → API marks user as purchased
10. Dashboard shows Status: Premium
11. Premium page shows unlocked content

### Phase 9: Deploy to Vercel

**Before pushing to GitHub:**
- Remove any hardcoded secrets from `package.json` (e.g., Stripe key in `dev:stripe` script). GitHub push protection will block the push. Use `$STRIPE_SECRET_KEY` env var reference instead.
- Use `import type` for type-only imports in TypeScript (e.g., `import type { ReactNode } from 'react'`). Vite's `verbatimModuleSyntax` enforces this, and the build will fail on Vercel otherwise.
- Run `npm run build` locally to catch any TypeScript/lint errors before pushing.

**Deployment steps:**
1. Create GitHub repo: `gh repo create hbradio/react-go-vercel-app --public --source=. --push`
2. Create Vercel project via API, linked to the GitHub repo:
   ```python
   POST /v10/projects
   {"name": "react-go-vercel-app", "framework": "vite", "gitRepository": {"type": "github", "repo": "hbradio/react-go-vercel-app"}}
   ```
3. Set all environment variables via API: `POST /v10/projects/{id}/env` for each var. Use `type: "encrypted"` for secrets, `type: "plain"` for `VITE_` vars. Set `STRIPE_WEBHOOK_SECRET` to a placeholder for now.
4. Push triggers auto-deploy. Poll `GET /v6/deployments?projectId={id}` until `readyState: READY`.
5. Production URL will be `https://react-go-vercel-app.vercel.app`.
6. Update Auth0 SPA callback URLs to include production domain (keep localhost URLs too):
   ```python
   PATCH /api/v2/clients/{id}
   {"callbacks": ["http://localhost:5179", "https://react-go-vercel-app.vercel.app"]}
   ```
7. Create Stripe webhook endpoint via API:
   ```python
   POST /v1/webhook_endpoints
   url=https://react-go-vercel-app.vercel.app/api/webhook&enabled_events[]=checkout.session.completed
   ```
   This returns a `secret` — update the Vercel `STRIPE_WEBHOOK_SECRET` env var with it via `PATCH /v9/projects/{id}/env/{env_id}`.
8. Redeploy to pick up the updated webhook secret (push a commit, or trigger via API with `POST /v13/deployments`).

**Vercel API notes:**
- Creating a deployment requires `repoId` (numeric) from the project's `link` object, not just the repo name.
- Updating an env var requires its ID — list them first via `GET /v9/projects/{id}/env`, find by key, then `PATCH /v9/projects/{id}/env/{env_id}`.
- Vercel auto-deploys on every push to the production branch once the GitHub repo is linked.

### Phase 10: Verify on Production

Use Chrome DevTools MCP (not Playwright — DevTools MCP is sufficient and simpler):
1. Navigate to `https://react-go-vercel-app.vercel.app`
2. Landing page loads with nav + login CTA
3. Click "Log in" → Auth0 Universal Login (no consent screen on HTTPS)
4. Auth0 auto-authenticates if session exists, otherwise sign up
5. Dashboard loads — calls `/api/user` which hits CockroachDB Cloud, creates user record, shows Status: Free
6. Click "Buy Access" → Stripe Checkout with correct product ($9.99)
7. Fill test card `4242424242424242`, uncheck "Save my info" first, pay
8. Redirected back to `/dashboard?purchased=true` — **Status: Premium** (webhook fired and updated DB)
9. Click "View premium content" → Premium page shows unlocked content
10. No consent screens, no re-auth on navigation

---

## Environment Variables

| Variable | Where | Example |
|----------|-------|---------|
| `VITE_AUTH0_DOMAIN` | Frontend | `dev-pl3ctcn34uwvp3e0.us.auth0.com` |
| `VITE_AUTH0_CLIENT_ID` | Frontend | `jHVpPOHGr52UYguebuNMHnxJ20aWrku7` |
| `VITE_AUTH0_AUDIENCE` | Frontend | `https://api.react-go-starter.vercel.app` |
| `VITE_STRIPE_PUBLISHABLE_KEY` | Frontend | `pk_test_...` |
| `AUTH0_DOMAIN` | Backend | `dev-pl3ctcn34uwvp3e0.us.auth0.com` |
| `AUTH0_AUDIENCE` | Backend | `https://api.react-go-starter.vercel.app` |
| `DATABASE_URL` | Backend | `postgresql://...` |
| `STRIPE_SECRET_KEY` | Backend | `sk_test_...` |
| `STRIPE_WEBHOOK_SECRET` | Backend | `whsec_...` |
| `STRIPE_PRICE_ID` | Backend | `price_...` |

---

## Local Dev Ports

| Service | Port |
|---------|------|
| Vite (React frontend) | 5179 |
| Go API server | 8089 |
| Docker PostgreSQL | 5433 |

---

## Key Libraries

**Go:**
- `github.com/jackc/pgx/v5` — PostgreSQL/CockroachDB driver
- `github.com/auth0/go-jwt-middleware/v2` — JWT validation (validator only)
- `github.com/stripe/stripe-go/v82` — Stripe SDK
- `github.com/rs/cors` — CORS for local dev
- `github.com/joho/godotenv` — .env loading for local dev

**React/TypeScript:**
- `@auth0/auth0-react` — Auth0 React SDK
- `react-router-dom` — Client-side routing

**Docker (local dev only):**
- `postgres:16-alpine` — Local database
- `stripe/stripe-cli` — Webhook forwarding

---

## Cloud Resources Created

These already exist from the first build. Check before re-creating.

| Service | Resource | ID |
|---------|----------|----|
| Auth0 | SPA Client | `jHVpPOHGr52UYguebuNMHnxJ20aWrku7` |
| Auth0 | API Resource Server | audience: `https://api.react-go-starter.vercel.app` |
| Auth0 | Tenant Domain | `dev-pl3ctcn34uwvp3e0.us.auth0.com` |
| CockroachDB | Cluster | `fdade18b-2c50-4c73-be04-b6b3e3f20721` |
| CockroachDB | SQL DNS | `react-go-starter-22263.j77.aws-us-east-2.cockroachlabs.cloud` |
| CockroachDB | Database | `starter_app` |
| CockroachDB | SQL User | `app_user` |
| Stripe | Product | `prod_U0BoHLSoh98hkz` |
| Stripe | Price ($9.99) | `price_1T2BQYLhIa34mtQg9hMp9VjR` |
| Vercel | User | `hbradio` |
| Vercel | Project | `prj_df49qM15GT8GbzbJFKSYKUP7EGTW` |
| Vercel | Production URL | `https://react-go-vercel-app.vercel.app` |
| GitHub | Repo | `hbradio/react-go-vercel-app` |
| Stripe | Webhook Endpoint | `we_1T2EQLLhIa34mtQgLElfXX7y` |

---

## Verification Approach

Use **Chrome DevTools MCP** for all verification — both local and production. It provides page snapshots (a11y tree), element interaction (click, fill, navigate), and screenshots. This is simpler and more reliable than Playwright for this use case. No browser installation or headless configuration needed.

---

## Style

Raw and minimal. CSS files show the pattern but leave styling for later. No frameworks, no component libraries, no animations.
