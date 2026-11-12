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
│   └── src/
│       ├── app/         pages: / , /collections/[handle], /products/[handle], /search, /cart (+ cart/actions.ts Server Actions)
│       ├── components/  Header, CartLink, ProductCard/Grid
│       └── lib/         catalog.ts and cart.ts (server-side API clients), url.ts (query-string helpers), format.ts (money)
└── services/
    ├── cart/            Redis-backed cart service (port 8082)
    │   ├── main.go
    │   └── internal/
    │       ├── cart/            domain: limits, Service, Store interface, in-memory store
    │       │   └── carttest/    shared contract tests every Store must pass
    │       ├── redisstore/      Redis implementation (the only package importing go-redis); apply.lua does each change atomically
    │       ├── catalogclient/   HTTP client for the catalog's GET /variants
    │       ├── server/          HTTP routes, handlers, middleware
    │       └── config/          environment-based settings
    └── catalog/         first Go service (products, variants, collections)
        ├── main.go              wiring and graceful shutdown
        ├── cmd/seed/            loads the sample catalog into MongoDB
        └── internal/
            ├── catalog/         domain: model, query types (filter, sort, cursor), Repository interface, Service, in-memory repo
            │   └── repotest/    shared contract tests that every Repository implementation must pass
            ├── mongostore/      MongoDB implementation of Repository (the only package importing the driver); queries.go builds filter/sort/facet documents
            ├── server/          HTTP routes, handlers, middleware
            ├── seed/            deterministic sample products (~53 active + 1 draft) and 15 collections
            └── config/          environment-based settings
