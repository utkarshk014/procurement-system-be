-- Stock / inventory masters, verbatim from the DBA's PROCEDURE_FUNCTION_1_6.xlsx (rows Sl 6-9, 15 Nov
-- 2026, installed on live): item group, category, manufacturer, unit of measurement (UOM), items and
-- the per-item stock row. Each list has insert, update (edit), delete and their validate functions.
-- Deleting an item goes through delete_mst_stk_items(item_cd, item_desc, item_code), which refuses an
-- item with stock or purchase information; a BEFORE DELETE trigger removes the item's stock row.
-- Only order changed: within a row, triggers come after the functions they execute (the sheet lists
-- the delete trigger before its function). Do not run this against the real database.

-- ---- mst_stk_group: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_group
(
    grp_cd          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_group      VARCHAR(250),
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP
);

-- ---- mst_stk_group: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_group
(
    take_item_group VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_item_group IS NULL OR TRIM(take_item_group) = '' THEN
        RAISE EXCEPTION 'Item group cannot be empty';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE LOWER(TRIM(item_group)) = LOWER(TRIM(take_item_group))
    ) THEN
        RAISE EXCEPTION 'Item group already exists';
    END IF;

    INSERT INTO pms.mst_stk_group
    (
        item_group,
        rec_created_at
    )
    VALUES
    (
        TRIM(take_item_group),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_stk_group: EDIT procedure
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_group
(
    take_grp_cd INTEGER,
    take_item_group VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate group code
    IF take_grp_cd IS NULL THEN
        RAISE EXCEPTION
            'Stock group code cannot be NULL';
    END IF;

    -- 2. Check group exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE grp_cd = take_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Stock group code % does not exist',
            take_grp_cd;
    END IF;

    -- 3. Validate group name
    IF NULLIF(BTRIM(take_item_group), '') IS NULL THEN
        RAISE EXCEPTION
            'Stock group name cannot be empty';
    END IF;

    -- 4. Check duplicate group name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE LOWER(BTRIM(item_group)) =
              LOWER(BTRIM(take_item_group))
          AND grp_cd <> take_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Stock group name already exists: %',
            take_item_group;
    END IF;

    -- 5. Update stock group
    UPDATE pms.mst_stk_group
    SET
        item_group = BTRIM(take_item_group),
        last_update_at = CURRENT_TIMESTAMP
    WHERE grp_cd = take_grp_cd;

END;
$$;

-- ---- mst_stk_group: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_group
(
    take_grp_cd INTEGER,
    take_item_group VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_valid BOOLEAN;
    v_message VARCHAR;
BEGIN

    SELECT f.is_valid, f.message
    INTO v_is_valid, v_message
    FROM pms.fn_validate_delete_mst_stk_group(
        take_grp_cd,
        take_item_group
    ) f;

    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%', v_message;
    END IF;

    DELETE FROM pms.mst_stk_group
    WHERE grp_cd = take_grp_cd
      AND LOWER(BTRIM(item_group)) =
          LOWER(BTRIM(take_item_group));

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Stock group not found or changed before deletion';
    END IF;

    RAISE NOTICE 'Stock group deleted successfully';

END;
$$;

-- ---- mst_stk_group: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_group
(
    take_grp_cd INT,
    take_item_group VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate item group
    IF take_item_group IS NULL
       OR TRIM(take_item_group) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Item group cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Check duplicate item group
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE LOWER(TRIM(item_group)) =
              LOWER(TRIM(take_item_group))
          AND grp_cd <> COALESCE(take_grp_cd, 0)
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Item group already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Check record availability for update
    IF COALESCE(take_grp_cd, 0) <> 0
       AND NOT EXISTS
       (
           SELECT 1
           FROM pms.mst_stk_group
           WHERE grp_cd = take_grp_cd
       ) THEN

        RETURN QUERY
        SELECT FALSE, 'Stock group record not found'::VARCHAR;
        RETURN;

    END IF;

    -- Validation successful
    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_stk_group: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_stk_group
(
    take_grp_cd INTEGER,
    take_item_group VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate group code
    IF take_grp_cd IS NULL THEN
        RETURN QUERY
        SELECT FALSE, 'Stock group code cannot be NULL'::VARCHAR;
        RETURN;
    END IF;

    -- Validate item group name
    IF NULLIF(BTRIM(take_item_group), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE, 'Item group cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Check group code and group name match
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group g
        WHERE g.grp_cd = take_grp_cd
          AND LOWER(BTRIM(g.item_group)) =
              LOWER(BTRIM(take_item_group))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Stock group code and item group do not match or do not exist'::VARCHAR;
        RETURN;
    END IF;

    -- Check group usage in stock items
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.grp_cd = take_grp_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Cannot delete. Stock items are linked to this group'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Stock group can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_stk_group: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_group_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_stk_group: TRIGGER # 1
CREATE TRIGGER trg_update_mst_stk_group_timestamp
BEFORE UPDATE ON pms.mst_stk_group
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_group_timestamp();

-- ---- mst_stk_category: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_category
(
    cat_cd          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category        VARCHAR(250),
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP
);

-- ---- mst_stk_category: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_category
(
    take_category VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_category IS NULL OR TRIM(take_category) = '' THEN
        RAISE EXCEPTION 'Category cannot be empty';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category
        WHERE LOWER(TRIM(category)) = LOWER(TRIM(take_category))
    ) THEN
        RAISE EXCEPTION 'Category already exists';
    END IF;

    INSERT INTO pms.mst_stk_category
    (
        category,
        rec_created_at
    )
    VALUES
    (
        TRIM(take_category),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_stk_category: EDIT procedure
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_category
(
    take_cat_cd INTEGER,
    take_category VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate category code
    IF take_cat_cd IS NULL THEN
        RAISE EXCEPTION
            'Stock category code cannot be NULL';
    END IF;

    -- 2. Check category exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category
        WHERE cat_cd = take_cat_cd
    ) THEN
        RAISE EXCEPTION
            'Stock category code % does not exist',
            take_cat_cd;
    END IF;

    -- 3. Validate category name
    IF NULLIF(BTRIM(take_category), '') IS NULL THEN
        RAISE EXCEPTION
            'Stock category name cannot be empty';
    END IF;

    -- 4. Check duplicate category name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category
        WHERE LOWER(BTRIM(category)) =
              LOWER(BTRIM(take_category))
          AND cat_cd <> take_cat_cd
    ) THEN
        RAISE EXCEPTION
            'Stock category name already exists: %',
            take_category;
    END IF;

    -- 5. Update stock category
    UPDATE pms.mst_stk_category
    SET
        category = BTRIM(take_category),
        last_update_at = CURRENT_TIMESTAMP
    WHERE cat_cd = take_cat_cd;

END;
$$;

-- ---- mst_stk_category: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_category
(
    take_cat_cd INTEGER,
    take_category VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_valid BOOLEAN;
    v_message VARCHAR;
BEGIN

    -- 1. Call validation function
    SELECT
        f.is_valid,
        f.message
    INTO
        v_is_valid,
        v_message
    FROM pms.fn_validate_delete_mst_stk_category
    (
        take_cat_cd,
        take_category
    ) f;

    -- 2. Stop if validation fails
    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%',
            COALESCE(v_message, 'Validation failed');
    END IF;

    -- 3. Delete category
    DELETE FROM pms.mst_stk_category
    WHERE cat_cd = take_cat_cd
      AND LOWER(BTRIM(category)) =
          LOWER(BTRIM(take_category));

    -- 4. Verify deletion
    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Stock category not found or changed before deletion';
    END IF;

    RAISE NOTICE
        'Stock category % deleted successfully',
        take_category;

END;
$$;

-- ---- mst_stk_category: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_category
(
    take_cat_cd INT,
    take_category VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate category
    IF take_category IS NULL OR TRIM(take_category) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Category cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Check duplicate category
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category
        WHERE LOWER(TRIM(category)) = LOWER(TRIM(take_category))
          AND cat_cd <> COALESCE(take_cat_cd, 0)
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Category already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Check record availability
    IF COALESCE(take_cat_cd, 0) <> 0
       AND NOT EXISTS
       (
           SELECT 1
           FROM pms.mst_stk_category
           WHERE cat_cd = take_cat_cd
       ) THEN

        RETURN QUERY
        SELECT FALSE, 'Stock category record not found'::VARCHAR;
        RETURN;

    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_stk_category: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_stk_category
(
    take_cat_cd INTEGER,
    take_category VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate category code
    IF take_cat_cd IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Stock category code cannot be NULL'::VARCHAR;
        RETURN;
    END IF;

    -- 2. Validate category name
    IF NULLIF(BTRIM(take_category), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Stock category name cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- 3. Check category code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category c
        WHERE c.cat_cd = take_cat_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Stock category code %s does not exist',
                   take_cat_cd
               )::VARCHAR;
        RETURN;
    END IF;

    -- 4. Check category code and name match
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_category c
        WHERE c.cat_cd = take_cat_cd
          AND LOWER(BTRIM(c.category)) =
              LOWER(BTRIM(take_category))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Stock category code and category name do not match'::VARCHAR;
        RETURN;
    END IF;

    -- 5. Check category usage in stock items
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.cat_cd = take_cat_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Cannot delete category %s. Stock items are linked to this category.',
                   take_category
               )::VARCHAR;
        RETURN;
    END IF;

    -- 6. Validation successful
    RETURN QUERY
    SELECT TRUE,
           'Stock category can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_stk_category: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_category_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_stk_category: TRIGGER # 1
CREATE TRIGGER trg_update_mst_stk_category_timestamp
BEFORE UPDATE ON pms.mst_stk_category
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_category_timestamp();

-- ---- mst_stk_manufacturer: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_manufacturer
(
    manf_cd         INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    manufacturer   VARCHAR(250),
    rec_created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at TIMESTAMP
);

-- ---- mst_stk_manufacturer: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_manufacturer
(
    take_manufacturer VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_manufacturer IS NULL
       OR TRIM(take_manufacturer) = '' THEN
        RAISE EXCEPTION 'Manufacturer cannot be empty';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer
        WHERE LOWER(TRIM(manufacturer)) =
              LOWER(TRIM(take_manufacturer))
    ) THEN
        RAISE EXCEPTION 'Manufacturer already exists';
    END IF;

    INSERT INTO pms.mst_stk_manufacturer
    (
        manufacturer,
        rec_created_at
    )
    VALUES
    (
        TRIM(take_manufacturer),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_stk_manufacturer: EDIT procedure
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_manufacturer
(
    take_manf_cd INTEGER,
    take_manufacturer VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate manufacturer code
    IF take_manf_cd IS NULL THEN
        RAISE EXCEPTION
            'Manufacturer code cannot be NULL';
    END IF;

    -- 2. Check manufacturer exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer
        WHERE manf_cd = take_manf_cd
    ) THEN
        RAISE EXCEPTION
            'Manufacturer code % does not exist',
            take_manf_cd;
    END IF;

    -- 3. Validate manufacturer name
    IF NULLIF(BTRIM(take_manufacturer), '') IS NULL THEN
        RAISE EXCEPTION
            'Manufacturer name cannot be empty';
    END IF;

    -- 4. Check duplicate manufacturer name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer
        WHERE LOWER(BTRIM(manufacturer)) =
              LOWER(BTRIM(take_manufacturer))
          AND manf_cd <> take_manf_cd
    ) THEN
        RAISE EXCEPTION
            'Manufacturer name already exists: %',
            take_manufacturer;
    END IF;

    -- 5. Update manufacturer
    UPDATE pms.mst_stk_manufacturer
    SET
        manufacturer = BTRIM(take_manufacturer),
        last_update_at = CURRENT_TIMESTAMP
    WHERE manf_cd = take_manf_cd;

END;
$$;

-- ---- mst_stk_manufacturer: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_manufacturer
(
    take_manf_cd INTEGER,
    take_manufacturer VARCHAR(250)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_valid BOOLEAN;
    v_message VARCHAR;
BEGIN

    -- 1. Call validation function
    SELECT
        f.is_valid,
        f.message
    INTO
        v_is_valid,
        v_message
    FROM pms.fn_validate_delete_mst_stk_manufacturer
    (
        take_manf_cd,
        take_manufacturer
    ) f;

    -- 2. Stop if validation fails
    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%',
            COALESCE(v_message, 'Validation failed');
    END IF;

    -- 3. Delete manufacturer
    DELETE FROM pms.mst_stk_manufacturer
    WHERE manf_cd = take_manf_cd
      AND LOWER(BTRIM(manufacturer)) =
          LOWER(BTRIM(take_manufacturer));

    -- 4. Verify deletion
    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Manufacturer not found or changed before deletion';
    END IF;

    RAISE NOTICE
        'Manufacturer % deleted successfully',
        take_manufacturer;

END;
$$;

-- ---- mst_stk_manufacturer: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_manufacturer
(
    take_manf_cd INT,
    take_manufacturer VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate manufacturer
    IF take_manufacturer IS NULL
       OR TRIM(take_manufacturer) = '' THEN

        RETURN QUERY
        SELECT FALSE, 'Manufacturer cannot be empty'::VARCHAR;
        RETURN;

    END IF;

    -- Check duplicate manufacturer
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer
        WHERE LOWER(TRIM(manufacturer)) =
              LOWER(TRIM(take_manufacturer))
          AND manf_cd <> COALESCE(take_manf_cd, 0)
    ) THEN

        RETURN QUERY
        SELECT FALSE, 'Manufacturer already exists'::VARCHAR;
        RETURN;

    END IF;

    -- Check record availability
    IF COALESCE(take_manf_cd, 0) <> 0
       AND NOT EXISTS
       (
           SELECT 1
           FROM pms.mst_stk_manufacturer
           WHERE manf_cd = take_manf_cd
       ) THEN

        RETURN QUERY
        SELECT FALSE, 'Manufacturer record not found'::VARCHAR;
        RETURN;

    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_stk_manufacturer: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_stk_manufacturer
(
    take_manf_cd INTEGER,
    take_manufacturer VARCHAR(250)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate manufacturer code
    IF take_manf_cd IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Manufacturer code cannot be NULL'::VARCHAR;
        RETURN;
    END IF;

    -- 2. Validate manufacturer name
    IF NULLIF(BTRIM(take_manufacturer), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Manufacturer name cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- 3. Check manufacturer code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer m
        WHERE m.manf_cd = take_manf_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Manufacturer code %s does not exist',
                   take_manf_cd
               )::VARCHAR;
        RETURN;
    END IF;

    -- 4. Check manufacturer code and name match
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_manufacturer m
        WHERE m.manf_cd = take_manf_cd
          AND LOWER(BTRIM(m.manufacturer)) =
              LOWER(BTRIM(take_manufacturer))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Manufacturer code and name do not match'::VARCHAR;
        RETURN;
    END IF;

    -- 5. Check manufacturer usage in stock items
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.manf_cd = take_manf_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Cannot delete manufacturer %s. Stock items are linked to this manufacturer.',
                   take_manufacturer
               )::VARCHAR;
        RETURN;
    END IF;

    -- 6. Validation successful
    RETURN QUERY
    SELECT TRUE,
           'Manufacturer can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_stk_manufacturer: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_manufacturer_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN

    NEW.last_update_at := CURRENT_TIMESTAMP;

    RETURN NEW;

END;
$$;

-- ---- mst_stk_manufacturer: TRIGGER # 1
CREATE TRIGGER trg_update_mst_stk_manufacturer_timestamp
BEFORE UPDATE ON pms.mst_stk_manufacturer
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_manufacturer_timestamp();

-- ---- mst_stk_uom: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_uom
(
    uom_cd          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uom             VARCHAR(5),
    uom_as_standard VARCHAR(50),
    enable_decimal  BOOLEAN DEFAULT FALSE,
    number_decimal  VARCHAR(1) DEFAULT '0',
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP
);

-- ---- mst_stk_uom: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_uom
(
    take_uom             VARCHAR,
    take_uom_as_standard VARCHAR,
    take_enable_decimal  BOOLEAN,
    take_number_decimal  VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF take_uom IS NULL OR TRIM(take_uom) = '' THEN
        RAISE EXCEPTION 'UOM cannot be empty';
    END IF;

    IF LENGTH(TRIM(take_uom)) > 5 THEN
        RAISE EXCEPTION 'UOM cannot exceed 5 characters';
    END IF;

    IF take_uom_as_standard IS NULL
       OR TRIM(take_uom_as_standard) = '' THEN
        RAISE EXCEPTION 'Standard UOM description cannot be empty';
    END IF;

    IF LENGTH(TRIM(take_uom_as_standard)) > 50 THEN
        RAISE EXCEPTION 'Standard UOM description cannot exceed 50 characters';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(TRIM(uom)) = LOWER(TRIM(take_uom))
    ) THEN
        RAISE EXCEPTION 'UOM already exists';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(TRIM(uom_as_standard)) =
              LOWER(TRIM(take_uom_as_standard))
    ) THEN
        RAISE EXCEPTION 'Standard UOM already exists';
    END IF;

    IF COALESCE(take_enable_decimal, FALSE) = FALSE THEN
        take_number_decimal := '0';
    ELSIF take_number_decimal IS NULL
       OR take_number_decimal !~ '^[0-9]$' THEN
        RAISE EXCEPTION 'Number decimal must be a single digit from 0 to 9';
    END IF;

    INSERT INTO pms.mst_stk_uom
    (
        uom,
        uom_as_standard,
        enable_decimal,
        number_decimal,
        rec_created_at
    )
    VALUES
    (
        UPPER(TRIM(take_uom)),
        TRIM(take_uom_as_standard),
        COALESCE(take_enable_decimal, FALSE),
        take_number_decimal,
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- mst_stk_uom: EDIT procedure
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_uom
(
    take_uom_cd          INTEGER,
    take_uom             VARCHAR(5),
    take_uom_as_standard VARCHAR(50),
    take_enable_decimal  BOOLEAN,
    take_number_decimal  VARCHAR(1)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate UOM code
    IF take_uom_cd IS NULL THEN
        RAISE EXCEPTION
            'UOM code cannot be NULL';
    END IF;

    -- 2. Check UOM record exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE uom_cd = take_uom_cd
    ) THEN
        RAISE EXCEPTION
            'UOM code % does not exist',
            take_uom_cd;
    END IF;

    -- 3. Validate UOM
    IF NULLIF(BTRIM(take_uom), '') IS NULL THEN
        RAISE EXCEPTION
            'UOM cannot be empty';
    END IF;

    -- 4. Validate standard UOM name
    IF NULLIF(BTRIM(take_uom_as_standard), '') IS NULL THEN
        RAISE EXCEPTION
            'Standard UOM name cannot be empty';
    END IF;

    -- 5. Check duplicate UOM
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(BTRIM(uom)) =
              LOWER(BTRIM(take_uom))
          AND uom_cd <> take_uom_cd
    ) THEN
        RAISE EXCEPTION
            'UOM already exists: %',
            take_uom;
    END IF;

    -- 6. Check duplicate standard UOM name
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(BTRIM(uom_as_standard)) =
              LOWER(BTRIM(take_uom_as_standard))
          AND uom_cd <> take_uom_cd
    ) THEN
        RAISE EXCEPTION
            'Standard UOM name already exists: %',
            take_uom_as_standard;
    END IF;

    -- 7. Validate decimal settings
    IF COALESCE(take_enable_decimal, FALSE) THEN

        IF take_number_decimal IS NULL
           OR take_number_decimal !~ '^[1-9]$'
        THEN
            RAISE EXCEPTION
                'Number of decimals must be between 1 and 9';
        END IF;

    ELSE

        -- Decimal disabled, force decimal places to zero
        take_number_decimal := '0';

    END IF;

    -- 8. Update UOM
    UPDATE pms.mst_stk_uom
    SET
        uom = UPPER(BTRIM(take_uom)),
        uom_as_standard = BTRIM(take_uom_as_standard),
        enable_decimal = COALESCE(take_enable_decimal, FALSE),
        number_decimal = take_number_decimal,
        last_update_at = CURRENT_TIMESTAMP
    WHERE uom_cd = take_uom_cd;

END;
$$;

-- ---- mst_stk_uom: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_uom
(
    take_uom_cd INTEGER,
    take_uom VARCHAR(5)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_valid BOOLEAN;
    v_message VARCHAR;
BEGIN

    -- 1. Call validation function
    SELECT
        f.is_valid,
        f.message
    INTO
        v_is_valid,
        v_message
    FROM pms.fn_validate_delete_mst_stk_uom
    (
        take_uom_cd,
        take_uom
    ) f;

    -- 2. Stop if validation fails
    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%',
            COALESCE(v_message, 'Validation failed');
    END IF;

    -- 3. Delete UOM
    DELETE FROM pms.mst_stk_uom
    WHERE uom_cd = take_uom_cd
      AND LOWER(BTRIM(uom)) =
          LOWER(BTRIM(take_uom));

    -- 4. Verify deletion
    IF NOT FOUND THEN
        RAISE EXCEPTION
            'UOM not found or changed before deletion';
    END IF;

    RAISE NOTICE
        'UOM % deleted successfully',
        take_uom;

END;
$$;

-- ---- mst_stk_uom: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_uom
(
    take_uom_cd          INT,
    take_uom             VARCHAR,
    take_uom_as_standard VARCHAR,
    take_enable_decimal  BOOLEAN,
    take_number_decimal  VARCHAR
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate UOM
    IF take_uom IS NULL OR TRIM(take_uom) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'UOM cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF LENGTH(TRIM(take_uom)) > 5 THEN
        RETURN QUERY
        SELECT FALSE, 'UOM cannot exceed 5 characters'::VARCHAR;
        RETURN;
    END IF;

    -- Validate standard UOM
    IF take_uom_as_standard IS NULL
       OR TRIM(take_uom_as_standard) = '' THEN
        RETURN QUERY
        SELECT FALSE, 'Standard UOM cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    IF LENGTH(TRIM(take_uom_as_standard)) > 50 THEN
        RETURN QUERY
        SELECT FALSE, 'Standard UOM cannot exceed 50 characters'::VARCHAR;
        RETURN;
    END IF;

    -- Check duplicate UOM
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(TRIM(uom)) = LOWER(TRIM(take_uom))
          AND uom_cd <> COALESCE(take_uom_cd, 0)
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'UOM already exists'::VARCHAR;
        RETURN;
    END IF;

    -- Check duplicate standard UOM
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom
        WHERE LOWER(TRIM(uom_as_standard)) =
              LOWER(TRIM(take_uom_as_standard))
          AND uom_cd <> COALESCE(take_uom_cd, 0)
    ) THEN
        RETURN QUERY
        SELECT FALSE, 'Standard UOM already exists'::VARCHAR;
        RETURN;
    END IF;

    -- Validate decimal precision
    IF COALESCE(take_enable_decimal, FALSE) = TRUE
       AND
       (
           take_number_decimal IS NULL
           OR take_number_decimal !~ '^[0-9]$'
       ) THEN
        RETURN QUERY
        SELECT FALSE,
        'Number decimal must be a single digit from 0 to 9'::VARCHAR;
        RETURN;
    END IF;

    -- Check record availability
    IF COALESCE(take_uom_cd, 0) <> 0
       AND NOT EXISTS
       (
           SELECT 1
           FROM pms.mst_stk_uom
           WHERE uom_cd = take_uom_cd
       ) THEN
        RETURN QUERY
        SELECT FALSE, 'UOM record not found'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_stk_uom: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_stk_uom
(
    take_uom_cd INTEGER,
    take_uom VARCHAR(5)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate UOM code
    IF take_uom_cd IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'UOM code cannot be NULL'::VARCHAR;
        RETURN;
    END IF;

    -- 2. Validate UOM name
    IF NULLIF(BTRIM(take_uom), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'UOM cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- 3. Check UOM code exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom u
        WHERE u.uom_cd = take_uom_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'UOM code %s does not exist',
                   take_uom_cd
               )::VARCHAR;
        RETURN;
    END IF;

    -- 4. Check UOM code and name match
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_uom u
        WHERE u.uom_cd = take_uom_cd
          AND LOWER(BTRIM(u.uom)) =
              LOWER(BTRIM(take_uom))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'UOM code and UOM name do not match'::VARCHAR;
        RETURN;
    END IF;

    -- 5. Check UOM usage in stock items
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.uom_cd = take_uom_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Cannot delete UOM %s. Stock items are linked to this UOM.',
                   take_uom
               )::VARCHAR;
        RETURN;
    END IF;

    -- 6. Validation successful
    RETURN QUERY
    SELECT TRUE,
           'UOM can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_stk_uom: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_uom_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN

    NEW.last_update_at := CURRENT_TIMESTAMP;

    RETURN NEW;

END;
$$;

-- ---- mst_stk_uom: TRIGGER # 1
CREATE OR REPLACE TRIGGER trg_update_mst_stk_uom_timestamp
BEFORE UPDATE ON pms.mst_stk_uom
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_uom_timestamp();

-- ---- mst_stk_items: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_items
(
    item_cd             INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_desc           VARCHAR(250),
    item_code           VARCHAR(20),

    grp_cd              INTEGER,
    cat_cd              INTEGER,
    manf_cd             INTEGER,
    uom_cd              INTEGER,

    goods_service_item  VARCHAR(10) DEFAULT 'Goods',
    item_consumable     BOOLEAN DEFAULT FALSE,
    item_is_spare       BOOLEAN DEFAULT FALSE,

    gst_applicable      BOOLEAN DEFAULT TRUE,
    gst_hsn_code        VARCHAR(8),
    gst_percentage      NUMERIC(5,2),
    sgst_percentage     NUMERIC(5,2),
    cgst_percentage     NUMERIC(5,2),

    track_serial        BOOLEAN DEFAULT FALSE,

    rec_created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at      TIMESTAMP,

    CONSTRAINT fk_stk_items_group
        FOREIGN KEY (grp_cd)
        REFERENCES pms.mst_stk_group(grp_cd),

    CONSTRAINT fk_stk_items_category
        FOREIGN KEY (cat_cd)
        REFERENCES pms.mst_stk_category(cat_cd),

    CONSTRAINT fk_stk_items_manufacturer
        FOREIGN KEY (manf_cd)
        REFERENCES pms.mst_stk_manufacturer(manf_cd),

    CONSTRAINT fk_stk_items_uom
        FOREIGN KEY (uom_cd)
        REFERENCES pms.mst_stk_uom(uom_cd),

    CONSTRAINT chk_goods_service_item
        CHECK (goods_service_item IN ('Goods', 'Service')),

    CONSTRAINT chk_gst_percentage
        CHECK (
            gst_percentage IS NULL
            OR gst_percentage BETWEEN 0 AND 100
        ),

    CONSTRAINT chk_sgst_percentage
        CHECK (
            sgst_percentage IS NULL
            OR sgst_percentage BETWEEN 0 AND 100
        ),

    CONSTRAINT chk_cgst_percentage
        CHECK (
            cgst_percentage IS NULL
            OR cgst_percentage BETWEEN 0 AND 100
        )
);

-- ---- mst_stk_items: PROCEDURE #1
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_items
(
    take_item_desc          VARCHAR,
    take_item_code          VARCHAR,
    take_grp_cd             INTEGER,
    take_cat_cd             INTEGER,
    take_manf_cd            INTEGER,
    take_uom_cd             INTEGER,
    take_goods_service_item VARCHAR,
    take_item_consumable    BOOLEAN,
    take_item_is_spare      BOOLEAN,
    take_gst_applicable     BOOLEAN,
    take_gst_hsn_code       VARCHAR,
    take_gst_percentage     NUMERIC(5,2),
    take_sgst_percentage    NUMERIC(5,2),
    take_cgst_percentage    NUMERIC(5,2),
    take_track_serial       BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN

-- Validate item code
IF NULLIF(BTRIM(take_item_code), '') IS NULL THEN
    RAISE EXCEPTION
        'Item code cannot be empty';
END IF;

-- Check duplicate item code (case-insensitive)
IF EXISTS
(
    SELECT 1
    FROM pms.mst_stk_items
    WHERE LOWER(BTRIM(item_code)) =
          LOWER(BTRIM(take_item_code))
) THEN
    RAISE EXCEPTION
        'Item code already exists: %',
        take_item_code;
END IF;


    IF take_item_desc IS NULL OR TRIM(take_item_desc) = '' THEN
        RAISE EXCEPTION 'Item description cannot be empty';
    END IF;


-- Validate item description
IF NULLIF(BTRIM(take_item_desc), '') IS NULL THEN
    RAISE EXCEPTION
        'Item description cannot be empty';
END IF;

-- Check duplicate item description
IF EXISTS
(
    SELECT 1
    FROM pms.mst_stk_items
    WHERE LOWER(BTRIM(item_desc)) =
          LOWER(BTRIM(take_item_desc))
) THEN
    RAISE EXCEPTION
        'Item description already exists: %',
        take_item_desc;
END IF;



    IF take_goods_service_item NOT IN ('Goods', 'Service') THEN
        RAISE EXCEPTION 'Invalid item type. Use Goods or Service';
    END IF;

    IF take_goods_service_item = 'Service'
       AND (COALESCE(take_item_consumable, FALSE)
            OR COALESCE(take_item_is_spare, FALSE)
            OR COALESCE(take_track_serial, FALSE)) THEN
        RAISE EXCEPTION 'Consumable, spare and serial tracking options are not applicable to services';
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = FALSE
       AND (
           COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0
       ) THEN
        RAISE EXCEPTION 'GST percentages must be zero or NULL when GST is not applicable';
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = TRUE
       AND (
           take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL
       ) THEN
        RAISE EXCEPTION 'GST percentages must be provided when GST is applicable';
    END IF;

    IF take_gst_applicable = TRUE
       AND take_gst_percentage <>
           (take_sgst_percentage + take_cgst_percentage) THEN
        RAISE EXCEPTION 'GST percentage must equal SGST plus CGST';
    END IF;

    INSERT INTO pms.mst_stk_items
    (
        item_desc,
        item_code,
        grp_cd,
        cat_cd,
        manf_cd,
        uom_cd,
        goods_service_item,
        item_consumable,
        item_is_spare,
        gst_applicable,
        gst_hsn_code,
        gst_percentage,
        sgst_percentage,
        cgst_percentage,
        track_serial
    )
    VALUES
    (
        TRIM(take_item_desc),
        NULLIF(TRIM(take_item_code), ''),
        take_grp_cd,
        take_cat_cd,
        take_manf_cd,
        take_uom_cd,
        take_goods_service_item,
        COALESCE(take_item_consumable, FALSE),
        COALESCE(take_item_is_spare, FALSE),
        COALESCE(take_gst_applicable, TRUE),
        NULLIF(TRIM(take_gst_hsn_code), ''),
        take_gst_percentage,
        take_sgst_percentage,
        take_cgst_percentage,
        COALESCE(take_track_serial, FALSE)
    );

END;
$$;
CREATE OR REPLACE PROCEDURE pms.insert_mst_stk_items
(
    take_item_desc          VARCHAR,
    take_item_code          VARCHAR,
    take_grp_cd             INTEGER,
    take_cat_cd             INTEGER,
    take_manf_cd            INTEGER,
    take_uom_cd             INTEGER,
    take_goods_service_item VARCHAR,
    take_item_consumable    BOOLEAN,
    take_item_is_spare      BOOLEAN,
    take_gst_applicable     BOOLEAN,
    take_gst_hsn_code       VARCHAR,
    take_gst_percentage     NUMERIC(5,2),
    take_sgst_percentage    NUMERIC(5,2),
    take_cgst_percentage    NUMERIC(5,2),
    take_track_serial       BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN

-- Validate item code
IF NULLIF(BTRIM(take_item_code), '') IS NULL THEN
    RAISE EXCEPTION
        'Item code cannot be empty';
END IF;

-- Check duplicate item code (case-insensitive)
IF EXISTS
(
    SELECT 1
    FROM pms.mst_stk_items
    WHERE LOWER(BTRIM(item_code)) =
          LOWER(BTRIM(take_item_code))
) THEN
    RAISE EXCEPTION
        'Item code already exists: %',
        take_item_code;
END IF;


    IF take_item_desc IS NULL OR TRIM(take_item_desc) = '' THEN
        RAISE EXCEPTION 'Item description cannot be empty';
    END IF;


-- Validate item description
IF NULLIF(BTRIM(take_item_desc), '') IS NULL THEN
    RAISE EXCEPTION
        'Item description cannot be empty';
END IF;

-- Check duplicate item description
IF EXISTS
(
    SELECT 1
    FROM pms.mst_stk_items
    WHERE LOWER(BTRIM(item_desc)) =
          LOWER(BTRIM(take_item_desc))
) THEN
    RAISE EXCEPTION
        'Item description already exists: %',
        take_item_desc;
END IF;



    IF take_goods_service_item NOT IN ('Goods', 'Service') THEN
        RAISE EXCEPTION 'Invalid item type. Use Goods or Service';
    END IF;

    IF take_goods_service_item = 'Service'
       AND (COALESCE(take_item_consumable, FALSE)
            OR COALESCE(take_item_is_spare, FALSE)
            OR COALESCE(take_track_serial, FALSE)) THEN
        RAISE EXCEPTION 'Consumable, spare and serial tracking options are not applicable to services';
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = FALSE
       AND (
           COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0
       ) THEN
        RAISE EXCEPTION 'GST percentages must be zero or NULL when GST is not applicable';
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = TRUE
       AND (
           take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL
       ) THEN
        RAISE EXCEPTION 'GST percentages must be provided when GST is applicable';
    END IF;

    IF take_gst_applicable = TRUE
       AND take_gst_percentage <>
           (take_sgst_percentage + take_cgst_percentage) THEN
        RAISE EXCEPTION 'GST percentage must equal SGST plus CGST';
    END IF;

    INSERT INTO pms.mst_stk_items
    (
        item_desc,
        item_code,
        grp_cd,
        cat_cd,
        manf_cd,
        uom_cd,
        goods_service_item,
        item_consumable,
        item_is_spare,
        gst_applicable,
        gst_hsn_code,
        gst_percentage,
        sgst_percentage,
        cgst_percentage,
        track_serial
    )
    VALUES
    (
        TRIM(take_item_desc),
        NULLIF(TRIM(take_item_code), ''),
        take_grp_cd,
        take_cat_cd,
        take_manf_cd,
        take_uom_cd,
        take_goods_service_item,
        COALESCE(take_item_consumable, FALSE),
        COALESCE(take_item_is_spare, FALSE),
        COALESCE(take_gst_applicable, TRUE),
        NULLIF(TRIM(take_gst_hsn_code), ''),
        take_gst_percentage,
        take_sgst_percentage,
        take_cgst_percentage,
        COALESCE(take_track_serial, FALSE)
    );

END;
$$;

-- ---- mst_stk_items: EDIT procedure
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_items
(
    take_item_cd           INTEGER,
    take_item_desc         VARCHAR(250),
    take_item_code         VARCHAR(20),
    take_grp_cd            INTEGER,
    take_cat_cd            INTEGER,
    take_manf_cd           INTEGER,
    take_uom_cd            INTEGER,
    take_goods_service_item VARCHAR(10),
    take_item_consumable   BOOLEAN,
    take_item_is_spare     BOOLEAN,
    take_gst_applicable    BOOLEAN,
    take_gst_hsn_code      VARCHAR(8),
    take_gst_percentage    NUMERIC(5,2),
    take_sgst_percentage   NUMERIC(5,2),
    take_cgst_percentage   NUMERIC(5,2),
    take_track_serial      BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Check item exists
    IF NOT EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE item_cd = take_item_cd
    ) THEN
        RAISE EXCEPTION
            'Item code % does not exist', take_item_cd;
    END IF;

    -- 2. Validate item description
    IF NULLIF(BTRIM(take_item_desc), '') IS NULL THEN
        RAISE EXCEPTION
            'Item description cannot be empty';
    END IF;

    -- 3. Check duplicate item description
    IF EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE LOWER(BTRIM(item_desc)) =
              LOWER(BTRIM(take_item_desc))
          AND item_cd <> take_item_cd
    ) THEN
        RAISE EXCEPTION
            'Item description already exists: %',
            take_item_desc;
    END IF;

    -- 4. Check duplicate item code
    IF NULLIF(BTRIM(take_item_code), '') IS NOT NULL
       AND EXISTS (
           SELECT 1
           FROM pms.mst_stk_items
           WHERE LOWER(BTRIM(item_code)) =
                 LOWER(BTRIM(take_item_code))
             AND item_cd <> take_item_cd
       )
    THEN
        RAISE EXCEPTION
            'Item code already exists: %',
            take_item_code;
    END IF;

    -- 5. Validate stock group
    IF take_grp_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_group
           WHERE grp_cd = take_grp_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid stock group code: %', take_grp_cd;
    END IF;

    -- 6. Validate category
    IF take_cat_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_category
           WHERE cat_cd = take_cat_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid category code: %', take_cat_cd;
    END IF;

    -- 7. Validate manufacturer
    IF take_manf_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_manufacturer
           WHERE manf_cd = take_manf_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid manufacturer code: %', take_manf_cd;
    END IF;

    -- 8. Validate UOM
    IF take_uom_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_uom
           WHERE uom_cd = take_uom_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid UOM code: %', take_uom_cd;
    END IF;

    -- 9. Validate Goods / Service
    IF take_goods_service_item IS NULL
       OR take_goods_service_item NOT IN ('Goods', 'Service')
    THEN
        RAISE EXCEPTION
            'Goods/Service must be Goods or Service';
    END IF;

    -- 10. Validate service item properties
    IF take_goods_service_item = 'Service'
       AND (
           COALESCE(take_item_consumable, FALSE)
           OR COALESCE(take_item_is_spare, FALSE)
           OR COALESCE(take_track_serial, FALSE)
       )
    THEN
        RAISE EXCEPTION
            'Service items cannot be consumable, spare or serial tracked';
    END IF;

    -- 11. Validate GST
    IF COALESCE(take_gst_applicable, FALSE) THEN

        IF take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL
        THEN
            RAISE EXCEPTION
                'GST, SGST and CGST percentages are required';
        END IF;

        IF take_gst_percentage < 0
           OR take_gst_percentage > 100
           OR take_sgst_percentage < 0
           OR take_sgst_percentage > 100
           OR take_cgst_percentage < 0
           OR take_cgst_percentage > 100
        THEN
            RAISE EXCEPTION
                'GST percentages must be between 0 and 100';
        END IF;

        IF take_gst_percentage <>
           take_sgst_percentage + take_cgst_percentage
        THEN
            RAISE EXCEPTION
                'GST percentage must equal SGST + CGST';
        END IF;

    ELSE

        IF COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0
        THEN
            RAISE EXCEPTION
                'GST percentages must be zero when GST is not applicable';
        END IF;

    END IF;

    -- 12. Update item master
    UPDATE pms.mst_stk_items
    SET
        item_desc          = BTRIM(take_item_desc),
        item_code          = NULLIF(BTRIM(take_item_code), ''),
        grp_cd             = take_grp_cd,
        cat_cd             = take_cat_cd,
        manf_cd            = take_manf_cd,
        uom_cd             = take_uom_cd,
        goods_service_item = take_goods_service_item,
        item_consumable    = COALESCE(take_item_consumable, FALSE),
        item_is_spare      = COALESCE(take_item_is_spare, FALSE),
        gst_applicable     = COALESCE(take_gst_applicable, FALSE),
        gst_hsn_code       = take_gst_hsn_code,
        gst_percentage     = COALESCE(take_gst_percentage, 0),
        sgst_percentage    = COALESCE(take_sgst_percentage, 0),
        cgst_percentage    = COALESCE(take_cgst_percentage, 0),
        track_serial       = COALESCE(take_track_serial, FALSE),
        last_update_at     = CURRENT_TIMESTAMP
    WHERE item_cd = take_item_cd;

END;
$$;
CREATE OR REPLACE PROCEDURE pms.update_mst_stk_items
(
    take_item_cd           INTEGER,
    take_item_desc         VARCHAR(250),
    take_item_code         VARCHAR(20),
    take_grp_cd            INTEGER,
    take_cat_cd            INTEGER,
    take_manf_cd           INTEGER,
    take_uom_cd            INTEGER,
    take_goods_service_item VARCHAR(10),
    take_item_consumable   BOOLEAN,
    take_item_is_spare     BOOLEAN,
    take_gst_applicable    BOOLEAN,
    take_gst_hsn_code      VARCHAR(8),
    take_gst_percentage    NUMERIC(5,2),
    take_sgst_percentage   NUMERIC(5,2),
    take_cgst_percentage   NUMERIC(5,2),
    take_track_serial      BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Check item exists
    IF NOT EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE item_cd = take_item_cd
    ) THEN
        RAISE EXCEPTION
            'Item code % does not exist', take_item_cd;
    END IF;

    -- 2. Validate item description
    IF NULLIF(BTRIM(take_item_desc), '') IS NULL THEN
        RAISE EXCEPTION
            'Item description cannot be empty';
    END IF;

    -- 3. Check duplicate item description
    IF EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE LOWER(BTRIM(item_desc)) =
              LOWER(BTRIM(take_item_desc))
          AND item_cd <> take_item_cd
    ) THEN
        RAISE EXCEPTION
            'Item description already exists: %',
            take_item_desc;
    END IF;

    -- 4. Check duplicate item code
    IF NULLIF(BTRIM(take_item_code), '') IS NOT NULL
       AND EXISTS (
           SELECT 1
           FROM pms.mst_stk_items
           WHERE LOWER(BTRIM(item_code)) =
                 LOWER(BTRIM(take_item_code))
             AND item_cd <> take_item_cd
       )
    THEN
        RAISE EXCEPTION
            'Item code already exists: %',
            take_item_code;
    END IF;

    -- 5. Validate stock group
    IF take_grp_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_group
           WHERE grp_cd = take_grp_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid stock group code: %', take_grp_cd;
    END IF;

    -- 6. Validate category
    IF take_cat_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_category
           WHERE cat_cd = take_cat_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid category code: %', take_cat_cd;
    END IF;

    -- 7. Validate manufacturer
    IF take_manf_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_manufacturer
           WHERE manf_cd = take_manf_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid manufacturer code: %', take_manf_cd;
    END IF;

    -- 8. Validate UOM
    IF take_uom_cd IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pms.mst_stk_uom
           WHERE uom_cd = take_uom_cd
       )
    THEN
        RAISE EXCEPTION
            'Invalid UOM code: %', take_uom_cd;
    END IF;

    -- 9. Validate Goods / Service
    IF take_goods_service_item IS NULL
       OR take_goods_service_item NOT IN ('Goods', 'Service')
    THEN
        RAISE EXCEPTION
            'Goods/Service must be Goods or Service';
    END IF;

    -- 10. Validate service item properties
    IF take_goods_service_item = 'Service'
       AND (
           COALESCE(take_item_consumable, FALSE)
           OR COALESCE(take_item_is_spare, FALSE)
           OR COALESCE(take_track_serial, FALSE)
       )
    THEN
        RAISE EXCEPTION
            'Service items cannot be consumable, spare or serial tracked';
    END IF;

    -- 11. Validate GST
    IF COALESCE(take_gst_applicable, FALSE) THEN

        IF take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL
        THEN
            RAISE EXCEPTION
                'GST, SGST and CGST percentages are required';
        END IF;

        IF take_gst_percentage < 0
           OR take_gst_percentage > 100
           OR take_sgst_percentage < 0
           OR take_sgst_percentage > 100
           OR take_cgst_percentage < 0
           OR take_cgst_percentage > 100
        THEN
            RAISE EXCEPTION
                'GST percentages must be between 0 and 100';
        END IF;

        IF take_gst_percentage <>
           take_sgst_percentage + take_cgst_percentage
        THEN
            RAISE EXCEPTION
                'GST percentage must equal SGST + CGST';
        END IF;

    ELSE

        IF COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0
        THEN
            RAISE EXCEPTION
                'GST percentages must be zero when GST is not applicable';
        END IF;

    END IF;

    -- 12. Update item master
    UPDATE pms.mst_stk_items
    SET
        item_desc          = BTRIM(take_item_desc),
        item_code          = NULLIF(BTRIM(take_item_code), ''),
        grp_cd             = take_grp_cd,
        cat_cd             = take_cat_cd,
        manf_cd            = take_manf_cd,
        uom_cd             = take_uom_cd,
        goods_service_item = take_goods_service_item,
        item_consumable    = COALESCE(take_item_consumable, FALSE),
        item_is_spare      = COALESCE(take_item_is_spare, FALSE),
        gst_applicable     = COALESCE(take_gst_applicable, FALSE),
        gst_hsn_code       = take_gst_hsn_code,
        gst_percentage     = COALESCE(take_gst_percentage, 0),
        sgst_percentage    = COALESCE(take_sgst_percentage, 0),
        cgst_percentage    = COALESCE(take_cgst_percentage, 0),
        track_serial       = COALESCE(take_track_serial, FALSE),
        last_update_at     = CURRENT_TIMESTAMP
    WHERE item_cd = take_item_cd;

END;
$$;

-- ---- mst_stk_items: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_items
(
    take_item_cd INTEGER,
    take_item_desc VARCHAR(250),
    take_item_code VARCHAR(20)
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_valid BOOLEAN;
    v_message VARCHAR;
BEGIN

    -- Lock item record during validation and deletion
    PERFORM 1
    FROM pms.mst_stk_items
    WHERE item_cd = take_item_cd
    FOR UPDATE;

    -- 1. Call validation function
    SELECT
        f.is_valid,
        f.message
    INTO
        v_is_valid,
        v_message
    FROM pms.fn_validate_delete_mst_stk_items
    (
        take_item_cd,
        take_item_desc,
        take_item_code
    ) f;

    -- 2. Stop if validation fails
    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%',
            COALESCE(v_message, 'Validation failed');
    END IF;

    -- Lock transaction summary to prevent concurrent updates
    PERFORM 1
    FROM pms.mst_stk_items_transactions
    WHERE item_cd = take_item_cd
    FOR UPDATE;

    -- Revalidate after acquiring the transaction lock
    SELECT
        f.is_valid,
        f.message
    INTO
        v_is_valid,
        v_message
    FROM pms.fn_validate_delete_mst_stk_items
    (
        take_item_cd,
        take_item_desc,
        take_item_code
    ) f;

    IF NOT COALESCE(v_is_valid, FALSE) THEN
        RAISE EXCEPTION '%',
            COALESCE(v_message, 'Validation failed');
    END IF;

    -- 3. Delete transaction summary
    DELETE FROM pms.mst_stk_items_transactions
    WHERE item_cd = take_item_cd;

    -- 4. Delete stock item
    DELETE FROM pms.mst_stk_items
    WHERE item_cd = take_item_cd
      AND LOWER(BTRIM(item_desc)) =
          LOWER(BTRIM(take_item_desc))
      AND LOWER(BTRIM(item_code)) =
          LOWER(BTRIM(take_item_code));

    -- 5. Verify deletion
    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Stock item not found or changed before deletion';
    END IF;

    RAISE NOTICE
        'Stock item % deleted successfully',
        take_item_cd;

END;
$$;

-- ---- mst_stk_items: FUNCTION #1 (his 15 Nov evening text with duplicate checks; category and
-- manufacturer checked only when given, and 0 means a new item, as installed on live 15 Nov)
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_items(
    take_item_cd INTEGER,
    take_item_desc VARCHAR,
    take_item_code VARCHAR,
    take_grp_cd INTEGER,
    take_cat_cd INTEGER,
    take_manf_cd INTEGER,
    take_uom_cd INTEGER,
    take_goods_service_item VARCHAR,
    take_item_consumable BOOLEAN,
    take_item_is_spare BOOLEAN,
    take_gst_applicable BOOLEAN,
    take_gst_hsn_code VARCHAR,
    take_gst_percentage NUMERIC(5,2),
    take_sgst_percentage NUMERIC(5,2),
    take_cgst_percentage NUMERIC(5,2),
    take_track_serial BOOLEAN
)
RETURNS TABLE (
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate item description
    IF NULLIF(TRIM(take_item_desc), '') IS NULL THEN
        RETURN QUERY SELECT FALSE,
            'Item description is required'::VARCHAR;
        RETURN;
    END IF;

    -- Duplicate item description
    IF EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE LOWER(TRIM(item_desc)) =
              LOWER(TRIM(take_item_desc))
          AND (take_item_cd IS NULL
               OR item_cd <> take_item_cd)
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Item description already exists'::VARCHAR;
        RETURN;
    END IF;

    -- Duplicate item code
    IF NULLIF(TRIM(take_item_code), '') IS NOT NULL
       AND EXISTS (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE LOWER(TRIM(item_code)) =
              LOWER(TRIM(take_item_code))
          AND (take_item_cd IS NULL
               OR item_cd <> take_item_cd)
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Item code already exists'::VARCHAR;
        RETURN;
    END IF;

    -- Validate Goods / Service
    IF take_goods_service_item IS NULL
       OR take_goods_service_item NOT IN
          ('Goods', 'Service') THEN
        RETURN QUERY SELECT FALSE,
            'Item type must be Goods or Service'::VARCHAR;
        RETURN;
    END IF;

    -- Validate group
    IF NOT EXISTS (
        SELECT 1 FROM pms.mst_stk_group
        WHERE grp_cd = take_grp_cd
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Invalid stock group'::VARCHAR;
        RETURN;
    END IF;

    -- Validate category (optional: only when given)
    IF take_cat_cd IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pms.mst_stk_category
        WHERE cat_cd = take_cat_cd
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Invalid stock category'::VARCHAR;
        RETURN;
    END IF;

    -- Validate manufacturer (optional: only when given)
    IF take_manf_cd IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pms.mst_stk_manufacturer
        WHERE manf_cd = take_manf_cd
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Invalid manufacturer'::VARCHAR;
        RETURN;
    END IF;

    -- Validate UOM
    IF NOT EXISTS (
        SELECT 1 FROM pms.mst_stk_uom
        WHERE uom_cd = take_uom_cd
    ) THEN
        RETURN QUERY SELECT FALSE,
            'Invalid unit of measurement'::VARCHAR;
        RETURN;
    END IF;

    -- Service restrictions
    IF take_goods_service_item = 'Service'
       AND (
           COALESCE(take_item_consumable, FALSE)
           OR COALESCE(take_item_is_spare, FALSE)
           OR COALESCE(take_track_serial, FALSE)
       ) THEN
        RETURN QUERY SELECT FALSE,
            'Service cannot be consumable, spare or serial tracked'::VARCHAR;
        RETURN;
    END IF;

    -- GST validation
    IF COALESCE(take_gst_applicable, FALSE) = FALSE THEN
        IF COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0 THEN
            RETURN QUERY SELECT FALSE,
                'GST percentages must be zero when GST is not applicable'::VARCHAR;
            RETURN;
        END IF;
    ELSE
        IF take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL THEN
            RETURN QUERY SELECT FALSE,
                'GST percentages are required'::VARCHAR;
            RETURN;
        END IF;

        IF take_gst_percentage <>
           take_sgst_percentage + take_cgst_percentage THEN
            RETURN QUERY SELECT FALSE,
                'GST must equal SGST + CGST'::VARCHAR;
            RETURN;
        END IF;
    END IF;

    -- Validate item exists for update
    IF COALESCE(take_item_cd, 0) <> 0
       AND NOT EXISTS (
           SELECT 1 FROM pms.mst_stk_items
           WHERE item_cd = take_item_cd
       ) THEN
        RETURN QUERY SELECT FALSE,
            'Item code not found for update'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY SELECT TRUE,
        'Validation successful'::VARCHAR;

END;
$$;

-- ---- mst_stk_items: DELETE FUNCTION VALIDATE
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_stk_items
(
    take_item_cd INTEGER,
    take_item_desc VARCHAR(250),
    take_item_code VARCHAR(20)
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- 1. Validate item code
    IF take_item_cd IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Item code cannot be NULL'::VARCHAR;
        RETURN;
    END IF;

    -- 2. Validate item description
    IF NULLIF(BTRIM(take_item_desc), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Item description cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- 3. Validate item reference code
    IF NULLIF(BTRIM(take_item_code), '') IS NULL THEN
        RETURN QUERY
        SELECT FALSE,
               'Item reference code cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- 4. Check item exists
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.item_cd = take_item_cd
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               FORMAT(
                   'Stock item %s does not exist',
                   take_item_cd
               )::VARCHAR;
        RETURN;
    END IF;

    -- 5. Check item description and code match
    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items i
        WHERE i.item_cd = take_item_cd
          AND LOWER(BTRIM(i.item_desc)) =
              LOWER(BTRIM(take_item_desc))
          AND LOWER(BTRIM(i.item_code)) =
              LOWER(BTRIM(take_item_code))
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Item code, description and reference code do not match'::VARCHAR;
        RETURN;
    END IF;

    -- 6. Check stock balances and movements
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items_transactions t
        WHERE t.item_cd = take_item_cd
          AND
          (
              COALESCE(t.stk_ob, 0) <> 0
              OR COALESCE(t.stk_cr, 0) <> 0
              OR COALESCE(t.stk_db, 0) <> 0
              OR COALESCE(t.stk_cr_jv, 0) <> 0
              OR COALESCE(t.stk_db_jv, 0) <> 0
              OR COALESCE(t.stk_cb, 0) <> 0
          )
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Cannot delete item. Stock transactions or balances exist.'::VARCHAR;
        RETURN;
    END IF;

    -- 7. Check purchase information
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items_transactions t
        WHERE t.item_cd = take_item_cd
          AND
          (
              t.last_purchase_date IS NOT NULL
              OR NULLIF(BTRIM(t.last_supply_vendor_name), '') IS NOT NULL
              OR COALESCE(t.last_purchase_qty, 0) <> 0
              OR COALESCE(t.last_purchase_rate, 0) <> 0
              OR COALESCE(t.last_gst_rate, 0) <> 0
          )
    ) THEN
        RETURN QUERY
        SELECT FALSE,
               'Cannot delete item. Purchase information exists.'::VARCHAR;
        RETURN;
    END IF;

    -- 8. Validation successful
    RETURN QUERY
    SELECT TRUE,
           'Stock item can be deleted'::VARCHAR;

END;
$$;

-- ---- mst_stk_items: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_items_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- ---- mst_stk_items: DELETE TRIGGER FUNCTION
CREATE OR REPLACE FUNCTION pms.fn_delete_stk_items_transactions()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN

    -- Delete corresponding transaction record
    DELETE FROM pms.mst_stk_items_transactions
    WHERE item_cd = OLD.item_cd;

    RETURN OLD;

END;
$$;

-- ---- mst_stk_items: TRIGGER # 1
CREATE OR REPLACE TRIGGER trg_update_mst_stk_items_timestamp
BEFORE UPDATE ON pms.mst_stk_items
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_items_timestamp();

-- ---- mst_stk_items: DELETE TRIGGER
CREATE TRIGGER trg_delete_stk_items_transactions
BEFORE DELETE
ON pms.mst_stk_items
FOR EACH ROW
EXECUTE FUNCTION pms.fn_delete_stk_items_transactions();

-- ---- mst_stk_items_transactions: Create TABLE
CREATE TABLE IF NOT EXISTS pms.mst_stk_items_transactions
(
    item_cd                      INTEGER PRIMARY KEY,

    last_purchase_date           DATE,
    last_supply_vendor_name      VARCHAR(250),
    last_purchase_rate           NUMERIC(10,2) DEFAULT 0,
    last_purchased_for_inst_code VARCHAR(9),
    last_purchase_qty            NUMERIC(15,2) DEFAULT 0,
    last_gst_rate                NUMERIC(15,2) DEFAULT 0,

    stk_ob                       NUMERIC(15,2) DEFAULT 0,
    stk_cr                       NUMERIC(15,2) DEFAULT 0,
    stk_db                       NUMERIC(15,2) DEFAULT 0,
    stk_cr_jv                    NUMERIC(15,2) DEFAULT 0,
    stk_db_jv                    NUMERIC(15,2) DEFAULT 0,
    stk_cb                       NUMERIC(15,2) DEFAULT 0,

    rec_created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at               TIMESTAMP,

    CONSTRAINT fk_stk_items_transactions_item
        FOREIGN KEY (item_cd)
        REFERENCES pms.mst_stk_items(item_cd)
);

-- ---- mst_stk_items_transactions: ALTER FUNCTION #2
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_items_transactions_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.last_update_at := CURRENT_TIMESTAMP;

    RETURN NEW;
END;
$$;

-- ---- mst_stk_items_transactions: trigger function
CREATE OR REPLACE FUNCTION pms.fn_insert_stk_items_transactions()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN

    INSERT INTO pms.mst_stk_items_transactions
    (
        item_cd,
        last_purchase_rate,
        last_purchase_qty,
        last_gst_rate,
        stk_ob,
        stk_cr,
        stk_db,
        stk_cr_jv,
        stk_db_jv,
        stk_cb
    )
    VALUES
    (
        NEW.item_cd,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0,
        0
    )
    ON CONFLICT (item_cd) DO NOTHING;

    RETURN NEW;

END;
$$;

-- ---- mst_stk_items_transactions: TRIGGER # 1
CREATE TRIGGER trg_insert_stk_items_transactions
AFTER INSERT
ON pms.mst_stk_items
FOR EACH ROW
EXECUTE FUNCTION pms.fn_insert_stk_items_transactions();

-- ---- mst_stk_items_transactions: EDIT TRIGGER # 2
DROP TRIGGER IF EXISTS
    trg_update_mst_stk_items_transactions_timestamp
ON pms.mst_stk_items_transactions;

CREATE TRIGGER trg_update_mst_stk_items_transactions_timestamp
BEFORE UPDATE
ON pms.mst_stk_items_transactions
FOR EACH ROW
EXECUTE FUNCTION
    pms.fn_update_mst_stk_items_transactions_timestamp();

