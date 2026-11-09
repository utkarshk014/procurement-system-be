# Procurement System — Backend (`pms-api`)

Multi-tenant procurement management API in Go. Phase 1 covers **platform
admin**, **tenant onboarding**, **JWT authentication** and the **setup wizard**
— step 1 financial year, step 2 the single company record (`sl = 1`) — on top
of the DBA's PostgreSQL schema `pms`. Phase 2 adds making another year current,
the company's tax and contact details (Form 3), trusts (Form 4), and Form 5:
places, branches, regions and institutions.

| | |
|---|---|
| Language | Go 1.26 |
| HTTP | `go-chi/chi` v5 (+ `cors`, `httprate`) |
| Database | PostgreSQL 11 via `jackc/pgx` v5, migrations with `pressly/goose` v3 |
| Auth | JWT (`golang-jwt/jwt` v5) access + refresh tokens, `argon2id` passwords |
| Validation | `go-playground/validator` v10 + Indian rules (mobile, PIN, PAN, GSTIN) |
| Tests | `testify`, `testcontainers-go` (real Postgres 11 in Docker) |

## Documentation

| Doc | What's in it |
|---|---|
| [Architecture](docs/ARCHITECTURE.md) | Layers, folder map, multi-tenancy, database objects, design decisions |
| [End-to-end flows](docs/FLOWS.md) | Sequence diagrams: bootstrap, onboarding, login/refresh, setup wizard (financial year → company) |
| [API reference](docs/API.md) | Every endpoint, request/response bodies, errors, curl examples |

## Quick start (local)

Needs Go 1.26+ and Docker.

```bash
cp .env.example .env
make secrets          # paste the two JWT secrets into .env
make db-up            # Postgres 11 on :5433, pre-loaded with the DBA's pms objects
make migrate          # creates pms.sys_tenant / sys_user / sys_refresh_token
make admin EMAIL=you@example.com NAME="Your Name"   # prompts for a password
make run              # API on http://localhost:8080
```

Then:

```bash
curl -s localhost:8080/healthz
curl -s -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"…"}' \
  localhost:8080/api/v1/auth/login
```

## Tests

```bash
make test-unit   # fast, no Docker
make test        # unit + end-to-end (spins up Postgres 11 with the DBA's SQL)
```

## Project layout

```
src/
├── main.go                     # wiring + graceful shutdown
├── cmd/admin/                  # CLI: migrate, migrate-status, create-platform-admin
├── delivery/rest/              # HTTP: router, middlewares, controllers, routers
├── internal/
│   ├── domain/                 # models, interfaces, errors — no dependencies
│   ├── usecase/                # business rules
│   └── repository/             # Postgres access (incl. the DBA's procedures)
├── external/                   # third-party service clients (none yet)
├── packages/                   # reusable libs: database, validation, security, logger
└── utils/                      # config
db/legacy/                      # the DBA's pms objects, for local dev and tests only
```

## Working against the real database

The real `esispms` database already has the DBA's `finyr_master` and
`company_master` tables with their functions and procedures. Before the API can run against it, `make migrate` must be run once
with `CONTROL_DATABASE_URL` pointing at it, which **creates three new tables**
(`pms.sys_tenant`, `pms.sys_user`, `pms.sys_refresh_token`) plus goose's
`pms.sys_schema_migrations`. Agree this with the DBA first.
