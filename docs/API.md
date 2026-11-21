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
| POST | `/api/v1/institutions/import` | tenant_admin | Check (`?dry_run=true`) or import an .xlsx of institutions |
| GET | `/api/v1/institutions/import/template` | tenant_admin | The import template (.xlsx) |
| GET | `/api/v1/vendors` | tenant_admin | Every vendor with its item groups, sorted by name (`[]` when none) |
| GET | `/api/v1/vendors/options` | tenant_admin | Vendor types, supply types, certificate slots |
| GET | `/api/v1/vendors/export` | tenant_admin | Every vendor as an .xlsx in the import layout |
| POST | `/api/v1/vendors/import` | tenant_admin | Check (`?dry_run=true`) or import an .xlsx of vendors |
| GET | `/api/v1/vendors/import/template` | tenant_admin | The vendor import template (.xlsx) |
| GET | `/api/v1/vendors/{id}` | tenant_admin | One vendor with its financials and certificates (404 if unknown) |
| POST | `/api/v1/vendors/validate` | tenant_admin | Dry-run a create |
| POST | `/api/v1/vendors` | tenant_admin | Add a vendor with its item groups (201) |
| PUT | `/api/v1/vendors/{id}` | tenant_admin | Save a vendor and replace its item groups (200) |
| DELETE | `/api/v1/vendors/{id}` | tenant_admin | Delete a vendor, its financials, certificates and files (204) |
| POST | `/api/v1/vendors/{id}/financials` | tenant_admin | Add a financial year (201) |
| PUT · DELETE | `/api/v1/vendors/{id}/financials/{fin_year}` | tenant_admin | Replace (200) or delete (204) a year |
| PUT | `/api/v1/vendors/{id}/certificates` | tenant_admin | Save the certificates (200) |
| POST | `/api/v1/vendors/{id}/files` | tenant_admin | Upload a PDF, JPG or PNG (201), returns its key |
| GET | `/api/v1/vendors/{id}/files/url?key=` | tenant_admin | A 10-minute link to open a file |

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

## Stock masters (tenant_admin)

Item groups, categories, manufacturers, units of measurement (UOM) and items, on the DBA's
`pms.mst_stk_*` tables. His tables number their rows themselves, so every row is addressed by
a numeric **`id`** (his `grp_cd`, `cat_cd`, `manf_cd`, `uom_cd`, `item_cd`). There is **no
edit** yet (no update procedures) and only **items** can be deleted (open items).

| Method | Path | Success | Errors |
|---|---|---|---|
| GET | `/api/v1/stock/{list}` | `200` every row, sorted by name | — |
| GET | `/api/v1/stock/{list}/{id}` | `200` the row | `404` |
| POST | `/api/v1/stock/{list}/validate` | `200 { "valid": true, "entry": {…} }` (nothing written, `id` 0) | `409` · `422` |
| POST | `/api/v1/stock/{list}` | `201` the saved row | `409` · `422` |
| DELETE | `/api/v1/stock/items/{id}` | `204` | `404` · `409` |
| GET | `/api/v1/stock/items/export` | `200` every item as an .xlsx in the import layout | — |
| POST | `/api/v1/stock/items/import` | `200` an import report (see *Excel import of items and vendors*) | `422` |
| GET | `/api/v1/stock/items/import/template` | `200` the item import template (.xlsx) | — |

`{list}` is `groups`, `categories`, `manufacturers`, `uoms` or `items`.

### Groups, categories, manufacturers

```json
{ "id": 1, "name": "Snack Foods", "rec_created_at": "2026-11-15T12:21:03Z", "last_update_at": null }
```

