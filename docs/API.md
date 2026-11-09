# API reference

Base URL: `http://localhost:8080` locally. All bodies are JSON
(`Content-Type: application/json`); unknown fields are rejected.

## Conventions

**Success**

```json
{ "data": { … } }
```

**Error**

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Some fields need attention",
    "fields": [
      { "field": "pin", "message": "must be a 6-digit PIN code" },
      { "field": "fin_st_dt", "message": "must be 1 April (start of the Indian financial year)" }
    ],
    "request_id": "9f2c41d0a7b3e6c1"
  }
}
```

`code` is one of `invalid_params` (400), `unauthorized` (401), `forbidden`
(403), `not_found` (404), `conflict` (409), `validation_failed` (422),
`method_not_allowed` (405), `rate_limited` (429), `internal_error` (500),
`service_unavailable` (503, a database is unreachable), `timeout` (504).

**Auth**: `Authorization: Bearer <access_token>` on every route except
`/healthz`, `/auth/login`, `/auth/refresh`, `/auth/logout`.

## Endpoints

| Method | Path | Who | Purpose |
|---|---|---|---|
| GET | `/healthz` | public | API + database up |
| POST | `/api/v1/auth/login` | public (rate-limited per IP) | Email + password → token pair |
| POST | `/api/v1/auth/refresh` | refresh token | Refresh token → new pair |
| POST | `/api/v1/auth/logout` | refresh token | Revoke a refresh token |
| GET | `/api/v1/auth/me` | logged in | Current user + tenant |
| GET | `/api/v1/meta/indian-states` | logged in | State/UT list for dropdowns |
| GET | `/api/v1/meta/gst-states` | logged in | GST state codes and names, sorted by code |
| POST | `/api/v1/admin/tenants` | platform_admin | Create tenant |
| GET | `/api/v1/admin/tenants` | platform_admin | List tenants |
| POST | `/api/v1/admin/tenants/{tenantID}/admins` | platform_admin | Create a tenant admin |
| GET | `/api/v1/setup/status` | tenant_admin | Wizard progress / next step |
| GET | `/api/v1/financial-years/options` | tenant_admin | Years that may be created (previous, current, future) |
| GET | `/api/v1/financial-years` | tenant_admin | All financial years |
| GET | `/api/v1/financial-years/current` | tenant_admin | The current year (404 if none) |
| POST | `/api/v1/financial-years/validate` | tenant_admin | Dry-run a create |
| POST | `/api/v1/financial-years` | tenant_admin | Create a financial year |
| GET | `/api/v1/company` | tenant_admin | The company (404 before first save) |
| POST | `/api/v1/company/validate` | tenant_admin | Dry-run a save |
| PUT | `/api/v1/company` | tenant_admin | Create (201) or update (200) the single company |
| GET | `/api/v1/company/tax-details` | tenant_admin | The company's tax and contact details (404 before first save) |
| POST | `/api/v1/company/tax-details/validate` | tenant_admin | Dry-run a save |
| PUT | `/api/v1/company/tax-details` | tenant_admin | Create (201) or update (200) the tax and contact details |
| GET | `/api/v1/trusts` | tenant_admin | All trusts, sorted by code (`[]` when none) |
| GET | `/api/v1/trusts/{code}` | tenant_admin | One trust (404 if unknown) |
| POST | `/api/v1/trusts/validate` | tenant_admin | Dry-run a create |
| POST | `/api/v1/trusts` | tenant_admin | Create a trust (201) |
| PUT | `/api/v1/trusts/{code}` | tenant_admin | Update a trust (200); the code never changes |

---

## Auth

### `POST /api/v1/auth/login`

```json
{ "email": "admin@esis.in", "password": "••••••••••" }
```

`200`

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs…",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs…",
    "token_type": "Bearer",
    "expires_in": 900,
    "refresh_expires_in": 2592000,
    "user": {
      "id": 2, "email": "admin@esis.in", "full_name": "ESIS Admin", "role": "tenant_admin",
      "tenant": { "id": 1, "code": "ESIS", "name": "ESIS Educational Society" }
    }
  }
}
```

`401` wrong email or password (same message for both) · `403` account or tenant disabled · `422` missing fields · `429` too many attempts from this IP.

A disabled user or suspended tenant gets `401` on every later request, and refresh stops working.

Access token payload (decode on the frontend; the server never trusts it without verifying):

