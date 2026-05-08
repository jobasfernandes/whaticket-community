# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project-specific hard rules (override anything below)

1. **NUNCA escreva comentários em código.** Nem que expliquem "por quê", nem em português, nem em inglês. Zero comentários. Identificadores devem ser auto-explicativos. Esta regra sobrepõe a guidance global ("Comente o porquê, não o quê") — para este projeto: zero comentários, ponto. Documentação de regras de negócio vai para o `memory` MCP, NUNCA inline.
2. **NÃO comitar sem pedido explícito.** Mesmo em modo auto, criar commits é ação visível no histórico — peça confirmação.

## Repository layout

Three top-level projects:

- `backend/` — Go 1.25, chi v5 + GORM + Postgres. HTTP API + WebSocket + business rules. Talks to RabbitMQ (events to/from worker), Postgres (state), MinIO/S3 (outbound media).
- `worker/` — Go 1.25, whatsmeow + RabbitMQ + MinIO/S3. The only process that talks to WhatsApp.
- `frontend/` — React 16 + Material-UI 4, bundled with **Vite 4**. Env vars use the `VITE_` prefix; runtime config in `frontend/vite.config.js`.

Top-level orchestration:

- `docker-compose.local.yaml` — postgres, rabbitmq, minio (+ minio-init), backend, worker, frontend (profile-gated). This is the dev stack.
- `docker-compose.prod.yaml` — production-shaped stack.

The legacy Node/Sequelize backend was dropped in commit `089c661` (chore: drop legacy Node + restructure for Go stack). Anything you find that talks about Sequelize, MySQL, `npm run db:migrate`, `services/TicketServices/*.ts`, etc. is historical — see `git log --all` if you need it, do not assume it still applies.

## Common commands

### Backend (`cd backend`)

```bash
go run ./cmd/server                          # start the API on :8080
go run ./cmd/migrate up                      # apply pending Goose migrations (default cmd is "up")
go run ./cmd/migrate down                    # roll back the latest migration
go run ./cmd/seed                            # ensure admin user + base settings exist
go test ./...                                # all tests (some are integration; testcontainers will be spun up)
go test ./internal/<pkg>/... -run <Name>     # focused test
go vet ./...
gofmt -w .
```

Migrations are SQL files under `backend/migrations/` embedded via `backend/migrations/embed.go` and run by `pressly/goose`. Adding a migration = drop a new `NNNNN_<name>.sql` next to the existing ones.

### Worker (`cd worker`)

```bash
go run ./cmd/worker                          # start the whatsmeow worker (health on :8081)
go test ./...                                # includes minio + rabbitmq integration tests via testcontainers
```

The worker keeps its whatsmeow sqlstore on disk at `WORKER_SQLSTORE_PATH` (default `/var/lib/worker/whatsmeow.db` in container, mounted via the `worker-data` volume in compose).

### Frontend (`cd frontend`)

```bash
npm run dev          # vite dev server on :3000 (alias: npm start)
npm run build        # vite build -> build/
npm run preview      # vite preview of the built bundle
```

### Docker (full stack)

```bash
docker compose -f docker-compose.local.yaml up -d --build
docker compose -f docker-compose.local.yaml --profile frontend up -d --build      # include frontend container
docker compose -f docker-compose.local.yaml logs -f backend worker
docker compose -f docker-compose.local.yaml down                                  # keeps volumes
docker compose -f docker-compose.local.yaml down -v                               # nukes postgres + rabbitmq + minio + worker-data
```

`AUTO_MIGRATE=true` and `AUTO_SEED=true` are set in `docker-compose.local.yaml`, so the backend container migrates+seeds on every start. For local `go run`, set them explicitly in your env or run `cmd/migrate` and `cmd/seed` manually.

## Architecture — the parts that aren't obvious from the file tree

### Two-process design

