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
├── .mcp.json                   # Chrome DevTools MCP config (for verification)
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
├── scripts/
│   └── stripe-listen.js        # Cross-platform Stripe CLI launcher (reads key from .env)
├── migrations/
│   └── 001_init.sql            # Reference schema (auto-applied by db.go)
├── go.mod / go.sum
├── package.json                # npm scripts: dev, dev:api, dev:stripe, dev:docker, dev:all
├── vite.config.ts              # Port 5179, proxy /api → localhost:8089
├── vercel.json                 # SPA rewrites
├── .env / .env.example
├── .gitignore
└── README.md                   # Prerequisites, setup, and run instructions
```

### Key Design Decisions

1. **One Go file per endpoint** in `/api/`. Each exports a uniquely-named handler (e.g., `HealthHandler`, `UserHandler`). Vercel auto-detects the exported `http.HandlerFunc`. The local dev server imports them by name.

2. **Auth0 Universal Login** (redirect-based) via `@auth0/auth0-react`. No custom login forms.

3. **Auth0 JWT validation in Go** using `auth0/go-jwt-middleware/v2` validator (used directly, not as HTTP middleware — works in serverless). The `/userinfo` endpoint is called once per user to get their email on first login.

4. **Docker PostgreSQL for local dev** (port 5433). CockroachDB Cloud for production. Same SQL, same `pgx` driver, zero code branching. CockroachDB Cloud's port 26257 is often blocked by corporate firewalls.

5. **Stripe Checkout** for one-time $9.99 purchase. Webhook handled locally via Stripe CLI Docker container forwarding events to `host.docker.internal`.

6. **Auto-migration** in `pkg/db/db.go` using `sync.Once` — runs `CREATE TABLE IF NOT EXISTS` on first DB connection. No separate migration step needed.

7. **Non-standard local ports** to avoid conflicts: frontend on 5179, API on 8089, PostgreSQL on 5433.

8. **One-command local dev** via `npm run dev:all` — starts Docker PostgreSQL, Go API, Vite, and Stripe CLI concurrently using the `concurrently` npm package.

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

All via management APIs. **Always create fresh resources** — ignore any that already exist in these accounts from previous builds.

**Auth0:**
1. Create a new SPA application via POST `/api/v2/clients`:
   - `name`: `"React Go Starter"`
   - `app_type`: `"spa"`
   - `callbacks`: `["http://localhost:5179"]` (add Vercel domain later in Phase 9)
   - `allowed_logout_urls`, `web_origins`, `allowed_origins`: same
   - `token_endpoint_auth_method`: `"none"` (SPA = public client)
   - `is_first_party`: `true` ← **critical for skipping consent screen**
   - Save the returned `client_id` — this becomes `VITE_AUTH0_CLIENT_ID`
2. Create API resource server via POST `/api/v2/resource-servers`:
   - `identifier` (audience): `https://api.react-go-starter.vercel.app`
   - `signing_alg`: `RS256`
   - `skip_consent_for_verifiable_first_party_clients`: `true` ← **critical — without this, a consent screen appears on every page navigation**
3. Verify Username-Password-Authentication connection is enabled for the new SPA client:
   - GET `/api/v2/clients/{client_id}/connections` — it's usually enabled by default

**CockroachDB Cloud:**
1. Create a new serverless cluster: POST `/api/v1/clusters` (provider: AWS, region: us-east-2)
2. Poll until `state: CREATED` (usually instant)
3. Create SQL user: POST `/api/v1/clusters/{id}/sql-users` with `{"name": "app_user", "password": "Str0ngP@ss2024!"}`
4. Create database: POST `/api/v1/clusters/{id}/databases` with `{"name": "starter_app"}`
5. Construct and save the production connection string: `postgresql://app_user:Str0ngP%40ss2024%21@{sql_dns}:26257/starter_app?sslmode=verify-full` (URL-encode the `@` and `!` in the password)

**Stripe:**
1. Create a new product: POST `/v1/products` with `name=Premium Content Access&description=One-time purchase to unlock premium content` (form-encoded, not JSON)
2. Create a one-time price: POST `/v1/prices` with `unit_amount=999&currency=usd&product={product_id}`
3. Save the `price_id` — this becomes `STRIPE_PRICE_ID`

