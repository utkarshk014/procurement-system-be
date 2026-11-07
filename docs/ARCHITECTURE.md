# Architecture

## 1. Layers

The service follows Clean Architecture, the same shape as our PFM service:
dependencies only point inwards, towards `domain`.

```mermaid
flowchart TB
    subgraph Delivery["delivery/rest — HTTP"]
        R[routers] --> M[middlewares<br/>RequestID · Logger · Recover · Timeout · CORS · JWT · Role]
        M --> C[controllers]
    end
    subgraph App["internal/usecase — business rules"]
        UA[auth] & UT[tenant] & US[setup] & UF[finyear] & UC[company]
    end
    subgraph Domain["internal/domain — models · interfaces · errors"]
        D[(no imports from the service)]
    end
    subgraph Infra["Adapters"]
        RP[internal/repository<br/>Postgres + DBA procedures]
        EX[external<br/>3rd-party APIs — later]
    end
    C --> App
    App --> Domain
    RP -. implements .-> Domain
    EX -. implements .-> Domain
    RP --> PG[(PostgreSQL 11<br/>schema pms)]
```

| Layer | Folder | Owns | Must not |
|---|---|---|---|
| Domain | `src/internal/domain/<module>` | Models, DTOs with `validate` tags, `Usecase` / `Repository` interfaces, `AppError`, `Principal` | Import anything else in the service |
| Use case | `src/internal/usecase/<module>` | Normalisation, rules, orchestration, which errors to return | Know about HTTP or SQL |
| Repository | `src/internal/repository/<module>` | SQL, transactions, calling the DBA's functions/procedures, mapping DB errors | Contain business rules |
| External | `src/external/<service>` | Clients for third-party APIs (email, SMS, WhatsApp…) | — (empty for now) |
| Delivery | `src/delivery/rest` | Routes, auth middleware, JSON in/out, status codes | Contain business rules |
| Packages | `src/packages/*` | Reusable, service-agnostic code: DB pools & migrations, validators, password hashing, logger | Import `internal/*` |

Every module repeats the same five pieces, so a new master (trust, vendor,
item…) is a copy of the company module:

```
domain/<m>/models.go        domain/<m>/interfaces.go
usecase/<m>/<m>.go          repository/<m>/postgres.go
delivery/rest/controllers/<m>/<m>.go   + routes in routers/routers.go
```

Registries wire them once: `repository/index.go` → `usecase/index.go` →
`delivery/rest/index.go`, all called from `main.go`.

## 2. Folder map

```
src/
├── main.go                              config → logger → DB pools → repos → usecases → HTTP
├── cmd/admin/main.go                    migrate · migrate-status · create-platform-admin
├── utils/config.go                      env config (caarlos0/env + .env)
├── packages/
│   ├── database/manager.go              control pool + one pool per tenant db_key
│   ├── database/migrate.go              goose, embedded migrations, table pms.sys_schema_migrations
│   ├── database/migrations/*.sql        our platform tables (pms.sys_*)
│   ├── validation/india.go              mobile · PIN · PAN · GSTIN(checksum) · email · website · states
│   ├── validation/validator.go          go-playground validator → field violations (all at once)
│   ├── security/security.go             argon2id hashing, timing-safe "no such user"
│   └── logger/logger.go                 slog (text locally, JSON in production)
├── internal/
│   ├── domain/
│   │   ├── errors.go                    AppError kinds → HTTP status in one place
│   │   ├── principal.go                 Role, Principal (the caller), context helpers
│   │   ├── auth/                        User, Claims (JWT), TokenPair, interfaces
│   │   ├── tenant/                      Tenant, inputs, interfaces
│   │   ├── setup/                       wizard Status and steps
│   │   ├── finyear/                     FinancialYear, input, options, year maths, DBA-message map
│   │   └── company/                     Company (sl = 1), CompanyInput, limits, DBA-message map
│   ├── usecase/
│   │   ├── index.go                     builds all use cases
│   │   ├── common/validate.go           DTO validation → domain.Validation
│   │   ├── auth/{auth,token}.go         login · refresh (rotating) · logout · authenticate · JWT issuer
│   │   ├── tenant/tenant.go             create tenant · list · create tenant admin
│   │   ├── setup/setup.go               next wizard step from year + company state
│   │   ├── finyear/finyear.go           options · rules · validate · create · list · current
│   │   └── company/company.go           needs a year · normalise · check · validate · save · get
│   └── repository/
│       ├── index.go                     builds all repositories
│       ├── pgutil/                      Querier, tenant pool lookup, PG error helpers
│       ├── auth/postgres.go             sys_user · sys_refresh_token
│       ├── tenant/postgres.go           sys_tenant
│       ├── finyear/postgres.go          finyr_master + DBA function/procedure, locked txn
│       └── company/postgres.go          company_master (sl = 1) + DBA function/procedure, locked txn
├── external/doc.go                      placeholder for third-party clients
└── delivery/rest/
    ├── index.go                         router, middleware chain, http.Server
    ├── routers/routers.go               every route and its guards
    ├── middlewares/                     JWT auth · role/tenant guards · request id · request log
    ├── controllers/{auth,admin,setup,finyear,company,meta}/
    └── utils/                           JSON decode (strict), Respond, WriteError
```

