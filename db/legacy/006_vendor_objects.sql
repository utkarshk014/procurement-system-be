-- Vendor masters, as installed on live by the DBA (15-21 Nov 2026, PROCEDURE_FUNCTION_1_7.xlsx sheet
-- VENDOR plus update_mst_vendor / delete_mst_vendor added 21 Nov): the vendor, its financials per
-- year, the stock groups it supplies, and its certificates (S3 object keys). Tables from pg_dump,
-- routines from pg_get_functiondef. His earlier drafts (sp_*_vendor_*, fn_validate_delete_vendor_
-- financials) are left out: nothing calls them. Do not run this against the real database.

-- Name: mst_vendor; Type: TABLE; Schema: pms; Owner: -

CREATE TABLE pms.mst_vendor (
    vendor_cd integer NOT NULL,
    name character varying(250),
    address character varying(250),
    city character varying(250),
    state character varying(250),
    country character varying(250),
    pin character varying(10),
    vendor_type character varying(50),
    mobile character varying(15),
    email character varying(150),
    website character varying(100),
    reg_date date,
    pan character varying(10),
    gstin character varying(15),
    gstin_state character varying(100),
    tan character varying(10),
    refered_by character varying(250),
    correspond_email character varying(150),
    correspond_mobile character varying(10),
    supply_type character varying(50),
    is_contruction_comp boolean,
    is_manufacturer boolean,
    company_since character varying(4),
    contact_1_person character varying(150),
    contact_1_designation character varying(150),
    contact_1_mobile character varying(10),
    contact_1_email character varying(150),
    contact_2_person character varying(150),
    contact_2_designation character varying(150),
    contact_2_mobile character varying(10),
    contact_2_email character varying(150),
    contact_3_person character varying(150),
    contact_3_designation character varying(150),
    contact_3_mobile character varying(10),
    contact_3_email character varying(150)
);

-- Name: mst_vendor_certificates; Type: TABLE; Schema: pms; Owner: -

CREATE TABLE pms.mst_vendor_certificates (
    vendor_cd integer NOT NULL,
    gst_uploaded boolean DEFAULT false NOT NULL,
    gst_s3_bucket_reference character varying,
    company_reg_uploaded boolean DEFAULT false NOT NULL,
    company_reg_s3_bucket_reference character varying,
    pan_uploaded boolean DEFAULT false NOT NULL,
    pan_s3_bucket_reference character varying,
    msme_uploaded boolean DEFAULT false NOT NULL,
    msme_s3_bucket_reference character varying,
    tan_uploaded boolean DEFAULT false NOT NULL,
    tan_s3_bucket_reference character varying,
    aadhar_uploaded boolean DEFAULT false NOT NULL,
    aadhar_s3_bucket_reference character varying,
    oth_1_uploaded boolean DEFAULT false NOT NULL,
    oth_1_caption character varying(100),
    oth_1_s3_bucket_reference character varying,
    oth_2_uploaded boolean DEFAULT false NOT NULL,
    oth_2_caption character varying(100),
    oth_2_s3_bucket_reference character varying,
    oth_3_uploaded boolean DEFAULT false NOT NULL,
    oth_3_caption character varying(100),
    oth_3_s3_bucket_reference character varying
);

-- Name: mst_vendor_financials; Type: TABLE; Schema: pms; Owner: -

CREATE TABLE pms.mst_vendor_financials (
    vendor_cd integer NOT NULL,
    fin_year character varying(9) NOT NULL,
    fin_turnover_declared numeric(15,2),
    fin_profit_declared numeric(15,2),
    fin_itr_no character varying(25),
    fin_itr_date date,
    fin_itr_s3_bucket_reference character varying
);

-- Name: mst_vendor_item_group; Type: TABLE; Schema: pms; Owner: -

CREATE TABLE pms.mst_vendor_item_group (
    vendor_cd integer NOT NULL,
    grp_cd integer NOT NULL
);

-- Name: mst_vendor_vendor_cd_seq; Type: SEQUENCE; Schema: pms; Owner: -

ALTER TABLE pms.mst_vendor ALTER COLUMN vendor_cd ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME pms.mst_vendor_vendor_cd_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

-- Name: mst_vendor_certificates mst_vendor_certificates_pkey; Type: CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_certificates
    ADD CONSTRAINT mst_vendor_certificates_pkey PRIMARY KEY (vendor_cd);

-- Name: mst_vendor mst_vendor_pkey; Type: CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor
    ADD CONSTRAINT mst_vendor_pkey PRIMARY KEY (vendor_cd);

-- Name: mst_vendor_financials pk_mst_vendor_financials; Type: CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_financials
    ADD CONSTRAINT pk_mst_vendor_financials PRIMARY KEY (vendor_cd, fin_year);

-- Name: mst_vendor_item_group pk_mst_vendor_item_group; Type: CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_item_group
    ADD CONSTRAINT pk_mst_vendor_item_group PRIMARY KEY (vendor_cd, grp_cd);

-- Name: idx_mst_vendor_item_group_grp_cd; Type: INDEX; Schema: pms; Owner: -

CREATE INDEX idx_mst_vendor_item_group_grp_cd ON pms.mst_vendor_item_group USING btree (grp_cd);

-- Name: uq_mst_vendor_gstin; Type: INDEX; Schema: pms; Owner: -

CREATE UNIQUE INDEX uq_mst_vendor_gstin ON pms.mst_vendor USING btree (upper(btrim((gstin)::text))) WHERE (NULLIF(btrim((gstin)::text), ''::text) IS NOT NULL);

-- Name: uq_mst_vendor_name; Type: INDEX; Schema: pms; Owner: -

CREATE UNIQUE INDEX uq_mst_vendor_name ON pms.mst_vendor USING btree (lower(btrim((name)::text)));

-- Name: uq_mst_vendor_pan; Type: INDEX; Schema: pms; Owner: -

CREATE UNIQUE INDEX uq_mst_vendor_pan ON pms.mst_vendor USING btree (upper(btrim((pan)::text))) WHERE (NULLIF(btrim((pan)::text), ''::text) IS NOT NULL);

-- Name: mst_vendor_certificates fk_mst_vendor_certificates_vendor; Type: FK CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_certificates
    ADD CONSTRAINT fk_mst_vendor_certificates_vendor FOREIGN KEY (vendor_cd) REFERENCES pms.mst_vendor(vendor_cd) ON UPDATE CASCADE ON DELETE RESTRICT;

-- Name: mst_vendor_financials fk_mst_vendor_financials_vendor; Type: FK CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_financials
    ADD CONSTRAINT fk_mst_vendor_financials_vendor FOREIGN KEY (vendor_cd) REFERENCES pms.mst_vendor(vendor_cd) ON UPDATE CASCADE ON DELETE RESTRICT;

-- Name: mst_vendor_item_group fk_mst_vendor_item_group_stock_group; Type: FK CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_item_group
    ADD CONSTRAINT fk_mst_vendor_item_group_stock_group FOREIGN KEY (grp_cd) REFERENCES pms.mst_stk_group(grp_cd) ON UPDATE CASCADE ON DELETE RESTRICT;

-- Name: mst_vendor_item_group fk_mst_vendor_item_group_vendor; Type: FK CONSTRAINT; Schema: pms; Owner: -

ALTER TABLE ONLY pms.mst_vendor_item_group
    ADD CONSTRAINT fk_mst_vendor_item_group_vendor FOREIGN KEY (vendor_cd) REFERENCES pms.mst_vendor(vendor_cd) ON UPDATE CASCADE ON DELETE RESTRICT;


-- ---- fn_validate_mst_vendor
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_vendor(p_vendor pms.mst_vendor)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_field TEXT;
    v_value TEXT;
