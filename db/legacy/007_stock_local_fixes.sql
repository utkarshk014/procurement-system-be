-- LOCAL FIX, not the DBA's text (sent as FIX_fn_validate_mst_stk_items.sql). Two lines of his
-- 15 Nov evening fn_validate_mst_stk_items (006) changed; drop this file once he installs it.
--
-- 1. Category and manufacturer are optional, but "IF NOT EXISTS (... WHERE cat_cd = take_cat_cd)"
--    is true when take_cat_cd is NULL, so every item without a category or manufacturer was
--    refused ("Invalid stock category" / "Invalid manufacturer"). Now checked only when given.
-- 2. "take_item_cd IS NOT NULL AND NOT EXISTS" treated 0 as an existing item ("Item code not
--    found for update"); his other validate functions use COALESCE(id, 0) <> 0, so 0 and NULL
--    both mean a new item again.

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