**Vercel:** (project created later in Phase 9)

### Phase 2: Project Scaffolding

**Critical: `git init` and commit existing files BEFORE scaffolding.** The Vite `--overwrite` flag deletes all non-Vite files.

```bash
git init && git add . && git commit -m "initial: reference docs"
npm config set prefix "C:\Development\Repositories\Experiments"
npm create vite@latest . -- --template react-ts --overwrite
npm install @auth0/auth0-react react-router-dom
npm install --save-dev concurrently
go mod init react-go-vercel-app
```

Create config files:
- `vercel.json`: `{"rewrites": [{"source": "/(.*)", "destination": "/index.html"}]}`
- `vite.config.ts`: port 5179, proxy `/api` → `http://localhost:8089`
- `.env.example`: template with all env var names
- `.env`: actual values from Phase 1 (gitignored). Set `STRIPE_WEBHOOK_SECRET=placeholder` for now — updated in Phase 7.
- `.gitignore`: add `.env`, `*.db`, `*.db-wal`, `*.db-shm`, `*.exe`, Go build artifacts
- `.mcp.json`: Chrome DevTools MCP config for verification:
  ```json
  {"mcpServers":{"chrome-devtools":{"command":"cmd","args":["/c","npx","-y","chrome-devtools-mcp@latest","--","--headless=false","--isolated"],"type":"stdio"}}}
  ```

Create `scripts/stripe-listen.js` — a cross-platform helper that reads `STRIPE_SECRET_KEY` from `.env` and spawns the Docker Stripe CLI container. This is needed because `$ENV_VAR` expansion in npm scripts doesn't work on Windows.

```js
import { execSync } from 'child_process'
import { readFileSync } from 'fs'
const env = readFileSync('.env', 'utf8')
const match = env.match(/^STRIPE_SECRET_KEY=(.+)$/m)
if (!match) { console.error('STRIPE_SECRET_KEY not found in .env'); process.exit(1) }
execSync(`docker run --rm --name stripe-cli stripe/stripe-cli listen --api-key ${match[1].trim()} --forward-to http://host.docker.internal:8089/api/webhook -s`, { stdio: 'inherit' })
```

Update `package.json` scripts:
```json
"dev": "vite",
"dev:api": "go run ./cmd/local-server",
"dev:stripe": "node scripts/stripe-listen.js",
"dev:docker": "docker start starter-postgres || docker run -d --name starter-postgres -e POSTGRES_USER=app_user -e POSTGRES_PASSWORD=localdev -e POSTGRES_DB=starter_app -p 5433:5432 postgres:16-alpine",
"dev:all": "npm run dev:docker && concurrently -n api,web,stripe -c blue,green,yellow \"npm run dev:api\" \"npm run dev\" \"npm run dev:stripe\""
```

### Phase 3: Database Layer

Docker PostgreSQL is started by `npm run dev:docker` (or `npm run dev:all`).

Set `DATABASE_URL=postgresql://app_user:localdev@localhost:5433/starter_app?sslmode=disable` in `.env`.

Implement `pkg/db/db.go`:
- `sync.Once` initialization with `sql.Open("pgx", DATABASE_URL)`
- Auto-migration: `CREATE TABLE IF NOT EXISTS users (...)` on first connection
- Schema: `id UUID DEFAULT gen_random_uuid(), auth0_id TEXT UNIQUE, email TEXT, stripe_customer_id TEXT, has_purchased BOOLEAN DEFAULT false, created_at TIMESTAMPTZ DEFAULT now(), updated_at TIMESTAMPTZ DEFAULT now()`

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

**TypeScript note:** Use `import type` (not `import`) for type-only imports like `ReactNode`. Vite's `verbatimModuleSyntax` enforces this and the build will fail without it.