BEGIN
    -- Vendor name is required.
    IF NULLIF(BTRIM(p_vendor.name), '') IS NULL THEN
        RAISE EXCEPTION 'Vendor name is required.';
    END IF;

    -- Case-insensitive duplicate name check.
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor v
        WHERE LOWER(BTRIM(v.name)) = LOWER(BTRIM(p_vendor.name))
          AND v.vendor_cd IS DISTINCT FROM p_vendor.vendor_cd
    ) THEN
        RAISE EXCEPTION 'Vendor name ""%"" already exists.', p_vendor.name;
    END IF;

    -- PAN format and duplicate check.
    IF NULLIF(BTRIM(p_vendor.pan), '') IS NOT NULL THEN
        IF p_vendor.pan !~ '^[A-Z]{5}[0-9]{4}[A-Z]$' THEN
            RAISE EXCEPTION 'PAN must contain 5 uppercase letters, 4 digits and 1 uppercase letter.';
        END IF;

        IF EXISTS
        (
            SELECT 1
            FROM pms.mst_vendor v
            WHERE UPPER(BTRIM(v.pan)) = p_vendor.pan
              AND v.vendor_cd IS DISTINCT FROM p_vendor.vendor_cd
        ) THEN
            RAISE EXCEPTION 'PAN ""%"" already exists.', p_vendor.pan;
        END IF;
    END IF;

    -- GSTIN format and duplicate check.
    IF NULLIF(BTRIM(p_vendor.gstin), '') IS NOT NULL THEN
        IF p_vendor.gstin !~ '^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$' THEN
            RAISE EXCEPTION 'Invalid GSTIN format.';
        END IF;

        IF NULLIF(BTRIM(p_vendor.pan), '') IS NOT NULL
           AND SUBSTRING(p_vendor.gstin FROM 3 FOR 10) <> p_vendor.pan THEN
            RAISE EXCEPTION 'GSTIN does not match the supplied PAN.';
        END IF;

        IF EXISTS
        (
            SELECT 1
            FROM pms.mst_vendor v
            WHERE UPPER(BTRIM(v.gstin)) = p_vendor.gstin
              AND v.vendor_cd IS DISTINCT FROM p_vendor.vendor_cd
        ) THEN
            RAISE EXCEPTION 'GSTIN ""%"" already exists.', p_vendor.gstin;
        END IF;
    END IF;

    -- TAN format.
    IF NULLIF(BTRIM(p_vendor.tan), '') IS NOT NULL
       AND p_vendor.tan !~ '^[A-Z]{4}[0-9]{5}[A-Z]$' THEN
        RAISE EXCEPTION 'TAN must contain 4 uppercase letters, 5 digits and 1 uppercase letter.';
    END IF;

    -- Mobile formats: optional + followed by 7 to 14 digits.
    IF NULLIF(BTRIM(p_vendor.mobile), '') IS NOT NULL
       AND p_vendor.mobile !~ '^[+]?[0-9]{7,14}$' THEN
        RAISE EXCEPTION 'Invalid vendor mobile number.';
    END IF;

    -- Correspondence and contact mobile numbers: exactly 10 digits.
    FOR v_field, v_value IN
        SELECT *
        FROM (VALUES
            ('correspond_mobile', p_vendor.correspond_mobile),
            ('contact_1_mobile',  p_vendor.contact_1_mobile),
            ('contact_2_mobile',  p_vendor.contact_2_mobile),
            ('contact_3_mobile',  p_vendor.contact_3_mobile)
        ) AS mobile_fields(field_name, field_value)
    LOOP
        IF NULLIF(BTRIM(v_value), '') IS NOT NULL
           AND v_value !~ '^[0-9]{10}$' THEN
            RAISE EXCEPTION '% must contain exactly 10 digits.', v_field;
        END IF;
    END LOOP;

    -- Basic email format checks.
    FOR v_field, v_value IN
        SELECT *
        FROM (VALUES
            ('email',            p_vendor.email),
            ('correspond_email', p_vendor.correspond_email),
            ('contact_1_email',   p_vendor.contact_1_email),
            ('contact_2_email',   p_vendor.contact_2_email),
            ('contact_3_email',   p_vendor.contact_3_email)
        ) AS email_fields(field_name, field_value)
    LOOP
        IF NULLIF(BTRIM(v_value), '') IS NOT NULL
           AND v_value !~* '^[^[:space:]@]+@[^[:space:]@]+[.][^[:space:]@]+$' THEN
            RAISE EXCEPTION 'Invalid email address in %.', v_field;
        END IF;
    END LOOP;

    -- Four-digit company establishment year.
    IF NULLIF(BTRIM(p_vendor.company_since), '') IS NOT NULL THEN
        IF p_vendor.company_since !~ '^[0-9]{4}$' THEN
            RAISE EXCEPTION 'Company since must be a four-digit year.';
        END IF;

        IF p_vendor.company_since::INTEGER < 1
           OR p_vendor.company_since::INTEGER >
              EXTRACT(YEAR FROM CURRENT_DATE)::INTEGER THEN
            RAISE EXCEPTION 'Company since must be between 0001 and the current year.';
        END IF;
    END IF;
END;
$function$
;

