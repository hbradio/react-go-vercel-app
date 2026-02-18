# Implementation Tips

Lessons learned from the initial build. These would have saved debugging time if known upfront.

---

## CRITICAL: Commit before scaffolding

**Always `git add . && git commit` before running `npm create vite@latest . -- --overwrite` or any scaffolding tool.** The `--overwrite` flag deletes existing files in the directory. If you haven't committed, those files are gone forever. This happened to `spec.md`, `creds.md`, `Auth0-react.md`, and `Auth0-go.md` during this build.

---

## Vite Scaffolding

### Non-empty directory requires `--overwrite`
Running `npm create vite@latest . -- --template react-ts` in a directory with existing files results in `Operation cancelled`. You must pass `--overwrite`:
```bash
npm create vite@latest . -- --template react-ts --overwrite
```
This silently deletes all non-Vite files in the directory (markdown files, config files, etc.), which is why the "commit first" rule above is critical.

---

## Windows Bash Environment

### Shell state does not persist between tool calls
Environment variables set with `export` in one Bash invocation do not exist in the next. Multi-step workflows that depend on a token or variable must either:
- Save to a file: `echo $TOKEN > /tmp/token.txt` and read it back
- Do everything in a single Bash call chained with `&&`

### curl quoting breaks on Windows bash
Single-quoted JSON payloads in curl (`--data '{...}'`) fail with `curl: option : blank argument where content is expected`. The Windows bash layer mangles the quotes.

**Fix:** Use Python's `urllib.request` for all HTTP calls instead of curl. It's available without pip installs and handles quoting/encoding reliably on Windows.

```python
python -c "
import urllib.request, json

data = json.dumps({'key': 'value'}).encode()
req = urllib.request.Request('https://example.com/api', data=data,
    headers={'Content-Type': 'application/json'})
resp = json.loads(urllib.request.urlopen(req).read())
print(resp)
"
```

### Piping curl to python also fails
`curl ... | python -c "..."` double-fails because both curl and the pipe have quoting issues on Windows bash.

### `pkill` doesn't exist on Windows — use `taskkill`
`pkill -f "process-name"` returns exit code 1 on Windows Git Bash. Use the Windows-native command instead:
```bash
taskkill //F //IM "local-server.exe"
```
Note the double slashes (`//F`) — Git Bash interprets single `/F` as a path.

### `pip` command not found — use `python -m pip`
On Windows, `pip` may not be on PATH. Always use:
```bash
python -m pip install package-name
```

### `ls -la` returns exit code 2
Windows Git Bash `ls` often returns exit code 2 even when it works. Don't rely on exit codes from `ls` for control flow.

---

## Auth0 Management API

### Get a management token first, save it to a file
```python
# Save token for reuse across calls
token = get_token_response['access_token']
with open('C:/tmp/auth0_token.txt', 'w') as f:
    f.write(token)
```
Tokens last 24 hours by default, so one token covers an entire setup session.

### `enabled_clients` is NOT writable on PATCH /connections/{id}
Attempting `PATCH /api/v2/connections/{id}` with `{"enabled_clients": [...]}` returns:
```
400: "Additional properties not allowed: enabled_clients"
```
The `enabled_clients` field is **read-only** on the connections endpoint.

**How to check instead:** Use `GET /api/v2/clients/{client_id}/connections` to see which connections are already enabled for a client. In practice, the `Username-Password-Authentication` connection is enabled by default for new SPA apps — no manual enabling needed.

### Listing connections with `fields` filter can hide data
Requesting `GET /connections?fields=name,strategy,id,enabled_clients` returned `enabled_clients: []` even when connections were enabled. The field filtering behaved unexpectedly. Use `GET /clients/{id}/connections` instead to check client-connection associations.

### Resource server (API) creation is straightforward
```python
data = {
    'name': 'My API',
    'identifier': 'https://api.myapp.example.com',  # This becomes the audience
    'signing_alg': 'RS256',
    'token_lifetime': 86400
}
# POST /api/v2/resource-servers
```
The `identifier` becomes the `audience` value used in both the React Auth0Provider and Go JWT validation.

### Updating SPA client callback URLs
Use `PATCH /api/v2/clients/{client_id}` with:
```python
{
    'callbacks': ['http://localhost:5173'],
    'allowed_logout_urls': ['http://localhost:5173'],
    'web_origins': ['http://localhost:5173'],
    'allowed_origins': ['http://localhost:5173'],
    'grant_types': ['authorization_code', 'implicit', 'refresh_token'],
    'token_endpoint_auth_method': 'none'  # Required for SPA (public client)
}
```

