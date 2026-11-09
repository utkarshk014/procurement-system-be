-- Form 5 objects (places, branches, regions, institutions) the DBA installed in
-- esispms on 9 Nov 2026, copied verbatim from TABLE_PROCEDURE_FUNCTION_1_1.xlsx.
-- mst_branch_validate is on live but not in the sheet; its text is taken from the
-- live catalog (pg_get_functiondef). Do not run this against the real database.
-- Known problems are NOT fixed here; see 003_local_fixes.sql.

-- ---- mst_place: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_place
(
    place_code      VARCHAR(3) PRIMARY KEY,
    place_name      VARCHAR(250) NOT NULL UNIQUE,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP,

    CONSTRAINT chk_mst_place_code_length
        CHECK (CHAR_LENGTH(place_code) = 3)
);

-- ---- mst_place: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_place_insert
(
    IN take_place_code VARCHAR(3),
    IN take_place_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate place code
    IF take_place_code IS NULL OR
       LENGTH(TRIM(take_place_code)) <> 3 THEN
        RAISE EXCEPTION 'Place code must contain exactly 3 characters';
    END IF;

    -- Validate place name
    IF take_place_name IS NULL OR
       TRIM(take_place_name) = '' THEN
        RAISE EXCEPTION 'Place name cannot be empty';
    END IF;

    -- Check duplicate place code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_place
        WHERE place_code = UPPER(TRIM(take_place_code))
    ) THEN
        RAISE EXCEPTION 'Place code % already exists',
            UPPER(TRIM(take_place_code));
    END IF;

    -- Check duplicate place name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_place
        WHERE UPPER(TRIM(place_name)) =
              UPPER(TRIM(take_place_name))
    ) THEN
        RAISE EXCEPTION 'Place name % already exists',
            TRIM(take_place_name);
    END IF;

    -- Insert record
    INSERT INTO pms.mst_place
    (
        place_code,
        place_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_place_code)),
        TRIM(take_place_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_place: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.mst_place_validate
(
    take_place_code VARCHAR(3),
    take_place_name VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate place code
    IF take_place_code IS NULL OR
       TRIM(take_place_code) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Place code cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    IF LENGTH(TRIM(take_place_code)) <> 3 THEN

        RETURN QUERY
        SELECT FALSE, 'Place code must contain exactly 3 characters'::VARCHAR;
        RETURN;

    END IF;

    -- Validate place name
    IF take_place_name IS NULL OR
       TRIM(take_place_name) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Place name cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Check duplicate place code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_place
        WHERE place_code = UPPER(TRIM(take_place_code))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Place code already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Check duplicate place name (case insensitive)
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_place
        WHERE LOWER(TRIM(place_name)) =
              LOWER(TRIM(take_place_name))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Place name already exists'::VARCHAR;
        RETURN;

    END IF;

    -- All validations passed
    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_place: FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_mst_place_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_place: TRIGGER # 1
CREATE TRIGGER trg_mst_place_update
BEFORE UPDATE ON pms.mst_place
FOR EACH ROW
EXECUTE FUNCTION pms.fn_mst_place_update();

-- ---- mst_branch: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_branch
(
    branch_code      VARCHAR(3) PRIMARY KEY,
    branch_name      VARCHAR(250) NOT NULL UNIQUE,
    rec_created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at   TIMESTAMP,

    CONSTRAINT chk_mst_branch_code_length
        CHECK (CHAR_LENGTH(branch_code) = 3)
);

-- ---- mst_branch: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_branch_insert
(
    IN take_branch_code VARCHAR(3),
    IN take_branch_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_branch_code IS NULL OR
       LENGTH(TRIM(take_branch_code)) <> 3 THEN
        RAISE EXCEPTION
        'Branch code must contain exactly 3 characters';
    END IF;

    IF take_branch_name IS NULL OR
       TRIM(take_branch_name) = '' THEN
        RAISE EXCEPTION 'Branch name cannot be empty';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_branch
        WHERE branch_code = UPPER(TRIM(take_branch_code))
    ) THEN
        RAISE EXCEPTION 'Branch code already exists';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_branch
        WHERE LOWER(TRIM(branch_name)) =
              LOWER(TRIM(take_branch_name))
    ) THEN
        RAISE EXCEPTION 'Branch name already exists';
    END IF;

    INSERT INTO pms.mst_branch
    (
        branch_code,
        branch_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_branch_code)),
        TRIM(take_branch_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_branch: FUNCTION (from the live catalog, not in the sheet)
CREATE OR REPLACE FUNCTION pms.mst_branch_validate(take_branch_code character varying, take_branch_name character varying)
 RETURNS TABLE(is_valid boolean, error_message character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN

    -- Branch code validation
    IF take_branch_code IS NULL OR
       TRIM(take_branch_code) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Branch code cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    IF LENGTH(TRIM(take_branch_code)) <> 3 THEN

        RETURN QUERY
        SELECT FALSE, 'Branch code must contain exactly 3 characters'::VARCHAR;
        RETURN;

    END IF;

    -- Branch name validation
    IF take_branch_name IS NULL OR
       TRIM(take_branch_name) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Branch name cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Duplicate branch code validation
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_branch
        WHERE branch_code = UPPER(TRIM(take_branch_code))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Branch code already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Duplicate branch name validation
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_branch
        WHERE LOWER(TRIM(branch_name)) =
              LOWER(TRIM(take_branch_name))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Branch name already exists'::VARCHAR;
        RETURN;

    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$function$;

-- ---- mst_region: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_region
(
    regn_code       VARCHAR(3) PRIMARY KEY,
    regn_name       VARCHAR(250) NOT NULL UNIQUE,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP,

    CONSTRAINT chk_mst_region_code_length
        CHECK (CHAR_LENGTH(regn_code) = 3)
);

-- ---- mst_region: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_region_insert
(
    IN take_regn_code VARCHAR(3),
    IN take_regn_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate region code
    IF take_regn_code IS NULL OR
       LENGTH(TRIM(take_regn_code)) <> 3 THEN
        RAISE EXCEPTION
        'Region code must contain exactly 3 characters';
    END IF;

    -- Validate region name
    IF take_regn_name IS NULL OR
       TRIM(take_regn_name) = '' THEN
        RAISE EXCEPTION 'Region name cannot be empty';
    END IF;

    -- Check duplicate region code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_region
        WHERE regn_code = UPPER(TRIM(take_regn_code))
    ) THEN
        RAISE EXCEPTION 'Region code already exists';
    END IF;

    -- Check duplicate region name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_region
        WHERE LOWER(TRIM(regn_name)) =
              LOWER(TRIM(take_regn_name))
    ) THEN
        RAISE EXCEPTION 'Region name already exists';
    END IF;

    -- Insert region
    INSERT INTO pms.mst_region
    (
        regn_code,
        regn_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_regn_code)),
        TRIM(take_regn_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_region: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.mst_region_validate
(
    take_regn_code VARCHAR(3),
    take_regn_name VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Region code validation
    IF take_regn_code IS NULL OR
       TRIM(take_regn_code) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Region code cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    IF LENGTH(TRIM(take_regn_code)) <> 3 THEN

        RETURN QUERY
        SELECT FALSE,
        'Region code must contain exactly 3 characters'::VARCHAR;
        RETURN;

    END IF;

    -- Region name validation
    IF take_regn_name IS NULL OR
       TRIM(take_regn_name) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Region name cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Duplicate region code validation
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_region
        WHERE regn_code = UPPER(TRIM(take_regn_code))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Region code already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Duplicate region name validation
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_region
        WHERE LOWER(TRIM(regn_name)) =
              LOWER(TRIM(take_regn_name))
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Region name already exists'::VARCHAR;
        RETURN;

    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_region: FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_mst_region_update_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_region: TRIGGER # 1
CREATE TRIGGER trg_mst_region_update_timestamp
BEFORE UPDATE ON pms.mst_region
FOR EACH ROW
EXECUTE FUNCTION pms.fn_mst_region_update_timestamp();

-- ---- mst_inst: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_inst
(
    inst_code           VARCHAR(9) PRIMARY KEY,
    inst_reg_code       VARCHAR(6) NOT NULL UNIQUE,
    inst_full_code      VARCHAR(14) NOT NULL UNIQUE,
    inst_name           VARCHAR(250),
    inst_add            VARCHAR(250),
    inst_place          VARCHAR(250),
    place_code          VARCHAR(3),
    place_name          VARCHAR(250),
    branch_code         VARCHAR(3),
    branch_name         VARCHAR(250),
    trust_code          VARCHAR(3),
    trust_name          VARCHAR(250),
    inst_sl_idx         INTEGER,
    trust_sl_idx        INTEGER,
    branch_sl_idx       INTEGER,
    place_sl_idx        INTEGER,

    reg_date            DATE,
    from_fin_yr         VARCHAR(9),

    inst_email          VARCHAR(150),
    inst_tel            VARCHAR(100),

    inst_head_name      VARCHAR(100),
    inst_head_desig     VARCHAR(100),
    inst_head_mobile    VARCHAR(10),
    inst_head_email     VARCHAR(150),

    inst_acnt_name      VARCHAR(100),
    inst_acnt_mobile   VARCHAR(10),
    inst_acnt_email     VARCHAR(150),

    inst_liason_name    VARCHAR(100),
    inst_liason_desig   VARCHAR(100),
    inst_liason_mobile  VARCHAR(10),
    inst_liason_email   VARCHAR(150),

    rec_created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at      TIMESTAMP,

    CONSTRAINT chk_inst_code_length
        CHECK (CHAR_LENGTH(inst_code) = 9),

    CONSTRAINT chk_inst_reg_code_length
        CHECK (CHAR_LENGTH(inst_reg_code) = 6),

    CONSTRAINT chk_inst_full_code_length
        CHECK (CHAR_LENGTH(inst_full_code) = 14),

    CONSTRAINT fk_mst_inst_place
        FOREIGN KEY (place_code)
        REFERENCES pms.mst_place(place_code),

    CONSTRAINT fk_mst_inst_branch
        FOREIGN KEY (branch_code)
        REFERENCES pms.mst_branch(branch_code),

    CONSTRAINT fk_mst_inst_trust
        FOREIGN KEY (trust_code)
        REFERENCES pms.mst_trust(trust_code)
);

-- ---- mst_inst: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.sp_mst_inst_insert
(
    IN take_inst_code          VARCHAR(9),
    IN take_inst_reg_code      VARCHAR(6),
    IN take_inst_full_code     VARCHAR(14),
    IN take_inst_name          VARCHAR(250),
    IN take_inst_add           VARCHAR(250),
    IN take_inst_place         VARCHAR(250),

    IN take_place_code         VARCHAR(3),
    IN take_branch_code        VARCHAR(3),
    IN take_trust_code         VARCHAR(3),

    IN take_inst_sl_idx        INTEGER,
    IN take_trust_sl_idx       INTEGER,
    IN take_branch_sl_idx      INTEGER,
    IN take_place_sl_idx       INTEGER,

    IN take_reg_date           DATE,
    IN take_from_fin_yr        VARCHAR(9),

    IN take_inst_email         VARCHAR(150),
    IN take_inst_tel           VARCHAR(100),

    IN take_inst_head_name     VARCHAR(100),
    IN take_inst_head_desig    VARCHAR(100),
    IN take_inst_head_mobile   VARCHAR(10),
    IN take_inst_head_email    VARCHAR(150),

    IN take_inst_acnt_name     VARCHAR(100),
    IN take_inst_acnt_mobile   VARCHAR(10),
    IN take_inst_acnt_email    VARCHAR(150),

    IN take_inst_liason_name   VARCHAR(100),
    IN take_inst_liason_desig  VARCHAR(100),
    IN take_inst_liason_mobile VARCHAR(10),
    IN take_inst_liason_email  VARCHAR(150)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_place_name  VARCHAR(250);
    v_branch_name VARCHAR(250);
    v_trust_name  VARCHAR(250);
BEGIN

    -- 1. Mandatory validation
    IF NULLIF(TRIM(take_inst_code), '') IS NULL
       OR LENGTH(TRIM(take_inst_code)) <> 9 THEN
        RAISE EXCEPTION
        'Institution code must contain exactly 9 characters';
    END IF;

    IF NULLIF(TRIM(take_inst_reg_code), '') IS NULL
       OR LENGTH(TRIM(take_inst_reg_code)) <> 6 THEN
        RAISE EXCEPTION
        'Institution registration code must contain exactly 6 characters';
    END IF;

    IF NULLIF(TRIM(take_inst_full_code), '') IS NULL
       OR LENGTH(TRIM(take_inst_full_code)) <> 14 THEN
        RAISE EXCEPTION
        'Full institution code must contain exactly 14 characters';
    END IF;

    -- 2. Duplicate institution code
    IF EXISTS (
        SELECT 1
        FROM pms.mst_inst
        WHERE inst_code = UPPER(TRIM(take_inst_code))
    ) THEN
        RAISE EXCEPTION 'Institution code already exists';
    END IF;

    -- 3. Duplicate registration code
    IF EXISTS (
        SELECT 1
        FROM pms.mst_inst
        WHERE inst_reg_code = UPPER(TRIM(take_inst_reg_code))
    ) THEN
        RAISE EXCEPTION 'Registration code already exists';
    END IF;

    -- 4. Duplicate full code
    IF EXISTS (
        SELECT 1
        FROM pms.mst_inst
        WHERE inst_full_code = UPPER(TRIM(take_inst_full_code))
    ) THEN
        RAISE EXCEPTION 'Full institution code already exists';
    END IF;

    -- 5. Retrieve place name
    IF take_place_code IS NOT NULL THEN
        SELECT place_name
        INTO v_place_name
        FROM pms.mst_place
        WHERE place_code = UPPER(TRIM(take_place_code));

        IF NOT FOUND THEN
            RAISE EXCEPTION 'Invalid place code: %', take_place_code;
        END IF;
    END IF;

    -- 6. Retrieve branch name
    IF take_branch_code IS NOT NULL THEN
        SELECT branch_name
        INTO v_branch_name
        FROM pms.mst_branch
        WHERE branch_code = UPPER(TRIM(take_branch_code));

        IF NOT FOUND THEN
            RAISE EXCEPTION 'Invalid branch code: %', take_branch_code;
        END IF;
    END IF;

    -- 7. Retrieve trust name
    IF take_trust_code IS NOT NULL THEN
        SELECT trust_name
        INTO v_trust_name
        FROM pms.mst_trust
        WHERE trust_code = UPPER(TRIM(take_trust_code));

        IF NOT FOUND THEN
            RAISE EXCEPTION 'Invalid trust code: %', take_trust_code;
        END IF;
    END IF;

    -- 8. Insert institution record
    INSERT INTO pms.mst_inst
    (
        inst_code,
        inst_reg_code,
        inst_full_code,
        inst_name,
        inst_add,
        inst_place,

        place_code,
        place_name,
        branch_code,
        branch_name,
        trust_code,
        trust_name,

        inst_sl_idx,
        trust_sl_idx,
        branch_sl_idx,
        place_sl_idx,

        reg_date,
        from_fin_yr,

        inst_email,
        inst_tel,

        inst_head_name,
        inst_head_desig,
        inst_head_mobile,
        inst_head_email,

        inst_acnt_name,
        inst_acnt_mobile,
        inst_acnt_email,

        inst_liason_name,
        inst_liason_desig,
        inst_liason_mobile,
        inst_liason_email,

        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_inst_code)),
        UPPER(TRIM(take_inst_reg_code)),
        UPPER(TRIM(take_inst_full_code)),
        TRIM(take_inst_name),
        TRIM(take_inst_add),
        TRIM(take_inst_place),

        UPPER(TRIM(take_place_code)),
        v_place_name,
        UPPER(TRIM(take_branch_code)),
        v_branch_name,
        UPPER(TRIM(take_trust_code)),
        v_trust_name,

        take_inst_sl_idx,
        take_trust_sl_idx,
        take_branch_sl_idx,
        take_place_sl_idx,

        take_reg_date,
        take_from_fin_yr,

        take_inst_email,
        take_inst_tel,

        take_inst_head_name,
        take_inst_head_desig,
        take_inst_head_mobile,
        take_inst_head_email,

        take_inst_acnt_name,
        take_inst_acnt_mobile,
        take_inst_acnt_email,

        take_inst_liason_name,
        take_inst_liason_desig,
        take_inst_liason_mobile,
        take_inst_liason_email,

        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_inst: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_mst_inst_validate
(
    take_inst_code       VARCHAR(9),
    take_inst_reg_code   VARCHAR(6),
    take_inst_full_code  VARCHAR(14),
    take_place_code      VARCHAR(3),
    take_branch_code     VARCHAR(3),
    take_trust_code      VARCHAR(3),
    take_inst_email      VARCHAR(150),
    take_inst_head_mobile VARCHAR(10),
    take_inst_head_email VARCHAR(150),
    take_inst_acnt_mobile VARCHAR(10),
    take_inst_acnt_email VARCHAR(150),
    take_inst_liason_mobile VARCHAR(10),
    take_inst_liason_email VARCHAR(150)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Institution code validation
    IF take_inst_code IS NULL OR
       TRIM(take_inst_code) = '' THEN
        RETURN QUERY SELECT FALSE, 'Institution code is mandatory'::VARCHAR;
        RETURN;
    END IF;

    IF LENGTH(TRIM(take_inst_code)) <> 9 THEN
        RETURN QUERY SELECT FALSE, 'Institution code must be exactly 9 characters'::VARCHAR;
        RETURN;
    END IF;

    -- 2. Registration code validation
    IF take_inst_reg_code IS NULL OR
       TRIM(take_inst_reg_code) = '' THEN
        RETURN QUERY SELECT FALSE, 'Registration code is mandatory'::VARCHAR;
        RETURN;
    END IF;

    IF LENGTH(TRIM(take_inst_reg_code)) <> 6 THEN
        RETURN QUERY SELECT FALSE, 'Registration code must be exactly 6 characters'::VARCHAR;
        RETURN;
    END IF;

    -- 3. Full code validation
    IF take_inst_full_code IS NULL OR
       TRIM(take_inst_full_code) = '' THEN
        RETURN QUERY SELECT FALSE, 'Full institution code is mandatory'::VARCHAR;
        RETURN;
    END IF;

    IF LENGTH(TRIM(take_inst_full_code)) <> 14 THEN
        RETURN QUERY SELECT FALSE, 'Full institution code must be exactly 14 characters'::VARCHAR;
        RETURN;
    END IF;

    -- 4. Duplicate institution code
    IF EXISTS (
        SELECT 1 FROM pms.mst_inst
        WHERE inst_code = UPPER(TRIM(take_inst_code))
    ) THEN
        RETURN QUERY SELECT FALSE, 'Institution code already exists'::VARCHAR;
        RETURN;
    END IF;

    -- 5. Duplicate registration code
    IF EXISTS (
        SELECT 1 FROM pms.mst_inst
        WHERE inst_reg_code = UPPER(TRIM(take_inst_reg_code))
    ) THEN
        RETURN QUERY SELECT FALSE, 'Registration code already exists'::VARCHAR;
        RETURN;
    END IF;

    -- 6. Duplicate full code
    IF EXISTS (
        SELECT 1 FROM pms.mst_inst
        WHERE inst_full_code = UPPER(TRIM(take_inst_full_code))
    ) THEN
        RETURN QUERY SELECT FALSE, 'Full institution code already exists'::VARCHAR;
        RETURN;
    END IF;

    -- 7. Place code validation
    IF take_place_code IS NOT NULL AND
       NOT EXISTS (
           SELECT 1 FROM pms.mst_place
           WHERE place_code = UPPER(TRIM(take_place_code))
       ) THEN
        RETURN QUERY SELECT FALSE, 'Invalid place code'::VARCHAR;
        RETURN;
    END IF;

    -- 8. Branch code validation
    IF take_branch_code IS NOT NULL AND
       NOT EXISTS (
           SELECT 1 FROM pms.mst_branch
           WHERE branch_code = UPPER(TRIM(take_branch_code))
       ) THEN
        RETURN QUERY SELECT FALSE, 'Invalid branch code'::VARCHAR;
        RETURN;
    END IF;

    -- 9. Trust code validation
    IF take_trust_code IS NOT NULL AND
       NOT EXISTS (
           SELECT 1 FROM pms.mst_trust
           WHERE trust_code = UPPER(TRIM(take_trust_code))
       ) THEN
        RETURN QUERY SELECT FALSE, 'Invalid trust code'::VARCHAR;
        RETURN;
    END IF;

    -- 10. Email validation
    IF take_inst_email IS NOT NULL
       AND TRIM(take_inst_email) <> ''
       AND take_inst_email !~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$' THEN
        RETURN QUERY SELECT FALSE, 'Invalid institution email address'::VARCHAR;
        RETURN;
    END IF;

    IF take_inst_head_email IS NOT NULL
       AND TRIM(take_inst_head_email) <> ''
       AND take_inst_head_email !~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$' THEN
        RETURN QUERY SELECT FALSE, 'Invalid institution head email address'::VARCHAR;
        RETURN;
    END IF;

    IF take_inst_acnt_email IS NOT NULL
       AND TRIM(take_inst_acnt_email) <> ''
       AND take_inst_acnt_email !~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$' THEN
        RETURN QUERY SELECT FALSE, 'Invalid accountant email address'::VARCHAR;
        RETURN;
    END IF;

    IF take_inst_liason_email IS NOT NULL
       AND TRIM(take_inst_liason_email) <> ''
       AND take_inst_liason_email !~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$' THEN
        RETURN QUERY SELECT FALSE, 'Invalid liaison email address'::VARCHAR;
        RETURN;
    END IF;

    -- 11. Mobile number validation
    IF take_inst_head_mobile IS NOT NULL
       AND TRIM(take_inst_head_mobile) <> ''
       AND TRIM(take_inst_head_mobile) !~ '^[0-9]{10}$' THEN
        RETURN QUERY SELECT FALSE, 'Institution head mobile must contain 10 digits'::VARCHAR;
        RETURN;
    END IF;

    IF take_inst_acnt_mobile IS NOT NULL
       AND TRIM(take_inst_acnt_mobile) <> ''
       AND TRIM(take_inst_acnt_mobile) !~ '^[0-9]{10}$' THEN
        RETURN QUERY SELECT FALSE, 'Accountant mobile must contain 10 digits'::VARCHAR;
        RETURN;
    END IF;

    IF take_inst_liason_mobile IS NOT NULL
       AND TRIM(take_inst_liason_mobile) <> ''
       AND TRIM(take_inst_liason_mobile) !~ '^[0-9]{10}$' THEN
        RETURN QUERY SELECT FALSE, 'Liaison mobile must contain 10 digits'::VARCHAR;
        RETURN;
    END IF;

    -- All validations passed
    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;
