# GCFeed Project Context

## Purpose

GCFeed is a short-video Feed system. It provides a practical engineering baseline for account, video publishing, Feed delivery, recommendation, interaction, relation, governance, monitoring, and Web client workflows.

## Technology

- Go API with Gin and GORM.
- MySQL for durable data.
- Redis for Feed cache, hot ranking, counters, and short-lived state.
- RabbitMQ for asynchronous write-behind and fanout work.
- JWT for API authentication.
- React and Vite for the Web client.
- OpenSpec for change proposals and long-lived capability specifications.

## Architecture

Backend code lives in `apps/api` and follows four layers:

- `domain/{module}`: entities, business invariants, domain errors, repository interfaces.
- `application/{module}`: use cases, cursors, idempotency, cross-entity workflows.
- `infra`: MySQL, Redis, RabbitMQ, JWT, configuration, persistence models.
- `interfaces/http`: HTTP handlers, DTOs, middleware, and route registration.

The standard dependency assembly order is:

```text
Config -> DB/Redis/RabbitMQ/JWT -> Repository -> Service -> Handler -> Router
```

## Documentation Map

- `README.md`: project entry, startup commands, documentation map.
- `docs/product.md`: product scope, module map, P0/P1 status.
- `docs/quickread.md`: newcomer code-reading guide.
- `docs/architecture.md`: system diagrams and core flows.
- `docs/engineering.md`: engineering standards.
- `docs/optimization.md`: Feed performance and stability strategy.
- `docs/uiux.md`: Web UI/UX specification.
- `docs/modules/*.md`: module-level designs.

## Conventions

- New backend modules follow Domain -> Application -> Infrastructure -> Interfaces.
- New APIs use REST resource paths and cursor pagination for lists.
- Write APIs use `Idempotency-Key` when repeat submission is realistic.
- API tests live under `apps/api/test`.
- Web changes should preserve the current React/Vite structure unless page growth requires a split.
- Documentation updates are part of feature delivery.

## Verification

Use these checks when relevant:

```bash
openspec validate --all --strict
cd apps/api && go test ./...
cd apps/web && npm run build
```