```json
{ "typ": "access", "sub": "2", "role": "tenant_admin", "email": "admin@esis.in",
  "name": "ESIS Admin", "tid": 1, "tcode": "ESIS", "iss": "pms-api",
  "iat": 1794035149, "exp": 1794036049, "jti": "0eb2…" }
```

### `POST /api/v1/auth/refresh`

`{ "refresh_token": "eyJ…" }` → `200` same shape as login. The refresh token
you sent is revoked — store the new one. `401` expired, revoked, already used,
or not a refresh token.

### `POST /api/v1/auth/logout`

`{ "refresh_token": "eyJ…" }` → `204`. Discard the access token on the client.

### `GET /api/v1/auth/me`

`200` the `user` object from login.

---

## Admin (platform_admin)

### `POST /api/v1/admin/tenants`

```json
{ "code": "esis", "name": "ESIS Educational Society", "db_key": "esis" }
```

| Field | Rule |
|---|---|
| `code` | required, 2–20 letters/digits, stored upper-case, unique |
| `name` | required, ≤ 200 |
| `db_key` | required, must exist in `TENANT_DATABASES`, unique across tenants |

`201` tenant · `409` code or db_key taken · `422` invalid fields.

### `GET /api/v1/admin/tenants`

`200` `[ { "id", "code", "name", "db_key", "status", "created_at", "updated_at" } ]`

### `POST /api/v1/admin/tenants/{tenantID}/admins`

```json
{ "email": "admin@esis.in", "full_name": "ESIS Admin", "password": "at-least-10-chars" }
```

`201` user (no password hash) · `404` tenant · `409` email taken or tenant suspended · `422` invalid fields.

---

## Setup wizard (tenant_admin, own tenant only)

### `GET /api/v1/setup/status`

```json
{ "data": { "financial_year_done": true, "current_financial_year": "2026-2027",
            "company_done": false, "next_step": "company" } }
```

`next_step` is `financial_year`, `company` or `done`.

### Financial year object

```json
{ "finyr": "2026-2027", "finyr_st_date": "2026-04-01", "finyr_end_date": "2027-03-31",
  "finyr_is_current": true, "rec_created_at": "2026-11-07T07:26:10.854711Z" }
```

### `GET /api/v1/financial-years/options`

The previous year, the current one, then `FY_FUTURE_YEARS` future years (default 1), oldest first.
`kind` is `previous`, `current` or `future`.

```json
{ "data": [
  { "finyr": "2025-2026", "finyr_st_date": "2025-04-01", "finyr_end_date": "2026-03-31", "kind": "previous", "exists": true },
  { "finyr": "2026-2027", "finyr_st_date": "2026-04-01", "finyr_end_date": "2027-03-31", "kind": "current",  "exists": false },
  { "finyr": "2027-2028", "finyr_st_date": "2027-04-01", "finyr_end_date": "2028-03-31", "kind": "future",   "exists": false }
] }
```

### `POST /api/v1/financial-years` · `POST /api/v1/financial-years/validate`

```json
{ "finyr": "2026-2027" }
```

| Field | Required | Rule |
|---|---|---|
| `finyr` | ✓ | `YYYY-YYYY`, consecutive years, any year in the options list (else `must be between 2025-2026 and 2027-2028`) |
| `finyr_st_date` | | if sent, must be 1 April of the first year |
| `finyr_end_date` | | if sent, must be 31 March of the second year |
| `finyr_is_current` | | ignored (forced `true`) while no year is current; otherwise defaults to `true` for today's financial year and `false` for the previous and future ones. `true` un-marks the others |

Create: `201` the year · Validate: `200 { "valid": true, "financial_year": {…} }`
`409` already exists / overlaps · `422` field errors.

### `GET /api/v1/financial-years` · `GET /api/v1/financial-years/current`

`200` list (newest first) / the current year · `404` when there is no current year.

---

## Company (tenant_admin, single record sl = 1)

### Company object

```json
{
  "sl": 1, "name": "ESIS Educational Society", "address": "12 MG Road",
  "city": "Bengaluru", "state": "Karnataka", "country": "India", "pin": "560001",
  "mobile": "9876543210", "email": "office@esis.in", "website": "esis.in",
  "header_1": "Head Office", "header_2": "", "shortname": "ESIS",
  "fin_st_dt": "2026-04-01",
  "time_zone": "2026-11-07T07:26:10.881627Z",
  "rec_created_at": "2026-11-07T07:26:10.881268Z"
}
```

