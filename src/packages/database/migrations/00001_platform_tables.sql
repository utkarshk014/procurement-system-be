-- Platform tables: who can log in, and which tenant they belong to.
-- They live in the pms schema with a sys_ prefix to stay clearly apart from
-- the business tables (company_master, mst_*). Written for PostgreSQL 11.

-- +goose Up
CREATE TABLE pms.sys_tenant
(
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(20)  NOT NULL,
    name        VARCHAR(200) NOT NULL,
    db_key      VARCHAR(50)  NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT sys_tenant_code_uq   UNIQUE (code),
    -- One tenant per database: the business tables carry no tenant column,
    -- so sharing a database would mix two tenants' data.
    CONSTRAINT sys_tenant_db_key_uq UNIQUE (db_key),
    CONSTRAINT sys_tenant_status_chk CHECK (status IN ('active', 'suspended'))
);

CREATE TABLE pms.sys_user
(
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email          VARCHAR(200) NOT NULL,
    full_name      VARCHAR(200) NOT NULL,
    password_hash  TEXT         NOT NULL,
    role           VARCHAR(30)  NOT NULL,
    tenant_id      BIGINT       REFERENCES pms.sys_tenant (id),
    is_active      BOOLEAN      NOT NULL DEFAULT TRUE,
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT sys_user_role_chk CHECK (role IN ('platform_admin', 'tenant_admin')),
    -- Platform admins sit above every tenant; everyone else belongs to exactly one.
    CONSTRAINT sys_user_role_tenant_chk CHECK (
        (role = 'platform_admin' AND tenant_id IS NULL) OR
        (role <> 'platform_admin' AND tenant_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX sys_user_email_uq ON pms.sys_user (lower(email));
CREATE INDEX sys_user_tenant_idx ON pms.sys_user (tenant_id);

-- Every refresh token issued, keyed by its JWT id (jti). Access tokens are
-- not stored; a refresh token only works while its row is unrevoked, so
-- logout and reuse detection (revoke all of a user's tokens) take effect immediately.
CREATE TABLE pms.sys_refresh_token
(
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    jti         VARCHAR(64)  NOT NULL,
    user_id     BIGINT       NOT NULL REFERENCES pms.sys_user (id) ON DELETE CASCADE,
    ip          VARCHAR(64),
    user_agent  VARCHAR(300),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ  NOT NULL,
    revoked_at  TIMESTAMPTZ,
    CONSTRAINT sys_refresh_token_jti_uq UNIQUE (jti)
);

CREATE INDEX sys_refresh_token_user_idx ON pms.sys_refresh_token (user_id);
CREATE INDEX sys_refresh_token_expires_idx ON pms.sys_refresh_token (expires_at);

-- +goose Down
DROP TABLE pms.sys_refresh_token;
DROP TABLE pms.sys_user;
DROP TABLE pms.sys_tenant;