## 3. Multi-tenancy

A **tenant** is one client organisation (e.g. ESIS). Inside a tenant the
business hierarchy is company → trusts → institutions.

**Model: one database per tenant**, with a central registry.

```mermaid
flowchart LR
    API[pms-api] -->|users, tenants, tokens| CTRL[(Control DB<br/>pms.sys_*)]
    API -->|tenant ESIS| T1[(esispms<br/>pms.company_master …)]
    API -.->|future tenant| T2[(other DB<br/>same pms schema)]
    CTRL -- "sys_tenant.db_key → TENANT_DATABASES" --> API
```

Why this model:

- The DBA's procedures are hard-coded to the `pms` schema and his tables have
  no tenant column. With a database per tenant they work **unchanged**.
- Strongest isolation: one tenant's data can never appear in another's query.
- `sys_tenant.db_key` is **unique**, so two tenants can never share a database.
- Moving a tenant is a config change (`TENANT_DATABASES`), not a code change.

How a request finds its database: the access token's user → `GetPrincipal`
re-loads user + tenant (and checks both are active) → `Principal.DBKey` →
`database.Manager.Tenant(db_key)` → that pool. The tenant **never** comes from
the request body, URL or a header the caller controls.

Today the control tables and ESIS's business tables share `esispms` (we can
create tables in `pms` but not new schemas or databases).

## 4. Authentication and authorisation

| Token | Lifetime | Signed with | Carries | Stored? |
|---|---|---|---|---|
| Access | 15 min (`ACCESS_TOKEN_TTL`) | `JWT_ACCESS_SECRET` (HS256) | `sub`, `role`, `email`, `name`, `tid`, `tcode`, `typ=access` | No |
| Refresh | 30 days (`REFRESH_TOKEN_TTL`) | `JWT_REFRESH_SECRET` (HS256) | `sub`, `role`, `typ=refresh`, `jti` | `jti` in `pms.sys_refresh_token` |

- Different secrets **and** a `typ` claim: an access token can never be used to
  refresh and vice-versa. `alg` is pinned to HS256 (no `alg: none`).
- Every request re-loads the user from the database, so deactivating a user or
  suspending a tenant locks them out **immediately**, not when the token expires.
- Refresh **rotates**: each refresh token works once (`UPDATE … SET revoked_at
  … RETURNING`), logout revokes it, expired rows are cleaned hourly.
- Login answers the same for "wrong password" and "no such email", and spends
  the same argon2id time on both (OWASP parameters: 19 MiB, 2 passes). Login is rate-limited
  per IP (`LOGIN_RATE_LIMIT_PER_MINUTE`, JSON `429`); behind a load balancer set `TRUST_PROXY=true`
  so the limit applies per client, not to the proxy's IP.

| Role | Can |
|---|---|
| `platform_admin` | Create/list tenants, create tenant admins. No tenant, so no company routes. |
| `tenant_admin` | Company setup for **their** tenant. No admin routes. |

There is no public sign-up. The first platform admin is created with the CLI.

## 5. Database objects

| Object | Owner | Purpose | How we use it |
|---|---|---|---|
| `pms.finyr_master` | DBA (`gensuperuser`) | Financial years (PK `finyr`, check end ≥ start) | Read with `SELECT`; written only through the procedure |
| `pms.finyr_master_validate(...)` | DBA | Returns `'SUCCESS'` or a message | Called on validate, and again inside the create transaction |
| `pms.mst_finyr_insert(...)` | DBA | Insert; `is_current = true` un-marks the others; RAISEs on duplicates | Called for create. No update procedure exists |
| `pms.company_master` | DBA (`gensuperuser`) | Company details, one row `sl = 1` | Read with `SELECT`; written only through the procedure |
| `pms.company_master_validate(...)` | DBA | Returns `(is_valid, message)` | Called on validate, and again inside the save transaction |
| `pms.company_master_insert(...)` | DBA | Upsert by `sl` | Called for every save with `sl = 1`. It does **not** validate, so we always validate first |
| `pms.sw_config`, `pms.config_sw_insert(...)` | DBA | App settings | Not used yet |
| `pms.sys_tenant` / `sys_user` / `sys_refresh_token` | Us (migration 00001) | Platform | Tenants, logins, refresh tokens |
| `pms.sys_schema_migrations` | Us (goose) | Migration bookkeeping | — |