```
              ┌──────────────┐    AMQP    ┌──────────────┐
   browser ── │  backend Go  │ ◄────────► │  worker Go   │ ── whatsmeow ── WhatsApp
              │ chi+GORM+ws  │            │ whatsmeow    │
              └──────┬───────┘            └──────┬───────┘
                     │                           │
                  Postgres                    sqlstore (on disk)
                     │                           │
                  MinIO  ◄──── outbound media ───┘
```

- **Backend** owns all business rules, HTTP/WS surface, and Postgres. It never imports whatsmeow. It uploads outbound media to MinIO, then publishes a `command` envelope on RabbitMQ telling the worker which session should send what.
- **Worker** owns the whatsmeow client per-WhatsApp-session, the sqlstore, and inbound media downloads. It publishes `event` envelopes for the backend to consume. RPC (request/reply) is used for calls that need a synchronous answer (e.g., pairing, listing chats).

### Backend feature packages (`backend/internal/<feature>/`)

Vertical-slice layout — one package per bounded context. Today's set:

- `auth` — JWT issue/verify, refresh cookie, `tokenVersion` invalidation.
- `user`, `contact`, `queue`, `quickanswer`, `setting` — CRUD + WS broadcast on mutation.
- `ticket` — ticket lifecycle, queue routing, WS authz hook for the WS hub.
- `message` — outbound message orchestration; uploads media to MinIO, publishes `command` to RMQ.
- `whatsapp` — WhatsApp connection management, RPC bridge to worker. `StartAllSessions` boots existing sessions on backend start.
- `waevents` — RMQ consumer that ingests inbound events from the worker (message in, ack, presence, …) and applies them to the DB / WS.
- `ws` — chi-mounted WebSocket hub at `GET /ws`. JWT validated in handshake; ticket-room access checked via `TicketAuthorizer`.
- `rmq` — amqp091 client wrapper, envelope shape, publisher, consumer, RPC client+server, topology setup.
- `media` — minio-go uploader. Returns `NoopClient` when `BACKEND_S3_ENDPOINT` is unset (uploads disabled, send still works for non-media).
- `db`, `dbmigrate`, `dbseed` — Postgres connection, Goose runner, idempotent admin seed.
- `platform/errors`, `platform/log` — `AppError{Code, Status, Message}` + slog setup. Imported as plain packages, no DI.

`cmd/server/main.go` is the wiring root — read it to understand who depends on whom. There is **no DI container**; deps are passed as plain `&Deps{...}` structs. `cmd/server/adapters.go` holds the small adapter types that bridge cross-package interfaces (e.g., `messageTicketAdapter`).

### Worker feature packages (`worker/internal/<feature>/`)