### Input body (save and validate)

Same fields **without** `sl`, `time_zone`, `rec_created_at` (sending them is a 422).

| Field | Required | Rule |
|---|---|---|
| `name` | ✓ | ≤ 300 |
| `shortname` | ✓ | ≤ 50 |
| `fin_st_dt` | | `YYYY-MM-DD`, must be 1 April and, if sent, the `finyr_st_date` of an existing financial year (`must be the start of an existing financial year`); defaults to the current financial year's start |
| `address` | | ≤ 300 |
| `city` | | ≤ 200 |
| `state` | | ≤ 200; Indian state/UT when country is India |
| `country` | | ≤ 200, defaults to `India` |
| `pin` | | 6 digits, not starting with 0 |
| `mobile` | | 10-digit Indian mobile; `+91`, spaces, dashes accepted |
| `email` | | valid email, ≤ 200, stored lower-case |
| `website` | | domain or http(s) URL, ≤ 150 |
| `header_1`, `header_2` | | ≤ 200 each |

### `GET /api/v1/company`

`200` the company · `404` not set up yet.

### `POST /api/v1/company/validate`

`200 { "valid": true, "company": { …normalised, sl 1… } }` · `409` no financial year yet · `422` field errors.

### `PUT /api/v1/company`

`201` first save (created) · `200` later saves (updated; `sl`, `time_zone`,
`rec_created_at` kept) · `409` no financial year yet · `422` field errors.

---

## Company tax and contact details (tenant_admin, Form 3, single record sl = 1)

### Tax details object

Empty text is `""`, an empty date is `null`. The mail password is never returned;
`comm_mail_pwd_set` says whether one is stored.

```json
{ "sl": 1,
  "pan_enabled": true, "pan_enabled_dt": "2026-04-01", "pan": "ABCDE1234F",
  "gstin_enabled": true, "gstin_enabled_dt": "2026-04-01", "gstin": "29ABCDE1234F1ZW", "gstin_state": "Karnataka",
  "comm_mail_id": "accounts@esis.in", "comm_mail_pwd_set": true,
  "comm_mobile": "9876543210", "comm_whatsapp": "9876543210",
  "rec_created_at": "2026-11-09T10:12:00Z" }
```

### Input body (save and validate)

The object **without** `sl`, `comm_mail_pwd_set`, `rec_created_at` (sending them is a 422),
plus optional `comm_mail_pwd`. `PUT` replaces the record, so send every field.
No field is mandatory; anything entered must be valid.

| Field | Rule (checked only when filled) |
|---|---|
| `pan` | trim, upper-case; a PAN like `ABCDE1234F` |
| `pan_enabled` | bool, default `false`. The number stays optional when enabled (Uncle, 9 Nov) |
| `pan_enabled_dt` | `YYYY-MM-DD`, not after today. Only allowed when `pan_enabled`; enabled with no date → today |
| `gstin` | trim, upper-case; valid GSTIN (format + check digit); first 2 digits a code from `/meta/gst-states`; if `pan` is filled, characters 3–12 must equal it |
| `gstin_enabled`, `gstin_enabled_dt` | same as PAN, against `gstin` |
| `gstin_state` | ≤ 100, matched to the official spelling. With a `gstin`: empty → filled from its code; filled → must match its code |
| `comm_mail_id` | trim, lower-case, ≤ 150, valid email |
| `comm_mail_pwd` | write-only, ≤ 150, not trimmed. Missing or `null` → keep the stored one; `""` → clear it; text → replace it |
| `comm_mobile`, `comm_whatsapp` | 10-digit Indian mobile; `+91`, spaces, dashes accepted |

Example messages: `pan`: *must be a PAN like ABCDE1234F* ·
`gstin`: *is not a valid GSTIN*, *PAN inside GSTIN must match the PAN* ·
`gstin_state`: *does not match GSTIN state code 29 (Karnataka)* ·
`pan_enabled_dt`: *cannot be after today (2026-11-09)*, *can only be set when PAN is enabled*.

### `GET /api/v1/company/tax-details`

`200` the details · `404` not saved yet.

### `POST /api/v1/company/tax-details/validate`

`200 { "valid": true, "tax_details": {…} }` · `409` company not saved yet · `422` every failing field.

### `PUT /api/v1/company/tax-details`

`201` first save · `200` later saves · `409` company not saved yet (`Save the company details first`) · `422` every failing field.

