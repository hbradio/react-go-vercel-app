# React + Go Vercel App Design

## Overview

I want a serverless app starter to deploy to Vercel. It should have:
* A Go API backend
* A React SPA frontend
* CockroachDB
* Auth0 auth
* Stripe one-time purchase to unlock content

## Details

### Go API backend
Do whatever is cleanest and more Vercel-native: a single Go file that serves many endpoints, or many Go files with one for each endpoint.

### CockroachDB Cloud
* Pair with Auth0 to create a user model. Pair with Stripe to store subscription level.
* Use a simple local db when running locally.
* Do all setup for Cockroach yourself via their management api. Your CockroachDB Cloud service account info is in creds.md

### Auth0
* Support username/password.
* Do all Auth0 setup via the management console. Install their cli if you need to. Keys, examples, and docs are in creds.md

### React SPA frontend
* Use typescript.

### Stripe
* A subscription that unlocks full access to the app
* Do all Stripe account setup yourself via the API. See creds.md for keys and docs.

## Out of Scope
* No email sending, for password reset nor for anything else.

## Guidance
* Look up the latest docs as often as possible; do not rely on your knowledge. This applies to Vercel, CockroachDb, React bootstrapping, Stripe, Auth0
* You all of the cloud configuration (Auth0, CockroachDB, Stripe, Vercel) yourself. creds.md should have all of the credentials you need. Stop and prompt me if you're missing some credential.
* I want to end up with human-maintainable code. This is a starter/boilderplate app. Make code as lean and minimal as possible while still being solid.
* Do not be afraid to use popular third party libraries in both React and Go.
* Always set `npm config set prefix "C:\Development\Repositories\Experiments"` before installing any global npm tools.

## Style
* Leave the style raw and brutal. Add css files to show me the pattern, but leave it mostly unstyled so that I can style it later.

## Verification
* I want you to iterate and fix issues yourself, and only come to me with a fully running app. Use tests and Playwright to do this.
* Use your Playwright MCP server to verify every feature you build.
* Use whatever harness you need to serve the Go API endpoints locally for testing
* Use Playwright to execute and verify every feature.
  * You will not be able to install Chrome headless for Playwright. Configure it to use the already installed full Chrome.
* Use the chrome-devtools MCP server when appropriate to verify changes.

## Order of Work
* You should first verify your administration API access to Auth0, CockroachDB, Stripe, and Vercel using the credentials and instructions in creds.md.
