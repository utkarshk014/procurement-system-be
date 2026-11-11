-- Delete procedures and their validate functions, from the DBA's TABLE_PROCEDURE_FUNCTION_1_2.xlsx
-- (11 Nov 2026). Seven are copied verbatim. Three did not work on Postgres 11 and are marked
-- LOCAL FIX below; the fixed text is what we sent the DBA to install on live
-- (delete_fixes.sql). Do not run this against the real database.

-- ---- finyr_master: delete procedure (verbatim)
CREATE OR REPLACE PROCEDURE pms.finyr_master_delete
(
    take_finyr VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    DELETE FROM pms.finyr_master
    WHERE finyr = take_finyr;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Financial year % does not exist', take_finyr;
    END IF;

END;
$$;

-- ---- finyr_master: DELETE FUNCTION VALIDATE
-- LOCAL FIX: the sheet reads company_master.finyr, which does not exist ("column finyr does
-- not exist" on every call). The company's year is fin_st_dt, a date. Also refuses the current
-- year and a year that a trust or an institution starts in.
CREATE OR REPLACE FUNCTION pms.finyr_master_validate_delete
(
    take_finyr VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check financial year is supplied
    IF take_finyr IS NULL OR TRIM(take_finyr) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Financial year cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check financial year exists in master
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.finyr_master
        WHERE finyr = take_finyr
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Financial year does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- The current year cannot be deleted
    IF EXISTS
    (
        SELECT 1
        FROM pms.finyr_master
        WHERE finyr = take_finyr
          AND finyr_is_current
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Financial year is the current year'::VARCHAR;
        RETURN;
    END IF;

    -- Check whether financial year is used in company master (fin_st_dt falls inside it)
    IF EXISTS
    (
        SELECT 1
        FROM pms.company_master c
        JOIN pms.finyr_master f ON f.finyr = take_finyr
        WHERE c.fin_st_dt BETWEEN f.finyr_st_date AND f.finyr_end_date
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Financial year is already used in Company Master'::VARCHAR;
        RETURN;
    END IF;

    -- Check whether a trust starts in this year
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_trust
        WHERE wrk_st_finyr = take_finyr
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Financial year is already used in Trust Master (mst_trust)'::VARCHAR;
        RETURN;
    END IF;

    -- Check whether an institution starts in this year
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_inst
        WHERE from_fin_yr = take_finyr
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Financial year is already used in Institution Master (mst_inst)'::VARCHAR;
        RETURN;
    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Record can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_trust: delete procedure (verbatim)
CREATE OR REPLACE PROCEDURE pms.mst_trust_delete
(
    take_trust_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check trust code
    IF take_trust_code IS NULL OR TRIM(take_trust_code) = '' THEN
        RAISE EXCEPTION 'Trust code cannot be empty';
    END IF;

    -- Delete record
    DELETE FROM pms.mst_trust
    WHERE trust_code = UPPER(TRIM(take_trust_code));

    -- Check whether record exists
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Trust code % does not exist',
            take_trust_code;
    END IF;

END;
$$;

-- ---- mst_trust: DELETE FUNCTION VALIDATE (verbatim)
CREATE OR REPLACE FUNCTION pms.mst_trust_validate_delete
(
    take_trust_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check empty trust code
    IF take_trust_code IS NULL OR TRIM(take_trust_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Trust code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check trust code format
    IF UPPER(TRIM(take_trust_code)) !~ '^[A-Z]{3}$' THEN
        RETURN QUERY
        SELECT FALSE,
        'Trust code must contain exactly 3 alphabetic characters'::VARCHAR;
        RETURN;
    END IF;

    -- Check trust code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_trust
        WHERE trust_code = UPPER(TRIM(take_trust_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Trust code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Check trust code is used in mst_inst
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_inst
        WHERE trust_code = UPPER(TRIM(take_trust_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Trust code is already used in Institution Master (mst_inst)'::VARCHAR;
        RETURN;
    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Trust record can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_place: delete procedure (verbatim)
CREATE OR REPLACE PROCEDURE pms.mst_place_delete
(
    take_place_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check place code
    IF take_place_code IS NULL OR TRIM(take_place_code) = '' THEN
        RAISE EXCEPTION 'Place code cannot be empty';
    END IF;

    -- Delete record
    DELETE FROM pms.mst_place
    WHERE place_code = UPPER(TRIM(take_place_code));

    -- Check whether record exists
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Place code % does not exist',
            take_place_code;
    END IF;

END;
$$;

-- ---- mst_place: DELETE FUNCTION VALIDATE
-- LOCAL FIX: the sheet has "IS NU+I6LL" (a spreadsheet typo for IS NULL), so the function
-- does not install at all.
CREATE OR REPLACE FUNCTION pms.mst_place_validate_delete
(
    take_place_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check empty place code
    IF take_place_code IS NULL OR TRIM(take_place_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Place code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check place code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_place
        WHERE place_code = UPPER(TRIM(take_place_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Place code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Check place code is used in institution master
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_inst
        WHERE place_code = UPPER(TRIM(take_place_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Place code is already used in Institution Master (mst_inst)'::VARCHAR;
        RETURN;
    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Place record can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_branch: delete procedure (verbatim)
CREATE OR REPLACE PROCEDURE pms.mst_branch_delete
(
    take_branch_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check branch code
    IF take_branch_code IS NULL OR TRIM(take_branch_code) = '' THEN
        RAISE EXCEPTION 'Branch code cannot be empty';
    END IF;

    -- Delete record
    DELETE FROM pms.mst_branch
    WHERE branch_code = UPPER(TRIM(take_branch_code));

    -- Check whether record exists
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Branch code % does not exist',
            take_branch_code;
    END IF;

END;
$$;

-- ---- mst_branch: DELETE FUNCTION VALIDATE (verbatim)
CREATE OR REPLACE FUNCTION pms.mst_branch_validate_delete
(
    take_branch_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check empty branch code
    IF take_branch_code IS NULL OR TRIM(take_branch_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Branch code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check branch code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_branch
        WHERE branch_code = UPPER(TRIM(take_branch_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Branch code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Check branch code is used in mst_inst
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_inst
        WHERE branch_code = UPPER(TRIM(take_branch_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Branch code is already used in Institution Master (mst_inst)'::VARCHAR;
        RETURN;
    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Branch record can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_region: delete procedure
-- LOCAL FIX: the sheet uses region_code; the column is regn_code ("column region_code does
-- not exist" on every call).
CREATE OR REPLACE PROCEDURE pms.mst_region_delete
(
    take_region_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check region code
    IF take_region_code IS NULL OR TRIM(take_region_code) = '' THEN
        RAISE EXCEPTION 'Region code cannot be empty';
    END IF;

    -- Delete record
    DELETE FROM pms.mst_region
    WHERE regn_code = UPPER(TRIM(take_region_code));

    -- Check whether record exists
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Region code % does not exist',
            take_region_code;
    END IF;

END;
$$;

-- ---- mst_region: DELETE FUNCTION VALIDATE
-- LOCAL FIX: regn_code, not region_code. mst_inst has no region column; a region is used by
-- an institution when its reg code starts with the region code (MYS-01).
CREATE OR REPLACE FUNCTION pms.mst_region_validate_delete
(
    take_region_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Check empty region code
    IF take_region_code IS NULL OR TRIM(take_region_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Region code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check region code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_region
        WHERE regn_code = UPPER(TRIM(take_region_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Region code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Check region code is used in mst_inst (as the start of a reg code)
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_inst
        WHERE inst_reg_code LIKE UPPER(TRIM(take_region_code)) || '-%'
    ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Cannot delete! Region code is already used in Institution Master (mst_inst)'::VARCHAR;
        RETURN;
    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Region record can be deleted'::VARCHAR;

END;
$$;