`db/legacy/001_pms_objects.sql` is a verbatim copy of the DBA's SQL (the
PROCUREMENT.ESIS bundle) so local Postgres and the end-to-end test run against
the exact same functions and procedures. The bundle has no `CREATE TABLE` for
`finyr_master` / `company_master`; those two are rebuilt from the live catalog.

## 6. Setup wizard

The walkthrough's wizard, in order. `GET /setup/status` returns `next_step`.

1. **Financial year** (`pms.finyr_master`)
2. **Company** (`pms.company_master`, one row `sl = 1`) — refused until a current financial year exists

### Financial year rules

Checked before the DBA's function, which on its own lets through `abc`,
`2026-20270` (then the insert fails on the 9-character column), year labels
that don't match the dates, and 18-month ranges.

| Rule | Source |
|---|---|
| `finyr` is `YYYY-YYYY` with consecutive years | walkthrough ("9 characters, start year – end year") |
| Only the current or the previous financial year (by today in `DEFAULT_TIMEZONE`) | walkthrough (02-11-2026 → 2026-2027 or 2025-2026) |
| Dates are 1 April → 31 March; sent dates must match | walkthrough ("1st April to 31st March of next year") |
| While no year is current, the new one is made current | otherwise the company step could never start (no procedure changes it later) |
| Not a duplicate, no overlap | DBA function (re-run in a locked transaction) |

## 7. Company rules

Applied in this order; **all** field problems are returned together.

1. **Normalise** — trim everything; mobile `+91 98765-43210` → `9876543210`;
   email lower-cased; website trailing `/` removed; country defaults to
   `India`; Indian state matched case-insensitively to its official spelling.
2. **Our checks** (column limits included, so nothing is ever too long for the table):

   | Field | Rule |
   |---|---|
   | `name` | required, ≤ 300 |
   | `shortname` | required, ≤ 50 |
   | `fin_st_dt` | `YYYY-MM-DD`, must be **1 April**; defaults to the current financial year's start |
   | `pin` | optional, 6 digits, not starting with 0 |
   | `mobile` | optional, 10 digits starting 6–9 (after normalising) |
   | `email` | optional, ≤ 200, same pattern as the DBA's check |
   | `website` | optional, ≤ 150, domain with optional http(s) |
   | `state` | ≤ 200; if country is India, one of the 36 states/UTs |
   | `address` / `city` / `country` / `header_1` / `header_2` | ≤ 300 / 200 / 200 / 200 / 200 |

3. **DBA's check** — `pms.company_master_validate`; its message is mapped to
   the field it concerns.
4. **Save** — one transaction holding an advisory lock: look up `sl = 1`
   (exists → update, keep `time_zone`; missing → create), re-run the DBA's
   check, `CALL pms.company_master_insert(1, …)`.

There is exactly one company (DBA: "by default it would have only one record
with sl=1, any alteration would update to same record"). The backend owns `sl`
(never accepted from the client; unknown body fields are rejected) and
`time_zone` (stamped on first save in `DEFAULT_TIMEZONE`, kept afterwards).

## 8. Errors

Use cases return `domain.AppError`; `delivery/rest/utils.WriteError` is the
only place that maps them to HTTP.

| Kind | HTTP | When |
|---|---|---|
| `invalid_params` | 400 | Malformed JSON, wrong content type, bad query |
| `unauthorized` | 401 | No/invalid/expired token, wrong credentials |
| `forbidden` | 403 | Wrong role, disabled account, platform admin on tenant routes |
| `not_found` | 404 | Unknown record or route |
| `rate_limited` | 429 | Too many login attempts from one IP |
| `conflict` | 409 | Duplicate financial year / overlap / tenant code / email / db_key; company before financial year |
| `validation_failed` | 422 | Field rules failed (with `fields[]`) |
| `internal_error` | 500 | Anything unexpected; logged with cause, caller sees a generic message |

## 9. Open items for the DBA

1. Agree to the `pms.sys_*` tables (or provide a separate `platform` schema).
2. `company_master.time_zone` is `TIMESTAMPTZ`, so the zone name is lost;
   `VARCHAR` holding `Asia/Kolkata` would keep it.
3. A `rec_updated_at` column on `company_master`.
4. No procedure to change which financial year is current (or edit one) after creation.
5. `finyr_master_validate` does not check the `YYYY-YYYY` format, the 1 April–31 March
   dates or the 9-character limit — we enforce them in the API.
6. Confirm the 1 April rule for `fin_st_dt` and which company fields are mandatory.