Body: `{ "name": "Snack Foods" }`. Required, spaces tidied, ≤ 250, unique **ignoring case and
spaces** (`409`, *is already used by #1*).

### Units of measurement

```json
{ "id": 2, "uom": "KG", "uom_as_standard": "Kilograms", "enable_decimal": true, "number_decimal": 3 }
```

| Field | Rule |
|---|---|
| `uom` | required, trimmed, upper-cased, ≤ 5, no spaces, unique ignoring case |
| `uom_as_standard` | required, spaces tidied, ≤ 50, unique ignoring case (e.g. `Numbers` for `NOS`) |
| `enable_decimal` | boolean, default `false` |
| `number_decimal` | decimals off: omit or `0` (stored `0`). Decimals on: required, `1`–`9` |

### Items

```json
{
  "id": 1, "item_desc": "Lays 50g", "item_code": "LAY-50",
  "grp_cd": 1, "item_group": "Snack Foods", "cat_cd": 1, "category": "Snacks",
  "manf_cd": 1, "manufacturer": "PepsiCo", "uom_cd": 1, "uom": "NOS",
  "goods_service_item": "Goods", "item_consumable": true, "item_is_spare": false,
  "gst_applicable": true, "gst_hsn_code": "1905",
  "gst_percentage": 18, "sgst_percentage": 9, "cgst_percentage": 9,
  "track_serial": false
}
```

`item_group`, `category`, `manufacturer` and `uom` are read-only names of the linked rows;
send only the ids.

| Field | Rule |
|---|---|
| `item_desc` | required, spaces tidied, ≤ 250, unique ignoring case and spaces (`409`) |
| `item_code` | optional, upper-cased, ≤ 20, letters, digits and `- / .`, unique (`409`) |
| `grp_cd` · `uom_cd` | **required** (Uncle, 15 Nov); must exist (`422`, *does not exist*) |
| `cat_cd` · `manf_cd` | optional; must exist when sent |
| `goods_service_item` | required, `Goods` or `Service` (any case) |
| `item_consumable` · `item_is_spare` · `track_serial` | booleans; goods only (`422`, *applies to goods only*) |
| `gst_applicable` | boolean, default `true` |
| `gst_hsn_code` | optional, 4, 6 or 8 digits (HSN for goods, SAC for services) |
| `gst_percentage` · `sgst_percentage` · `cgst_percentage` | GST applies: all three required, 0–100, ≤ 2 decimals, GST = SGST + CGST. GST does not apply: omit or `0` (stored empty) |

Every problem comes back at once. Delete refuses (`409`) an item with stock or a purchase.

---

## Vendors (tenant_admin)

On the DBA's `pms.mst_vendor`, `mst_vendor_item_group` (the stock groups a vendor supplies),
`mst_vendor_financials` (one row per year) and `mst_vendor_certificates` (one row per vendor).
Every write goes through his procedures, by named arguments. A vendor is addressed by its numeric
`id` (his `vendor_cd`).

### Vendor object

```json
{
  "id": 1, "name": "ABC Computers", "address": "14 Residency Road", "city": "Bengaluru",
  "state": "Karnataka", "country": "", "pin": "560025", "vendor_type": "Proprietor",
  "mobile": "08023456789", "email": "sales@abc.in", "website": "abc.in", "reg_date": "2020-04-01",
  "pan": "ABCDE1234F", "gstin": "29ABCDE1234F1ZW", "gstin_state": "Karnataka", "tan": "BLRA12345B",
  "refered_by": "", "correspond_email": "", "correspond_mobile": "9845012345",
  "supply_type": "Sales and service", "is_contruction_comp": false, "is_manufacturer": true,
  "company_since": "1998",
  "contacts": [{ "person": "Ravi Kumar", "designation": "Sales Manager", "mobile": "9845012345", "email": "ravi@abc.in" }, {…}, {…}],
  "item_groups": [{ "grp_cd": 2, "item_group": "Printers" }]
}
```

`GET /vendors/{id}` adds `financials` (newest year first) and `certificates` (every slot).
Field names are the DBA's, including his spellings `refered_by` and `is_contruction_comp`.

### Body (`POST /vendors`, `PUT /vendors/{id}`, `/validate`)

The vendor's fields (not `id`, `gstin_state`, `item_groups`), `contacts` (up to 3) and
`grp_cds` (the stock group ids it supplies; replaces the list on every save). Every problem
comes back at once.

| Field | Rule |
|---|---|
| `name` | required, spaces tidied, ≤ 250, unique ignoring case and spaces (`409`) |
| `vendor_type` | optional: `Proprietor`, `Partnership`, `Ltd` or `Public` (any case) |
| `supply_type` | optional: `Sales`, `Service` or `Sales and service` |
| `pin` | India (country empty or India): 6 digits. Elsewhere: letters, digits, spaces, `-`, 3–10 |
| `mobile` | the vendor's phone: optional `+` and 7–14 digits (spaces and dashes dropped) |
| `correspond_mobile` · `contacts[].mobile` | 10-digit Indian mobile (`+91`/`0` dropped) |
| `email` · `correspond_email` · `contacts[].email` | email format, lower-cased |
| `pan` | PAN format, upper-cased, unique (`409`) |
| `gstin` | GSTIN format and check digit, upper-cased, unique (`409`), must carry the PAN; `gstin_state` is set from its code |
| `tan` | `ABCD12345E` format, upper-cased |
| `reg_date` | `YYYY-MM-DD`, not in the future |
| `company_since` | a year from 1800 to this year |
| `contacts[].person` | required when the contact has anything else |
| `grp_cds` | each must exist (`422`) |

An emptied field is stored as NULL. Delete removes the vendor with its groups, financials,
certificates and their files; a stock group a vendor supplies cannot be deleted (`409`).

### Financials

```json
{ "fin_year": "2025-2026", "fin_turnover_declared": 12500000.5, "fin_profit_declared": -25000,
  "fin_itr_no": "ACK123456", "fin_itr_date": "2025-09-15",
  "fin_itr_s3_bucket_reference": "esis/vendors/1/b54efadfce33cd20-ITR-2025.pdf" }
```

`fin_year` two consecutive years (`2025-2026`), not after the current financial year, one row
per year (`409`). Turnover ≥ 0, profit may be negative, both ≤ 2 decimals. ITR number ≤ 25
(upper-cased), date not in the future. `PUT …/financials/{fin_year}` replaces the whole year
(the year itself never changes). A replaced or deleted ITR file is removed from storage.

### Certificates

Slots: `gst`, `company_reg`, `pan`, `msme`, `tan`, `aadhar`, `oth_1`, `oth_2`, `oth_3`.

```json
{ "gst": { "file": "esis/vendors/1/ed35cf0c0f75afda-GST-certificate.pdf" },
  "oth_1": { "file": "esis/vendors/1/1b13d4c81ece193b-iso.png", "caption": "ISO 9001 certificate" } }
```

`PUT` sends every slot to keep (a missing or empty `file` clears it). A slot is marked uploaded
exactly when it has a file. The `oth_*` slots need a caption (≤ 100) when they have a file.
A file no slot points at any more is removed from storage.

### Files

Upload first, then save the returned `key` in a certificate or financial year:

`POST /vendors/{id}/files` (multipart field `file`, PDF, JPG or PNG by content, ≤ 10 MB) →
`201 { "key": "esis/vendors/1/…-GST-certificate.pdf", "name": "…", "size": 1234, "content_type": "application/pdf" }`.

A key must belong to the vendor (`<tenant>/vendors/<id>/`), else `422`. The bucket is private:
`GET /vendors/{id}/files/url?key=…` returns `{ "url": "…" }`, valid 10 minutes. Without S3
configured (`S3_BUCKET`), uploads and links answer `503`.

---

## Excel import of items and vendors

`POST /stock/items/import` and `POST /vendors/import` take an `.xlsx` as `multipart/form-data`,
field `file` (5 MB; 5,000 items or 2,000 vendors at most). They work like the institution
import: only the first sheet is read, row 1 is the header, headers are matched ignoring case,
spaces and punctuation (`%` reads as *percent*, so `GST %` is `gst_percentage`), blank rows are
ignored. `?dry_run=true` saves nothing; without it every ready row is saved in **one
transaction** after the duplicates, links and the DBA's check are re-run under the lock (a
change meanwhile is `409` and nothing is saved). Problem rows are skipped, so fixing them and
uploading the same file again imports only those.