-- ---- fn_validate_update_mst_vendor
CREATE OR REPLACE FUNCTION pms.fn_validate_update_mst_vendor(p_vendor_cd integer, p_name character varying, p_mobile character varying, p_email character varying, p_pan character varying, p_gstin character varying, p_tan character varying, p_correspond_email character varying, p_correspond_mobile character varying, p_company_since character varying, p_contact_1_mobile character varying, p_contact_1_email character varying, p_contact_2_mobile character varying, p_contact_2_email character varying, p_contact_3_mobile character varying, p_contact_3_email character varying)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_year INTEGER;
BEGIN
    -- Check vendor exists
    IF NOT EXISTS (
        SELECT 1 FROM pms.mst_vendor
        WHERE vendor_cd = p_vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Vendor code % does not exist', p_vendor_cd;
    END IF;

    -- Vendor name required
    IF NULLIF(BTRIM(p_name), '') IS NULL THEN
        RAISE EXCEPTION 'Vendor name cannot be blank';
    END IF;

    -- Duplicate vendor name
    IF EXISTS (
        SELECT 1 FROM pms.mst_vendor
        WHERE LOWER(BTRIM(name)) = LOWER(BTRIM(p_name))
          AND vendor_cd <> p_vendor_cd
    ) THEN
        RAISE EXCEPTION 'Vendor name already exists: %', p_name;
    END IF;

    -- PAN validation
    IF NULLIF(BTRIM(p_pan), '') IS NOT NULL THEN
        IF p_pan !~ '^[A-Z]{5}[0-9]{4}[A-Z]$' THEN
            RAISE EXCEPTION 'Invalid PAN: %', p_pan;
        END IF;

        IF EXISTS (
            SELECT 1 FROM pms.mst_vendor
            WHERE pan = p_pan
              AND vendor_cd <> p_vendor_cd
        ) THEN
            RAISE EXCEPTION 'PAN already registered: %', p_pan;
        END IF;
    END IF;

    -- GSTIN validation
    IF NULLIF(BTRIM(p_gstin), '') IS NOT NULL THEN
        IF p_gstin !~ '^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$' THEN
            RAISE EXCEPTION 'Invalid GSTIN: %', p_gstin;
        END IF;

        IF NULLIF(BTRIM(p_pan), '') IS NOT NULL
           AND SUBSTRING(p_gstin FROM 3 FOR 10) <> p_pan THEN
            RAISE EXCEPTION 'GSTIN PAN does not match vendor PAN';
        END IF;

        IF EXISTS (
            SELECT 1 FROM pms.mst_vendor
            WHERE gstin = p_gstin
              AND vendor_cd <> p_vendor_cd
        ) THEN
            RAISE EXCEPTION 'GSTIN already registered: %', p_gstin;
        END IF;
    END IF;

    -- TAN validation
    IF NULLIF(BTRIM(p_tan), '') IS NOT NULL
       AND p_tan !~ '^[A-Z]{4}[0-9]{5}[A-Z]$' THEN
        RAISE EXCEPTION 'Invalid TAN: %', p_tan;
    END IF;

    -- Vendor mobile
    IF NULLIF(BTRIM(p_mobile), '') IS NOT NULL
       AND p_mobile !~ '^\+?[0-9]{7,14}$' THEN
        RAISE EXCEPTION 'Invalid vendor mobile number';
    END IF;

    -- Correspondence and contact mobiles
    IF (
        SELECT COUNT(*)
        FROM unnest(ARRAY[
            p_correspond_mobile,
            p_contact_1_mobile,
            p_contact_2_mobile,
            p_contact_3_mobile
        ]) AS x(mobile_no)
        WHERE NULLIF(BTRIM(mobile_no), '') IS NOT NULL
          AND mobile_no !~ '^[0-9]{10}$'
    ) > 0 THEN
        RAISE EXCEPTION
            'Correspondence/contact mobile must contain 10 digits';
    END IF;

    -- Email validation
    IF (
        SELECT COUNT(*)
        FROM unnest(ARRAY[
            p_email,
            p_correspond_email,
            p_contact_1_email,
            p_contact_2_email,
            p_contact_3_email
        ]) AS x(email_id)
        WHERE NULLIF(BTRIM(email_id), '') IS NOT NULL
          AND email_id !~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$'
    ) > 0 THEN
        RAISE EXCEPTION 'Invalid vendor/contact email address';
    END IF;

    -- Company since
    IF NULLIF(BTRIM(p_company_since), '') IS NOT NULL THEN
        IF p_company_since !~ '^[0-9]{4}$' THEN
            RAISE EXCEPTION 'Company since must be a 4-digit year';
        END IF;

        v_year := p_company_since::INTEGER;

        IF v_year < 1
           OR v_year > EXTRACT(YEAR FROM CURRENT_DATE)::INTEGER THEN
            RAISE EXCEPTION 'Invalid company since year: %',
                p_company_since;
        END IF;
    END IF;

    RETURN TRUE;
END;
$function$
;

-- ---- fn_validate_delete_mst_vendor
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_vendor(p_vendor_cd integer)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate vendor code
    IF p_vendor_cd IS NULL THEN
        RAISE EXCEPTION
            'Vendor code cannot be NULL.';
    END IF;

    -- Check vendor exists
    IF NOT EXISTS (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = p_vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Vendor code % does not exist.',
            p_vendor_cd;
    END IF;

    RETURN TRUE;
END;
$function$
;

-- ---- insert_mst_vendor
CREATE OR REPLACE PROCEDURE pms.insert_mst_vendor(IN take_name character varying, IN take_address character varying DEFAULT NULL::character varying, IN take_city character varying DEFAULT NULL::character varying, IN take_state character varying DEFAULT NULL::character varying, IN take_country character varying DEFAULT NULL::character varying, IN take_pin character varying DEFAULT NULL::character varying, IN take_vendor_type character varying DEFAULT NULL::character varying, IN take_mobile character varying DEFAULT NULL::character varying, IN take_email character varying DEFAULT NULL::character varying, IN take_website character varying DEFAULT NULL::character varying, IN take_reg_date date DEFAULT NULL::date, IN take_pan character varying DEFAULT NULL::character varying, IN take_gstin character varying DEFAULT NULL::character varying, IN take_gstin_state character varying DEFAULT NULL::character varying, IN take_tan character varying DEFAULT NULL::character varying, IN take_refered_by character varying DEFAULT NULL::character varying, IN take_correspond_email character varying DEFAULT NULL::character varying, IN take_correspond_mobile character varying DEFAULT NULL::character varying, IN take_supply_type character varying DEFAULT NULL::character varying, IN take_is_contruction_comp boolean DEFAULT false, IN take_is_manufacturer boolean DEFAULT false, IN take_company_since character varying DEFAULT NULL::character varying, IN take_contact_1_person character varying DEFAULT NULL::character varying, IN take_contact_1_designation character varying DEFAULT NULL::character varying, IN take_contact_1_mobile character varying DEFAULT NULL::character varying, IN take_contact_1_email character varying DEFAULT NULL::character varying, IN take_contact_2_person character varying DEFAULT NULL::character varying, IN take_contact_2_designation character varying DEFAULT NULL::character varying, IN take_contact_2_mobile character varying DEFAULT NULL::character varying, IN take_contact_2_email character varying DEFAULT NULL::character varying, IN take_contact_3_person character varying DEFAULT NULL::character varying, IN take_contact_3_designation character varying DEFAULT NULL::character varying, IN take_contact_3_mobile character varying DEFAULT NULL::character varying, IN take_contact_3_email character varying DEFAULT NULL::character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_vendor pms.mst_vendor%ROWTYPE;
BEGIN
    v_vendor.name                  := NULLIF(BTRIM(take_name), '');
    v_vendor.address               := take_address;
    v_vendor.city                  := take_city;
    v_vendor.state                 := take_state;
    v_vendor.country               := take_country;
    v_vendor.pin                   := take_pin;
    v_vendor.vendor_type           := take_vendor_type;
    v_vendor.mobile                := NULLIF(BTRIM(take_mobile), '');
    v_vendor.email                 := NULLIF(BTRIM(take_email), '');
    v_vendor.website               := take_website;
    v_vendor.reg_date              := take_reg_date;
    v_vendor.pan                   := UPPER(NULLIF(BTRIM(take_pan), ''));
    v_vendor.gstin                 := UPPER(NULLIF(BTRIM(take_gstin), ''));
    v_vendor.gstin_state           := take_gstin_state;
    v_vendor.tan                   := UPPER(NULLIF(BTRIM(take_tan), ''));
    v_vendor.refered_by            := take_refered_by;
    v_vendor.correspond_email      := NULLIF(BTRIM(take_correspond_email), '');
    v_vendor.correspond_mobile     := NULLIF(BTRIM(take_correspond_mobile), '');
    v_vendor.supply_type           := take_supply_type;
    v_vendor.is_contruction_comp   := COALESCE(take_is_contruction_comp, FALSE);
    v_vendor.is_manufacturer       := COALESCE(take_is_manufacturer, FALSE);
    v_vendor.company_since         := NULLIF(BTRIM(take_company_since), '');

    v_vendor.contact_1_person      := take_contact_1_person;
    v_vendor.contact_1_designation := take_contact_1_designation;
    v_vendor.contact_1_mobile      := NULLIF(BTRIM(take_contact_1_mobile), '');
    v_vendor.contact_1_email       := NULLIF(BTRIM(take_contact_1_email), '');

    v_vendor.contact_2_person      := take_contact_2_person;
    v_vendor.contact_2_designation := take_contact_2_designation;
    v_vendor.contact_2_mobile      := NULLIF(BTRIM(take_contact_2_mobile), '');
    v_vendor.contact_2_email       := NULLIF(BTRIM(take_contact_2_email), '');

    v_vendor.contact_3_person      := take_contact_3_person;
    v_vendor.contact_3_designation := take_contact_3_designation;
    v_vendor.contact_3_mobile      := NULLIF(BTRIM(take_contact_3_mobile), '');
    v_vendor.contact_3_email       := NULLIF(BTRIM(take_contact_3_email), '');

    PERFORM pms.fn_validate_mst_vendor(v_vendor);

    INSERT INTO pms.mst_vendor
    (
        name, address, city, state, country, pin,
        vendor_type, mobile, email, website, reg_date,
        pan, gstin, gstin_state, tan, refered_by,
        correspond_email, correspond_mobile, supply_type,
        is_contruction_comp, is_manufacturer, company_since,
        contact_1_person, contact_1_designation,
        contact_1_mobile, contact_1_email,
        contact_2_person, contact_2_designation,
        contact_2_mobile, contact_2_email,
        contact_3_person, contact_3_designation,
        contact_3_mobile, contact_3_email
    )
    VALUES
    (
        v_vendor.name, v_vendor.address, v_vendor.city,
        v_vendor.state, v_vendor.country, v_vendor.pin,
        v_vendor.vendor_type, v_vendor.mobile, v_vendor.email,
        v_vendor.website, v_vendor.reg_date,
        v_vendor.pan, v_vendor.gstin, v_vendor.gstin_state,
        v_vendor.tan, v_vendor.refered_by,
        v_vendor.correspond_email, v_vendor.correspond_mobile,
        v_vendor.supply_type, v_vendor.is_contruction_comp,
        v_vendor.is_manufacturer, v_vendor.company_since,
        v_vendor.contact_1_person, v_vendor.contact_1_designation,
        v_vendor.contact_1_mobile, v_vendor.contact_1_email,
        v_vendor.contact_2_person, v_vendor.contact_2_designation,
        v_vendor.contact_2_mobile, v_vendor.contact_2_email,
        v_vendor.contact_3_person, v_vendor.contact_3_designation,
        v_vendor.contact_3_mobile, v_vendor.contact_3_email
    )
    RETURNING vendor_cd INTO v_vendor.vendor_cd;

    RAISE NOTICE 'Vendor created successfully. Vendor code: %',
                 v_vendor.vendor_cd;
END;
$procedure$
;

-- ---- update_mst_vendor
CREATE OR REPLACE PROCEDURE pms.update_mst_vendor(IN p_vendor_cd integer, IN p_name character varying, IN p_address character varying, IN p_city character varying, IN p_state character varying, IN p_country character varying, IN p_pin character varying, IN p_vendor_type character varying, IN p_mobile character varying, IN p_email character varying, IN p_website character varying, IN p_reg_date date, IN p_pan character varying, IN p_gstin character varying, IN p_gstin_state character varying, IN p_tan character varying, IN p_refered_by character varying, IN p_correspond_email character varying, IN p_correspond_mobile character varying, IN p_supply_type character varying, IN p_is_contruction_comp boolean, IN p_is_manufacturer boolean, IN p_company_since character varying, IN p_contact_1_person character varying, IN p_contact_1_designation character varying, IN p_contact_1_mobile character varying, IN p_contact_1_email character varying, IN p_contact_2_person character varying, IN p_contact_2_designation character varying, IN p_contact_2_mobile character varying, IN p_contact_2_email character varying, IN p_contact_3_person character varying, IN p_contact_3_designation character varying, IN p_contact_3_mobile character varying, IN p_contact_3_email character varying)
 LANGUAGE plpgsql
AS $procedure$
BEGIN
    -- Lock vendor before validation/update
    PERFORM 1
    FROM pms.mst_vendor
    WHERE vendor_cd = p_vendor_cd
    FOR UPDATE;

    -- Validate vendor details
    PERFORM pms.fn_validate_update_mst_vendor(
        p_vendor_cd,
        p_name,
        p_mobile,
        p_email,
        p_pan,
        p_gstin,
        p_tan,
        p_correspond_email,
        p_correspond_mobile,
        p_company_since,
        p_contact_1_mobile,
        p_contact_1_email,
        p_contact_2_mobile,
        p_contact_2_email,
        p_contact_3_mobile,
        p_contact_3_email
    );

    UPDATE pms.mst_vendor
    SET
        name = BTRIM(p_name),
        address = p_address,
        city = p_city,
        state = p_state,
        country = p_country,
        pin = p_pin,
        vendor_type = p_vendor_type,
        mobile = p_mobile,
        email = p_email,
        website = p_website,
        reg_date = p_reg_date,
        pan = NULLIF(BTRIM(p_pan), ''),
        gstin = NULLIF(BTRIM(p_gstin), ''),
        gstin_state = p_gstin_state,
        tan = NULLIF(BTRIM(p_tan), ''),
        refered_by = p_refered_by,
        correspond_email = p_correspond_email,
        correspond_mobile = p_correspond_mobile,
        supply_type = p_supply_type,
        is_contruction_comp = p_is_contruction_comp,
        is_manufacturer = p_is_manufacturer,
        company_since = p_company_since,
        contact_1_person = p_contact_1_person,
        contact_1_designation = p_contact_1_designation,
        contact_1_mobile = p_contact_1_mobile,
        contact_1_email = p_contact_1_email,
        contact_2_person = p_contact_2_person,
        contact_2_designation = p_contact_2_designation,
        contact_2_mobile = p_contact_2_mobile,
        contact_2_email = p_contact_2_email,
        contact_3_person = p_contact_3_person,
        contact_3_designation = p_contact_3_designation,
        contact_3_mobile = p_contact_3_mobile,
        contact_3_email = p_contact_3_email
    WHERE vendor_cd = p_vendor_cd;

    RAISE NOTICE
        'Vendor code % updated successfully',
        p_vendor_cd;
END;
$procedure$
;

-- ---- delete_mst_vendor
CREATE OR REPLACE PROCEDURE pms.delete_mst_vendor(IN p_vendor_cd integer)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_financial_count INTEGER := 0;
    v_item_group_count INTEGER := 0;
    v_certificate_count INTEGER := 0;
    v_vendor_count INTEGER := 0;
BEGIN
    -- Lock vendor record
    PERFORM 1
    FROM pms.mst_vendor
    WHERE vendor_cd = p_vendor_cd
    FOR UPDATE;

    -- Validate vendor
    PERFORM pms.fn_validate_delete_mst_vendor(
        p_vendor_cd
    );

    -- Delete vendor financials
    DELETE FROM pms.mst_vendor_financials
    WHERE vendor_cd = p_vendor_cd;

    GET DIAGNOSTICS
        v_financial_count = ROW_COUNT;

    -- Delete vendor item groups
    DELETE FROM pms.mst_vendor_item_group
    WHERE vendor_cd = p_vendor_cd;

    GET DIAGNOSTICS
        v_item_group_count = ROW_COUNT;

    -- Delete vendor certificates
    DELETE FROM pms.mst_vendor_certificates
    WHERE vendor_cd = p_vendor_cd;

    GET DIAGNOSTICS
        v_certificate_count = ROW_COUNT;

    -- Delete vendor master
    DELETE FROM pms.mst_vendor
    WHERE vendor_cd = p_vendor_cd;

    GET DIAGNOSTICS
        v_vendor_count = ROW_COUNT;

    IF v_vendor_count <> 1 THEN
        RAISE EXCEPTION
            'Unable to delete vendor code %.',
            p_vendor_cd;
    END IF;

    RAISE NOTICE
        'Vendor % deleted successfully. Financials: %, Item Groups: %, Certificates: %',
        p_vendor_cd,
        v_financial_count,
        v_item_group_count,
        v_certificate_count;

END;
$procedure$
;

-- ---- fn_validate_mst_vendor_financials
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_vendor_financials(take_vendor_cd integer, take_fin_year character varying, take_fin_turnover_declared numeric, take_fin_profit_declared numeric, take_fin_itr_no character varying, take_fin_itr_date date, take_fin_itr_s3_bucket_reference character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_fin_year TEXT := BTRIM(take_fin_year);
BEGIN
    -- Vendor must exist.
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = take_vendor_cd
    ) THEN
        RAISE EXCEPTION 'Vendor code % does not exist.', take_vendor_cd;
    END IF;

    -- Financial year must follow YYYY-YYYY.
    IF NULLIF(v_fin_year, '') IS NULL THEN
        RAISE EXCEPTION 'Financial year is required.';
    END IF;

    IF v_fin_year !~ '^[0-9]{4}-[0-9]{4}$' THEN
        RAISE EXCEPTION 'Financial year must be in YYYY-YYYY format, for example 2025-2026.';
    END IF;

    IF SUBSTRING(v_fin_year FROM 1 FOR 4)::INTEGER < 1
       OR SUBSTRING(v_fin_year FROM 6 FOR 4)::INTEGER <>
          SUBSTRING(v_fin_year FROM 1 FOR 4)::INTEGER + 1 THEN
        RAISE EXCEPTION 'Financial year must contain consecutive years, for example 2025-2026.';
    END IF;

    -- One record per vendor per financial year.
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_financials
        WHERE vendor_cd = take_vendor_cd
          AND fin_year = v_fin_year
    ) THEN
        RAISE EXCEPTION
            'Financial details already exist for vendor % and financial year %.',
            take_vendor_cd, v_fin_year;
    END IF;

    -- NUMERIC(15,2): maximum 13 integer digits.
    IF take_fin_turnover_declared IS NOT NULL THEN
        IF take_fin_turnover_declared::TEXT IN
           ('NaN', 'Infinity', '-Infinity') THEN
            RAISE EXCEPTION 'Turnover must be a finite number.';
        END IF;

        IF take_fin_turnover_declared < 0 THEN
            RAISE EXCEPTION 'Declared turnover cannot be negative.';
        END IF;

        IF ROUND(take_fin_turnover_declared, 2) >
           9999999999999.99 THEN
            RAISE EXCEPTION 'Declared turnover exceeds NUMERIC(15,2) capacity.';
        END IF;
    END IF;

    IF take_fin_profit_declared IS NOT NULL THEN
        IF take_fin_profit_declared::TEXT IN
           ('NaN', 'Infinity', '-Infinity') THEN
            RAISE EXCEPTION 'Profit must be a finite number.';
        END IF;

        IF ABS(ROUND(take_fin_profit_declared, 2)) >
           9999999999999.99 THEN
            RAISE EXCEPTION 'Declared profit exceeds NUMERIC(15,2) capacity.';
        END IF;
    END IF;

    IF LENGTH(BTRIM(take_fin_itr_no)) > 25 THEN
        RAISE EXCEPTION 'ITR number cannot exceed 25 characters.';
    END IF;

    -- ITR date and S3 reference remain optional.
END;
$function$
;

-- ---- fn_validate_update_mst_vendor_financials
CREATE OR REPLACE FUNCTION pms.fn_validate_update_mst_vendor_financials(take_vendor_cd integer, take_fin_year character varying, take_fin_turnover_declared numeric, take_fin_profit_declared numeric, take_fin_itr_no character varying, take_fin_itr_date date, take_fin_itr_s3_bucket_reference character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_fin_year TEXT := NULLIF(BTRIM(take_fin_year), '');
BEGIN
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF v_fin_year IS NULL THEN
        RAISE EXCEPTION 'Financial year is required.';
    END IF;

    IF v_fin_year !~ '^[0-9]{4}-[0-9]{4}$' THEN
        RAISE EXCEPTION
            'Financial year must be in YYYY-YYYY format, for example 2025-2026.';
    END IF;

    IF SUBSTRING(v_fin_year FROM 1 FOR 4)::INTEGER < 1
       OR SUBSTRING(v_fin_year FROM 6 FOR 4)::INTEGER <>
          SUBSTRING(v_fin_year FROM 1 FOR 4)::INTEGER + 1 THEN
        RAISE EXCEPTION 'Financial year must contain consecutive years.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = take_vendor_cd
    ) THEN
        RAISE EXCEPTION 'Vendor code % does not exist.', take_vendor_cd;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_financials
        WHERE vendor_cd = take_vendor_cd
          AND fin_year = v_fin_year
    ) THEN
        RAISE EXCEPTION
            'Financial record not found for vendor % and financial year %.',
            take_vendor_cd, v_fin_year;
    END IF;

    IF take_fin_turnover_declared IS NOT NULL THEN
        IF take_fin_turnover_declared::TEXT IN
           ('NaN', 'Infinity', '-Infinity') THEN
            RAISE EXCEPTION 'Turnover must be a finite number.';
        END IF;

        IF take_fin_turnover_declared < 0 THEN
            RAISE EXCEPTION 'Declared turnover cannot be negative.';
        END IF;

        IF ROUND(take_fin_turnover_declared, 2) >
           9999999999999.99 THEN
            RAISE EXCEPTION
                'Declared turnover exceeds NUMERIC(15,2) capacity.';
        END IF;
    END IF;

    -- Negative profit is allowed to represent a loss.
    IF take_fin_profit_declared IS NOT NULL THEN
        IF take_fin_profit_declared::TEXT IN
           ('NaN', 'Infinity', '-Infinity') THEN
            RAISE EXCEPTION 'Profit must be a finite number.';
        END IF;

        IF ABS(ROUND(take_fin_profit_declared, 2)) >
           9999999999999.99 THEN
            RAISE EXCEPTION
                'Declared profit exceeds NUMERIC(15,2) capacity.';
        END IF;
    END IF;

    IF LENGTH(BTRIM(take_fin_itr_no)) > 25 THEN
        RAISE EXCEPTION 'ITR number cannot exceed 25 characters.';
    END IF;

    -- ITR date and S3 reference are optional.
END;
$function$
;

-- ---- fn_validate_delete_mst_vendor_financials
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_vendor_financials(take_vendor_cd integer, take_fin_year character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_fin_year TEXT := NULLIF(BTRIM(take_fin_year), '');
BEGIN
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF v_fin_year IS NULL THEN
        RAISE EXCEPTION 'Financial year is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_financials
        WHERE vendor_cd = take_vendor_cd
          AND fin_year = v_fin_year
    ) THEN
        RAISE EXCEPTION
            'Financial record not found for vendor % and financial year %.',
            take_vendor_cd, v_fin_year;
    END IF;
END;
$function$
;

-- ---- insert_mst_vendor_financials
CREATE OR REPLACE PROCEDURE pms.insert_mst_vendor_financials(IN take_vendor_cd integer, IN take_fin_year character varying, IN take_fin_turnover_declared numeric DEFAULT NULL::numeric, IN take_fin_profit_declared numeric DEFAULT NULL::numeric, IN take_fin_itr_no character varying DEFAULT NULL::character varying, IN take_fin_itr_date date DEFAULT NULL::date, IN take_fin_itr_s3_bucket_reference character varying DEFAULT NULL::character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_fin_year      TEXT := NULLIF(BTRIM(take_fin_year), '');
    v_fin_itr_no    TEXT := NULLIF(BTRIM(take_fin_itr_no), '');
    v_s3_reference  TEXT :=
        NULLIF(BTRIM(take_fin_itr_s3_bucket_reference), '');
BEGIN
    PERFORM pms.fn_validate_mst_vendor_financials
    (
        take_vendor_cd,
        v_fin_year::VARCHAR,
        take_fin_turnover_declared,
        take_fin_profit_declared,
        v_fin_itr_no::VARCHAR,
        take_fin_itr_date,
        v_s3_reference::VARCHAR
    );

    INSERT INTO pms.mst_vendor_financials
    (
        vendor_cd,
        fin_year,
        fin_turnover_declared,
        fin_profit_declared,
        fin_itr_no,
        fin_itr_date,
        fin_itr_s3_bucket_reference
    )
    VALUES
    (
        take_vendor_cd,
        v_fin_year,
        take_fin_turnover_declared,
        take_fin_profit_declared,
        v_fin_itr_no,
        take_fin_itr_date,
        v_s3_reference
    );
END;
$procedure$
;

-- ---- update_mst_vendor_financials
CREATE OR REPLACE PROCEDURE pms.update_mst_vendor_financials(IN take_vendor_cd integer, IN take_fin_year character varying, IN take_fin_turnover_declared numeric, IN take_fin_profit_declared numeric, IN take_fin_itr_no character varying, IN take_fin_itr_date date, IN take_fin_itr_s3_bucket_reference character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_fin_year TEXT := NULLIF(BTRIM(take_fin_year), '');
    v_itr_no   TEXT := NULLIF(BTRIM(take_fin_itr_no), '');
    v_s3_ref   TEXT :=
        NULLIF(BTRIM(take_fin_itr_s3_bucket_reference), '');
BEGIN
    PERFORM pms.fn_validate_update_mst_vendor_financials
    (
        take_vendor_cd,
        v_fin_year::VARCHAR,
        take_fin_turnover_declared,
        take_fin_profit_declared,
        v_itr_no::VARCHAR,
        take_fin_itr_date,
        v_s3_ref::VARCHAR
    );

    UPDATE pms.mst_vendor_financials
    SET fin_turnover_declared       = take_fin_turnover_declared,
        fin_profit_declared         = take_fin_profit_declared,
        fin_itr_no                  = v_itr_no,
        fin_itr_date                = take_fin_itr_date,
        fin_itr_s3_bucket_reference = v_s3_ref
    WHERE vendor_cd = take_vendor_cd
      AND fin_year = v_fin_year;

    -- Handles a record deleted after validation.
    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Financial record not found for vendor % and financial year %.',
            take_vendor_cd, v_fin_year;
    END IF;
END;
$procedure$
;

-- ---- delete_mst_vendor_financials
CREATE OR REPLACE PROCEDURE pms.delete_mst_vendor_financials(IN take_vendor_cd integer, IN take_fin_year character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_fin_year TEXT := NULLIF(BTRIM(take_fin_year), '');
BEGIN
    PERFORM pms.fn_validate_delete_mst_vendor_financials
    (
        take_vendor_cd,
        v_fin_year::VARCHAR
    );

    DELETE FROM pms.mst_vendor_financials
    WHERE vendor_cd = take_vendor_cd
      AND fin_year = v_fin_year;

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Financial record not found for vendor % and financial year %.',
            take_vendor_cd, v_fin_year;
    END IF;
END;
$procedure$
;

-- ---- fn_validate_mst_vendor_item_group
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_vendor_item_group(take_vendor_cd integer, take_grp_cd integer)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF take_grp_cd IS NULL THEN
        RAISE EXCEPTION 'Item group code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = take_vendor_cd
    ) THEN
        RAISE EXCEPTION 'Vendor code % does not exist.', take_vendor_cd;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE grp_cd = take_grp_cd
    ) THEN
        RAISE EXCEPTION 'Item group code % does not exist.', take_grp_cd;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_item_group
        WHERE vendor_cd = take_vendor_cd
          AND grp_cd = take_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Item group % is already assigned to vendor %.',
            take_grp_cd, take_vendor_cd;
    END IF;
END;
$function$
;

-- ---- fn_validate_update_mst_vendor_item_group
CREATE OR REPLACE FUNCTION pms.fn_validate_update_mst_vendor_item_group(take_old_vendor_cd integer, take_old_grp_cd integer, take_new_vendor_cd integer, take_new_grp_cd integer)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF take_old_vendor_cd IS NULL OR take_old_grp_cd IS NULL THEN
        RAISE EXCEPTION 'Existing vendor code and item group code are required.';
    END IF;

    IF take_new_vendor_cd IS NULL OR take_new_grp_cd IS NULL THEN
        RAISE EXCEPTION 'New vendor code and item group code are required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_item_group
        WHERE vendor_cd = take_old_vendor_cd
          AND grp_cd = take_old_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Item group % is not assigned to vendor %.',
            take_old_grp_cd, take_old_vendor_cd;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = take_new_vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Vendor code % does not exist.', take_new_vendor_cd;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_group
        WHERE grp_cd = take_new_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Item group code % does not exist.', take_new_grp_cd;
    END IF;

    -- Exclude the assignment being updated.
    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_item_group
        WHERE vendor_cd = take_new_vendor_cd
          AND grp_cd = take_new_grp_cd
          AND NOT
          (
              vendor_cd = take_old_vendor_cd
              AND grp_cd = take_old_grp_cd
          )
    ) THEN
        RAISE EXCEPTION
            'Item group % is already assigned to vendor %.',
            take_new_grp_cd, take_new_vendor_cd;
    END IF;
END;
$function$
;

-- ---- fn_validate_delete_mst_vendor_item_group
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_vendor_item_group(take_vendor_cd integer, take_grp_cd integer)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF take_grp_cd IS NULL THEN
        RAISE EXCEPTION 'Item group code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_item_group
        WHERE vendor_cd = take_vendor_cd
          AND grp_cd = take_grp_cd
    ) THEN
        RAISE EXCEPTION
            'Item group % is not assigned to vendor %.',
            take_grp_cd, take_vendor_cd;
    END IF;
