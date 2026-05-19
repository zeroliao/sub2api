# Codex Guide for `sub2api`

`sub2api` is the application source repository. It contains the backend, frontend, migrations, and Docker image inputs for the Sub2API service. Production deployment is managed separately by `../zero007-sub2api-ops`.

## Read First

- `README.md` / `README_CN.md`: product overview and setup.
- `DEV_GUIDE.md`: development workflow.
- `../zero007-sub2api-ops/AGENTS.md`: only when the change will be deployed or affects production operations.

## Architecture Map

- `backend/`: Go backend.
  - `cmd/server/`: application entrypoint and wiring.
  - `internal/handler/`: HTTP/API handlers and routing behavior.
  - `internal/repository/`: persistence, caching, operational queries, integrations.
  - `internal/pkg/`: upstream compatibility, OAuth helpers, validation, shared packages.
  - `migrations/`: SQL migrations; review carefully for destructive changes.
  - `ent/`: generated Ent code; do not hand-edit unless the local workflow expects it.
- `frontend/`: Vue 3 + Vite frontend.
  - `src/views/`: page-level admin/user/auth views.
  - `src/components/`: reusable UI and feature components.
  - `src/stores/`: Pinia stores.
  - `src/router/`: routes and guards.
- `deploy/`: application-level deployment examples and Docker support, not the production ops source of truth.
- `Dockerfile`, `backend/Dockerfile`, `Dockerfile.goreleaser`: image build inputs.

## Toolchain

- Go version: `1.26.3` (`backend/go.mod`).
- Frontend: Node + pnpm, Vue 3, Vite, TypeScript, Vitest.
- Backend tests/builds are usually run from `backend/`.
- Frontend tests/builds are usually run from `frontend/`.

## Common Commands

From repository root:

```powershell
make build
make test
make test-backend
make test-frontend
```

Backend-focused:

```powershell
cd backend
go test ./...
```

Frontend-focused:

```powershell
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run <test files>
pnpm --dir frontend run build
```

## Implementation Rules

- Follow existing local patterns before adding new abstractions.
- Keep Ent-generated code and migrations consistent with repository conventions.
- For SQL migrations, avoid destructive statements unless explicitly requested and documented.
- If a migration may include `DROP TABLE`, `DROP COLUMN`, broad `DELETE`, or lossy type changes, flag it for ops deployment confirmation.
- Do not put production secrets in this repository.
- Do not treat `sub2api/deploy/docker-compose*.yml` as production truth. Production compose lives in `../zero007-sub2api-ops/deploy/docker-compose.yml`.

## When Deployment Is Needed

Application code changes are not deployed by copying source to the server. The normal path is:

1. Build a Docker image from this repository.
2. Push the image to a registry.
3. Update `../zero007-sub2api-ops/deploy/docker-compose.yml` to the immutable image digest.
4. Commit and push both repositories.
5. Run Git-backed ops deployment from `zero007-sub2api-ops`.

If the sibling ops repository is missing on a new machine, clone `git@github.com:zeroliao/zero007-sub2api-ops.git` next to this repository before doing production deployment work.

## Deployment Consent Rule

Do not deploy after code changes unless the user explicitly asked for deployment in the same request or confirmed it after being asked.

- Direct deploy is allowed when the user says things like "改完并部署", "直接部署", "自动部署", "部署到服务器", or equivalent.
- If the user only asks to implement, fix, optimize, or prepare a change, stop after local verification and ask before running any production deploy command.
- `validate-candidate`, `doctor`, `status`, `active-slot`, and `logs` are allowed as checks, but `deploy`, `bluegreen-deploy`, `start-deploy`, and `start-bluegreen-deploy` require explicit deploy intent or confirmation.