---

## CockroachDB Cloud API

### Port 26257 may be blocked by corporate firewalls
CockroachDB Cloud uses port 26257 for SQL connections. Corporate networks often block non-standard ports. Port 443 is typically open but CockroachDB Cloud doesn't offer SQL over 443.

**Workaround for local dev:** Use Docker PostgreSQL instead. PostgreSQL is CockroachDB-compatible (same SQL, same `pgx` driver, zero code changes):
```bash
docker run -d --name starter-postgres \
  -e POSTGRES_USER=app_user \
  -e POSTGRES_PASSWORD=localdev \
  -e POSTGRES_DB=starter_app \
  -p 5432:5432 postgres:16-alpine
```
Then set `DATABASE_URL=postgresql://app_user:localdev@localhost:5432/starter_app?sslmode=disable`

### Cluster creation is fast but async
`POST /api/v1/clusters` returns immediately with `state: CREATING`. Poll `GET /api/v1/clusters/{id}` until `state: CREATED`. In practice, serverless clusters are ready almost instantly (first poll succeeded).

### Connection string format
```
postgresql://app_user:PASSWORD@react-go-starter-22263.j77.aws-us-east-2.cockroachlabs.cloud:26257/starter_app?sslmode=verify-full
```

### SQL user and database creation are separate calls
1. `POST /api/v1/clusters/{id}/sql-users` with `{"name": "app_user", "password": "..."}`
2. `POST /api/v1/clusters/{id}/databases` with `{"name": "starter_app"}`

---

## Stripe API

### Basic auth with secret key
Stripe uses HTTP Basic Auth where the secret key is the username and password is empty:
```python
import base64
auth = base64.b64encode(f'{secret_key}:'.encode()).decode()
headers = {'Authorization': f'Basic {auth}'}
```

### Product + Price creation is two calls
1. Create product: `POST /v1/products` with `name=X&description=Y`
2. Create price: `POST /v1/prices` with `unit_amount=999&currency=usd&product={product_id}`

Note: Stripe API uses form-encoded bodies, not JSON.

### Key IDs to save
After creation, save `product.id` (e.g. `prod_XXX`) and `price.id` (e.g. `price_XXX`). The price ID is needed for creating Checkout Sessions later.

### Auth0 access tokens don't include email by default
When requesting tokens for a custom API (audience), the access token contains `sub` (Auth0 user ID) but NOT the user's email. To get the email on the backend, call Auth0's `/userinfo` endpoint with the access token:
```go
req, _ := http.NewRequest("GET", "https://"+domain+"/userinfo", nil)
req.Header.Set("Authorization", "Bearer "+accessToken)
```
Only needed on first login (to create the DB record). After that, read from the database.

### Auth0 consent screen keeps appearing — fix with skip_consent
When using a custom API audience, Auth0 shows a consent screen ("Authorize App") on **every** token request unless you configure two things:
1. Mark the resource server to skip consent for first-party clients:
   ```python
   PATCH /api/v2/resource-servers/{id}
   {"skip_consent_for_verifiable_first_party_clients": true}
   ```
2. Mark the SPA client as first-party:
   ```python
   PATCH /api/v2/clients/{id}
   {"is_first_party": true}
   ```
Without both of these, the consent screen appears on every page reload/navigation that triggers a token refresh, making the app unusable. Do this immediately after creating the resource server.

---

## Vercel Go Functions

### Handler function naming pattern
All Go files in `/api/` share `package handler`. Give each handler a **unique name** (e.g., `HealthHandler`, `UserHandler`), not all `Handler`. Vercel compiles each file independently and auto-detects the single exported `http.HandlerFunc`. The local dev server imports them by name:
```go
mux.HandleFunc("/api/health", handler.HealthHandler)
mux.HandleFunc("/api/user", handler.UserHandler)
```

---

## Vercel Deployment

### Auth header format
```
Authorization: Bearer {token}
```
Verify with `GET /v2/user` to confirm the token works and get the username.

### GitHub push protection blocks hardcoded secrets
GitHub's push protection scans for API keys in committed files. The Stripe secret key in `package.json`'s `dev:stripe` npm script triggered this:
```
GITHUB PUSH PROTECTION — Push cannot contain secrets
Stripe Test API Secret Key — path: package.json:9
```
**Fix:** Reference env vars instead of hardcoding: `--api-key $STRIPE_SECRET_KEY`

