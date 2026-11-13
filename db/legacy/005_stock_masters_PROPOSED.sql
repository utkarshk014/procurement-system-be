-- PROPOSED stock / inventory masters: Item Group (Stock Group), Category, Manufacturer.
-- Written by us from the old desktop screens Uncle sent on 12 Nov 2026 (Group, Category,
-- Manufacturer: one name per row, Add / Edit / Save). NOT installed on live; Uncle reviews,
-- changes if needed and installs. Same shape as mst_place (3-letter code + unique name) so the
-- API reuses the code-list module. Two deliberate differences from mst_place:
--   * the code must be 3 capital letters (CHECK), the rule the API already applies to every list;
--   * the name is unique ignoring case and outer spaces (unique index on LOWER(TRIM(name))), so
--     'SNACK' and 'Snack' cannot both be saved even by two saves at the same moment.
-- No items table exists yet, so the delete checks only test that the row exists. Once an
-- item master references these codes, each validate_delete must refuse a code in use.

-- ======================================================================
-- ---- mst_item_group: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_item_group
(
    grp_code        VARCHAR(3) PRIMARY KEY,
    grp_name        VARCHAR(250) NOT NULL,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP,

    CONSTRAINT chk_mst_item_group_code
        CHECK (grp_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mst_item_group_name
    ON pms.mst_item_group (LOWER(TRIM(grp_name)));

-- ---- mst_item_group: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_item_group_insert
(
    IN take_grp_code VARCHAR(3),
    IN take_grp_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate item group code
    IF take_grp_code IS NULL OR
       UPPER(TRIM(take_grp_code)) !~ '^[A-Z]{3}$' THEN
        RAISE EXCEPTION 'Item group code must contain exactly 3 letters';
    END IF;

    -- Validate item group name
    IF take_grp_name IS NULL OR
       TRIM(take_grp_name) = '' THEN
        RAISE EXCEPTION 'Item group name cannot be empty';
    END IF;

    -- Check duplicate item group code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_group
        WHERE grp_code = UPPER(TRIM(take_grp_code))
    ) THEN
        RAISE EXCEPTION 'Item group code % already exists',
            UPPER(TRIM(take_grp_code));
    END IF;

    -- Check duplicate item group name (case insensitive)
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_group
        WHERE LOWER(TRIM(grp_name)) =
              LOWER(TRIM(take_grp_name))
    ) THEN
        RAISE EXCEPTION 'Item group name % already exists',
            TRIM(take_grp_name);
    END IF;

    -- Insert record
    INSERT INTO pms.mst_item_group
    (
        grp_code,
        grp_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_grp_code)),
        TRIM(take_grp_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_item_group: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.mst_item_group_validate
(
    take_grp_code VARCHAR(3),
    take_grp_name VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_grp_code IS NULL OR
       TRIM(take_grp_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Item group code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF UPPER(TRIM(take_grp_code)) !~ '^[A-Z]{3}$' THEN
        RETURN QUERY
        SELECT FALSE, 'Item group code must contain exactly 3 letters'::VARCHAR;
        RETURN;
    END IF;

    IF take_grp_name IS NULL OR
       TRIM(take_grp_name) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Item group name cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_group
        WHERE grp_code = UPPER(TRIM(take_grp_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Item group code already exists'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_group
        WHERE LOWER(TRIM(grp_name)) =
              LOWER(TRIM(take_grp_name))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Item group name already exists'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_item_group: FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_mst_item_group_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_item_group: TRIGGER # 1
DROP TRIGGER IF EXISTS trg_mst_item_group_update ON pms.mst_item_group;
CREATE TRIGGER trg_mst_item_group_update
BEFORE UPDATE ON pms.mst_item_group
FOR EACH ROW
EXECUTE FUNCTION pms.fn_mst_item_group_update();

-- ---- mst_item_group: delete procedure
CREATE OR REPLACE PROCEDURE pms.mst_item_group_delete
(
    take_grp_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_grp_code IS NULL OR TRIM(take_grp_code) = '' THEN
        RAISE EXCEPTION 'Item group code cannot be empty';
    END IF;

    DELETE FROM pms.mst_item_group
    WHERE grp_code = UPPER(TRIM(take_grp_code));

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Item group code % does not exist',
            take_grp_code;
    END IF;

END;
$$;

-- ---- mst_item_group: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.mst_item_group_validate_delete
(
    take_grp_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_grp_code IS NULL OR TRIM(take_grp_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Item group code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_item_group
        WHERE grp_code = UPPER(TRIM(take_grp_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Item group code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Add here: refuse a code used by the item master, once that table exists.

    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Item group record can be deleted'::VARCHAR;

END;
$$;

-- ======================================================================
-- ---- mst_item_category: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_item_category
(
    cat_code        VARCHAR(3) PRIMARY KEY,
    cat_name        VARCHAR(250) NOT NULL,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP,

    CONSTRAINT chk_mst_item_category_code
        CHECK (cat_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mst_item_category_name
    ON pms.mst_item_category (LOWER(TRIM(cat_name)));

-- ---- mst_item_category: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_item_category_insert
(
    IN take_cat_code VARCHAR(3),
    IN take_cat_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate category code
    IF take_cat_code IS NULL OR
       UPPER(TRIM(take_cat_code)) !~ '^[A-Z]{3}$' THEN
        RAISE EXCEPTION 'Category code must contain exactly 3 letters';
    END IF;

    -- Validate category name
    IF take_cat_name IS NULL OR
       TRIM(take_cat_name) = '' THEN
        RAISE EXCEPTION 'Category name cannot be empty';
    END IF;

    -- Check duplicate category code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_category
        WHERE cat_code = UPPER(TRIM(take_cat_code))
    ) THEN
        RAISE EXCEPTION 'Category code % already exists',
            UPPER(TRIM(take_cat_code));
    END IF;

    -- Check duplicate category name (case insensitive)
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_category
        WHERE LOWER(TRIM(cat_name)) =
              LOWER(TRIM(take_cat_name))
    ) THEN
        RAISE EXCEPTION 'Category name % already exists',
            TRIM(take_cat_name);
    END IF;

    -- Insert record
    INSERT INTO pms.mst_item_category
    (
        cat_code,
        cat_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_cat_code)),
        TRIM(take_cat_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_item_category: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.mst_item_category_validate
(
    take_cat_code VARCHAR(3),
    take_cat_name VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_cat_code IS NULL OR
       TRIM(take_cat_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Category code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF UPPER(TRIM(take_cat_code)) !~ '^[A-Z]{3}$' THEN
        RETURN QUERY
        SELECT FALSE, 'Category code must contain exactly 3 letters'::VARCHAR;
        RETURN;
    END IF;

    IF take_cat_name IS NULL OR
       TRIM(take_cat_name) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Category name cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_category
        WHERE cat_code = UPPER(TRIM(take_cat_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Category code already exists'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_item_category
        WHERE LOWER(TRIM(cat_name)) =
              LOWER(TRIM(take_cat_name))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Category name already exists'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_item_category: FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_mst_item_category_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_item_category: TRIGGER # 1
DROP TRIGGER IF EXISTS trg_mst_item_category_update ON pms.mst_item_category;
CREATE TRIGGER trg_mst_item_category_update
BEFORE UPDATE ON pms.mst_item_category
FOR EACH ROW
EXECUTE FUNCTION pms.fn_mst_item_category_update();

-- ---- mst_item_category: delete procedure
CREATE OR REPLACE PROCEDURE pms.mst_item_category_delete
(
    take_cat_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_cat_code IS NULL OR TRIM(take_cat_code) = '' THEN
        RAISE EXCEPTION 'Category code cannot be empty';
    END IF;

    DELETE FROM pms.mst_item_category
    WHERE cat_code = UPPER(TRIM(take_cat_code));

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Category code % does not exist',
            take_cat_code;
    END IF;

END;
$$;

-- ---- mst_item_category: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.mst_item_category_validate_delete
(
    take_cat_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_cat_code IS NULL OR TRIM(take_cat_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Category code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_item_category
        WHERE cat_code = UPPER(TRIM(take_cat_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Category code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Add here: refuse a code used by the item master, once that table exists.

    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Category record can be deleted'::VARCHAR;

END;
$$;

-- ======================================================================
-- ---- mst_manufacturer: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_manufacturer
(
    mfr_code        VARCHAR(3) PRIMARY KEY,
    mfr_name        VARCHAR(250) NOT NULL,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP,

    CONSTRAINT chk_mst_manufacturer_code
        CHECK (mfr_code ~ '^[A-Z]{3}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mst_manufacturer_name
    ON pms.mst_manufacturer (LOWER(TRIM(mfr_name)));

-- ---- mst_manufacturer: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.mst_manufacturer_insert
(
    IN take_mfr_code VARCHAR(3),
    IN take_mfr_name VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate manufacturer code
    IF take_mfr_code IS NULL OR
       UPPER(TRIM(take_mfr_code)) !~ '^[A-Z]{3}$' THEN
        RAISE EXCEPTION 'Manufacturer code must contain exactly 3 letters';
    END IF;

    -- Validate manufacturer name
    IF take_mfr_name IS NULL OR
       TRIM(take_mfr_name) = '' THEN
        RAISE EXCEPTION 'Manufacturer name cannot be empty';
    END IF;

    -- Check duplicate manufacturer code
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_manufacturer
        WHERE mfr_code = UPPER(TRIM(take_mfr_code))
    ) THEN
        RAISE EXCEPTION 'Manufacturer code % already exists',
            UPPER(TRIM(take_mfr_code));
    END IF;

    -- Check duplicate manufacturer name (case insensitive)
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_manufacturer
        WHERE LOWER(TRIM(mfr_name)) =
              LOWER(TRIM(take_mfr_name))
    ) THEN
        RAISE EXCEPTION 'Manufacturer name % already exists',
            TRIM(take_mfr_name);
    END IF;

    -- Insert record
    INSERT INTO pms.mst_manufacturer
    (
        mfr_code,
        mfr_name,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_mfr_code)),
        TRIM(take_mfr_name),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_manufacturer: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.mst_manufacturer_validate
(
    take_mfr_code VARCHAR(3),
    take_mfr_name VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    error_message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_mfr_code IS NULL OR
       TRIM(take_mfr_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF UPPER(TRIM(take_mfr_code)) !~ '^[A-Z]{3}$' THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer code must contain exactly 3 letters'::VARCHAR;
        RETURN;
    END IF;

    IF take_mfr_name IS NULL OR
       TRIM(take_mfr_name) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer name cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_manufacturer
        WHERE mfr_code = UPPER(TRIM(take_mfr_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer code already exists'::VARCHAR;
        RETURN;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_manufacturer
        WHERE LOWER(TRIM(mfr_name)) =
              LOWER(TRIM(take_mfr_name))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer name already exists'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_manufacturer: FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_mst_manufacturer_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_manufacturer: TRIGGER # 1
DROP TRIGGER IF EXISTS trg_mst_manufacturer_update ON pms.mst_manufacturer;
CREATE TRIGGER trg_mst_manufacturer_update
BEFORE UPDATE ON pms.mst_manufacturer
FOR EACH ROW
EXECUTE FUNCTION pms.fn_mst_manufacturer_update();

-- ---- mst_manufacturer: delete procedure
CREATE OR REPLACE PROCEDURE pms.mst_manufacturer_delete
(
    take_mfr_code VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_mfr_code IS NULL OR TRIM(take_mfr_code) = '' THEN
        RAISE EXCEPTION 'Manufacturer code cannot be empty';
    END IF;

    DELETE FROM pms.mst_manufacturer
    WHERE mfr_code = UPPER(TRIM(take_mfr_code));

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Manufacturer code % does not exist',
            take_mfr_code;
    END IF;

END;
$$;

-- ---- mst_manufacturer: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.mst_manufacturer_validate_delete
(
    take_mfr_code VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_mfr_code IS NULL OR TRIM(take_mfr_code) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_manufacturer
        WHERE mfr_code = UPPER(TRIM(take_mfr_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Manufacturer code does not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Add here: refuse a code used by the item master, once that table exists.

    RETURN QUERY
    SELECT TRUE,
    'Validation successful. Manufacturer record can be deleted'::VARCHAR;

END;
$$;
