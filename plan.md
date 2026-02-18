# Plan: React + Go Vercel Serverless Starter App

## Context

Build a greenfield serverless starter/boilerplate app on Vercel with a Go API backend, React SPA frontend (TypeScript), CockroachDB Cloud for persistence, Auth0 for authentication, and Stripe for subscription billing. The repo currently only contains reference docs (`spec.md`, `creds.md`, `Auth0-react.md`, `Auth0-go.md`) and a `.mcp.json` for Chrome DevTools MCP.

The goal is a lean, human-maintainable starter that demonstrates the full integration pattern: user signs up via Auth0, their record is stored in CockroachDB, and they can purchase access via Stripe to unlock premium content.

---

## Architecture

```
react-go-vercel-app/
├── api/                        # Go serverless functions (1 file = 1 endpoint)
│   ├── health.go               # GET  /api/health
│   ├── user.go                 # GET  /api/user (protected - returns user + sub status)
│   ├── create-checkout.go      # POST /api/create-checkout (protected - Stripe one-time purchase)
│   └── webhook.go              # POST /api/webhook (Stripe webhook - unprotected)
├── pkg/                        # Shared Go packages
│   ├── db/db.go                # CockroachDB connection pool
│   ├── auth/auth.go            # Auth0 JWT validation
│   └── models/user.go          # User model & queries
├── src/                        # React SPA (Vite + TypeScript)
│   ├── main.tsx                # Auth0Provider wrapper
│   ├── App.tsx                 # Router + layout
│   ├── pages/
│   │   ├── Home.tsx            # Landing page (public)
│   │   ├── Dashboard.tsx       # Protected - shows sub status + upgrade CTA
│   │   └── Premium.tsx         # Protected - gated behind active subscription
│   ├── components/
│   │   ├── AuthButtons.tsx     # Login/Logout
│   │   ├── ProtectedRoute.tsx  # Auth gate wrapper
│   │   └── PurchaseGate.tsx    # Purchase gate wrapper
│   └── lib/
│       └── api.ts              # Fetch wrapper with Auth0 token injection
├── cmd/
│   └── local-server/main.go    # Local dev server (loads all handlers)
├── migrations/
│   └── 001_init.sql            # Schema: users table
├── go.mod
├── package.json
├── tsconfig.json
├── vite.config.ts
├── vercel.json
├── index.html
├── .env.example
└── .gitignore
```

### Key Decisions

1. **One Go file per endpoint** in `/api/` - this is the Vercel-native pattern. Each exports a `Handler(w, r)` function. Shared code lives in `/pkg/`.

2. **Auth0 Universal Login** (redirect-based) via `@auth0/auth0-react` SDK. No custom login forms - lean on Auth0's hosted page.

3. **Auth0 JWT validation in Go** using `github.com/auth0-community/go-auth0` or manual JWKS validation with `github.com/golang-jwt/jwt` + JWKS fetching. The Go API validates the `Authorization: Bearer <token>` header on protected endpoints.

4. **CockroachDB Cloud** for all environments (including local dev). Accessed via `database/sql` with `pgx` driver. Simplest code path - single `DATABASE_URL` env var.

5. **Stripe Checkout** for one-time purchases (not Elements) - simplest integration. One Product with a one-time Price. Webhook records purchase in DB to unlock premium content.

6. **vercel.json** uses `rewrites` to send non-API/non-file requests to `index.html` for SPA routing. API routes are handled automatically by filesystem precedence.

7. **Local dev**: A small Go server in `cmd/local-server/` mounts all API handlers. Frontend uses Vite dev server with proxy to the local Go server.

---

## Implementation Phases

### Phase 0: Verify Cloud API Access
Verify connectivity to all four services using credentials from `creds.md`:
- **Auth0**: Get management API token via client credentials, call `GET /api/v2/clients`
- **CockroachDB**: Call Cloud API with service account key, list clusters
- **Stripe**: Call `GET /v1/products` with secret key
- **Vercel**: Call `GET /v2/user` with bearer token
- Fix any credential issues before proceeding.

### Phase 1: Cloud Service Setup
Using the management APIs:

**Auth0:**
- Create SPA Application (for React frontend) via `POST /api/v2/clients`
  - Set callback URLs: `http://localhost:5173, https://<vercel-domain>`
  - Set logout URLs and web origins similarly
- Create API (Resource Server) via `POST /api/v2/resource-servers`
  - Identifier/audience: `https://api.react-go-starter.vercel.app`
- Enable Username-Password connection for the SPA app

**CockroachDB Cloud:**
- Create a Serverless cluster via Cloud API (`POST /api/v1/clusters`)
- Create a SQL user
- Create a database
- Get connection string

**Stripe:**
- Create a Product (`POST /v1/products`) - e.g., "Premium Content Access"
- Create a one-time Price (`POST /v1/prices`) - e.g., $9.99 one-time
- Note the price ID for checkout session creation

**Vercel:**
- Create project via API or `vercel link`
- Will set environment variables after all other services are configured

### Phase 2: Project Scaffolding
- `git init`, initial commit
- `npm create vite@latest . -- --template react-ts` (in current dir)
- `npm install @auth0/auth0-react react-router-dom`
- `go mod init react-go-vercel-app`
- `go get` dependencies: `pgx`, `jwt`, `stripe-go`, `cors`
- Create `vercel.json`:
  ```json
  {
    "$schema": "https://openapi.vercel.sh/vercel.json",
    "rewrites": [{ "source": "/(.*)", "destination": "/index.html" }]
  }
  ```