- `src/main.tsx`: Auth0Provider with `domain`, `clientId`, `audience`, `redirect_uri`, scope `openid profile email`. Wrap with BrowserRouter.
- `src/lib/api.ts`: `useApi()` hook returning `fetchWithAuth(url, options)` that injects Bearer token via `getAccessTokenSilently()`.
- `src/App.tsx`: Routes for `/`, `/dashboard`, `/premium`. Nav with AuthButtons. Show loading state while Auth0 initializes.
- `src/pages/Home.tsx`: Public landing, login CTA.
- `src/pages/Dashboard.tsx`: Protected. Calls `/api/user`, shows profile + purchase status. Shows PurchaseGate if not purchased, or link to premium if purchased.
- `src/pages/Premium.tsx`: Protected. Calls `/api/user`, wraps content in PurchaseGate.
- `src/components/AuthButtons.tsx`: Login/Logout using useAuth0 hooks. Shows email when logged in.
- `src/components/ProtectedRoute.tsx`: Calls `loginWithRedirect()` if not authenticated. Uses `import type { ReactNode }`.
- `src/components/PurchaseGate.tsx`: If not purchased, shows buy button that POSTs to `/api/create-checkout` and redirects to Stripe. Uses `import type { ReactNode }`.
- CSS: minimal/raw, just enough to show the pattern.

### Phase 7: Local Dev Server + Stripe CLI

`cmd/local-server/main.go`:
- Loads `.env` via `godotenv`
- Mounts all handlers on an `http.ServeMux`
- CORS via `rs/cors` allowing `http://localhost:5179`
- Default port 8089

**First-time Stripe CLI setup:**
1. Run `npm run dev:stripe` (or `npm run dev:all`)
2. The Stripe CLI prints a webhook signing secret: `whsec_...`
3. Copy this value into `.env` as `STRIPE_WEBHOOK_SECRET`
4. Restart (Ctrl+C, then `npm run dev:all` again)
5. This only needs to be done once — the secret persists across CLI restarts.

### Phase 8: Verify Locally

Run all services with one command:
```bash
npm run dev:all
```

This starts: Docker PostgreSQL → then concurrently: Go API on :8089, Vite on :5179, Stripe CLI webhook forwarding.