### TypeScript `verbatimModuleSyntax` breaks Vercel builds
Vite's default `tsconfig.json` enables `verbatimModuleSyntax`. Type-only imports must use `import type`:
```typescript
// WRONG — fails on Vercel build
import { ReactNode } from 'react'

// RIGHT
import type { ReactNode } from 'react'
```
The error is: `TS1484: 'ReactNode' is a type and must be imported using a type-only import`. Always run `npm run build` locally before pushing.

### Vercel deployment API requires numeric `repoId`
When triggering deployments via `POST /v13/deployments`, the `gitSource` object requires the numeric `repoId`, not just the repo name:
```python
{
    "gitSource": {
        "type": "github",
        "repo": "hbradio/react-go-vercel-app",
        "repoId": 1161043543,  # Get from GET /v9/projects/{id} → link.repoId
        "ref": "master"
    }
}
```

### Updating Vercel env vars requires the env var ID
You can't just PATCH by key name. List all env vars first, find the ID, then update:
```python
# List: GET /v9/projects/{id}/env → find env var by key
# Update: PATCH /v9/projects/{id}/env/{env_id} with {"value": "new_value"}
```

### Env var changes require redeployment
After updating environment variables on a Vercel project, existing deployments don't pick up the changes. You must trigger a new deployment (push a commit or call the deployment API).

### CockroachDB Cloud port 26257 works from Vercel
Port 26257 is blocked by corporate firewalls locally, but Vercel's infrastructure can reach it. The auto-migration in `db.go` successfully creates the `users` table on the first API request from Vercel.

### Auth0 consent screen disappears on production (HTTPS)
The `skip_consent_for_verifiable_first_party_clients` setting only works on HTTPS origins that Auth0 can verify. On localhost (HTTP), the consent screen still appears on every token request. On production (HTTPS), it's completely skipped — confirmed working.

### Stripe webhook works end-to-end on production
The production webhook flow is seamless:
1. Create webhook endpoint via Stripe API: `POST /v1/webhook_endpoints` with production URL
2. Get the `secret` from the response
3. Set it as `STRIPE_WEBHOOK_SECRET` on Vercel
4. Redeploy
5. After purchase, Stripe sends `checkout.session.completed` → webhook verifies signature → updates CockroachDB → user sees "Premium" immediately on redirect back

### Vercel auto-deploys on GitHub push
Once the Vercel project is linked to a GitHub repo, every push to the production branch triggers an automatic deployment. No need to manually trigger via API after the initial setup.

---

## Go Module Tips

### Use `GOPROXY=direct` when the default proxy fails
Large packages (like `modernc.org/sqlite`) can fail to download through the default Go module proxy with `unexpected EOF`. Using `GOPROXY=direct` bypasses the proxy:
```bash
GOPROXY=direct go get modernc.org/sqlite
```

### Use `go mod tidy` after adding dependencies
`go get` adds the dependency but may not fetch all transitive deps needed for compilation. The specific error is:
```
missing go.sum entry for module providing package gopkg.in/go-jose/go-jose.v2
```
Always run `go mod tidy` after `go get` to resolve transitive dependencies.

### Auto-migration pattern for serverless
In serverless (Vercel), you can't run migrations as a separate step. Embed idempotent migrations in the db package's init:
```go
var once sync.Once
func GetDB() (*sql.DB, error) {
    once.Do(func() {
        pool, _ = sql.Open("pgx", os.Getenv("DATABASE_URL"))
        pool.Exec("CREATE TABLE IF NOT EXISTS ...")
    })
    return pool, nil
}
```
The `sync.Once` ensures migration runs exactly once per function instance.

### `auth0/go-jwt-middleware/v2` works without middleware
For serverless functions without a router, use the `validator` package directly — don't need the actual middleware wrapper:
```go
import "github.com/auth0/go-jwt-middleware/v2/validator"
// ...
claims, err := v.ValidateToken(ctx, tokenString)
validated := claims.(*validator.ValidatedClaims)
subject := validated.RegisteredClaims.Subject
```

---

## General Patterns

### Use Python for all API calls on Windows
Rather than fighting with curl quoting, use a single Python script pattern:
```python
python -c "
import urllib.request, json
# ... all API logic here
"
```

### Save intermediate results to files
When a workflow spans multiple tool calls, save IDs, tokens, and connection strings to `/tmp/` or a local file so they survive across calls.

### Check existing resources before creating
Always list existing resources first (`GET /products`, `GET /clients`, etc.) to avoid duplicates and understand what's already configured.

### Prefer Docker PostgreSQL over SQLite for local dev
When CockroachDB Cloud isn't reachable, Docker PostgreSQL is a much better local substitute than SQLite. Same SQL dialect, same driver (`pgx`), zero code branching. SQLite requires separate migration SQL, different parameter placeholders (`?` vs `$1`), different boolean/timestamp types, and a separate driver — all adding unnecessary complexity.

