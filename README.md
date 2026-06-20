# Sub2API Admin Plus

Sub2API Admin Plus is an operations automation extension built from the Sub2API codebase.

MVP 0 keeps the Sub2API frontend/backend architecture, UI conventions, build scripts, and deployment layout as a runnable baseline. The current business layer already includes real operations APIs, pages, scheduler task generation, Chrome-extension result ingestion, and a minimal Chrome MV3 executor. Supplier-specific browser page adapters are still being built.

## Scope

- Keep the Sub2API Go/Gin backend structure.
- Keep the Sub2API Vue/Vite/Tailwind frontend structure and UI style.
- Reuse Sub2API admin authentication semantics.
- Reuse PostgreSQL and Redis infrastructure, with Admin Plus data isolated by database and Redis key prefix.
- Add operations automation features incrementally.

## Current Status

Implemented:

- Supplier parent records.
- Supplier account/key child bindings to local Sub2API `accounts.id`.
- Rate, balance, health, promotion, billing, reconciliation, extension task, and action recommendation APIs.
- Scheduler API and page for generating idempotent Chrome extension tasks.
- Chrome extension task result ingestion into rate, balance, promotion, health, and billing tables.
- Browser login credentials encrypted at rest and exposed only through a valid extension task lease.
- Minimal Chrome extension executor in `extension/`.
- Chrome extension parser smoke tests in `extension/test-parser.cjs`.
- Local Sub2API read adapter for real `accounts` and `usage_logs`.
- Local Sub2API Redis read adapter for account concurrency and waiting queue runtime.
- Admin Plus operation pages, including supplier bindings, account runtime, billing reconciliation, and local usage.
- API E2E script using real HTTP, PostgreSQL, and Redis fixtures.

Not implemented yet:

- Supplier-specific Chrome extension adapters for stable Sub2API/New API page login, scraping, and bill export.
- Sub2API window-cost/runtime limit adapter beyond current concurrency keys.
- Notification and audit execution loop.
- Confirmed action execution through Sub2API Admin API.

## MVP 0 Rules

- Do not modify the upstream Sub2API repository at `/Users/coso/Documents/dev/go/sub2api`.
- Do not rewrite the Go module path yet; the backend still imports `github.com/Wei-Shaw/sub2api` internally to keep the cloned baseline buildable.
- Do not delete large Sub2API backend/frontend modules until the baseline is verified.
- Keep product and architecture notes in `docs/`.

## Source Baseline

- Source path: `/Users/coso/Documents/dev/go/sub2api`
- Source commit: `4a5665da5b2c6b83c4597844ea6e573746c821b1`

## Development

Backend:

```bash
cd backend
go test ./...
go build -o bin/server ./cmd/server
```

Frontend:

```bash
cd frontend
pnpm install
pnpm run typecheck
pnpm run build
```

Focused verification:

```bash
cd backend
go test ./internal/adminplus/... ./internal/handler/adminplus/... ./internal/server/routes/...

cd ../frontend
pnpm run typecheck
pnpm run test:run -- src/router/__tests__/admin-plus-routes.spec.ts

cd ..
node tools/admin-plus-e2e.mjs
```

E2E defaults:

- `ADMIN_PLUS_BASE_URL=http://localhost:3000`
- `ADMIN_PLUS_E2E_EMAIL=admin@sub2api-admin-plus.local`
- `ADMIN_PLUS_E2E_PASSWORD=AdminPlus@123456`
- `ADMIN_PLUS_E2E_DB_URL=postgresql://root:root@127.0.0.1:5432/sub2api_admin_plus?sslmode=disable`
- `ADMIN_PLUS_E2E_REDIS_URL=redis://127.0.0.1:6379/0`

The E2E script creates `e2e-*` rows in PostgreSQL and temporary Redis runtime keys to verify real API/DB/Redis paths. These rows and keys are test fixtures, not mock production collection.

## Chrome Extension

The minimal MV3 executor lives in `extension/`.

It can:

- import the current Admin Plus `auth_token` from an Admin Plus tab;
- claim extension tasks;
- fetch supplier browser credentials with `task_id + device_id + lease_token`;
- open the supplier dashboard and run generic Sub2API/New API-like DOM extraction;
- complete the task only when real page data is parsed, otherwise fail the task.

Generic DOM extraction is intentionally conservative and covered by `node extension/test-parser.cjs`. Production support still requires supplier-specific adapters for each real dashboard shape.

## Sub2API Read Integration

Admin Plus writes its own data to the Admin Plus database. To read real local Sub2API accounts and usage from another database, set:

```bash
export SUB2API_READONLY_DATABASE_URL='postgresql://root:root@127.0.0.1:5432/sub2api?sslmode=disable'
```

If this variable is not set, the backend falls back to the current database connection for local MVP verification.

To read Sub2API runtime concurrency from another Redis DB or URL, set one of:

```bash
export SUB2API_READONLY_REDIS_DB=0
export SUB2API_READONLY_REDIS_URL='redis://127.0.0.1:6379/0'
```

If neither variable is set, Admin Plus reuses the current Redis client. The runtime adapter only reads Sub2API keys such as `concurrency:account:{id}` and `wait:account:{id}`.

## Documentation

- Product requirements: `docs/sub2api-admin-plus-prd.md`
- Code structure plan: `docs/code-structure.md`
- MVP baseline/progress: `docs/mvp0-baseline.md`