END;
$function$
;

-- ---- insert_mst_vendor_item_group
CREATE OR REPLACE PROCEDURE pms.insert_mst_vendor_item_group(IN take_vendor_cd integer, IN take_grp_cd integer)
 LANGUAGE plpgsql
AS $procedure$
BEGIN
    PERFORM pms.fn_validate_mst_vendor_item_group
    (
        take_vendor_cd,
        take_grp_cd
    );

    INSERT INTO pms.mst_vendor_item_group
    (
        vendor_cd,
        grp_cd
    )
    VALUES
    (
        take_vendor_cd,
        take_grp_cd
    );
END;
$procedure$
;

-- ---- update_mst_vendor_item_group
CREATE OR REPLACE PROCEDURE pms.update_mst_vendor_item_group(IN take_old_vendor_cd integer, IN take_old_grp_cd integer, IN take_new_vendor_cd integer, IN take_new_grp_cd integer)
 LANGUAGE plpgsql
AS $procedure$
BEGIN
    PERFORM pms.fn_validate_update_mst_vendor_item_group
    (
        take_old_vendor_cd,
        take_old_grp_cd,
        take_new_vendor_cd,
        take_new_grp_cd
    );

    UPDATE pms.mst_vendor_item_group
    SET vendor_cd = take_new_vendor_cd,
        grp_cd    = take_new_grp_cd
    WHERE vendor_cd = take_old_vendor_cd
      AND grp_cd = take_old_grp_cd;

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Item group % is no longer assigned to vendor %.',
            take_old_grp_cd, take_old_vendor_cd;
    END IF;
