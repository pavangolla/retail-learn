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
        ├── main.go              wiring and graceful shutdown
        ├── cmd/seed/            loads the sample catalog into MongoDB
        └── internal/
            ├── catalog/         domain: model, validation, Repository interface, Service, in-memory repo
            ├── mongostore/      MongoDB implementation of Repository (the only package importing the driver)
            ├── server/          HTTP routes, handlers, middleware
            ├── seed/            deterministic sample products (~53 active + 1 draft)
            └── config/          environment-based settings
```
Future services (cart, order, inventory, account, notification) go in `services/<name>/`, each with its own `go.mod` and its own database or collection. Services never read another service's data directly.

## Commands
- Start infra: `docker compose up -d` (MongoDB on 27017, Redis on 6379)
- Stop infra: `docker compose down` (add `-v` to wipe data)
- First-time web setup: `cd web && npm install` (needs internet). Create `web/.env.local` with `CATALOG_URL=http://localhost:8081`
- Frontend: `cd web && npm run dev` (http://localhost:3000)
- Catalog service: `cd services/catalog && go run .` (http://localhost:8081, env: `PORT`, `MONGO_URI`, `MONGO_DB`; see `.env.example`). Exits at startup if MongoDB is unreachable.
- Seed the catalog: `cd services/catalog && go run ./cmd/seed` (idempotent; upserts by handle)
- Catalog API: `GET /health`, `GET /products?limit=20&after=<handle>`, `GET /products/{handle}`
- First-time Go deps (needs internet): `cd services/catalog && go get go.mongodb.org/mongo-driver/v2 && go mod tidy`
- Go tests: `cd services/catalog && go test ./...`
- Mongo integration tests: `cd services/catalog && MONGO_URI=mongodb://localhost:27017 go test ./internal/mongostore/` (skipped when `MONGO_URI` is unset)
- Frontend checks: `cd web && npm run lint && npm run build`

## Architecture decisions
- Start as a modular monolith-style set of small services, and split further only when it teaches something.
- Data model mirrors Shopify: Product, Options (Color, Size), Variants (SKU, price, compare-at price, inventory), Images, Tags, Collections.
- Variants are embedded in the product document (one read, atomic updates). Inventory is NOT stored in the catalog; it becomes its own service in Phase 4.
- The public identifier of a product is its `handle` (lowercase slug). List endpoints return small summaries (card data), detail endpoints return the full product.
- Pagination is keyset-based (`after=<last handle>`), not offset-based. The service asks the repository for `limit+1` rows to know whether a next page exists.
- Product image URLs are paths served by the storefront (`/placeholders/<name>.svg` from `web/public/placeholders/`), not catalog-hosted files.
- Storefront-facing repository methods only ever return `active` products; drafts are invisible (404).
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
- [x] Phase 0: foundations (repo, compose, scaffold web and catalog, web calls `/health`)
- [ ] Phase 1: catalog and storefront
  - [ ] 1a: data model, Mongo store, seed data, `/products` and `/products/{handle}` (code written; needs `go get` + Mongo integration run on the local machine)
  - [ ] 1b: tag-based collections, filters (type, tag, price), sort, text search
  - [ ] 1c: storefront pages (collection, product with variant selector, search) and placeholder SVGs
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
- 2026-10-10: Phase 1a replaced the TCP health check with the official MongoDB driver (v2). The service now fails fast at startup if Mongo is unreachable; `/health` still reports 503 if Mongo goes down later. Config changed from `MONGO_ADDR` to `MONGO_URI` + `MONGO_DB`.
- 2026-10-10: Cloud sessions cannot compile the Mongo driver, so the Mongo layer is isolated in `internal/mongostore`; everything else is tested against `catalog.MemoryRepo`. Cloud-side type-checking of driver code uses a throwaway API stub and does not replace running the integration tests locally.
- 2026-10-10: Next.js 16 scaffold enables `cacheComponents`. Do not use `export const dynamic`; wrap components that fetch live data (calls to Go services) in `<Suspense>`, and only use `"use cache"` where caching is intended.
