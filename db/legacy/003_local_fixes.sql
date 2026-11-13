-- LOCAL FIXES, not the DBA's text. Postgres 11 runs these after 002 so local
-- development and the e2e test can create institutions in the real code format.
-- Remove each fix once the DBA installs the same change on live (open items in
-- docs/ARCHITECTURE.md). The same text is what we sent him to install.
--
-- 1. sp_mst_inst_insert reads trust_name from pms.mst_trust; the column is trust_desc,
--    so every insert with a trust fails: column "trust_name" does not exist.
-- 2. inst_code must be 9 characters, but the client's codes are 7 (ACU-009) and the
--    full code is inst_code ':' inst_reg_code = 7 + 1 + 6 = 14. With 9 no full code
--    can ever be 14, so nothing could be saved. Changed to 7 in the table check, the
--    procedure and the validate function.
--    The DBA installed this validate function on live on 12 Nov 2026 (identical text);
--    the table check and the procedure below are still local fixes until he confirms them.

ALTER TABLE pms.mst_inst DROP CONSTRAINT chk_inst_code_length;
ALTER TABLE pms.mst_inst ADD CONSTRAINT chk_inst_code_length CHECK (CHAR_LENGTH(inst_code) = 7);

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
       OR LENGTH(TRIM(take_inst_code)) <> 7 THEN
        RAISE EXCEPTION
        'Institution code must contain exactly 7 characters';
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
        SELECT trust_desc
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

    IF LENGTH(TRIM(take_inst_code)) <> 7 THEN
        RETURN QUERY SELECT FALSE, 'Institution code must be exactly 7 characters'::VARCHAR;
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