END;
$procedure$
;

-- ---- delete_mst_vendor_item_group
CREATE OR REPLACE PROCEDURE pms.delete_mst_vendor_item_group(IN take_vendor_cd integer, IN take_grp_cd integer)
 LANGUAGE plpgsql
AS $procedure$
BEGIN
    PERFORM pms.fn_validate_delete_mst_vendor_item_group
    (
        take_vendor_cd,
        take_grp_cd
    );

    DELETE FROM pms.mst_vendor_item_group
    WHERE vendor_cd = take_vendor_cd
      AND grp_cd = take_grp_cd;

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Item group % is no longer assigned to vendor %.',
            take_grp_cd, take_vendor_cd;
    END IF;
END;
$procedure$
;

-- ---- fn_validate_mst_vendor_certificates
CREATE OR REPLACE FUNCTION pms.fn_validate_mst_vendor_certificates(p_certificate pms.mst_vendor_certificates)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_label     TEXT;
    v_uploaded  BOOLEAN;
    v_reference TEXT;
    v_caption   TEXT;
BEGIN
    IF p_certificate.vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = p_certificate.vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Vendor code % does not exist.', p_certificate.vendor_cd;
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_certificates
        WHERE vendor_cd = p_certificate.vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Certificate record already exists for vendor %.',
            p_certificate.vendor_cd;
    END IF;

    FOR v_label, v_uploaded, v_reference IN
        SELECT *
        FROM (VALUES
            ('GST',
             p_certificate.gst_uploaded,
             p_certificate.gst_s3_bucket_reference),
            ('Company registration',
             p_certificate.company_reg_uploaded,
             p_certificate.company_reg_s3_bucket_reference),
            ('PAN',
             p_certificate.pan_uploaded,
             p_certificate.pan_s3_bucket_reference),
            ('MSME',
             p_certificate.msme_uploaded,
             p_certificate.msme_s3_bucket_reference),
            ('TAN',
             p_certificate.tan_uploaded,
             p_certificate.tan_s3_bucket_reference),
            ('Aadhar',
             p_certificate.aadhar_uploaded,
             p_certificate.aadhar_s3_bucket_reference),
            ('Other certificate 1',
             p_certificate.oth_1_uploaded,
             p_certificate.oth_1_s3_bucket_reference),
            ('Other certificate 2',
             p_certificate.oth_2_uploaded,
             p_certificate.oth_2_s3_bucket_reference),
            ('Other certificate 3',
             p_certificate.oth_3_uploaded,
             p_certificate.oth_3_s3_bucket_reference)
        ) AS certificates(label, uploaded, reference)
    LOOP
        IF v_uploaded IS NULL THEN
            RAISE EXCEPTION '% upload flag cannot be NULL.', v_label;
        END IF;

        IF v_uploaded
           AND NULLIF(BTRIM(v_reference), '') IS NULL THEN
            RAISE EXCEPTION
                '% S3 reference is required when marked as uploaded.',
                v_label;
        END IF;

        IF NOT v_uploaded
           AND NULLIF(BTRIM(v_reference), '') IS NOT NULL THEN
            RAISE EXCEPTION
                '% must be marked as uploaded when an S3 reference is supplied.',
                v_label;
        END IF;
    END LOOP;

    FOR v_label, v_uploaded, v_caption IN
        SELECT *
        FROM (VALUES
            ('Other certificate 1',
             p_certificate.oth_1_uploaded,
             p_certificate.oth_1_caption),
            ('Other certificate 2',
             p_certificate.oth_2_uploaded,
             p_certificate.oth_2_caption),
            ('Other certificate 3',
             p_certificate.oth_3_uploaded,
             p_certificate.oth_3_caption)
        ) AS captions(label, uploaded, caption)
    LOOP
        IF v_uploaded AND NULLIF(BTRIM(v_caption), '') IS NULL THEN
            RAISE EXCEPTION
                '% caption is required when marked as uploaded.', v_label;
        END IF;

        IF LENGTH(v_caption) > 100 THEN
            RAISE EXCEPTION
                '% caption cannot exceed 100 characters.', v_label;
        END IF;
    END LOOP;