### Use a non-default port for Docker PostgreSQL
Map Docker PostgreSQL to a non-standard port (e.g., 5433) to avoid conflicts with any locally installed PostgreSQL:
```bash
docker run -d --name starter-postgres \
  -e POSTGRES_USER=app_user \
  -e POSTGRES_PASSWORD=localdev \
  -e POSTGRES_DB=starter_app \
  -p 5433:5432 postgres:16-alpine
```

---

## Stripe Testing

### Test card for sandbox checkout
Use card number `4242 4242 4242 4242` with any future expiration date and any 3-digit CVC. This always succeeds in Stripe sandbox mode.

### Stripe webhooks can't reach localhost — use Stripe CLI Docker
After completing a Stripe Checkout payment locally, the `checkout.session.completed` webhook won't fire because Stripe can't reach `localhost`. Use the Stripe CLI Docker container (see "Stripe CLI via Docker" section below) to forward webhooks locally. On production (Vercel), webhooks work natively via the registered webhook endpoint.

### Verify payments via Stripe API
After a test purchase, confirm it went through:
```python
GET /v1/checkout/sessions?limit=1
# Check: status=complete, payment_status=paid
```

### Uncheck "Save my information" on Stripe Checkout
The Stripe Checkout page has a "Save my information for faster checkout" checkbox (Stripe Link) enabled by default. When checked, it requires a phone number field. Uncheck it first if you want to test with just card details.

### Stripe Checkout form field order for MCP automation
When filling the Stripe Checkout card form via Chrome DevTools MCP:
1. Click the "Card" radio button to expand the card form
2. Fill fields: Card number, Expiration (MM/YY format), CVC, Cardholder name, ZIP
3. Click "Pay" button
The card fields are inside Stripe's iframe-based form — use the `fill_form` tool to fill multiple fields at once.

### stripe-go API version mismatch causes webhook 400s
`webhook.ConstructEvent()` rejects events if the Stripe API version in the event doesn't match the version stripe-go was built for. The Stripe CLI sends events with the latest API version, which may be newer than what stripe-go expects.

**Fix:** Use `ConstructEventWithOptions` with `IgnoreAPIVersionMismatch: true`:
```go
event, err := webhook.ConstructEventWithOptions(body, sig, secret, webhook.ConstructEventOptions{
    IgnoreAPIVersionMismatch: true,
})
```
The error message is: `Received event with API version X, but stripe-go expects API version Y`.

### Stripe CLI via Docker for local webhook forwarding
When the Stripe CLI can't be installed natively, use the Docker image:
```bash
docker run --rm --name stripe-cli stripe/stripe-cli listen \
  --api-key sk_test_... \
  --forward-to http://host.docker.internal:PORT/api/webhook \
  -s
```
Key details:
- Use `host.docker.internal` (not `localhost`) to reach the host machine from Docker
- Do NOT use `--network host` on Windows/Mac — it's Linux-only
- The `-s` flag suppresses update checks
- The CLI prints a `whsec_...` signing secret on startup — put this in `.env` as `STRIPE_WEBHOOK_SECRET`
- The secret persists across CLI restarts (cached by Stripe)
- Start the CLI BEFORE the API server, copy the `whsec_` into `.env`, then start the API server

---

## Verification with Chrome DevTools MCP

### Use Chrome DevTools MCP instead of Playwright
The Chrome DevTools MCP server (`chrome-devtools-mcp`) provides everything needed for end-to-end verification: page navigation, element clicking, form filling, text waiting, and screenshots. No need for Playwright — it avoids browser installation issues, headless configuration, and test framework boilerplate.

### Key MCP tools for testing
- `navigate_page` — go to a URL
- `take_snapshot` — get the page's a11y tree (shows all elements with UIDs)
- `click` — click an element by UID
- `fill_form` — fill multiple form fields at once
- `wait_for` — wait for text to appear (with timeout)
- `take_screenshot` — visual verification

### Testing flow pattern
1. Navigate to the page
2. `wait_for` the expected text to confirm it loaded
3. `take_snapshot` to find element UIDs
4. `click` or `fill_form` to interact
5. `wait_for` the next expected state
6. Repeat

### Stripe Checkout is testable via MCP
The Stripe Checkout page (hosted on `checkout.stripe.com`) is fully accessible via Chrome DevTools MCP. The card form fields, radio buttons, and checkboxes all have UIDs and respond to `fill_form` and `click`.