- Create `.env.example` with all required env vars
- Create `.env` (gitignored) with actual values from cloud setup

### Phase 3: Database Layer
- Write `migrations/001_init.sql`:
  ```sql
  CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth0_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    stripe_customer_id TEXT,
    has_purchased BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
  );
  ```
- Run migration against CockroachDB Cloud
- Implement `pkg/db/db.go` - CockroachDB connection pool via `DATABASE_URL`
- Implement `pkg/models/user.go` - GetOrCreateUser, MarkPurchased

### Phase 4: Auth Layer (Go)
- Implement `pkg/auth/auth.go`:
  - Fetch JWKS from Auth0
  - Validate JWT tokens
  - Extract user claims (sub, email)
  - Middleware function: `Authenticate(next http.HandlerFunc) http.HandlerFunc`

### Phase 5: Go API Endpoints
- `api/health.go` - Simple health check, returns `{"status":"ok"}`
- `api/user.go` - Protected. Validates JWT, calls GetOrCreateUser, returns user record with purchase status
- `api/create-checkout.go` - Protected. Creates Stripe Checkout Session (mode: `payment`) for the one-time price, returns session URL
- `api/webhook.go` - Unprotected (verified by Stripe signature). Handles `checkout.session.completed` event. Marks user as purchased in DB.

### Phase 6: React Frontend
- `src/main.tsx` - Auth0Provider with domain, clientId, audience, redirect_uri
- `src/lib/api.ts` - `fetchWithAuth(url, options)` that injects Auth0 access token
- `src/App.tsx` - React Router with routes: `/`, `/dashboard`, `/premium`
- `src/pages/Home.tsx` - Public landing with login CTA
- `src/pages/Dashboard.tsx` - Protected. Shows user info, purchase status, buy button if not purchased
- `src/pages/Premium.tsx` - Protected + purchase-gated. Simple placeholder ("This content is only visible to purchasers.")
- `src/components/AuthButtons.tsx` - Login/Logout using Auth0 hooks
- `src/components/ProtectedRoute.tsx` - Redirects to login if not authenticated
- `src/components/PurchaseGate.tsx` - Shows purchase CTA if user hasn't bought content
- Minimal CSS files showing the pattern (raw/brutal as spec requests)

### Phase 7: Local Dev Server
- `cmd/local-server/main.go` - HTTP server on `:8080` that mounts all API handlers
- `vite.config.ts` - Proxy `/api` to `http://localhost:8080`
- Add npm scripts: `dev` (vite), `dev:api` (go run local server), `dev:all` (concurrent)

### Phase 8: Vercel Deployment
- Set all environment variables on Vercel project via API:
  - `AUTH0_DOMAIN`, `AUTH0_AUDIENCE`, `AUTH0_CLIENT_ID`
  - `DATABASE_URL` (CockroachDB connection string)
  - `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID`
  - `VITE_AUTH0_DOMAIN`, `VITE_AUTH0_CLIENT_ID`, `VITE_AUTH0_AUDIENCE`
- Create GitHub repo via `gh`, push code
- Link Vercel project to GitHub repo
- Deploy and get production URL
- Update Auth0 callback URLs with production domain
- Set up Stripe webhook endpoint pointing to production `/api/webhook`

### Phase 9: Verification
- Use Playwright (configured for full Chrome, not headless) to verify:
  1. Landing page loads
  2. Login redirects to Auth0
  3. After login, dashboard shows user info
  4. "Buy" button creates Stripe checkout
  5. After purchase (test mode), premium page is accessible
- Use chrome-devtools MCP to inspect network requests and console
- Fix any issues found

---

## Environment Variables

| Variable | Where | Description |
|----------|-------|-------------|
| `VITE_AUTH0_DOMAIN` | Frontend | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Frontend | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Frontend | Auth0 API identifier |
| `AUTH0_DOMAIN` | Backend | Auth0 tenant domain |
| `AUTH0_AUDIENCE` | Backend | Auth0 API identifier |
| `DATABASE_URL` | Backend | CockroachDB connection string |
| `STRIPE_SECRET_KEY` | Backend | Stripe secret key |
| `STRIPE_WEBHOOK_SECRET` | Backend | Stripe webhook signing secret |
| `STRIPE_PRICE_ID` | Backend | Stripe Price ID for one-time purchase |
| `VITE_STRIPE_PUBLISHABLE_KEY` | Frontend | Stripe publishable key |

---

## Key Libraries

**Go:**
- `github.com/jackc/pgx/v5` - PostgreSQL/CockroachDB driver
- `github.com/golang-jwt/jwt/v5` - JWT parsing/validation
- `github.com/stripe/stripe-go/v82` - Stripe SDK
- `github.com/rs/cors` - CORS handling

**React:**
- `@auth0/auth0-react` - Auth0 React SDK
- `react-router-dom` - Client-side routing

---

## Verification Plan

1. **Unit**: Go handler tests with httptest for each endpoint
2. **Integration**: Playwright tests against local dev server
   - Configure Playwright to use installed full Chrome (not download Chromium)
   - Test auth flow end-to-end (Auth0 redirect → callback → dashboard)
   - Test Stripe checkout flow (create session → redirect)
   - Test purchase gating (premium page blocked → allowed after purchase)
3. **Production**: Deploy to Vercel, run same Playwright suite against prod URL
4. **Chrome DevTools MCP**: Verify network requests, check for console errors
