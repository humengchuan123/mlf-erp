# Repository guidance

- Toolchain: Go 1.25.11+ and pnpm 8.6.12. The pnpm workspace is `apps/*` + `packages/*`; install with `pnpm install`.
- Go entrypoints are in `cmd/` (`api`, `worker`, `migrate`, `seed`, `demo`, `glbackfill`). `cmd/api` wires services; keep business logic in `internal/modules/<domain>` and shared infrastructure in `internal/platform`.
- Modules follow `http → app → domain` and `infra → domain`; keep framework and database dependencies out of `domain`. Web app is `apps/web`; shared frontend packages are in `packages/`.
- Preserve tenancy: business data is scoped by `tenant_id`; `org_id` is the tenant-internal organization boundary. API money values are strings, domain calculations use `shopspring/decimal`, and PostgreSQL stores `numeric`—never use floats for money. Gate features by capability, not deployment edition.
- Migrations are numbered SQL files in `db/migrations` and embedded into the binary. Read `db/migrations/README.md` before changing them; data-repair migrations may have no-op downs, so rollback is not necessarily safe.
- Run Go checks in CI order: `go vet ./...`, then `go test ./...`. For focused tests, use `go test ./internal/modules/<domain>/...` or add `-run TestName`.
- Web CI order: `pnpm typecheck:web`, `pnpm test:frontend`, then `pnpm build:web`. `test:frontend` is a static completeness check, not a browser test suite.
- API integration scripts are `pnpm test:api:stage3b`, `pnpm test:api:stage3c`, and `pnpm test:api:stage3d`. They require PowerShell and Docker, migrate and seed the Compose PostgreSQL database, and run the API on port 18080; use only when that local database can be modified.
