-- Stock / inventory masters, verbatim from the DBA's TABLE_PROCEDURE_FUNCTION_1_3.xlsx (rows Sl 6-9):
-- item group, category, manufacturer, unit of measurement (UOM), items and the per-item stock row
-- (mst_stk_items_transactions, created by a trigger when an item is inserted). Installed on live
-- (Postgres 16) by 14 Nov 2026. One change from the sheet, already made on live: the column
-- last_purchase_qyu is last_purchase_qty (the insert trigger writes last_purchase_qty, so with
-- the sheet's spelling every item insert failed). Do not run this against the real database.

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

    IF take_item_desc IS NULL OR TRIM(take_item_desc) = '' THEN
        RAISE EXCEPTION 'Item description cannot be empty';
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

-- ---- mst_stk_items: delete procedure
CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_items
(
    take_item_cd INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE item_cd = take_item_cd
    ) THEN
        RAISE EXCEPTION 'Stock item record not found';
    END IF;

    DELETE FROM pms.mst_stk_items
    WHERE item_cd = take_item_cd;

END;
$$;

-- ---- mst_stk_items: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_stk_items
(
    take_item_cd           INTEGER,
    take_item_desc         VARCHAR,
    take_item_code         VARCHAR,
    take_grp_cd            INTEGER,
    take_cat_cd            INTEGER,
    take_manf_cd           INTEGER,
    take_uom_cd            INTEGER,
    take_goods_service_item VARCHAR,
    take_item_consumable   BOOLEAN,
    take_item_is_spare     BOOLEAN,
    take_gst_applicable    BOOLEAN,
    take_gst_hsn_code      VARCHAR,
    take_gst_percentage    NUMERIC(5,2),
    take_sgst_percentage   NUMERIC(5,2),
    take_cgst_percentage   NUMERIC(5,2),
    take_track_serial      BOOLEAN
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate item description
    IF take_item_desc IS NULL OR TRIM(take_item_desc) = '' THEN
        RETURN QUERY SELECT FALSE, 'Item description cannot be empty'::VARCHAR;
        RETURN;
    END IF;

    -- Validate item type
    IF take_goods_service_item IS NULL
       OR take_goods_service_item NOT IN ('Goods', 'Service') THEN
        RETURN QUERY SELECT FALSE, 'Item type must be Goods or Service'::VARCHAR;
        RETURN;
    END IF;

    -- Validate group
    IF take_grp_cd IS NOT NULL AND NOT EXISTS
    (
        SELECT 1 FROM pms.mst_stk_group
        WHERE grp_cd = take_grp_cd
    ) THEN
        RETURN QUERY SELECT FALSE, 'Stock group not found'::VARCHAR;
        RETURN;
    END IF;

    -- Validate category
    IF take_cat_cd IS NOT NULL AND NOT EXISTS
    (
        SELECT 1 FROM pms.mst_stk_category
        WHERE cat_cd = take_cat_cd
    ) THEN
        RETURN QUERY SELECT FALSE, 'Stock category not found'::VARCHAR;
        RETURN;
    END IF;

    -- Validate manufacturer
    IF take_manf_cd IS NOT NULL AND NOT EXISTS
    (
        SELECT 1 FROM pms.mst_stk_manufacturer
        WHERE manf_cd = take_manf_cd
    ) THEN
        RETURN QUERY SELECT FALSE, 'Manufacturer not found'::VARCHAR;
        RETURN;
    END IF;

    -- Validate UOM
    IF take_uom_cd IS NOT NULL AND NOT EXISTS
    (
        SELECT 1 FROM pms.mst_stk_uom
        WHERE uom_cd = take_uom_cd
    ) THEN
        RETURN QUERY SELECT FALSE, 'Unit of measurement not found'::VARCHAR;
        RETURN;
    END IF;

    -- Validate service item options
    IF take_goods_service_item = 'Service'
       AND (
           COALESCE(take_item_consumable, FALSE)
           OR COALESCE(take_item_is_spare, FALSE)
           OR COALESCE(take_track_serial, FALSE)
       ) THEN
        RETURN QUERY
        SELECT FALSE, 'Consumable, spare and serial tracking are not applicable to services'::VARCHAR;
        RETURN;
    END IF;

    -- Validate GST
    IF COALESCE(take_gst_applicable, TRUE) = FALSE
       AND (
           COALESCE(take_gst_percentage, 0) <> 0
           OR COALESCE(take_sgst_percentage, 0) <> 0
           OR COALESCE(take_cgst_percentage, 0) <> 0
       ) THEN
        RETURN QUERY
        SELECT FALSE, 'GST percentages must be zero or NULL when GST is not applicable'::VARCHAR;
        RETURN;
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = TRUE
       AND (
           take_gst_percentage IS NULL
           OR take_sgst_percentage IS NULL
           OR take_cgst_percentage IS NULL
       ) THEN
        RETURN QUERY
        SELECT FALSE, 'GST percentages must be provided'::VARCHAR;
        RETURN;
    END IF;

    IF COALESCE(take_gst_applicable, TRUE) = TRUE
       AND take_gst_percentage <>
           (take_sgst_percentage + take_cgst_percentage) THEN
        RETURN QUERY
        SELECT FALSE, 'GST must equal SGST plus CGST'::VARCHAR;
        RETURN;
    END IF;

    -- Check record availability for update
    IF COALESCE(take_item_cd, 0) <> 0
       AND NOT EXISTS
       (
           SELECT 1 FROM pms.mst_stk_items
           WHERE item_cd = take_item_cd
       ) THEN
        RETURN QUERY SELECT FALSE, 'Stock item record not found'::VARCHAR;
        RETURN;
    END IF;

    RETURN QUERY
    SELECT TRUE, 'Validation successful'::VARCHAR;

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

-- ---- mst_stk_items: TRIGGER # 1
CREATE OR REPLACE TRIGGER trg_update_mst_stk_items_timestamp
BEFORE UPDATE ON pms.mst_stk_items
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_items_timestamp();

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

-- ---- mst_stk_items_transactions: FUNCTION #1
CREATE OR REPLACE FUNCTION pms.fn_insert_mst_stk_items_transactions()
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
        stk_cb,
        rec_created_at
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
        0,
        CURRENT_TIMESTAMP
    );

    RETURN NEW;

END;
$$;

-- ---- mst_stk_items_transactions: trigger function
CREATE OR REPLACE FUNCTION pms.fn_update_mst_stk_items_transactions_timestamp()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN

    NEW.last_update_at := CURRENT_TIMESTAMP;

    RETURN NEW;

END;
$$;

-- ---- mst_stk_items_transactions: TRIGGER # 1
CREATE  TRIGGER trg_update_mst_stk_items_transactions_timestamp
BEFORE UPDATE ON pms.mst_stk_items_transactions
FOR EACH ROW
EXECUTE FUNCTION pms.fn_update_mst_stk_items_transactions_timestamp();

-- ---- mst_stk_items_transactions: TRIGGER # 2
CREATE TRIGGER trg_insert_mst_stk_items_transactions
AFTER INSERT ON pms.mst_stk_items
FOR EACH ROW
EXECUTE FUNCTION pms.fn_insert_mst_stk_items_transactions();

