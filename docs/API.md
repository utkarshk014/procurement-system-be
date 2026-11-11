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
| PUT | `/api/v1/financial-years/current` | tenant_admin | Make another existing year the current one |
| DELETE | `/api/v1/financial-years/{finyr}` | tenant_admin | Delete a year that is not current and unused (204) |
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
| DELETE | `/api/v1/trusts/{code}` | tenant_admin | Delete a trust no institution uses (204) |
| GET | `/api/v1/places` · `/branches` · `/regions` | tenant_admin | One code list, sorted by name (`[]` when none) |
| GET | `/api/v1/places/{code}` (and the other two) | tenant_admin | One row (404 if unknown) |
| POST | `/api/v1/places/validate` (and the other two) | tenant_admin | Dry-run a create |
| POST | `/api/v1/places` (and the other two) | tenant_admin | Add a row (201) |
| DELETE | `/api/v1/places/{code}` (and the other two) | tenant_admin | Delete a row no institution uses (204) |
| GET | `/api/v1/institutions` | tenant_admin | One page of institutions, filtered, sorted by full code |
| GET | `/api/v1/institutions/{code}` | tenant_admin | One institution by ACM code (404 if unknown) |
| GET | `/api/v1/institutions/next-code` | tenant_admin | Suggested next ACM and reg codes |
| POST | `/api/v1/institutions/validate` | tenant_admin | Dry-run a create |
| POST | `/api/v1/institutions` | tenant_admin | Add an institution (201) |

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

The previous year, the current one, then `FY_FUTURE_YEARS` future years, oldest first.
`kind` is `previous`, `current` or `future`. The default is 0 future years (Uncle, 9 Nov:
a year may be created only once today falls in it), so live returns only the first two rows
below; the `future` row appears only when `FY_FUTURE_YEARS` is raised.

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

### `PUT /api/v1/financial-years/current`

```json
{ "finyr": "2027-2028" }
```

`200` the year, now current; every other year is un-marked, so exactly one is current.
Already current → `200`, nothing changes. `422` on `finyr` when it is missing, not
`YYYY-YYYY` (*must look like 2026-2027 (two consecutive years)*) or not an existing year
(*must be an existing financial year*). Only `finyr` is accepted. There is no date edit:
the label fixes the dates.

### `DELETE /api/v1/financial-years/{finyr}`

`204` when deleted. `409` for the current year (*The current financial year cannot be
deleted. Make another year current first.*) and for a year the DBA's
`finyr_master_validate_delete` refuses, with his message, e.g. *Cannot delete! Financial
year is already used in Trust Master (mst_trust)*. `404` for an unknown year.

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
and never changes. `{code}` in the path is upper-cased before lookup.

`DELETE /trusts/{code}`: `204` when deleted; `409` with the DBA's message while an
institution uses the trust (*Cannot delete! Trust code is already used in Institution
Master (mst_inst)*); `404` when unknown. Uncle allows a delete only while the trust has no
transactions; there are no transaction tables yet, so his check covers institutions only.

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

## Places, branches, regions (tenant_admin, Form 5)

Three small lists an institution points into. They share one shape and one set of
rules; only the path and the DBA objects differ (`pms.mst_place` / `mst_branch` /
`mst_region`, each with insert and delete procedures and validate functions). There is
**no edit**: the DBA has given no update procedure (open items). A row can be deleted
while no institution uses it.

### Object and body

```json
{ "code": "MYS", "name": "Mysore", "rec_created_at": "2026-11-09T10:12:00Z", "last_update_at": null }
```

`POST` and `/validate` take `code` and `name` only.

| Field | Rule |
|---|---|
| `code` | required, trim, upper-case, exactly 3 letters `^[A-Z]{3}$` (Uncle's own check only counts 3 characters; we keep the stricter trust rule, pending his answer); not already used (`409`, *is already taken*) |
| `name` | required, spaces tidied, ≤ 250; unique **ignoring case** (`409`, *is already used by MYS*) |

After our checks the DBA's validate function runs (it stops at its first problem; its
`Place code…` / `Place name…` messages map to `code` / `name`, a duplicate is a 409),
then one locked transaction re-checks and calls his insert procedure.

| Endpoint | Success | Errors |
|---|---|---|
| `GET /places` | `200` list sorted by name | — |
| `GET /places/{code}` | `200` the row | `404` |
| `POST /places/validate` | `200 { "valid": true, "entry": {…} }` | `409` · `422` |
| `POST /places` | `201` the row | `409` · `422` |
| `DELETE /places/{code}` | `204` | `404` · `409` with the DBA's message while an institution uses it |

