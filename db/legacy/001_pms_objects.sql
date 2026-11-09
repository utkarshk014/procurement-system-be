-- Objects the DBA created in the esispms database (schema pms), copied verbatim
-- from the PROCUREMENT.ESIS bundle so local development and tests run against
-- the same tables, functions and procedures. Do not run this against the real
-- database.
--
-- The bundle has no CREATE TABLE for finyr_master or company_master; those two
-- are reconstructed from the live catalog (columns, defaults, constraints).

CREATE SCHEMA IF NOT EXISTS pms;

CREATE TABLE pms.sw_config
(
    config_key          VARCHAR(50) PRIMARY KEY,
    parity_boolean     BOOLEAN,
    parity_text_1      VARCHAR(250),
    parity_text_2      VARCHAR(250),
    parity_text_3      VARCHAR(250),
    parity_date        DATE,
    parity_time        TIME,
    parity_timestamp   TIMESTAMP,
    rec_created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE pms.finyr_master
(
    finyr             VARCHAR(9) NOT NULL,
    finyr_st_date     DATE,
    finyr_end_date    DATE,
    finyr_is_current  BOOLEAN DEFAULT FALSE,
    rec_created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT finyr_master_pkey PRIMARY KEY (finyr),
    CONSTRAINT chk_finyr_dates CHECK (finyr_end_date >= finyr_st_date)
);

CREATE TABLE pms.company_master
(
    sl              INTEGER PRIMARY KEY,
    name            VARCHAR(300),
    address         VARCHAR(300),
    city            VARCHAR(200),
    state           VARCHAR(200),
    country         VARCHAR(200),
    pin             VARCHAR(10),
    mobile          VARCHAR(15),
    email           VARCHAR(200),
    website         VARCHAR(150),
    header_1        VARCHAR(200),
    header_2        VARCHAR(200),
    shortname       VARCHAR(50),
    fin_st_dt       DATE,
    time_zone       TIMESTAMPTZ,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


-- ---- PROC_pms.config_sw_insert.TXT
CREATE OR REPLACE PROCEDURE pms.config_sw_insert
(
    take_config_key          VARCHAR(50),
    take_parity_boolean      BOOLEAN,
    take_parity_text_1       VARCHAR(250),
    take_parity_text_2       VARCHAR(250),
    take_parity_text_3       VARCHAR(250),
    take_parity_date         DATE,
    take_parity_time         TIME,
    take_parity_timestamp    TIMESTAMP
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO pms.sw_config
    (
        config_key,
        parity_boolean,
        parity_text_1,
        parity_text_2,
        parity_text_3,
        parity_date,
        parity_time,
        parity_timestamp,
        rec_created_at
    )
    VALUES
    (
        take_config_key,
        take_parity_boolean,
        take_parity_text_1,
        take_parity_text_2,
        take_parity_text_3,
        take_parity_date,
        take_parity_time,
        take_parity_timestamp,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (config_key)
    DO UPDATE SET
        parity_boolean   = EXCLUDED.parity_boolean,
        parity_text_1    = EXCLUDED.parity_text_1,
        parity_text_2    = EXCLUDED.parity_text_2,
        parity_text_3    = EXCLUDED.parity_text_3,
        parity_date      = EXCLUDED.parity_date,
        parity_time      = EXCLUDED.parity_time,
        parity_timestamp = EXCLUDED.parity_timestamp;

END;
$$;


-- ---- FUNC pms.finyr_master_validate.txt
CREATE OR REPLACE FUNCTION pms.finyr_master_validate
(
    take_finyr              VARCHAR,
    take_finyr_st_date      DATE,
    take_finyr_end_date     DATE,
    take_finyr_is_current   BOOLEAN
)
RETURNS VARCHAR
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate financial year
    IF take_finyr IS NULL OR TRIM(take_finyr) = '' THEN
        RETURN 'Financial year is required';
    END IF;

    -- Validate duplicate financial year
    IF EXISTS (
        SELECT 1
        FROM pms.finyr_master
        WHERE finyr = take_finyr
    ) THEN
        RETURN 'Financial year already exists';
    END IF;

    -- Validate start date
    IF take_finyr_st_date IS NULL THEN
        RETURN 'Financial year start date is required';
    END IF;

    -- Validate end date
    IF take_finyr_end_date IS NULL THEN
        RETURN 'Financial year end date is required';
    END IF;

    -- Validate date range
    IF take_finyr_end_date < take_finyr_st_date THEN
        RETURN 'Financial year end date cannot be before start date';
    END IF;

    -- Validate overlapping financial years
    IF EXISTS (
        SELECT 1
        FROM pms.finyr_master
        WHERE take_finyr_st_date <= finyr_end_date
          AND take_finyr_end_date >= finyr_st_date
    ) THEN
        RETURN 'Financial year dates overlap with an existing financial year';
    END IF;

    RETURN 'SUCCESS';

END;
$$;

-- ---- PROC_pms.mst_finyr_insert.txt
CREATE OR REPLACE PROCEDURE pms.mst_finyr_insert
(
    take_finyr              VARCHAR,
    take_finyr_st_date      DATE,
    take_finyr_end_date     DATE,
    take_finyr_is_current   BOOLEAN
)
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pms.finyr_master
        WHERE finyr = take_finyr
    ) THEN
        RAISE EXCEPTION 'Financial year % already exists', take_finyr;
    END IF;
    IF take_finyr_st_date IS NULL OR take_finyr_end_date IS NULL THEN
        RAISE EXCEPTION 'Financial year start date and end date are required';
    END IF;

    IF take_finyr_end_date < take_finyr_st_date THEN
        RAISE EXCEPTION 'Financial year end date cannot be before start date';
    END IF;

    IF take_finyr_is_current = TRUE THEN
        UPDATE pms.finyr_master
        SET finyr_is_current = FALSE;
    END IF;
    INSERT INTO pms.finyr_master
    (
        finyr,
        finyr_st_date,
        finyr_end_date,
        finyr_is_current,
        rec_created_at
    )
    VALUES
    (
        take_finyr,
        take_finyr_st_date,
        take_finyr_end_date,
        COALESCE(take_finyr_is_current, FALSE),
        CURRENT_TIMESTAMP
    );

END;
$$;

-- ---- FUNC_pms.company_master_validate.txt
CREATE OR REPLACE FUNCTION pms.company_master_validate
(
    take_sl          INTEGER,
    take_name        VARCHAR(300),
    take_mobile      VARCHAR(15),
    take_email       VARCHAR(200),
    take_pin         VARCHAR(10),
    take_shortname   VARCHAR(50),
    take_fin_st_dt   DATE
)
RETURNS TABLE
(
    is_valid BOOLEAN,
    message  VARCHAR(500)
)
LANGUAGE plpgsql
AS $$
BEGIN

    -- Validate SL
    IF take_sl IS NULL OR take_sl <= 0 THEN
        RETURN QUERY SELECT FALSE, 'Invalid company SL number'::VARCHAR(500);
        RETURN;
    END IF;

    -- Validate Company Name
    IF take_name IS NULL OR TRIM(take_name) = '' THEN
        RETURN QUERY SELECT FALSE, 'Company name is required'::VARCHAR(500);
        RETURN;
    END IF;

    -- Validate Mobile
    IF take_mobile IS NOT NULL AND TRIM(take_mobile) <> '' THEN
        IF take_mobile !~ '^[0-9]{10,15}$' THEN
            RETURN QUERY SELECT FALSE, 'Invalid mobile number'::VARCHAR(500);
            RETURN;
        END IF;
    END IF;

    -- Validate Email
    IF take_email IS NOT NULL AND TRIM(take_email) <> '' THEN
        IF take_email !~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' THEN
            RETURN QUERY SELECT FALSE, 'Invalid email address'::VARCHAR(500);
            RETURN;
        END IF;
    END IF;

    -- Validate PIN
    IF take_pin IS NOT NULL AND TRIM(take_pin) <> '' THEN
        IF take_pin !~ '^[0-9]{6}$' THEN
            RETURN QUERY SELECT FALSE, 'Invalid PIN code'::VARCHAR(500);
            RETURN;
        END IF;
    END IF;

    -- Validate Short Name
    IF take_shortname IS NULL OR TRIM(take_shortname) = '' THEN
        RETURN QUERY SELECT FALSE, 'Company short name is required'::VARCHAR(500);
        RETURN;
    END IF;

    -- Validate Financial Start Date
    IF take_fin_st_dt IS NULL THEN
        RETURN QUERY SELECT FALSE, 'Financial start date is required'::VARCHAR(500);
        RETURN;
    END IF;

    -- All validations passed
    RETURN QUERY SELECT TRUE, 'Validation successful'::VARCHAR(500);

END;
$$;

-- ---- PROC_pms.company_master_insert.txt
CREATE OR REPLACE PROCEDURE pms.company_master_insert
(
    take_sl              INTEGER,
    take_name            VARCHAR(300),
    take_address         VARCHAR(300),
    take_city            VARCHAR(200),
    take_state           VARCHAR(200),
    take_country         VARCHAR(200),
    take_pin             VARCHAR(10),
    take_mobile          VARCHAR(15),
    take_email           VARCHAR(200),
    take_website         VARCHAR(150),
    take_header_1        VARCHAR(200),
    take_header_2        VARCHAR(200),
    take_shortname       VARCHAR(50),
    take_fin_st_dt       DATE,
    take_time_zone       TIMESTAMPTZ
)
LANGUAGE plpgsql
AS $$
BEGIN

    INSERT INTO pms.company_master
    (
        sl,
        name,
        address,
        city,
        state,
        country,
        pin,
        mobile,
        email,
        website,
        header_1,
        header_2,
        shortname,
        fin_st_dt,
        time_zone,
        rec_created_at
    )
    VALUES
    (
        take_sl,
        take_name,
        take_address,
        take_city,
        take_state,
        take_country,
        take_pin,
        take_mobile,
        take_email,
        take_website,
        take_header_1,
        take_header_2,
        take_shortname,
        take_fin_st_dt,
        take_time_zone,
        CURRENT_TIMESTAMP
    )
    ON CONFLICT (sl)
    DO UPDATE SET
        name       = EXCLUDED.name,
        address    = EXCLUDED.address,
        city       = EXCLUDED.city,
        state      = EXCLUDED.state,
        country    = EXCLUDED.country,
        pin        = EXCLUDED.pin,
        mobile     = EXCLUDED.mobile,
        email      = EXCLUDED.email,
        website    = EXCLUDED.website,
        header_1   = EXCLUDED.header_1,
        header_2   = EXCLUDED.header_2,
        shortname  = EXCLUDED.shortname,
        fin_st_dt  = EXCLUDED.fin_st_dt,
        time_zone  = EXCLUDED.time_zone;

END;
$$;


-- ---- tables_VER_01_1.txt (v1.01): mst_company_tax_details
CREATE TABLE pms.mst_company_tax_details
(
    sl                  INTEGER PRIMARY KEY,
    pan_enabled         BOOLEAN DEFAULT FALSE,
    pan_enabled_dt      DATE,
    pan                 VARCHAR(10),
    gstin_enabled       BOOLEAN DEFAULT FALSE,
    gstin_enabled_dt    DATE,
    gstin               VARCHAR(15),
    gstin_state         VARCHAR(100),
    comm_mail_id        VARCHAR(150),
    comm_mail_pwd       VARCHAR(150),
    comm_mobile         VARCHAR(10),
    comm_whatsapp       VARCHAR(10),
    rec_created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT cons1_mst_company_tax_details_company
        FOREIGN KEY (sl)
        REFERENCES pms.company_master(sl)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

-- ---- tables_VER_01_1.txt (v1.01): mst_trust
CREATE TABLE pms.mst_trust
(
    trust_code      VARCHAR(3) PRIMARY KEY,
    trust_desc      VARCHAR(100) NOT NULL UNIQUE,
    wrk_st_finyr    VARCHAR,
    wrk_st_date     DATE,
    rec_created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_update_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_trust_code
        CHECK (trust_code ~ '^[A-Z]{3}$')
);

-- ---- PROC_pms.mst_company_tax_details_inser.txt
CREATE OR REPLACE PROCEDURE pms.mst_company_tax_details_insert
(
    take_sl                  INTEGER,
    take_pan_enabled         BOOLEAN,
    take_pan_enabled_dt      DATE,
    take_pan                 VARCHAR(10),
    take_gstin_enabled       BOOLEAN,
    take_gstin_enabled_dt    DATE,
    take_gstin               VARCHAR(15),
    take_gstin_state         VARCHAR(100),
    take_comm_mail_id        VARCHAR(150),
    take_comm_mail_pwd       VARCHAR(150),
    take_comm_mobile         VARCHAR(10),
    take_comm_whatsapp       VARCHAR(10)
)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO pms.mst_company_tax_details
    (
        sl,
        pan_enabled,
        pan_enabled_dt,
        pan,
        gstin_enabled,
        gstin_enabled_dt,
        gstin,
        gstin_state,
        comm_mail_id,
        comm_mail_pwd,
        comm_mobile,
        comm_whatsapp,
        rec_created_at
    )
    VALUES
    (
        take_sl,
        take_pan_enabled,
        take_pan_enabled_dt,
        take_pan,
        take_gstin_enabled,
        take_gstin_enabled_dt,
        take_gstin,
        take_gstin_state,
        take_comm_mail_id,
        take_comm_mail_pwd,
        take_comm_mobile,
        take_comm_whatsapp,
        CURRENT_TIMESTAMP
    )

    ON CONFLICT (sl)
    DO UPDATE SET
        pan_enabled      = EXCLUDED.pan_enabled,
        pan_enabled_dt   = EXCLUDED.pan_enabled_dt,
        pan              = EXCLUDED.pan,
        gstin_enabled    = EXCLUDED.gstin_enabled,
        gstin_enabled_dt = EXCLUDED.gstin_enabled_dt,
        gstin            = EXCLUDED.gstin,
        gstin_state      = EXCLUDED.gstin_state,
        comm_mail_id     = EXCLUDED.comm_mail_id,
        comm_mail_pwd    = EXCLUDED.comm_mail_pwd,
        comm_mobile      = EXCLUDED.comm_mobile,
        comm_whatsapp    = EXCLUDED.comm_whatsapp;
END;
$$;

-- ---- PROPOSED, not in Uncle's bundle yet: PROC_pms.mst_finyr_set_current
-- Written for Uncle to review and install on live (he agreed on 9 Nov 2026: "go ahead").
-- Replace with his installed text before the PUT /financial-years/current PR leaves draft.
CREATE OR REPLACE PROCEDURE pms.mst_finyr_set_current
(
    take_finyr   VARCHAR
)
LANGUAGE plpgsql
AS $$
BEGIN
    IF take_finyr IS NULL OR TRIM(take_finyr) = '' THEN
        RAISE EXCEPTION 'Financial year is required';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pms.finyr_master
        WHERE finyr = take_finyr
    ) THEN
        RAISE EXCEPTION 'Financial year % does not exist', take_finyr;
    END IF;

    -- Exactly one current year: the chosen one on, every other one off.
    UPDATE pms.finyr_master
    SET finyr_is_current = (finyr = take_finyr)
    WHERE finyr_is_current IS DISTINCT FROM (finyr = take_finyr);

END;
$$;