Every row gets the checks of a single create (`POST /stock/items`, `POST /vendors`), plus:
duplicates inside the file, duplicates against what is saved (item description or code; vendor
name, PAN or GSTIN), and names that must exist in their lists. The **export** writes the same
columns, so an exported file can be edited and re-imported (rows already there are reported, not
duplicated). Import only adds; it never changes a saved row.

**Items** (`GET /stock/items/import/template` has every column, an example row and a Help sheet):
`item_desc`*, `item_code`*, `goods_service_item`* (Goods or Service), `item_group`*, `uom`*,
`category`, `manufacturer` (names from their lists), `gst_applicable` (yes/no, empty = yes),
`gst_hsn_code`, `gst_percentage`, `sgst_percentage`, `cgst_percentage`, `item_consumable`,
`item_is_spare`, `track_serial` (yes/no). The DBA's `fn_validate_mst_stk_items` runs for every
row in one round trip.

**Vendors**: `name`*, `vendor_type`, `supply_type`, `item_groups` (group names separated by
commas), `address`, `city`, `state`, `country`, `pin`, `mobile`, `email`, `website`, `reg_date`
(YYYY-MM-DD, DD-MM-YYYY or DD/MM/YYYY), `company_since`, `refered_by`, `is_manufacturer`,
`is_contruction_comp` (yes/no), `pan`, `gstin`, `tan`, `correspond_email`, `correspond_mobile`,
`contact_1_person` … `contact_3_email`. Financials and certificates are not imported. The DBA's
`fn_validate_mst_vendor` raises instead of returning, so it runs once per ready row.

