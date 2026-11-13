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
│   ├── validation/gst.go                GST state codes · code → state · PAN inside GSTIN
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
│   │   ├── company/                     Company (sl = 1), CompanyInput, limits, DBA-message map
│   │   ├── taxdetails/                  TaxDetails (sl = 1, Form 3), input, write-only password
│   │   ├── trust/                       Trust (Form 4), inputs, code rule, DBA-message → field map
│   │   ├── codelist/                    Place · Branch · Region (Form 5): one Kind per list, Entry, DBA-message map
│   │   └── institution/                 Institution (Form 5), code rules, full code, filter/page, DBA-message map
│   ├── usecase/
│   │   ├── index.go                     builds all use cases
│   │   ├── common/validate.go           DTO validation → domain.Validation
│   │   ├── auth/{auth,token}.go         login · refresh (rotating) · logout · authenticate · JWT issuer
│   │   ├── tenant/tenant.go             create tenant · list · create tenant admin
│   │   ├── setup/setup.go               next wizard step from year + company state
│   │   ├── finyear/finyear.go           options · rules · validate · create · list · current
│   │   ├── company/company.go           needs a year · normalise · check · validate · save · get
│   │   ├── taxdetails/taxdetails.go     needs the company · normalise · check (all at once) · save · get
│   │   ├── trust/trust.go               normalise · check (FY exists, date inside it) · duplicates · validate · create · update
│   │   ├── codelist/codelist.go         one use case per list: normalise · duplicates · validate · create · list · get
│   │   └── institution/institution.go   normalise · check (all at once) · lookups exist · next codes · validate · create · list
│   └── repository/
│       ├── index.go                     builds all repositories
│       ├── pgutil/                      Querier, tenant pool lookup, PG error helpers
│       ├── auth/postgres.go             sys_user · sys_refresh_token
│       ├── tenant/postgres.go           sys_tenant
│       ├── finyear/postgres.go          finyr_master + DBA function/procedure, locked txn
│       ├── company/postgres.go          company_master (sl = 1) + DBA function/procedure, locked txn
│       ├── taxdetails/postgres.go       mst_company_tax_details (sl = 1) + DBA procedure, locked txn
│       ├── trust/postgres.go            mst_trust + DBA function/procedure, code/description checks in a locked txn
│       ├── codelist/postgres.go         mst_place / mst_branch / mst_region + DBA function/procedure, locked txn
│       └── institution/postgres.go      mst_inst + DBA function/procedure, paged search, next numbers, locked txn
├── external/doc.go                      placeholder for third-party clients
└── delivery/rest/
    ├── index.go                         router, middleware chain, http.Server
    ├── routers/routers.go               every route and its guards
    ├── middlewares/                     JWT auth · role/tenant guards · request id · request log
    ├── controllers/{auth,admin,setup,finyear,company,taxdetails,trust,codelist,institution,meta}/
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
- `sys_tenant.db_key` is **unique**, and startup refuses two `TENANT_DATABASES` keys that point at
  the same host/port/database, so two tenants can never share a database.