Test flow using Chrome DevTools MCP:
1. Landing page loads at `http://localhost:5179`
2. Click "Log in" → redirects to Auth0 Universal Login
3. Sign up with test credentials → consent screen appears (localhost-only, won't happen on HTTPS)
4. Dashboard shows user profile, Status: Free, Buy Access button
5. Click "Buy Access" → redirected to Stripe Checkout
6. Uncheck "Save my information" first to avoid phone number requirement
7. Fill test card: `4242424242424242`, any future exp, any CVC, any name/ZIP
8. Pay → redirected back to app
9. Stripe CLI forwards `checkout.session.completed` webhook → API marks user as purchased
10. Dashboard shows Status: Premium
11. Premium page shows unlocked content

### Phase 9: Deploy to Vercel

**Before pushing to GitHub:**
- Never hardcode secrets in committed files (e.g., Stripe key in npm scripts). GitHub push protection will block the push. The `scripts/stripe-listen.js` approach avoids this.
- Run `npm run build` locally to catch TypeScript/lint errors before pushing.

**Deployment steps:**
1. Create GitHub repo: `gh repo create hbradio/react-go-vercel-app --public --source=. --push`
2. Create a new Vercel project via API, linked to the GitHub repo:
   ```python
   POST /v10/projects
   {"name": "react-go-vercel-app", "framework": "vite", "gitRepository": {"type": "github", "repo": "hbradio/react-go-vercel-app"}}
   ```
3. Set all environment variables via `POST /v10/projects/{id}/env` for each var:
   - Use `type: "encrypted"` for backend secrets (`AUTH0_DOMAIN`, `AUTH0_AUDIENCE`, `DATABASE_URL`, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID`)
   - Use `type: "plain"` for frontend vars (`VITE_AUTH0_DOMAIN`, `VITE_AUTH0_CLIENT_ID`, `VITE_AUTH0_AUDIENCE`, `VITE_STRIPE_PUBLISHABLE_KEY`)
   - `DATABASE_URL` should be the CockroachDB Cloud connection string (port 26257 is accessible from Vercel)
   - Set `STRIPE_WEBHOOK_SECRET` to a placeholder for now
4. Push triggers auto-deploy. Poll `GET /v6/deployments?projectId={id}` until `readyState: READY`.
5. Get the production URL from the deployment aliases (e.g., `https://react-go-vercel-app.vercel.app`).
6. Update Auth0 SPA callback URLs to include production domain (keep localhost URLs too):
   ```python
   PATCH /api/v2/clients/{id}
   {"callbacks": ["http://localhost:5179", "https://<vercel-domain>", "https://<vercel-domain>/callback"],
    "allowed_logout_urls": ["http://localhost:5179", "https://<vercel-domain>"],
    "web_origins": ["http://localhost:5179", "https://<vercel-domain>"],
    "allowed_origins": ["http://localhost:5179", "https://<vercel-domain>"]}
   ```
7. Create Stripe webhook endpoint via API:
   ```
   POST /v1/webhook_endpoints
   url=https://<vercel-domain>/api/webhook&enabled_events[]=checkout.session.completed
   ```
   This returns a `secret` — update the Vercel `STRIPE_WEBHOOK_SECRET` env var with it:
   - List env vars: `GET /v9/projects/{id}/env`, find `STRIPE_WEBHOOK_SECRET` by key, get its `id`
   - Update: `PATCH /v9/projects/{id}/env/{env_id}` with `{"value": "whsec_..."}`
8. Redeploy to pick up the updated webhook secret (push a commit, or trigger via `POST /v13/deployments` — requires `repoId` from project's `link` object).

### Phase 10: Verify on Production

Use Chrome DevTools MCP:
1. Navigate to `https://<vercel-domain>`
2. Landing page loads with nav + login CTA
3. Click "Log in" → Auth0 Universal Login (**no consent screen on HTTPS**)
4. Auth0 auto-authenticates if session exists, otherwise sign up
5. Dashboard loads — `/api/user` hits CockroachDB Cloud, auto-creates table + user record, shows Status: Free
6. Click "Buy Access" → Stripe Checkout with correct product ($9.99)
7. Uncheck "Save my info" first, fill test card `4242424242424242`, pay
8. Redirected back to `/dashboard?purchased=true` — **Status: Premium** (webhook fired and updated CockroachDB)
9. Click "View premium content" → Premium page shows unlocked content
10. No consent screens, no re-auth on navigation

### Phase 11: Create README

Create `README.md` with:
- Prerequisites (Node.js, Go, Docker, Git) with install links and verification command
- Setup steps: clone, `npm install`, `cp .env.example .env`, fill in credentials, `docker pull` images
- One-command run: `npm run dev:all`
- First-time Stripe CLI webhook secret setup
- Individual service commands
- Stack overview, project structure, env var reference

---

## Environment Variables

| Variable | Where | Description |
|----------|-------|-------------|
| `VITE_AUTH0_DOMAIN` | Frontend | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Frontend | Auth0 SPA client ID (from Phase 1) |
| `VITE_AUTH0_AUDIENCE` | Frontend | Auth0 API identifier |
| `VITE_STRIPE_PUBLISHABLE_KEY` | Frontend | Stripe publishable key (from creds.md) |
| `AUTH0_DOMAIN` | Backend | Auth0 tenant domain |
| `AUTH0_AUDIENCE` | Backend | Auth0 API identifier |
| `DATABASE_URL` | Backend | Local: `postgresql://app_user:localdev@localhost:5433/starter_app?sslmode=disable` / Prod: CockroachDB Cloud connection string |
| `STRIPE_SECRET_KEY` | Backend | Stripe secret key (from creds.md) |
| `STRIPE_WEBHOOK_SECRET` | Backend | Local: from Stripe CLI output / Prod: from webhook endpoint creation |
| `STRIPE_PRICE_ID` | Backend | Stripe Price ID (from Phase 1) |

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

**npm (dev):**
- `concurrently` — Run multiple services in parallel

**Docker (local dev only):**
- `postgres:16-alpine` — Local database
- `stripe/stripe-cli` — Webhook forwarding

---

## Verification Approach

Use **Chrome DevTools MCP** for all verification — both local and production. It provides page snapshots (a11y tree), element interaction (click, fill, navigate), and screenshots. No browser installation, headless configuration, or test framework needed. The `.mcp.json` file at the project root configures it.

---

## Style

Raw and minimal. CSS files show the pattern but leave styling for later. No frameworks, no component libraries, no animations.