```
Future services (cart, order, inventory, account, notification) go in `services/<name>/`, each with its own `go.mod` and its own database or collection. Services never read another service's data directly.

## Commands
- Start infra: `docker compose up -d` (MongoDB on 27017, Redis on 6379)
- Stop infra: `docker compose down` (add `-v` to wipe data)
- First-time web setup: `cd web && npm install` (needs internet). Create `web/.env.local` with `CATALOG_URL=http://localhost:8081`
- Frontend: `cd web && npm run dev` (http://localhost:3000)
- Cart service: `cd services/cart && go run .` (http://localhost:8082, env: `PORT`, `REDIS_URL`, `CATALOG_URL`; needs Redis up and the catalog running). Exits at startup if Redis is unreachable.
- Catalog service: `cd services/catalog && go run .` (http://localhost:8081, env: `PORT`, `MONGO_URI`, `MONGO_DB`; see `.env.example`). Exits at startup if MongoDB is unreachable.
- Seed the catalog: `cd services/catalog && go run ./cmd/seed` (idempotent; upserts products and collections by handle; re-run after pulling 1b to backfill `priceFromCents`)
- Catalog API:
  - `GET /health`
  - `GET /products?limit=&after=<cursor>&sort=price_asc|price_desc|newest&type=&tag=&minPrice=&maxPrice=&q=` (`tag` repeatable, AND; prices in cents; `q` = text search)
  - `GET /products/facets` (same filters), `GET /products/{handle}`
  - `GET /collections`, `GET /collections/{handle}`, `GET /collections/{handle}/products` (same params as `/products`), `GET /collections/{handle}/facets`
  - `GET /variants?sku=A&sku=B` (price/title/image per SKU for other services; unknown and draft SKUs are omitted; max 50)
- Cart API (service on :8082):
  - `POST /carts` -> `{id}` (nothing is stored until the first item); `GET /carts/{id}` (priced from the catalog); `DELETE /carts/{id}`
  - `POST /carts/{id}/items` `{sku, quantity}` with required `Idempotency-Key` header (retry replies carry `Idempotent-Replayed: true`)
  - `PUT /carts/{id}/items/{sku}` `{quantity}` (0 removes), `DELETE /carts/{id}/items/{sku}`
  - Limits: quantity 1-10 per line, 50 lines, 30-day expiry renewed on change. 400 invalid, 409 limit reached, 503 catalog down.
- First-time Go deps (needs internet): `cd services/catalog && go get go.mongodb.org/mongo-driver/v2 && go mod tidy`; and for the cart: `cd services/cart && go get github.com/redis/go-redis/v9 && go mod tidy`
- Go tests: `cd services/catalog && go test ./...` and `cd services/cart && go test ./...`
- Mongo integration tests: `cd services/catalog && MONGO_URI=mongodb://localhost:27017 go test ./internal/mongostore/` (skipped when `MONGO_URI` is unset)
- Redis integration tests: `cd services/cart && REDIS_URL=redis://localhost:6379 go test ./internal/redisstore/` (skipped when `REDIS_URL` is unset)
- Frontend checks: `cd web && npm run lint && npm run build`

## Architecture decisions
- Start as a modular monolith-style set of small services, and split further only when it teaches something.
- Data model mirrors Shopify: Product, Options (Color, Size), Variants (SKU, price, compare-at price, inventory), Images, Tags, Collections.
- Variants are embedded in the product document (one read, atomic updates). Inventory is NOT stored in the catalog; it becomes its own service in Phase 4.
- The public identifier of a product is its `handle` (lowercase slug). List endpoints return small summaries (card data), detail endpoints return the full product.
- Pagination is keyset-based, not offset-based. `after` is an opaque base64url cursor holding the last item's sort key and handle, and it is bound to the sort order it was issued for (a mismatch is a 400). The service asks the repository for `limit+1` rows to know whether a next page exists. The handle is always the final sort key (it is unique), so order is total and pages never skip or repeat.
- Collections are rules, not lists: a collection is a handle, title and one tag; it contains every active product with that tag (Shopify "smart collection").
- `priceFromCents` (cheapest variant) is a derived field stored on the product document so Mongo can filter, sort and index on it. `ComputeDerived()` must run before every save (both repositories' `Upsert` do it).
- Search returns the top 50 by relevance, no paging. Mongo uses a weighted text index (title 10, productType 5, tags 5, description 1) with stemming; `MemoryRepo` approximates it with whole-word matching and the same weights, so tests share one contract (`repotest`) but not identical ranking details.
- `facets` is a reserved product handle (`/products/facets` is a literal route).
- Every `Repository` implementation must pass `repotest.Run`; every cart `Store` must pass `carttest.Run`.
- Services talk to each other only over HTTP APIs, never through each other's data. The cart asks the catalog's `GET /variants` for prices; it stores only SKU and quantity, so a price can never be stale or client-supplied. A delisted SKU stays in the cart as an unavailable line (removable, not charged).
- Cart storage is one Redis hash per cart (`cart:{id}`, field = SKU, value = quantity) with a 30-day sliding expiry. Every change runs through one Lua script (`redisstore/apply.lua`) so limits and the idempotency marker are enforced atomically. The `{id}` hash tag keeps a cart and its markers on one node in a Redis Cluster.
- Idempotency: `POST .../items` requires an `Idempotency-Key`; the marker (`idem:{id}:<key>`, 24h) is written in the same script only after success, so a retry never double-adds and a rejected add can be retried. `PUT` is idempotent by nature.
- A cart id (128 random bits) is the cart's only credential: it lives in an httpOnly, SameSite=Lax cookie set by the Next.js server, is shape-checked everywhere, and is never logged (the cart service logs the route pattern, not the URL).
- Next.js Server Actions (`web/src/app/cart/actions.ts`) are public POST endpoints: they re-validate their arguments and the cart service validates again. Cookies can only be written there (or in route handlers), not during render.
- Storefront state lives in the URL: filters, sort and paging are plain links (`?type=&min=&max=&sort=&after=`), so pages are shareable and need no client JavaScript. The only client component is the product `VariantPicker`.
- In Next 16, `params` and `searchParams` are Promises and reading them is dynamic: pages are a sync shell that renders an inner async component inside `<Suspense>`.
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
- [x] Phase 1: catalog and storefront
  - [x] 1a: data model, Mongo store, seed data, `/products` and `/products/{handle}` (Mongo integration tests and seed run verified locally)
  - [x] 1b: tag-based collections, filters (type, tag, price), sort, text search, facets (Mongo tests and seed run verified locally)
  - [x] 1c: storefront pages (home, collection with filters/sort/paging, product with variant selector, search) and placeholder SVGs (lint, build and click-through verified locally)
- [ ] Phase 2: cart (Redis) and accounts (auth, profile, addresses)
  - [x] 2a: cart service (Redis), catalog `GET /variants`, storefront cart (add to cart, header count, /cart page) (built in cloud; local Redis tests and click-through pending)
  - [ ] 2b: account service (register, login, Redis sessions, profile, addresses) and its pages
  - [ ] 2c: join them (merge the guest cart into the user's cart on login)
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
- 2026-10-16: Phase 1b added filters, sorts, text search, facets and tag-rule collections. Cursor pagination now covers every sort. Mongo query building lives in `mongostore/queries.go` as pure functions with unit tests that need no database. Cloud checks use a driver stub, so the real driver is only exercised by `MONGO_URI=... go test ./internal/mongostore/` locally.
- 2026-11-03: Phase 1c storefront was written without `node_modules` (npm registry blocked in cloud), so cloud checks covered only SVG validity, strict TypeScript on `src/lib`, and helper logic. Pages and components were verified locally with `npm run lint && npm run build` and by running the app (2026-11-05). Phase 1 complete.
- 2026-11-11: Phase 2a added `services/cart` (own go.mod, Redis via go-redis v9) and the catalog's `GET /variants`. The Lua script was verified against a real redis-server in the cloud sandbox; the Go glue was type-checked against a stub, so `REDIS_URL=... go test ./internal/redisstore/` locally is the real check. Web code is checked only for strict types and the cart client's retry logic (mocked fetch); pages need `npm run lint && npm run build` locally.