- A tenant database that is down does not stop the API: it is logged at startup and that
  tenant's requests get `503` until it is back; the control database is required.
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
  … RETURNING`), logout revokes it, expired rows are cleaned at startup and hourly.
- **Reuse detection**: replaying a refresh token that was used more than 30 s ago
  (the grace lets two tabs refresh together) revokes every refresh token of that user.
- Tokens are checked with 30 s of clock leeway so API instances with slightly different clocks agree.
- Login answers the same for "wrong password" and "no such email", and spends
  the same argon2id time on both (OWASP parameters: 19 MiB, 2 passes). Login is rate-limited
  per client IP (`LOGIN_RATE_LIMIT_PER_MINUTE`, JSON `429`).
- The client IP is the TCP peer, unless it is a proxy listed in `TRUSTED_PROXIES`; only then is
  `X-Forwarded-For` read, right to left, skipping trusted hops. Headers like `True-Client-IP` or
  `X-Real-IP` are never trusted, so a client cannot pick its own rate-limit bucket or logged IP.

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
| `pms.mst_finyr_insert(...)` | DBA | Insert; `is_current = true` un-marks the others; RAISEs on duplicates | Called for create |
| `pms.mst_finyr_set_current(take_finyr)` | DBA (written by us at Uncle's request, installed by him on live 9 Nov) | Marks one existing year current and un-marks the rest; RAISEs if missing | Called by `PUT /financial-years/current` |
| `pms.company_master` | DBA (`gensuperuser`) | Company details, one row `sl = 1` | Read with `SELECT`; written only through the procedure |
| `pms.company_master_validate(...)` | DBA | Returns `(is_valid, message)` | Called on validate, and again inside the save transaction |
| `pms.company_master_insert(...)` | DBA | Upsert by `sl` | Called for every save with `sl = 1`. It does **not** validate, so we always validate first |
| `pms.mst_company_tax_details` | DBA (v1.01) | Company tax and contact details (Form 3), one row `sl = 1`, FK to `company_master` | Read with `SELECT` (never the password column's value, only whether it is set); written only through the procedure |
| `pms.mst_company_tax_details_insert(...)` | DBA (v1.01) | Upsert by `sl`, 12 arguments, replaces every column | Called for every save with `sl = 1`. No validate function exists: every rule is ours |
| `pms.mst_trust` | DBA (v1.01) | Trusts (Form 4); PK `trust_code` (`^[A-Z]{3}$`), `trust_desc` UNIQUE (case-sensitive) | Read with `SELECT`; written only through the procedure |
| `pms.mst_trust_validate(...)` | DBA (v1.01) | Returns one row `(is_valid, take_error_msg)`; problems joined by `. ` | Called on validate, and again inside the create/update transaction. Fixed on live by Uncle on 9 Nov (`finyr_master` + `::VARCHAR`) |
| `pms.mst_trust_insert(...)` | DBA (v1.01) | Upsert by `trust_code`, sets `last_update_at` | Called for create and update. Installed on live 9 Nov 2026 |
| `pms.sw_config`, `pms.config_sw_insert(...)` | DBA | App settings | Not used yet |
| `pms.sys_tenant` / `sys_user` / `sys_refresh_token` | Us (migration 00001) | Platform | Tenants, logins, refresh tokens |
| `pms.sys_schema_migrations` | Us (goose) | Migration bookkeeping | — |

`db/legacy/001_pms_objects.sql` is a verbatim copy of the DBA's SQL (the
PROCUREMENT.ESIS bundle) so local Postgres and the end-to-end test run against
the exact same functions and procedures. The bundle has no `CREATE TABLE` for
`finyr_master` / `company_master`; those two are rebuilt from the live catalog.

The v1.01 tables (`mst_company_tax_details`, `mst_trust`) and
`mst_company_tax_details_insert` are copied from `tables_VER_01_1.txt` and the
procedure file in the v1.01 bundle.

`mst_trust_insert` comes from the v1.01 bundle. `mst_trust_validate` is the text
Uncle installed on live on 9 Nov: the bundle version with `pms.mst_fin_year` →
`pms.finyr_master` and a `::VARCHAR` cast on its message (without the cast
Postgres 11 raises 42804 *structure of query does not match function result type*
on every call).

`mst_finyr_set_current` at the end of the file is the text Uncle installed on live
on 9 Nov (we wrote it at his request).

`002_form5_objects.sql` is Form 5 (places, branches, regions, institutions) verbatim from
`TABLE_PROCEDURE_FUNCTION_1_1.xlsx`, installed on live on 9 Nov; `mst_branch_validate` is
on live but not in the sheet, so its text comes from the live catalog.
`003_local_fixes.sql` holds our **marked local fixes** to it, so institutions can be
created locally and in the e2e test: `sp_mst_inst_insert` reads `trust_desc` (not the
missing `trust_name`), and `inst_code` is 7 characters (not 9) in the table check, the
procedure and the validate function. Live still has both bugs (open items); drop each
fix once Uncle installs it.

`004_delete_objects.sql` holds the delete procedures and their `*_validate_delete` checks
from `TABLE_PROCEDURE_FUNCTION_1_2.xlsx` (11 Nov). Six are verbatim; four are **marked
local fixes** we sent the DBA (`delete_fixes.sql`): `finyr_master_validate_delete` read a
`company_master.finyr` column that does not exist, `mst_place_validate_delete` had a typo
(`IS NU+I6LL`) and did not install, and both region objects used `region_code` instead of
`regn_code`.

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
| The previous year, the current one, or up to `FY_FUTURE_YEARS` (default 0, 0–5) after it, by today in `DEFAULT_TIMEZONE` | Uncle, 9 Nov: a year only once today falls in it (the v1.01 walkthrough had allowed 2027-2028 in 2026-2027) |
| Dates are 1 April → 31 March; sent dates must match | walkthrough ("1st April to 31st March of next year") |
| Which year is current: while none is, the new one; else an explicit `finyr_is_current`; else today's year | decided inside the locked transaction, so creating the previous year first cannot leave it current (no procedure changes it later). A future year is current only when asked |
| Not a duplicate, no overlap | DBA function (re-run in a locked transaction) |
| Change the current year later | `PUT /financial-years/current`: under the same advisory lock as create, check the year exists, then `CALL pms.mst_finyr_set_current`; already current is a no-op |

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
   | `fin_st_dt` | `YYYY-MM-DD`, must be **1 April**, and if sent the `finyr_st_date` of an existing financial year; defaults to the current financial year's start |
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

## 8. Company tax and contact details (Form 3)

One record, `sl = 1`, saved through `pms.mst_company_tax_details_insert`.
The company must exist first (`409 Save the company details first`). No field
is mandatory; anything entered must be valid, and every problem is returned in
one 422.

| Field | Rule |
|---|---|
| `pan` | trim, upper-case; `IsPAN` |
| `pan_enabled`, `pan_enabled_dt` | The number stays optional when enabled (Uncle: "optional, but if they key PAN or GSTIN we should validate it"). The date is only allowed when enabled, cannot be after today, and defaults to today |
| `gstin` | trim, upper-case; `IsGSTIN` (format + check digit); first 2 digits a code from `GSTStates`; if `pan` is filled, characters 3–12 must equal it |
| `gstin_enabled`, `gstin_enabled_dt` | as for PAN, against `gstin` |
| `gstin_state` | ≤ 100, official spelling from `GSTStates`. With a GSTIN: empty → filled from its code; filled → must match its code |
| `comm_mail_id` | trim, lower-case, ≤ 150, `IsEmail` |
| `comm_mail_pwd` | write-only, ≤ 150, not trimmed. Missing or `null` keeps the stored one; `""` clears it; text replaces it. Never returned or logged; responses carry `comm_mail_pwd_set` |
| `comm_mobile`, `comm_whatsapp` | `NormalizeIndianMobile`, then `IsIndianMobile` |

**Save** — one transaction holding `pg_advisory_xact_lock(hashtext('pms.mst_company_tax_details'))`:
check `company_master` has `sl = 1`, read the stored password (`FOR UPDATE`) so a
missing one is kept (the procedure replaces every column), then
`CALL pms.mst_company_tax_details_insert(1, …)`. `201` when the row was created, `200` after.

## 9. Trusts (Form 4)

`trust_code` is the key: trim, upper-case, exactly 3 letters; it never changes.
A trust can be deleted while no institution uses it (see Deletes). Rules, all field problems returned together:

| Field | Rule |
|---|---|
| `trust_code` | required, `^[A-Z]{3}$`; not already used (409) |
| `trust_desc` | required, ≤ 100; unique ignoring case among other trusts (409). Uncle's validate compares case-insensitively while the table's UNIQUE is case-sensitive: we follow the stricter rule |
| `wrk_st_finyr` | required, must exist in `finyr_master` |
| `wrk_st_date` | required, `YYYY-MM-DD`, inside `wrk_st_finyr` (1 April → 31 March; pending Uncle) |

**Order:** our checks → duplicate code / description (409) → `mst_trust_validate`
(each sentence mapped to a field; a duplicate description is a 409) → one
transaction holding `pg_advisory_xact_lock(hashtext('pms.mst_trust'))`: for create
the code must be free, for update it must exist; the description must be free;
re-run the validate; `CALL pms.mst_trust_insert`. `mst_trust_insert` is an upsert,
so without the lock and the "code is free" check a create could silently
overwrite an existing trust.

## 10. Places, branches, regions and institutions (Form 5)

**Code lists.** `codelist.Kind` names a list's table, columns and DBA routines (all
constants), and one repository, use case and controller serve all three. Code: trim,
upper-case, `^[A-Z]{3}$`; name: spaces tidied, ≤ 250, unique ignoring case. Order: our
checks → duplicates (409) → the DBA's validate function → one transaction holding
`pg_advisory_xact_lock(hashtext('pms.mst_place'))` (per table): re-check, re-validate,
`CALL` his insert. His inserts RAISE on duplicates (P0001), mapped the same way. Deletes
go through his delete procedures (§11). No edit: he has given no update procedure.

**Institutions.**

| Rule | Why |
|---|---|
| `inst_code` = trust code + `-` + 3 digits, prefix must be `trust_code` | All 571 rows of `EXAMPLE INST.xlsx` (`ACU-009`) |
| `inst_reg_code` = region code + `-` + 2 digits, prefix must be an existing region | The sheet's reg prefixes (BGN, CBP, MYS) are 23 area codes, not places; `mst_inst` has no region column (open item) |
| `inst_full_code` = `inst_code:inst_reg_code`, built by the server | Always 14 characters, "ACM Code (14 Char.)" on the old screen |
| Trust, place, branch, region, `from_fin_yr` must exist; all problems in one 422 | Same style as every other form |
| Mobiles 10 digits after dropping `+91`/0; emails lower-case; `reg_date` defaults to today, never future | Same as the company form |
| `*_sl_idx` sent as NULL | Uncle's sheet: "auto calculation to be made later" |
| Next codes = highest number with that prefix + 1 | The old system numbers per trust and per region |

Write order: our checks and lookups → ACM/reg/full code free (409) →
`fn_mst_inst_validate` → one transaction holding `pg_advisory_xact_lock(hashtext('pms.mst_inst'))`:
re-check the codes, re-validate, `CALL pms.sp_mst_inst_insert`.

### Excel import

`packages/sheet` reads the first sheet of an `.xlsx` (excelize) as text, keeping number cells
as plain digits and date cells as dates, and builds the template. The import use case maps
headers by name (aliases include the old system's `ACM_*`), loads trusts, places, branches,
regions, years and the codes in use **once**, and checks every row with the same
`prepareWith` the single create uses (lookups come from memory instead of one query each).
The DBA's validate function runs for all rows in one `unnest … CROSS JOIN LATERAL` query, and
the save sends all `CALL`s in one pgx batch inside one locked transaction. This matters
because the live database is far from Render: 571 rows one by one would take over a minute.

## 11. Deletes

Every delete goes through the DBA: `pgutil.Delete` takes the table's own write lock, runs
his `*_validate_delete(code)`, then `CALL`s his `*_delete(code)`, in one transaction.
His "does not exist" is a `404`; any other refusal (*Cannot delete! … already used in …*)
is a `409` carrying his message, and a foreign key that still points at the row is a `409`
too. Financial years add one rule of ours inside the lock: the current year cannot be
deleted (it would leave no year current). Routes: `DELETE /financial-years/{finyr}`,
`/trusts/{code}`, `/places/{code}`, `/branches/{code}`, `/regions/{code}`. CORS allows
`DELETE`. Institutions have no delete procedure yet.

## 12. Errors

Use cases return `domain.AppError`; `delivery/rest/utils.WriteError` is the
only place that maps them to HTTP.

| Kind | HTTP | When |
|---|---|---|
| `invalid_params` | 400 | Malformed JSON, wrong content type, bad query |
| `unauthorized` | 401 | No/invalid/expired token, wrong credentials |
| `forbidden` | 403 | Wrong role, platform admin on tenant routes, disabled account at login (mid-session it is 401) |
| `not_found` | 404 | Unknown record or route |
| `method_not_allowed` | 405 | Route exists but not for this HTTP method |
| `rate_limited` | 429 | Too many login attempts from one IP |
| `conflict` | 409 | Duplicate financial year / overlap / tenant code / email / db_key; company before financial year; tax details before company; trust code or description taken |
| `validation_failed` | 422 | Field rules failed (with `fields[]`) |
| `internal_error` | 500 | Anything unexpected; logged with cause, caller sees a generic message |
| `service_unavailable` | 503 | A database could not be reached, or Postgres is shutting down or starting up |
| `timeout` | 504 | The request ran past `REQUEST_TIMEOUT` (e.g. a stuck database) |

## 13. Open items for the DBA

1. Agree to the `pms.sys_*` tables (or provide a separate `platform` schema).
2. `company_master.time_zone` is `TIMESTAMPTZ`, so the zone name is lost;
   `VARCHAR` holding `Asia/Kolkata` would keep it.
3. A `rec_updated_at` column on `company_master`.
4. Done 9 Nov: `pms.mst_finyr_set_current(take_finyr)` is installed on live. A year can be deleted (11 Nov) but not edited.
5. `finyr_master_validate` does not check the `YYYY-YYYY` format, the 1 April–31 March
   dates or the 9-character limit — we enforce them in the API.
6. Confirm the 1 April rule for `fin_st_dt` and which company fields are mandatory.
7. Done 9 Nov: Uncle installed the fixed `mst_trust_validate` (`finyr_master` + `::VARCHAR`) and `mst_trust_insert` on live. Worth one read-only check: `SELECT pg_get_functiondef('pms.mst_trust_validate'::regproc);`
8. Done 9 Nov: no future financial years (Uncle: "only if current date falls"). `FY_FUTURE_YEARS` defaults to 0 and can be raised without a code change.
9. `comm_mail_pwd` is the password for sending and receiving mail (Uncle, 9 Nov). Built write-only, never returned or logged; it is stored as plain text, so encrypt it in the app before mail sending is built.
10. PAN/GSTIN numbers are optional even when enabled, validated when keyed (Uncle, 9 Nov). Still open: is the enabled date the registration date, and may it be in the future? (built: not after today).
11. Must a trust's `wrk_st_date` fall inside its `wrk_st_finyr`? (built: inside).
12. Trust delete (11 Nov): `mst_trust_delete` and `mst_trust_validate_delete` are in v1.2 and work; Uncle granted `esispmsuser` the rights. His check covers institutions only; once transaction tables exist it must also refuse a trust with transactions, and edits must be blocked then too.
13. Form 5, `sp_mst_inst_insert` reads `trust_name` from `mst_trust`; the column is `trust_desc`, so every institution insert fails on live (`column "trust_name" does not exist`). Fixed locally in `003_local_fixes.sql`.
14. Form 5, `inst_code` is 9 characters in the table check, procedure and validate function, but the client's codes are 7 (`ACU-009`) and 7 + `:` + 6 = 14 is the full code. With 9, no full code can be 14, so nothing can be saved. We built 7; fixed locally. On 12 Nov 2026 the DBA changed `fn_mst_inst_validate` on live to 7 (his text matches ours exactly). Still to confirm on live: the table check `chk_inst_code_length` and `sp_mst_inst_insert` at 7 (and item 13). Until then a live insert still fails.
15. Form 5, `inst_reg_code` is UNIQUE, but the client's sheet reuses reg codes across trusts (`CBP-01` under ACU, OTU, SAM and SAT). Built: unique everywhere, as the table says.
16. Form 5, the reg code prefix looks like the region code, but `mst_inst` has no `regn_code`. Built: the prefix must be an existing region.
17. Form 5, delete procedures for place, branch and region arrived 11 Nov (built); still no update procedure for any of them, and no delete for institutions, so nothing can be edited (the old screen has Edit/Save).
18. Form 5, codes letters only? Uncle's checks only count 3 characters (`"1 a"` passes). Built: `^[A-Z]{3}$`, like trusts.
19. Form 5, the old screen has an accountant designation but `mst_inst` has no `inst_acnt_desig`; the table has a liaison officer the screen does not. Built: the table's fields.
20. Form 5, what is `reg_date`? Built: defaults to today, never in the future. Which fields are mandatory? Built: trust, codes, name, place, branch, start year.
21. Form 5, `mst_branch` and `mst_inst` have no `last_update_at` trigger (place and region do).
22. Deletes (v1.2, 11 Nov): four objects need the fixed text in `delete_fixes.sql` before they work on live: `finyr_master_validate_delete` (reads a missing `company_master.finyr`), `mst_place_validate_delete` (typo, does not install), `mst_region_delete` and `mst_region_validate_delete` (`region_code` → `regn_code`). Until then, deleting a financial year, place or region on live fails.