## Meta

### `GET /api/v1/meta/gst-states`

`200` the GST state codes in use, sorted by code; names spelled as in `/meta/indian-states`
(plus `97 Other Territory`). The retired codes 25 and 28 are not listed.

```json
{ "data": [ { "code": "01", "name": "Jammu and Kashmir" }, …, { "code": "29", "name": "Karnataka" }, …, { "code": "97", "name": "Other Territory" } ] }
```

## Trusts (tenant_admin, Form 4)

A company has several trusts (ESIS: ACO, KHE, SAH, SAM, SAT). `trust_code` is the key
and never changes. `{code}` in the path is upper-cased before lookup. There is no delete
yet: Uncle allows deleting a trust only while it has no transactions, which needs a DBA
procedure (not written yet).

### Trust object

```json
{ "trust_code": "SAT", "trust_desc": "Sri Adichunchanagiri Trust",
  "wrk_st_finyr": "2026-2027", "wrk_st_date": "2026-04-01",
  "rec_created_at": "2026-11-09T10:12:00Z", "last_update_at": "2026-11-09T10:12:00Z" }
```

### Body

`POST /trusts` and `/trusts/validate`: `trust_code`, `trust_desc`, `wrk_st_finyr`, `wrk_st_date`.
`PUT /trusts/{code}`: the same **without** `trust_code` (sending it is a 422).
Server-owned `rec_created_at` / `last_update_at` are never accepted.

| Field | Rule |
|---|---|
| `trust_code` | required, trim, upper-case, exactly 3 letters `^[A-Z]{3}$`; not already used (`409`, field `trust_code`: *is already taken*) |
| `trust_desc` | required, trim, ≤ 100; unique **ignoring case** among other trusts (`409`, field `trust_desc`: *is already used by trust SAT*) |
| `wrk_st_finyr` | required; must exist in `finyr_master` (*must be an existing financial year*) |
| `wrk_st_date` | required, `YYYY-MM-DD`; must fall inside `wrk_st_finyr`, 1 April → 31 March (*must fall inside 2026-2027 (2026-04-01 to 2027-03-31)*). Pending Uncle's confirmation |

After our checks, the DBA's `mst_trust_validate` runs; each sentence of its message
is mapped to a field (`Trust code…` → `trust_code`, `Trust description…` → `trust_desc`,
`…financial year…` → `wrk_st_finyr`, `Working start date…` → `wrk_st_date`, anything else → no field).

### Responses

| Endpoint | Success | Errors |
|---|---|---|
| `GET /trusts` | `200` list sorted by `trust_code` (empty list, not 404) | — |
| `GET /trusts/{code}` | `200` the trust | `404` |
| `POST /trusts/validate` | `200 { "valid": true, "trust": {…} }` | `409` code or description taken · `422` |
| `POST /trusts` | `201` the trust | `409` code or description taken · `422` |
| `PUT /trusts/{code}` | `200` the trust (`last_update_at` moves) | `404` · `409` description taken · `422` |

---

## curl walkthrough

```bash
B=http://localhost:8080
J='Content-Type: application/json'

# platform admin
ROOT=$(curl -s -H "$J" -d '{"email":"root@example.com","password":"…"}' $B/api/v1/auth/login | jq -r .data.access_token)
curl -s -H "$J" -H "Authorization: Bearer $ROOT" -d '{"code":"ESIS","name":"ESIS Educational Society","db_key":"esis"}' $B/api/v1/admin/tenants
curl -s -H "$J" -H "Authorization: Bearer $ROOT" -d '{"email":"admin@esis.in","full_name":"ESIS Admin","password":"…"}' $B/api/v1/admin/tenants/1/admins

# tenant admin — setup wizard
T=$(curl -s -H "$J" -d '{"email":"admin@esis.in","password":"…"}' $B/api/v1/auth/login | jq -r .data.access_token)
A="Authorization: Bearer $T"
curl -s -H "$A" $B/api/v1/setup/status
curl -s -H "$A" $B/api/v1/financial-years/options
curl -s -H "$J" -H "$A" -d '{"finyr":"2026-2027"}' $B/api/v1/financial-years
curl -s -X PUT -H "$J" -H "$A" -d '{"name":"ESIS Educational Society","shortname":"ESIS","mobile":"+91 98765 43210","pin":"560001","state":"karnataka"}' $B/api/v1/company
curl -s -H "$A" $B/api/v1/setup/status
```