- `whatsmeow` — client lifecycle (`manager.go`, `session.go`, `sqlstore.go`), event publishing (`events.go`), pairing helpers, JID parsing, LRM (long-running media), device metadata.
- `command` — handlers for inbound commands from backend: `send`, `query`, `modify`, `pairphone`, `dispatch`. One handler per verb.
- `media` — inbound (download from WA), outbound (upload to MinIO if backend didn't pre-upload), sticker conversion (`HugoSmits86/nativewebp`).
- `linkpreview` — OG-tag scraping for outbound link messages.
- `rmq` — same envelope/RMQ abstractions as the backend, in a separate copy (no shared module).
- `health` — `:8081/health` endpoint for compose healthchecks.
- `platform/errors`, `platform/log`, `config` — same role as backend.
- `testenv` — testcontainers helpers for minio + rabbitmq integration tests.

### Real-time: WebSocket rooms

`backend/internal/ws` mounts `GET /ws`. The hub validates the JWT in the handshake, joins rooms on demand, and is what `ticket`/`message`/`waevents` publish to when state changes. The `TicketAuthorizer` is set by `ticket` after construction so the hub can deny `ticket:<id>` joins for users who aren't allowed to see that ticket.

### Auth flow

- JWT access token in `Authorization: Bearer …` header. 15 min default lifetime.
- Refresh token in httpOnly cookie. 7 days. Bumping `User.tokenVersion` invalidates all refresh tokens for that user (kill-switch).
- `accessSecret` and `refreshSecret` come from env (`JWT_SECRET`, `JWT_REFRESH_SECRET`); in dev, defaults are set in `docker-compose.local.yaml`.

### RMQ envelope

All messages on the bus share an envelope shape (`backend/internal/rmq/envelope.go`):

```go
type Envelope struct {
    Version   string          `json:"v"`     // currently "1"
    Timestamp int64           `json:"ts"`    // epoch millis
    Type      string          `json:"type"`  // event/command verb
    UserID    int             `json:"userId"`
    Payload   json.RawMessage `json:"payload"`
    Error     *EnvelopeError  `json:"error,omitempty"`
}
```

Same struct copy-pasted to `worker/internal/rmq/envelope.go`. Keep both in sync — the worker is a separate Go module and there is intentionally no shared module today.

### Frontend runtime config

`frontend/src/config.js` reads `window.ENV` (injected at container start) and falls back to `import.meta.env.VITE_*` for local dev. Adding a client-side env var requires: add to `frontend/.env.example`, prefix with `VITE_`, and update the entrypoint script that materializes `window.ENV`.

### Database & migrations

- Postgres 14+ (compose runs `postgres:15-alpine`).
- Migrations: SQL files in `backend/migrations/`, embedded with `embed.go`, executed by Goose. See `internal/dbmigrate`.
- Models: GORM structs co-located in each feature package (e.g., `internal/user/user.go`).

## Code organization for AI-maintained Go (project override)

> This section **OVERRIDES** the global rule "Seguir Clean Architecture, SOLID e Hexagonal". For this codebase, optimize for AI legibility, not architectural purity.

The Go services are designed to be primarily maintained by AI (implementation, bug fixes, code review). The strategy is to keep each domain's context **co-located in one place** so an AI agent can open a single package and have everything it needs.

**Rules:**

1. **Vertical-slice packaging by feature** — `internal/auth/`, `internal/ticket/`, `internal/whatsapp/`, etc. Do NOT split by technical layer (no `internal/domain/`, `internal/application/`, `internal/infrastructure/`).
2. **2–4 files per feature package**, typical layout:
   - `<feature>.go` — GORM model + service functions (the business logic).
   - `handler.go` — chi HTTP handlers (or RMQ consumers, for the worker).
   - `<feature>_test.go` — tests.
   - Extra files only when one of the above grows past ~600 lines AND the split is along a clean seam (e.g., `auth.go` + `jwt.go`).
3. **No speculative interfaces.** Add an interface only when there is a second concrete implementation today, or when a test needs to fake a dependency at a clean seam. Do not abstract `gorm.DB`, `*amqp.Connection`, `http.Handler`, etc.
4. **Use the stdlib and chosen libs directly.** No wrapper packages "in case we swap". When the swap actually happens, the change is mechanical and the AI handles it.
5. **No comments.** Hard rule 1 — zero inline comments. Put non-obvious facts in the `memory` MCP graph instead.
6. **Convention over configuration.** Every feature package follows the same shape. Once the AI has implemented one, the rest are pattern-replay.
7. **Errors with context.** `internal/platform/errors` defines `AppError{Code, Status, Message}`. Wrap with `fmt.Errorf("...: %w", err)` to preserve the chain.
8. **Cross-cutting concerns** (logger, config, AppError, ID generators) live in `internal/platform/*` and are imported as plain packages — no DI container, no provider graph.

This guidance applies to both `backend/` and `worker/`.

## MCP usage rules

These MCPs are wired up and **must be used** in the situations below — not optionally. They are how analysis stays consistent and assertive across sessions.

### `sequentialthinking` — required for non-trivial reasoning

Call `mcp__sequentialthinking__sequentialthinking` **before acting** whenever the task involves:

- Planning a multi-step implementation or refactor.
- Designing architecture, picking between library/framework options, or weighing trade-offs.
- Debugging where the cause is not immediately visible from a single file.
- Decomposing a large user request into ordered work.
- Consolidating context before producing an extensive final answer.

Skip it only for trivial single-step operations (one-line answer, mechanical edit, single command). Output is internal — surface conclusions to the user, not the raw thought log.

### `memory` — knowledge graph across sessions

Use `mcp__memory__*` to persist facts that future sessions would otherwise have to rediscover. Schema convention for this project:

- **entityType** values: `Project`, `Service`, `Model`, `Provider`, `BusinessRule`, `MigrationDecision`, `Infra`.
- **Relations** in active voice: `depends-on`, `reads`, `writes`, `emits-event`, `enforces`, `belongs-to`, `has-many`, `replaces`.
- **Observations**: load-bearing facts (TTLs, timeouts, regexes, ordering constraints, why a decision was taken).

Read the graph (`mcp__memory__read_graph` / `mcp__memory__search_nodes`) at the start of a deep-analysis or refactor task. Update it as soon as a new invariant or decision is established.

### `context7` — library documentation

Per the global rule, any time the task touches a library, framework, SDK, or CLI tool (Go or otherwise), use `mcp__context7__resolve-library-id` followed by `mcp__context7__query-docs` instead of relying on training-data recall. Especially relevant for the Go libs in use: chi, GORM, pgx, goose, golang-jwt, coder/websocket, amqp091, minio-go, validator/v10, whatsmeow, modernc.org/sqlite, nativewebp.

### `whatsmeow-context` — Go WhatsApp library (worker)

`whatsmeow` is the only WhatsApp client in use today (worker-side). Use `mcp__whatsmeow-context__*` whenever:

- Implementing or designing any worker code that talks to WhatsApp (sessions, sending, events, media, groups, app state).
- Diagnosing pairing, ack, decryption, or device-linking issues.

Useful entry points: `whatsmeow_topicos`, `whatsmeow_resumo_modulo`, `whatsmeow_events`, `whatsmeow_messages`, `whatsmeow_media`, `whatsmeow_client_methods`, `whatsmeow_store`, `whatsmeow_examples`.

## Skills, plugins, and slash commands

The project enables `andrej-karpathy-skills@karpathy-skills` (anti-overengineering guidelines). The skills most aligned with this codebase, in priority order:

- `superpowers:verification-before-completion` — run `go test ./...`, `go vet ./...`, `gofmt -l` before claiming done.
- `andrej-karpathy-skills:karpathy-guidelines` — surgical changes, no speculative abstractions; matches hard rule 1 and the AI-maintained Go rules above.
- `superpowers:systematic-debugging` — for whatsmeow event/ack, RMQ, WS, GORM bugs.
- `superpowers:test-driven-development` — meaningful integration coverage exists in `rmq`, `media`, `whatsmeow`; new code should match.
- `superpowers:writing-plans` + `superpowers:executing-plans` — use for any multi-step refactor.
- `superpowers:requesting-code-review` — before merging into `master`.
- `simplify` — review delta for reuse before adding new helpers.
- `spec-writer` — feature scoping for new bounded contexts.

The `Stop` hook in `.claude/settings.json` runs `gofmt -l` against `backend/` and `worker/` at the end of each turn so unformatted Go is surfaced automatically.

## Conventions enforced in this repo

- **Go:** stdlib `log/slog` for logging; `errors.Is/As` for chain inspection; wrap errors with `fmt.Errorf("...: %w", err)`. No `panic` outside of `main.go` startup.
- **gofmt** is mandatory. `go vet ./...` should be clean. If you bring in a linter, prefer `golangci-lint` and add the config — don't ad-hoc.
- **Tests** use real backing services via `testcontainers-go` (Postgres, RabbitMQ, MinIO). They are slower but actually catch protocol bugs. Don't replace them with mocks.
- **Frontend ESLint** config is minimal (`react`, `react-hooks`); there is no `lint` script — run `npx eslint src/` if needed.
- **No new top-level directories** without updating this file. The AI agent's mental model of the repo lives here.