```json
{ "data": { "dry_run": true, "total": 6, "ready": 2, "imported": 0, "skipped": 4,
  "problems": [ { "row": 6, "key": "CH-1", "name": "Chair",
                  "errors": [ { "field": "uom_cd", "message": "\"Pieces\" is not in the units list" } ] } ],
  "missing": { "uom": [ { "name": "Pieces", "rows": 1 } ] } } }
```

`key` is the item code or the vendor's PAN. `missing` lists, per column (`item_group`,
`category`, `manufacturer`, `uom`, `item_groups`), every name the file uses that its list does
not have yet, most used first.

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

### `POST /institutions/import` (Excel import)

Upload an `.xlsx` as `multipart/form-data`, field `file` (5 MB, 5,000 rows at most; an old
`.xls` or a CSV is refused with *the file is not an Excel .xlsx workbook*). Only the first
sheet is read; row 1 is the header. Headers are matched ignoring case, spaces and
punctuation, and the old system's names work too:

| Column | Also accepted | Required |
|---|---|---|
| `trust_code` | `ACM_INST_TRUST`, `trust` | yes |
| `inst_code` | `ACM_CODE` | yes |
| `inst_reg_code` | `ACM_REG_CODE` | yes |
| `inst_name` | `ACM_INST_NAME`, `name` | yes |
| `place` | `ACM_INST_PLACE`, `place_code`, `place_name` | yes, a place **code or name** from the places list |
| `branch` | `ACM_INST_BRANCH`, `branch_code`, `branch_name` | yes, a branch **code or name** |
| `from_fin_yr`, `reg_date`, `inst_add`, `inst_place`, `inst_tel`, `inst_email`, head / accountant / liaison columns | | no |
| `inst_full_code`, `regn_code` | `ACM_FULL_CODE`, `ACM_INST_REG_SHORT_CODE` | no; checked against the codes, never stored |

Every row gets exactly the checks of `POST /institutions`, plus:
- the codes must not repeat inside the file;
- a place or branch must match a code or a name, where names ignore case and spaces;
- an empty `from_fin_yr` means the current year;
- a place given by name also becomes `inst_place` (the locality) when that column is empty;
- number cells keep all their digits, and date cells are read as dates.

Then the DBA's `fn_mst_inst_validate` runs for all rows in one query.

`?dry_run=true` saves nothing. Without it, **every ready row is saved in one transaction**:
the codes and the DBA's check are re-run under the create lock first, and if anything
changed meanwhile nothing is saved (`409`). Rows with problems are skipped, so fixing them
and uploading the same file again imports only those (the rest are *already used*).

```json
{ "data": { "dry_run": true, "total": 571, "ready": 120, "imported": 0, "skipped": 451,
  "default_from_fin_yr": "2026-2027",
  "problems": [ { "row": 3, "inst_code": "SAT-002", "inst_name": "…",
                  "errors": [ { "field": "place_code", "message": "\"Kunigal\" is not in the places list" } ] } ],
  "missing_places":   [ { "name": "Kunigal", "rows": 4 } ],
  "missing_branches": [ { "name": "Mysore Branch", "rows": 72 } ] } }
```

`missing_places` and `missing_branches` list every name the file uses that the lists do
not have yet, most used first, so they can be added in one go. `422` on `file` for a
missing required column, an empty sheet or a non-.xlsx file.

`GET /institutions/import/template` returns an `.xlsx` with every column, an example row
and a Help sheet.

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