END;
$function$
;

-- ---- fn_validate_update_mst_vendor_certificates
CREATE OR REPLACE FUNCTION pms.fn_validate_update_mst_vendor_certificates(p_certificate pms.mst_vendor_certificates)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_data      JSONB := TO_JSONB(p_certificate);
    v_prefix    TEXT;
    v_uploaded  BOOLEAN;
    v_reference TEXT;
    v_caption   TEXT;
BEGIN
    IF p_certificate.vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor
        WHERE vendor_cd = p_certificate.vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Vendor code % does not exist.', p_certificate.vendor_cd;
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_certificates
        WHERE vendor_cd = p_certificate.vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Certificate record does not exist for vendor %.',
            p_certificate.vendor_cd;
    END IF;

    FOREACH v_prefix IN ARRAY ARRAY[
        'gst', 'company_reg', 'pan', 'msme', 'tan',
        'aadhar', 'oth_1', 'oth_2', 'oth_3'
    ]
    LOOP
        v_uploaded := (v_data ->> (v_prefix || '_uploaded'))::BOOLEAN;
        v_reference := NULLIF(
            BTRIM(v_data ->> (v_prefix || '_s3_bucket_reference')), ''
        );

        IF v_uploaded IS NULL THEN
            RAISE EXCEPTION '% upload flag cannot be NULL.', v_prefix;
        END IF;

        IF v_uploaded AND v_reference IS NULL THEN
            RAISE EXCEPTION
                '% S3 reference is required when marked as uploaded.',
                v_prefix;
        END IF;

        IF NOT v_uploaded AND v_reference IS NOT NULL THEN
            RAISE EXCEPTION
                '% must be marked as uploaded when an S3 reference is supplied.',
                v_prefix;
        END IF;

        IF v_prefix IN ('oth_1', 'oth_2', 'oth_3') THEN
            v_caption := NULLIF(
                BTRIM(v_data ->> (v_prefix || '_caption')), ''
            );

            IF v_uploaded AND v_caption IS NULL THEN
                RAISE EXCEPTION
                    '% caption is required when marked as uploaded.',
                    v_prefix;
            END IF;

            IF LENGTH(v_caption) > 100 THEN
                RAISE EXCEPTION
                    '% caption cannot exceed 100 characters.', v_prefix;
            END IF;
        END IF;
    END LOOP;
