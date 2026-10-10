# Retail-Learn

A hobby project to learn how to build an e-commerce store end to end: Next.js storefront plus Go backend services. Purely for learning, so favor clarity and small steps over cleverness.

## Stack
- **Frontend:** Next.js (App Router), TypeScript, Tailwind CSS, in `web/`
- **Backend:** Go, in `services/<name>/`
- **Data:** MongoDB (catalog, orders, accounts), Redis (cart, sessions, cache)
- **Messaging:** Kafka (added in Phase 4, not yet in docker-compose)
- **Payments:** Stripe in test mode only (Phase 3)
- **Local infra:** Docker Compose. Kubernetes comes in Phase 6.

## Repo layout
```
Retail-Learn/
├── CLAUDE.md
├── docker-compose.yml
├── web/                 Next.js storefront
└── services/
    └── catalog/         first Go service (products, variants, collections)
```
Future services (cart, order, inventory, account, notification) go in `services/<name>/`, each with its own `go.mod` and its own database or collection. Services never read another service's data directly.

## Commands
- Start infra: `docker compose up -d` (MongoDB on 27017, Redis on 6379)
- Stop infra: `docker compose down` (add `-v` to wipe data)
- First-time web setup: `./scripts/setup-web.sh` (runs create-next-app, applies the Phase 0 page; needs internet)
- Frontend: `cd web && npm run dev` (http://localhost:3000)
- Catalog service: `cd services/catalog && go run .` (http://localhost:8081, env: `PORT`, `MONGO_ADDR`; see `.env.example`)
- Health check: `curl -i localhost:8081/health` (200 when Mongo is reachable, 503 when not)
- Go tests: `cd services/catalog && go test ./...`
- Frontend checks: `cd web && npm run lint && npm run build`

## Architecture decisions
- Start as a modular monolith-style set of small services, and split further only when it teaches something.
- Data model mirrors Shopify: Product, Options (Color, Size), Variants (SKU, price, compare-at price, inventory), Images, Tags, Collections.
- Money is stored as integer cents plus a currency code, never floats.
- Every service exposes `GET /health`.
- Cart and checkout write endpoints must be idempotent (idempotency key header).
- Prices and stock are always validated on the server, never trusted from the client.

## Conventions
- Go: standard library `net/http` first, small packages, table-driven tests, errors wrapped with context, config via environment variables.
- TypeScript: strict mode, server components by default, `"use client"` only where interactivity is needed (cart, filters).
- No secrets in git. Use `.env` files (gitignored) and keep an `.env.example`.
- Local-only: no real payments, no real customer data.

## Workflow rules for Claude
1. Plan first, then build. Show the plan for anything bigger than a small change.
2. Work in one vertical slice at a time (data, API, UI, tests), then stop for review.
3. Run the tests and the app yourself before saying something works.
4. Keep commits small with clear messages. Do not commit unless asked.
5. Explain new concepts briefly as you introduce them. This is a learning project.
6. Update this file when a decision, command, or convention changes.

## Roadmap
- [ ] Phase 0: foundations (repo, compose, scaffold web and catalog, web calls `/health`) — Go service done and tested; run `./scripts/setup-web.sh` to finish the web side, then tick this
- [ ] Phase 1: catalog and storefront (products, variants, collections, search, seed data)
- [ ] Phase 2: cart (Redis) and accounts (auth, profile, addresses)
- [ ] Phase 3: checkout, Stripe test payments, orders
- [ ] Phase 4: inventory reservation and Kafka events, transactional emails
- [ ] Phase 5: split into separate services, gateway, idempotency and retries
- [ ] Phase 6: admin dashboard, observability, CI, Kubernetes deploy
- [ ] Phase 7: stretch (reviews, wishlist, loyalty, recommendations)

## Decision log
- 2026-10-09: Chose Next.js + Go + MongoDB + Redis. Kafka deferred to Phase 4.
- 2026-10-10: Catalog `/health` checks Mongo with a plain TCP dial (stdlib only) for Phase 0. Swap to the official Mongo driver's `Ping` in Phase 1 when the service starts storing data (`go get go.mongodb.org/mongo-driver/v2/mongo`). Next.js calls the Go service from server components, never from the browser.
- 2026-10-10: Cloud sessions here cannot reach the npm registry or the Go module proxy (org egress policy). Anything needing `npm install` or `go get` is run on the local machine.
- 2026-10-10: Next.js 16 scaffold enables `cacheComponents`. Do not use `export const dynamic`; wrap components that fetch live data (calls to Go services) in `<Suspense>`, and only use `"use cache"` where caching is intended.