---

## Institutions (tenant_admin, Form 5)

Company → Trust → Institution. An institution has an **ACM code** (`inst_code`, the
trust code + a 3-digit number counted per trust, `SAT-001`), a **reg code**
(`inst_reg_code`, a region code + a 2-digit number, `MYS-01`) and a **full code** built
by the server as `inst_code:inst_reg_code` (`SAT-001:MYS-01`, 14 characters, as on the
old system's "ACM Code (14 Char.)"). This follows all 571 rows of the client's sheet.
Uncle's table says `inst_code` is 9 characters; with 9 no 14-character full code can be
formed, so **creating an institution on live fails until he changes it to 7** (open items).
There is no edit or delete yet (no DBA procedure for either).

### Institution object

```json
{ "inst_code": "SAT-001", "inst_reg_code": "MYS-01", "inst_full_code": "SAT-001:MYS-01",
  "inst_name": "BGS Public School", "inst_add": "Kuvempunagar", "inst_place": "Mysore (Kuvempunagar)",
  "place_code": "MYS", "place_name": "Mysore", "branch_code": "MYB", "branch_name": "Mysore Branch",
  "trust_code": "SAT", "trust_name": "Sri Adichunchanagiri Shikshana Trust",
  "reg_date": "2026-11-09", "from_fin_yr": "2026-2027",
  "inst_email": "office@bgs.test", "inst_tel": "0821 2541234",
  "inst_head_name": "Dr. Head", "inst_head_desig": "Principal", "inst_head_mobile": "9845012345", "inst_head_email": "head@bgs.test",
  "inst_acnt_name": "", "inst_acnt_mobile": "", "inst_acnt_email": "",
  "inst_liason_name": "", "inst_liason_desig": "", "inst_liason_mobile": "", "inst_liason_email": "",
  "rec_created_at": "2026-11-09T10:12:00Z", "last_update_at": null }
```

`place_name`, `branch_name` and `trust_name` are copied in by the DBA's procedure. JSON
names are his columns, including the spelling `liason`.

### Body (`POST /institutions` and `/validate`)

Every field of the object except `inst_full_code`, the three `*_name` copies and the
timestamps (sending them is a 422). All problems come back in one 422.

| Field | Rule |
|---|---|
| `trust_code` | required, an existing trust |
| `inst_code` | required, upper-cased, `^[A-Z]{3}-[0-9]{3}$` and must start with `trust_code` (*must start with the trust code SAT-*) |
| `inst_reg_code` | required, upper-cased, `^[A-Z]{3}-[0-9]{2}$`, and its first 3 letters must be an existing region |
| ACM, reg and full code | not already used (`409` on `inst_code` / `inst_reg_code`: *is already used by SAT-001*) |
| `inst_name` | required, ≤ 250 · `inst_add`, `inst_place` ≤ 250 (free text locality) |
| `place_code`, `branch_code` | required, existing place / branch |
| `from_fin_yr` | required, an existing financial year (the screen offers current and previous) |
| `reg_date` | optional `YYYY-MM-DD`, not in the future; today when left out |
| `inst_email`, `inst_head_email`, `inst_acnt_email`, `inst_liason_email` | optional, valid email, stored lower-case |
| `inst_head_mobile`, `inst_acnt_mobile`, `inst_liason_mobile` | optional; `+91`, spaces and a leading 0 are dropped, then exactly 10 digits |
| `inst_tel` ≤ 100 · names and designations ≤ 100 | optional |

After our checks the DBA's `fn_mst_inst_validate` runs; its first message is mapped to
its field (`Invalid place code` → `place_code`, `Accountant mobile…` → `inst_acnt_mobile`, …).

### `GET /institutions`

Query: `trust_code`, `branch_code`, `place_code`, `q` (part of the name or full code),
`page` (from 1), `page_size` (default 50, at most 200).

```json
{ "data": { "items": [ { …institution… } ], "total": 571, "page": 1, "page_size": 50 } }
```

### `GET /institutions/next-code?trust_code=SAT&regn_code=MYS`

The highest number used with each prefix, plus one: `{ "inst_code": "SAT-413", "inst_reg_code": "MYS-60" }`.
Either parameter may be left out (its field is then `""`); a prefix whose numbers are
used up (999 / 99) also returns `""`.

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