END;
$function$
;

-- ---- fn_validate_delete_mst_vendor_certificates
CREATE OR REPLACE FUNCTION pms.fn_validate_delete_mst_vendor_certificates(take_vendor_cd integer)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF take_vendor_cd IS NULL THEN
        RAISE EXCEPTION 'Vendor code is required.';
    END IF;

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_vendor_certificates
        WHERE vendor_cd = take_vendor_cd
    ) THEN
        RAISE EXCEPTION
            'Certificate record does not exist for vendor %.',
            take_vendor_cd;
    END IF;
END;
$function$
;

-- ---- insert_mst_vendor_certificates
CREATE OR REPLACE PROCEDURE pms.insert_mst_vendor_certificates(IN take_vendor_cd integer, IN take_gst_uploaded boolean DEFAULT false, IN take_gst_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_company_reg_uploaded boolean DEFAULT false, IN take_company_reg_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_pan_uploaded boolean DEFAULT false, IN take_pan_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_msme_uploaded boolean DEFAULT false, IN take_msme_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_tan_uploaded boolean DEFAULT false, IN take_tan_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_aadhar_uploaded boolean DEFAULT false, IN take_aadhar_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_oth_1_uploaded boolean DEFAULT false, IN take_oth_1_caption character varying DEFAULT NULL::character varying, IN take_oth_1_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_oth_2_uploaded boolean DEFAULT false, IN take_oth_2_caption character varying DEFAULT NULL::character varying, IN take_oth_2_s3_bucket_reference character varying DEFAULT NULL::character varying, IN take_oth_3_uploaded boolean DEFAULT false, IN take_oth_3_caption character varying DEFAULT NULL::character varying, IN take_oth_3_s3_bucket_reference character varying DEFAULT NULL::character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_certificate pms.mst_vendor_certificates%ROWTYPE;
BEGIN
    -- Check lengths before assigning to VARCHAR(100) fields.
    IF LENGTH(NULLIF(BTRIM(take_oth_1_caption), '')) > 100
       OR LENGTH(NULLIF(BTRIM(take_oth_2_caption), '')) > 100
       OR LENGTH(NULLIF(BTRIM(take_oth_3_caption), '')) > 100 THEN
        RAISE EXCEPTION 'Other certificate captions cannot exceed 100 characters.';
    END IF;

    v_certificate.vendor_cd := take_vendor_cd;

    v_certificate.gst_uploaded := take_gst_uploaded;
    v_certificate.gst_s3_bucket_reference :=
        NULLIF(BTRIM(take_gst_s3_bucket_reference), '');

    v_certificate.company_reg_uploaded := take_company_reg_uploaded;
    v_certificate.company_reg_s3_bucket_reference :=
        NULLIF(BTRIM(take_company_reg_s3_bucket_reference), '');

    v_certificate.pan_uploaded := take_pan_uploaded;
    v_certificate.pan_s3_bucket_reference :=
        NULLIF(BTRIM(take_pan_s3_bucket_reference), '');

    v_certificate.msme_uploaded := take_msme_uploaded;
    v_certificate.msme_s3_bucket_reference :=
        NULLIF(BTRIM(take_msme_s3_bucket_reference), '');

    v_certificate.tan_uploaded := take_tan_uploaded;
    v_certificate.tan_s3_bucket_reference :=
        NULLIF(BTRIM(take_tan_s3_bucket_reference), '');

    v_certificate.aadhar_uploaded := take_aadhar_uploaded;
    v_certificate.aadhar_s3_bucket_reference :=
        NULLIF(BTRIM(take_aadhar_s3_bucket_reference), '');

    v_certificate.oth_1_uploaded := take_oth_1_uploaded;
    v_certificate.oth_1_caption :=
        NULLIF(BTRIM(take_oth_1_caption), '');
    v_certificate.oth_1_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_1_s3_bucket_reference), '');

    v_certificate.oth_2_uploaded := take_oth_2_uploaded;
    v_certificate.oth_2_caption :=
        NULLIF(BTRIM(take_oth_2_caption), '');
    v_certificate.oth_2_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_2_s3_bucket_reference), '');

    v_certificate.oth_3_uploaded := take_oth_3_uploaded;
    v_certificate.oth_3_caption :=
        NULLIF(BTRIM(take_oth_3_caption), '');
    v_certificate.oth_3_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_3_s3_bucket_reference), '');

    PERFORM pms.fn_validate_mst_vendor_certificates(v_certificate);

    INSERT INTO pms.mst_vendor_certificates
    (
        vendor_cd,
        gst_uploaded, gst_s3_bucket_reference,
        company_reg_uploaded, company_reg_s3_bucket_reference,
        pan_uploaded, pan_s3_bucket_reference,
        msme_uploaded, msme_s3_bucket_reference,
        tan_uploaded, tan_s3_bucket_reference,
        aadhar_uploaded, aadhar_s3_bucket_reference,
        oth_1_uploaded, oth_1_caption, oth_1_s3_bucket_reference,
        oth_2_uploaded, oth_2_caption, oth_2_s3_bucket_reference,
        oth_3_uploaded, oth_3_caption, oth_3_s3_bucket_reference
    )
    VALUES
    (
        v_certificate.vendor_cd,
        v_certificate.gst_uploaded,
        v_certificate.gst_s3_bucket_reference,
        v_certificate.company_reg_uploaded,
        v_certificate.company_reg_s3_bucket_reference,
        v_certificate.pan_uploaded,
        v_certificate.pan_s3_bucket_reference,
        v_certificate.msme_uploaded,
        v_certificate.msme_s3_bucket_reference,
        v_certificate.tan_uploaded,
        v_certificate.tan_s3_bucket_reference,
        v_certificate.aadhar_uploaded,
        v_certificate.aadhar_s3_bucket_reference,
        v_certificate.oth_1_uploaded,
        v_certificate.oth_1_caption,
        v_certificate.oth_1_s3_bucket_reference,
        v_certificate.oth_2_uploaded,
        v_certificate.oth_2_caption,
        v_certificate.oth_2_s3_bucket_reference,
        v_certificate.oth_3_uploaded,
        v_certificate.oth_3_caption,
        v_certificate.oth_3_s3_bucket_reference
    );
END;
$procedure$
;

-- ---- update_mst_vendor_certificates
CREATE OR REPLACE PROCEDURE pms.update_mst_vendor_certificates(IN take_vendor_cd integer, IN take_gst_uploaded boolean, IN take_gst_s3_bucket_reference character varying, IN take_company_reg_uploaded boolean, IN take_company_reg_s3_bucket_reference character varying, IN take_pan_uploaded boolean, IN take_pan_s3_bucket_reference character varying, IN take_msme_uploaded boolean, IN take_msme_s3_bucket_reference character varying, IN take_tan_uploaded boolean, IN take_tan_s3_bucket_reference character varying, IN take_aadhar_uploaded boolean, IN take_aadhar_s3_bucket_reference character varying, IN take_oth_1_uploaded boolean, IN take_oth_1_caption character varying, IN take_oth_1_s3_bucket_reference character varying, IN take_oth_2_uploaded boolean, IN take_oth_2_caption character varying, IN take_oth_2_s3_bucket_reference character varying, IN take_oth_3_uploaded boolean, IN take_oth_3_caption character varying, IN take_oth_3_s3_bucket_reference character varying)
 LANGUAGE plpgsql
AS $procedure$
DECLARE
    v_certificate pms.mst_vendor_certificates%ROWTYPE;
BEGIN
    -- Validate lengths before assigning to VARCHAR(100) fields.
    IF LENGTH(BTRIM(take_oth_1_caption)) > 100
       OR LENGTH(BTRIM(take_oth_2_caption)) > 100
       OR LENGTH(BTRIM(take_oth_3_caption)) > 100 THEN
        RAISE EXCEPTION
            'Other certificate captions cannot exceed 100 characters.';
    END IF;

    v_certificate.vendor_cd := take_vendor_cd;

    v_certificate.gst_uploaded := take_gst_uploaded;
    v_certificate.gst_s3_bucket_reference :=
        NULLIF(BTRIM(take_gst_s3_bucket_reference), '');

    v_certificate.company_reg_uploaded := take_company_reg_uploaded;
    v_certificate.company_reg_s3_bucket_reference :=
        NULLIF(BTRIM(take_company_reg_s3_bucket_reference), '');

    v_certificate.pan_uploaded := take_pan_uploaded;
    v_certificate.pan_s3_bucket_reference :=
        NULLIF(BTRIM(take_pan_s3_bucket_reference), '');

    v_certificate.msme_uploaded := take_msme_uploaded;
    v_certificate.msme_s3_bucket_reference :=
        NULLIF(BTRIM(take_msme_s3_bucket_reference), '');

    v_certificate.tan_uploaded := take_tan_uploaded;
    v_certificate.tan_s3_bucket_reference :=
        NULLIF(BTRIM(take_tan_s3_bucket_reference), '');

    v_certificate.aadhar_uploaded := take_aadhar_uploaded;
    v_certificate.aadhar_s3_bucket_reference :=
        NULLIF(BTRIM(take_aadhar_s3_bucket_reference), '');

    v_certificate.oth_1_uploaded := take_oth_1_uploaded;
    v_certificate.oth_1_caption :=
        NULLIF(BTRIM(take_oth_1_caption), '');
    v_certificate.oth_1_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_1_s3_bucket_reference), '');

    v_certificate.oth_2_uploaded := take_oth_2_uploaded;
    v_certificate.oth_2_caption :=
        NULLIF(BTRIM(take_oth_2_caption), '');
    v_certificate.oth_2_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_2_s3_bucket_reference), '');

    v_certificate.oth_3_uploaded := take_oth_3_uploaded;
    v_certificate.oth_3_caption :=
        NULLIF(BTRIM(take_oth_3_caption), '');
    v_certificate.oth_3_s3_bucket_reference :=
        NULLIF(BTRIM(take_oth_3_s3_bucket_reference), '');

    PERFORM pms.fn_validate_update_mst_vendor_certificates(v_certificate);

    UPDATE pms.mst_vendor_certificates
    SET gst_uploaded                   = v_certificate.gst_uploaded,
        gst_s3_bucket_reference        = v_certificate.gst_s3_bucket_reference,
        company_reg_uploaded           = v_certificate.company_reg_uploaded,
        company_reg_s3_bucket_reference = v_certificate.company_reg_s3_bucket_reference,
        pan_uploaded                   = v_certificate.pan_uploaded,
        pan_s3_bucket_reference        = v_certificate.pan_s3_bucket_reference,
        msme_uploaded                  = v_certificate.msme_uploaded,
        msme_s3_bucket_reference       = v_certificate.msme_s3_bucket_reference,
        tan_uploaded                   = v_certificate.tan_uploaded,
        tan_s3_bucket_reference        = v_certificate.tan_s3_bucket_reference,
        aadhar_uploaded                = v_certificate.aadhar_uploaded,
        aadhar_s3_bucket_reference     = v_certificate.aadhar_s3_bucket_reference,
        oth_1_uploaded                 = v_certificate.oth_1_uploaded,
        oth_1_caption                  = v_certificate.oth_1_caption,
        oth_1_s3_bucket_reference      = v_certificate.oth_1_s3_bucket_reference,
        oth_2_uploaded                 = v_certificate.oth_2_uploaded,
        oth_2_caption                  = v_certificate.oth_2_caption,
        oth_2_s3_bucket_reference      = v_certificate.oth_2_s3_bucket_reference,
        oth_3_uploaded                 = v_certificate.oth_3_uploaded,
        oth_3_caption                  = v_certificate.oth_3_caption,
        oth_3_s3_bucket_reference      = v_certificate.oth_3_s3_bucket_reference
    WHERE vendor_cd = take_vendor_cd;

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Certificate record no longer exists for vendor %.',
            take_vendor_cd;
    END IF;
END;
$procedure$
;

-- ---- delete_mst_vendor_certificates
CREATE OR REPLACE PROCEDURE pms.delete_mst_vendor_certificates(IN take_vendor_cd integer)
 LANGUAGE plpgsql
AS $procedure$
BEGIN
    PERFORM pms.fn_validate_delete_mst_vendor_certificates
    (
        take_vendor_cd
    );

    DELETE FROM pms.mst_vendor_certificates
    WHERE vendor_cd = take_vendor_cd;

    IF NOT FOUND THEN
        RAISE EXCEPTION
            'Certificate record no longer exists for vendor %.',
            take_vendor_cd;
    END IF;
END;
$procedure$
;
