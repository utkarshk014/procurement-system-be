package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/utkarshk014/procurement-system-be/src/delivery/rest"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
	"github.com/utkarshk014/procurement-system-be/src/packages/database"
	"github.com/utkarshk014/procurement-system-be/src/utils"
	"github.com/xuri/excelize/v2"
)

// TestEndToEnd boots a throwaway PostgreSQL 11 with the DBA's pms objects
// (db/legacy), applies our migrations and drives the real router over HTTP:
// admin bootstrap → tenant onboarding → tenant admin → setup wizard
// (financial year, then the single company) → token refresh and logout.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Docker")
	}
	ctx := context.Background()

	pg, err := tcpostgres.Run(ctx, "postgres:11-alpine",
		tcpostgres.WithDatabase("esispms"),
		tcpostgres.WithUsername("pms"),
		tcpostgres.WithPassword("pms"),
		tcpostgres.WithInitScripts(legacyScripts()...),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, pg)
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dsnB := createTenantDatabase(t, ctx, dsn, "esis_b")
	dsnC := createTenantDatabase(t, ctx, dsn, "esis_c")
	const unreachableDSN = "postgres://pms:pms@127.0.0.1:1/nowhere?sslmode=disable&connect_timeout=1"

	cfg := &utils.Config{
		AppEnv:             "test",
		ControlDatabaseURL: dsn,
		TenantDatabases:    map[string]string{"esis": dsn, "esis_b": dsnB, "esis_c": dsnC, "down": unreachableDSN},
		DefaultTimeZone:    "Asia/Kolkata",
		JWTAccessSecret:    "test-access-secret-test-access-secret-0001",
		JWTRefreshSecret:   "test-refresh-secret-test-refresh-secret-0002",
		JWTIssuer:          "pms-api",
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
		RequestTimeout:     15 * time.Second,
		LoginRateLimit:     1000,
		FYFutureYears:      1,
	}

	_, err = database.NewManager(ctx, dsn, map[string]string{"esis": dsn, "alias": dsn + "&application_name=x"})
	require.ErrorContains(t, err, "point at the same database", "two keys on one database are refused")

	dbm, err := database.NewManager(ctx, cfg.ControlDatabaseURL, cfg.TenantDatabases)
	require.NoError(t, err, "an unreachable tenant database must not stop startup")
	t.Cleanup(dbm.Close)
	assert.Contains(t, dbm.Unreachable(), "down")

	applied, err := database.MigrateUp(ctx, dbm.Control())
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, applied)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repos := repository.NewRepositories(dbm)
	usecases := usecase.NewUsecases(cfg, repos, dbm, logger)
	srv := httptest.NewServer(rest.NewRouter(cfg, usecases, dbm.Control(), logger))
	t.Cleanup(srv.Close)
	api := &client{t: t, base: srv.URL}

	// Bootstrap: the first platform admin comes from the CLI path, not HTTP.
	_, err = usecases.Auth.CreateUser(ctx, domain_auth.CreateUserInput{
		Email: "Root@Platform.test", FullName: "Platform Root", Password: "root-password-123",
		Role: domain.RolePlatformAdmin,
	})
	require.NoError(t, err)

	t.Run("health is public", func(t *testing.T) {
		res := api.do("GET", "/healthz", "", nil)
		assert.Equal(t, 200, res.status)
	})

	t.Run("protected routes need a token", func(t *testing.T) {
		res := api.do("GET", "/api/v1/company/", "", nil)
		assert.Equal(t, 401, res.status)
		assert.Equal(t, "unauthorized", res.errCode())
		res = api.do("GET", "/api/v1/company/", "not-a-jwt", nil)
		assert.Equal(t, 401, res.status)
	})

	t.Run("login failures look the same", func(t *testing.T) {
		wrongPass := api.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "root@platform.test", "password": "nope-nope-nope"})
		noUser := api.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "ghost@platform.test", "password": "nope-nope-nope"})
		assert.Equal(t, 401, wrongPass.status)
		assert.Equal(t, 401, noUser.status)
		assert.Equal(t, wrongPass.errMessage(), noUser.errMessage())
	})

	rootTokens := api.login("ROOT@platform.test", "root-password-123")
	root := rootTokens["access_token"].(string)

	t.Run("me returns the platform admin", func(t *testing.T) {
		res := api.do("GET", "/api/v1/auth/me", root, nil)
		require.Equal(t, 200, res.status)
		assert.Equal(t, "platform_admin", res.data()["role"])
		assert.Nil(t, res.data()["tenant"])
	})

	t.Run("platform admin cannot use company routes", func(t *testing.T) {
		assert.Equal(t, 403, api.do("GET", "/api/v1/company/", root, nil).status)
	})

	var tenantID float64
	t.Run("tenant onboarding", func(t *testing.T) {
		res := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "esis", "name": "ESIS", "db_key": "esis"})
		require.Equal(t, 201, res.status, res.body)
		tenantID = res.data()["id"].(float64)
		assert.Equal(t, "ESIS", res.data()["code"], "code is upper-cased")

		dup := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "ESIS", "name": "Again", "db_key": "esis_b"})
		assert.Equal(t, 409, dup.status)
		assert.Equal(t, "is already taken", dup.fieldErrors()["code"])

		sameDB := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "OTHER", "name": "Other", "db_key": "esis"})
		assert.Equal(t, 409, sameDB.status)
		assert.Contains(t, sameDB.fieldErrors(), "db_key")

		unknownDB := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "NEW", "name": "New", "db_key": "nowhere"})
		assert.Equal(t, 422, unknownDB.status)
		assert.Contains(t, unknownDB.fieldErrors(), "db_key")

		list := api.do("GET", "/api/v1/admin/tenants", root, nil)
		assert.Len(t, list.list(), 1)
	})

	t.Run("tenant admin creation", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/admin/tenants/%d/admins", int(tenantID))
		short := api.do("POST", path, root, map[string]any{"email": "x@esis.in", "full_name": "X", "password": "short"})
		assert.Equal(t, 422, short.status)
		assert.Equal(t, "must be at least 10 characters", short.fieldErrors()["password"])

		res := api.do("POST", path, root, map[string]any{"email": "Admin@ESIS.in", "full_name": "ESIS Admin", "password": "esis-password-123"})
		require.Equal(t, 201, res.status, res.body)
		assert.Equal(t, "admin@esis.in", res.data()["email"])
		assert.Equal(t, "tenant_admin", res.data()["role"])
		assert.NotContains(t, res.body, "password_hash")

		dup := api.do("POST", path, root, map[string]any{"email": "admin@esis.in", "full_name": "Again", "password": "esis-password-123"})
		assert.Equal(t, 409, dup.status)

		missing := api.do("POST", "/api/v1/admin/tenants/999/admins", root, map[string]any{"email": "y@esis.in", "full_name": "Y", "password": "esis-password-123"})
		assert.Equal(t, 404, missing.status)
	})

	esisTokens := api.login("admin@esis.in", "esis-password-123")
	esis := esisTokens["access_token"].(string)

	t.Run("tenant admin sees their tenant and not admin routes", func(t *testing.T) {
		me := api.do("GET", "/api/v1/auth/me", esis, nil)
		tenant := me.data()["tenant"].(map[string]any)
		assert.Equal(t, "ESIS", tenant["code"])
		assert.Equal(t, 403, api.do("GET", "/api/v1/admin/tenants", esis, nil).status)
	})

	// "Today" decides which financial years may be created.
	ist, _ := time.LoadLocation("Asia/Kolkata")
	curStart := domain_finyear.StartYearFor(time.Now().In(ist))
	curFY := domain_finyear.FinYrLabel(curStart)
	prevFY := domain_finyear.FinYrLabel(curStart - 1)
	nextFY := domain_finyear.FinYrLabel(curStart + 1)
	prevStartDate := fmt.Sprintf("%d-04-01", curStart-1)
	curStartDate := fmt.Sprintf("%d-04-01", curStart)

	company := map[string]any{
		"name": "  ESIS Educational Society ", "address": "12 MG Road", "city": "Bengaluru",
		"state": "karnataka", "pin": "560001", "mobile": "+91 98765-43210",
		"email": "Office@ESIS.in", "website": "esis.in/", "header_1": "Head Office",
		"shortname": "ESIS",
	}

	t.Run("setup starts at the financial year step", func(t *testing.T) {
		res := api.do("GET", "/api/v1/setup/status", esis, nil)
		require.Equal(t, 200, res.status, res.body)
		assert.Equal(t, "financial_year", res.data()["next_step"])
		assert.Equal(t, false, res.data()["financial_year_done"])
		assert.Equal(t, false, res.data()["company_done"])
		assert.Equal(t, 404, api.do("GET", "/api/v1/financial-years/current", esis, nil).status)
		assert.Equal(t, 404, api.do("GET", "/api/v1/company/", esis, nil).status)
	})

	t.Run("company cannot be saved before a financial year", func(t *testing.T) {
		res := api.do("PUT", "/api/v1/company/", esis, company)
		assert.Equal(t, 409, res.status)
		assert.Equal(t, "Create the financial year first", res.errMessage())
	})

	t.Run("financial year options are previous, current, then future, oldest first", func(t *testing.T) {
		res := api.do("GET", "/api/v1/financial-years/options", esis, nil)
		require.Equal(t, 200, res.status)
		opts := res.list()
		require.Len(t, opts, 3)
		prev, cur, next := opts[0].(map[string]any), opts[1].(map[string]any), opts[2].(map[string]any)
		assert.Equal(t, prevFY, prev["finyr"])
		assert.Equal(t, "previous", prev["kind"])
		assert.Equal(t, curFY, cur["finyr"])
		assert.Equal(t, curStartDate, cur["finyr_st_date"])
		assert.Equal(t, "current", cur["kind"])
		assert.Equal(t, nextFY, next["finyr"])
		assert.Equal(t, "future", next["kind"])
		assert.Equal(t, false, next["exists"])
	})

	t.Run("financial year rules", func(t *testing.T) {
		tooFar := domain_finyear.FinYrLabel(curStart + 2)
		for in, field := range map[string]string{"abc": "finyr", "2026-2028": "finyr", "1999-2000": "finyr", tooFar: "finyr"} {
			res := api.do("POST", "/api/v1/financial-years/validate", esis, map[string]any{"finyr": in})
			assert.Equal(t, 422, res.status, in)
			assert.Contains(t, res.fieldErrors(), field, in)
		}
		res := api.do("POST", "/api/v1/financial-years/validate", esis, map[string]any{
			"finyr": curFY, "finyr_st_date": fmt.Sprintf("%d-05-01", curStart),
		})
		assert.Equal(t, 422, res.status)
		assert.Contains(t, res.fieldErrors(), "finyr_st_date")

		ok := api.do("POST", "/api/v1/financial-years/validate", esis, map[string]any{"finyr": curFY})
		require.Equal(t, 200, ok.status, ok.body)
		fy := ok.data()["financial_year"].(map[string]any)
		assert.Equal(t, curStartDate, fy["finyr_st_date"])
		assert.Equal(t, true, fy["finyr_is_current"], "first year becomes current")
	})

	t.Run("create the financial year through the DBA procedure", func(t *testing.T) {
		res := api.do("POST", "/api/v1/financial-years/", esis, map[string]any{"finyr": curFY})
		require.Equal(t, 201, res.status, res.body)
		assert.Equal(t, curFY, res.data()["finyr"])
		assert.Equal(t, true, res.data()["finyr_is_current"])
		assert.NotEmpty(t, res.data()["rec_created_at"])

		dup := api.do("POST", "/api/v1/financial-years/", esis, map[string]any{"finyr": curFY})
		assert.Equal(t, 409, dup.status)
		assert.Equal(t, "Financial year already exists", dup.fieldErrors()["finyr"])

		prev := api.do("POST", "/api/v1/financial-years/", esis, map[string]any{"finyr": prevFY})
		require.Equal(t, 201, prev.status, prev.body)
		assert.Equal(t, false, prev.data()["finyr_is_current"], "current stays current")

		cur := api.do("GET", "/api/v1/financial-years/current", esis, nil)
		assert.Equal(t, curFY, cur.data()["finyr"])
		assert.Len(t, api.do("GET", "/api/v1/financial-years/", esis, nil).list(), 2)

		var n int
		require.NoError(t, dbm.Control().QueryRow(ctx,
			`SELECT count(*) FROM pms.finyr_master WHERE finyr_is_current`).Scan(&n))
		assert.Equal(t, 1, n, "exactly one current year in the DBA's table")

		st := api.do("GET", "/api/v1/setup/status", esis, nil)
		assert.Equal(t, "company", st.data()["next_step"])
		assert.Equal(t, curFY, st.data()["current_financial_year"])
	})

	t.Run("a future year can be created and does not become current", func(t *testing.T) {
		res := api.do("POST", "/api/v1/financial-years/", esis, map[string]any{"finyr": nextFY})
		require.Equal(t, 201, res.status, res.body)
		assert.Equal(t, nextFY, res.data()["finyr"])
		assert.Equal(t, false, res.data()["finyr_is_current"])
		assert.Equal(t, fmt.Sprintf("%d-04-01", curStart+1), res.data()["finyr_st_date"])

		assert.Equal(t, curFY, api.do("GET", "/api/v1/financial-years/current", esis, nil).data()["finyr"])
		assert.Len(t, api.do("GET", "/api/v1/financial-years/", esis, nil).list(), 3)
		opts := api.do("GET", "/api/v1/financial-years/options", esis, nil).list()
		assert.Equal(t, true, opts[2].(map[string]any)["exists"])
	})

	t.Run("make another year current through the DBA procedure", func(t *testing.T) {
		res := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": prevFY})
		require.Equal(t, 200, res.status, res.body)
		assert.Equal(t, prevFY, res.data()["finyr"])
		assert.Equal(t, true, res.data()["finyr_is_current"])

		var current string
		require.NoError(t, dbm.Control().QueryRow(ctx,
			`SELECT string_agg(finyr, ',') FROM pms.finyr_master WHERE finyr_is_current`).Scan(&current))
		assert.Equal(t, prevFY, current, "exactly one current year")
		assert.Equal(t, prevFY, api.do("GET", "/api/v1/setup/status", esis, nil).data()["current_financial_year"])

		same := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": prevFY})
		assert.Equal(t, 200, same.status, "already current: 200, no change")

		missing := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": "1999-2000"})
		assert.Equal(t, 422, missing.status)
		assert.Equal(t, "must be an existing financial year", missing.fieldErrors()["finyr"])
		bad := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": "abc"})
		assert.Equal(t, 422, bad.status)
		assert.Contains(t, bad.fieldErrors(), "finyr")
		extra := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": curFY, "finyr_is_current": true})
		assert.Equal(t, 422, extra.status, "only finyr is accepted")

		back := api.do("PUT", "/api/v1/financial-years/current", esis, map[string]any{"finyr": curFY})
		require.Equal(t, 200, back.status, back.body)
		list := api.do("GET", "/api/v1/financial-years/", esis, nil).list()
		n := 0
		for _, y := range list {
			if y.(map[string]any)["finyr_is_current"] == true {
				n++
			}
		}
		assert.Equal(t, 1, n, "the list shows exactly one current year")
		assert.Equal(t, curFY, api.do("GET", "/api/v1/financial-years/current", esis, nil).data()["finyr"])
	})

	t.Run("company validate reports every problem at once", func(t *testing.T) {
		bad := map[string]any{"name": "", "pin": "5600", "mobile": "123", "email": "x@", "fin_st_dt": "2026-08-15"}
		res := api.do("POST", "/api/v1/company/validate", esis, bad)
		require.Equal(t, 422, res.status)
		f := res.fieldErrors()
		for _, field := range []string{"name", "shortname", "pin", "mobile", "email", "fin_st_dt"} {
			assert.Contains(t, f, field)
		}

		ok := api.do("POST", "/api/v1/company/validate", esis, company)
		require.Equal(t, 200, ok.status, ok.body)
		c := ok.data()["company"].(map[string]any)
		assert.Equal(t, true, ok.data()["valid"])
		assert.Equal(t, float64(1), c["sl"])
		assert.Equal(t, "ESIS Educational Society", c["name"])
		assert.Equal(t, "9876543210", c["mobile"])
		assert.Equal(t, "Karnataka", c["state"])
		assert.Equal(t, "India", c["country"])
		assert.Equal(t, curStartDate, c["fin_st_dt"], "defaults to the current financial year")

		unknown := api.do("POST", "/api/v1/company/validate", esis, map[string]any{"name": "X", "sl": 5})
		assert.Equal(t, 422, unknown.status)
		assert.Equal(t, "is not an accepted field", unknown.fieldErrors()["sl"])
	})

	t.Run("company fin_st_dt must be the start of an existing financial year", func(t *testing.T) {
		missing := clone(company)
		missing["fin_st_dt"] = fmt.Sprintf("%d-04-01", curStart-3)
		res := api.do("POST", "/api/v1/company/validate", esis, missing)
		require.Equal(t, 422, res.status, res.body)
		assert.Equal(t, "must be the start of an existing financial year", res.fieldErrors()["fin_st_dt"])

		other := clone(company)
		other["fin_st_dt"] = prevStartDate
		ok := api.do("POST", "/api/v1/company/validate", esis, other)
		require.Equal(t, 200, ok.status, ok.body)
		assert.Equal(t, prevStartDate, ok.data()["company"].(map[string]any)["fin_st_dt"])
	})

	taxDetails := map[string]any{
		"pan_enabled": true, "pan_enabled_dt": curStartDate, "pan": "abcde1234f",
		"gstin_enabled": true, "gstin_enabled_dt": curStartDate, "gstin": "29ABCDE1234F1ZW", "gstin_state": "",
		"comm_mail_id": "Accounts@ESIS.in", "comm_mail_pwd": "mail-secret-1",
		"comm_mobile": "+91 98765-43210", "comm_whatsapp": "9876543210",
	}

	t.Run("GST states are listed by code", func(t *testing.T) {
		res := api.do("GET", "/api/v1/meta/gst-states", esis, nil)
		require.Equal(t, 200, res.status)
		states := res.list()
		require.Len(t, states, 37)
		assert.Equal(t, map[string]any{"code": "01", "name": "Jammu and Kashmir"}, states[0])
		assert.Contains(t, states, map[string]any{"code": "29", "name": "Karnataka"})
		assert.Equal(t, 401, api.do("GET", "/api/v1/meta/gst-states", "", nil).status)
	})

	t.Run("tax details need the company first", func(t *testing.T) {
		assert.Equal(t, 404, api.do("GET", "/api/v1/company/tax-details", esis, nil).status)
		res := api.do("PUT", "/api/v1/company/tax-details", esis, taxDetails)
		assert.Equal(t, 409, res.status, res.body)
		assert.Equal(t, "Save the company details first", res.errMessage())
		assert.Equal(t, 409, api.do("POST", "/api/v1/company/tax-details/validate", esis, taxDetails).status)
	})

	var created map[string]any
	t.Run("first save creates the single company with sl 1", func(t *testing.T) {
		res := api.do("PUT", "/api/v1/company/", esis, company)
		require.Equal(t, 201, res.status, res.body)
		created = res.data()
		assert.Equal(t, float64(1), created["sl"])
		assert.Equal(t, "office@esis.in", created["email"])
		assert.Equal(t, "esis.in", created["website"])
		assert.Equal(t, curStartDate, created["fin_st_dt"])
		assert.NotEmpty(t, created["time_zone"])
		assert.NotEmpty(t, created["rec_created_at"])

		var mobile, state string
		err := dbm.Control().QueryRow(ctx, `SELECT mobile, state FROM pms.company_master WHERE sl = 1`).Scan(&mobile, &state)
		require.NoError(t, err)
		assert.Equal(t, "9876543210", mobile)
		assert.Equal(t, "Karnataka", state)
	})

	t.Run("later saves update the same record", func(t *testing.T) {
		edit := clone(company)
		edit["city"] = "Mysuru"
		res := api.do("PUT", "/api/v1/company/", esis, edit)
		require.Equal(t, 200, res.status, res.body)
		assert.Equal(t, float64(1), res.data()["sl"])
		assert.Equal(t, "Mysuru", res.data()["city"])
		assert.Equal(t, created["time_zone"], res.data()["time_zone"])
		assert.Equal(t, created["rec_created_at"], res.data()["rec_created_at"])

		assert.Equal(t, "Mysuru", api.do("GET", "/api/v1/company/", esis, nil).data()["city"])

		var rows int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.company_master`).Scan(&rows))
		assert.Equal(t, 1, rows, "only ever one company row")
	})

	t.Run("tax details: save, keep or clear the mail password, never return it", func(t *testing.T) {
		storedPassword := func() *string {
			var pwd *string
			require.NoError(t, dbm.Control().QueryRow(ctx,
				`SELECT comm_mail_pwd FROM pms.mst_company_tax_details WHERE sl = 1`).Scan(&pwd))
			return pwd
		}

		res := api.do("PUT", "/api/v1/company/tax-details", esis, taxDetails)
		require.Equal(t, 201, res.status, res.body)
		d := res.data()
		assert.Equal(t, float64(1), d["sl"])
		assert.Equal(t, "ABCDE1234F", d["pan"])
		assert.Equal(t, "Karnataka", d["gstin_state"], "filled from the GSTIN's state code")
		assert.Equal(t, "accounts@esis.in", d["comm_mail_id"])
		assert.Equal(t, "9876543210", d["comm_mobile"])
		assert.Equal(t, curStartDate, d["pan_enabled_dt"])
		assert.Equal(t, true, d["comm_mail_pwd_set"])
		assert.NotContains(t, res.body, "mail-secret-1")
		assert.NotContains(t, d, "comm_mail_pwd")
		require.NotNil(t, storedPassword())
		assert.Equal(t, "mail-secret-1", *storedPassword())

		get := api.do("GET", "/api/v1/company/tax-details", esis, nil)
		require.Equal(t, 200, get.status)
		assert.NotContains(t, get.body, "mail-secret-1")
		assert.Equal(t, true, get.data()["comm_mail_pwd_set"])

		keep := clone(taxDetails)
		delete(keep, "comm_mail_pwd")
		keep["comm_whatsapp"] = "9123456780"
		res = api.do("PUT", "/api/v1/company/tax-details", esis, keep)
		require.Equal(t, 200, res.status, res.body)
		assert.Equal(t, true, res.data()["comm_mail_pwd_set"], "no password sent keeps the stored one")
		assert.Equal(t, "9123456780", res.data()["comm_whatsapp"])
		assert.Equal(t, "mail-secret-1", *storedPassword())

		keep["comm_mail_pwd"] = nil
		require.Equal(t, 200, api.do("PUT", "/api/v1/company/tax-details", esis, keep).status)
		assert.Equal(t, "mail-secret-1", *storedPassword(), "null keeps it too")

		cleared := clone(keep)
		cleared["comm_mail_pwd"] = ""
		res = api.do("PUT", "/api/v1/company/tax-details", esis, cleared)
		require.Equal(t, 200, res.status, res.body)
		assert.Equal(t, false, res.data()["comm_mail_pwd_set"])
		assert.Nil(t, storedPassword())

		empty := api.do("PUT", "/api/v1/company/tax-details", esis, map[string]any{})
		require.Equal(t, 200, empty.status, "nothing is mandatory")
		assert.Equal(t, "", empty.data()["pan"])
		assert.Nil(t, empty.data()["pan_enabled_dt"])

		var rows int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_company_tax_details`).Scan(&rows))
		assert.Equal(t, 1, rows)
	})

	t.Run("tax details report every problem in one 422", func(t *testing.T) {
		bad := map[string]any{
			"pan": "ABC12", "gstin": "29ABCDE1234F1ZX", "gstin_state": "Atlantis",
			"comm_mail_id": "x@", "comm_mobile": "12345", "comm_whatsapp": "5876543210",
		}
		res := api.do("PUT", "/api/v1/company/tax-details", esis, bad)
		require.Equal(t, 422, res.status, res.body)
		f := res.fieldErrors()
		assert.Equal(t, "must be a PAN like ABCDE1234F", f["pan"])
		assert.Equal(t, "is not a valid GSTIN", f["gstin"])
		for _, field := range []string{"gstin_state", "comm_mail_id", "comm_mobile", "comm_whatsapp"} {
			assert.Contains(t, f, field)
		}

		mismatch := clone(taxDetails)
		mismatch["pan"] = "ABCDE1234G"
		mismatch["gstin_state"] = "Kerala"
		res = api.do("POST", "/api/v1/company/tax-details/validate", esis, mismatch)
		require.Equal(t, 422, res.status, res.body)
		assert.Equal(t, "PAN inside GSTIN must match the PAN", res.fieldErrors()["gstin"])
		assert.Equal(t, "does not match GSTIN state code 29 (Karnataka)", res.fieldErrors()["gstin_state"])

		ok := api.do("POST", "/api/v1/company/tax-details/validate", esis, taxDetails)
		require.Equal(t, 200, ok.status, ok.body)
		assert.Equal(t, true, ok.data()["valid"])
		assert.Equal(t, "Karnataka", ok.data()["tax_details"].(map[string]any)["gstin_state"])

		for _, serverOwned := range []string{"sl", "comm_mail_pwd_set", "rec_created_at"} {
			body := clone(taxDetails)
			body[serverOwned] = 1
			res := api.do("PUT", "/api/v1/company/tax-details", esis, body)
			assert.Equal(t, 422, res.status, serverOwned)
			assert.Equal(t, "is not an accepted field", res.fieldErrors()[serverOwned], serverOwned)
		}
	})

	t.Run("concurrent saves still leave one row", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				c := clone(company)
				c["header_2"] = fmt.Sprintf("Branch %d", i)
				assert.Equal(t, 200, api.do("PUT", "/api/v1/company/", esis, c).status, "existing company is updated")
			}(i)
		}
		wg.Wait()
		var rows int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.company_master`).Scan(&rows))
		assert.Equal(t, 1, rows)
	})

	t.Run("setup is done", func(t *testing.T) {
		st := api.do("GET", "/api/v1/setup/status", esis, nil)
		assert.Equal(t, "done", st.data()["next_step"])
		assert.Equal(t, true, st.data()["company_done"])
	})

	t.Run("trusts: list, create, duplicates, rules, update", func(t *testing.T) {
		empty := api.do("GET", "/api/v1/trusts/", esis, nil)
		require.Equal(t, 200, empty.status, empty.body)
		assert.NotNil(t, empty.list(), "an empty list, not 404")
		assert.Len(t, empty.list(), 0)
		assert.Equal(t, 403, api.do("GET", "/api/v1/trusts/", root, nil).status, "platform admin cannot use tenant routes")

		sat := map[string]any{"trust_code": "sat", "trust_desc": "Sri Adichunchanagiri Trust", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate}
		ok := api.do("POST", "/api/v1/trusts/validate", esis, sat)
		require.Equal(t, 200, ok.status, ok.body)
		assert.Equal(t, true, ok.data()["valid"])
		assert.Equal(t, "SAT", ok.data()["trust"].(map[string]any)["trust_code"])
		assert.Len(t, api.do("GET", "/api/v1/trusts/", esis, nil).list(), 0, "validate writes nothing")

		res := api.do("POST", "/api/v1/trusts/", esis, sat)
		require.Equal(t, 201, res.status, res.body)
		assert.Equal(t, "SAT", res.data()["trust_code"], "sat is stored as SAT")
		assert.Equal(t, curFY, res.data()["wrk_st_finyr"])
		assert.Equal(t, curStartDate, res.data()["wrk_st_date"])
		assert.NotEmpty(t, res.data()["rec_created_at"])
		firstUpdate := res.data()["last_update_at"]
		require.NotEmpty(t, firstUpdate)

		var code string
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT trust_code FROM pms.mst_trust`).Scan(&code))
		assert.Equal(t, "SAT", code)

		again := api.do("POST", "/api/v1/trusts/", esis, sat)
		assert.Equal(t, 409, again.status)
		assert.Equal(t, "is already taken", again.fieldErrors()["trust_code"])

		sameDesc := api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "ACU", "trust_desc": "SRI ADICHUNCHANAGIRI TRUST", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
		})
		assert.Equal(t, 409, sameDesc.status, sameDesc.body)
		assert.Equal(t, "is already used by trust SAT", sameDesc.fieldErrors()["trust_desc"])

		unknownFY := api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "ACU", "trust_desc": "Adichunchanagiri University", "wrk_st_finyr": "1999-2000", "wrk_st_date": "1999-04-01",
		})
		assert.Equal(t, 422, unknownFY.status)
		assert.Equal(t, "must be an existing financial year", unknownFY.fieldErrors()["wrk_st_finyr"])

		outside := api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "ACU", "trust_desc": "Adichunchanagiri University", "wrk_st_finyr": curFY, "wrk_st_date": fmt.Sprintf("%d-04-01", curStart+1),
		})
		assert.Equal(t, 422, outside.status)
		assert.Contains(t, outside.fieldErrors()["wrk_st_date"], "must fall inside "+curFY)

		badCode := api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "AC1", "trust_desc": "", "wrk_st_finyr": "", "wrk_st_date": "",
		})
		assert.Equal(t, 422, badCode.status)
		f := badCode.fieldErrors()
		for _, field := range []string{"trust_code", "trust_desc", "wrk_st_finyr", "wrk_st_date"} {
			assert.Contains(t, f, field)
		}

		acu := api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "ACU", "trust_desc": "Adichunchanagiri University", "wrk_st_finyr": prevFY, "wrk_st_date": prevStartDate,
		})
		require.Equal(t, 201, acu.status, acu.body)

		list := api.do("GET", "/api/v1/trusts/", esis, nil).list()
		require.Len(t, list, 2)
		assert.Equal(t, "ACU", list[0].(map[string]any)["trust_code"], "sorted by code")
		assert.Equal(t, "SAT", list[1].(map[string]any)["trust_code"])

		one := api.do("GET", "/api/v1/trusts/sat", esis, nil)
		require.Equal(t, 200, one.status)
		assert.Equal(t, "Sri Adichunchanagiri Trust", one.data()["trust_desc"])
		assert.Equal(t, 404, api.do("GET", "/api/v1/trusts/XYZ", esis, nil).status)

		upd := api.do("PUT", "/api/v1/trusts/sat", esis, map[string]any{
			"trust_desc": "Sri Adichunchanagiri Shikshana Trust", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
		})
		require.Equal(t, 200, upd.status, upd.body)
		assert.Equal(t, "Sri Adichunchanagiri Shikshana Trust", upd.data()["trust_desc"])
		assert.NotEqual(t, firstUpdate, upd.data()["last_update_at"], "last_update_at moves")
		assert.Equal(t, res.data()["rec_created_at"], upd.data()["rec_created_at"])

		taken := api.do("PUT", "/api/v1/trusts/SAT", esis, map[string]any{
			"trust_desc": "adichunchanagiri university", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
		})
		assert.Equal(t, 409, taken.status)
		assert.Equal(t, "is already used by trust ACU", taken.fieldErrors()["trust_desc"])

		missing := api.do("PUT", "/api/v1/trusts/XYZ", esis, map[string]any{
			"trust_desc": "Nobody", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
		})
		assert.Equal(t, 404, missing.status)

		withCode := api.do("PUT", "/api/v1/trusts/SAT", esis, map[string]any{
			"trust_code": "SAX", "trust_desc": "Renamed", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
		})
		assert.Equal(t, 422, withCode.status, "a code never changes")
		assert.Equal(t, "is not an accepted field", withCode.fieldErrors()["trust_code"])

		var rows int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_trust`).Scan(&rows))
		assert.Equal(t, 2, rows)
	})

	t.Run("racing creates of one trust code: one wins, none overwrite", func(t *testing.T) {
		statuses := concurrently(6, func(i int) int {
			return api.do("POST", "/api/v1/trusts/", esis, map[string]any{
				"trust_code": "KHE", "trust_desc": fmt.Sprintf("KHE Trust %d", i), "wrk_st_finyr": curFY, "wrk_st_date": curStartDate,
			}).status
		})
		assert.Equal(t, []int{201, 409, 409, 409, 409, 409}, statuses)
		var n int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_trust WHERE trust_code = 'KHE'`).Scan(&n))
		assert.Equal(t, 1, n)
	})

	t.Run("places, branches and regions: list, create, duplicates, rules", func(t *testing.T) {
		for _, l := range []struct{ path, label string }{{"places", "Place"}, {"branches", "Branch"}, {"regions", "Region"}} {
			base := "/api/v1/" + l.path + "/"
			empty := api.do("GET", base, esis, nil)
			require.Equal(t, 200, empty.status, empty.body)
			assert.NotNil(t, empty.list(), "an empty list, not 404")
			assert.Equal(t, 403, api.do("GET", base, root, nil).status, "platform admin cannot use tenant routes")

			mys := map[string]any{"code": " mys ", "name": "  Mysore   " + l.label + " "}
			ok := api.do("POST", base+"validate", esis, mys)
			require.Equal(t, 200, ok.status, ok.body)
			assert.Equal(t, "MYS", ok.data()["entry"].(map[string]any)["code"])
			assert.Len(t, api.do("GET", base, esis, nil).list(), 0, "validate writes nothing")

			res := api.do("POST", base, esis, mys)
			require.Equal(t, 201, res.status, res.body)
			assert.Equal(t, "MYS", res.data()["code"], "mys is stored as MYS")
			assert.Equal(t, "Mysore "+l.label, res.data()["name"], "spaces are tidied")
			assert.NotEmpty(t, res.data()["rec_created_at"])

			again := api.do("POST", base, esis, mys)
			assert.Equal(t, 409, again.status)
			assert.Equal(t, "is already taken", again.fieldErrors()["code"])

			sameName := api.do("POST", base, esis, map[string]any{"code": "MYZ", "name": "MYSORE " + strings.ToUpper(l.label)})
			assert.Equal(t, 409, sameName.status, sameName.body)
			assert.Equal(t, "is already used by MYS", sameName.fieldErrors()["name"])

			bad := api.do("POST", base, esis, map[string]any{"code": "1 a", "name": ""})
			assert.Equal(t, 422, bad.status)
			assert.Equal(t, "must be exactly 3 letters, e.g. MYS", bad.fieldErrors()["code"], "Uncle's check only counts 3 characters")
			assert.Equal(t, "is required", bad.fieldErrors()["name"])

			require.Equal(t, 201, api.do("POST", base, esis, map[string]any{"code": "BGN", "name": "B G Nagara " + l.label}).status)
			list := api.do("GET", base, esis, nil).list()
			require.Len(t, list, 2)
			assert.Equal(t, "BGN", list[0].(map[string]any)["code"], "sorted by name")

			one := api.do("GET", base+"mys", esis, nil)
			require.Equal(t, 200, one.status)
			assert.Equal(t, "Mysore "+l.label, one.data()["name"])
			assert.Equal(t, 404, api.do("GET", base+"XYZ", esis, nil).status)
			assert.Equal(t, 405, api.do("PUT", base+"MYS", esis, map[string]any{"name": "x"}).status, "no edit procedure yet")

			statuses := concurrently(4, func(i int) int {
				return api.do("POST", base, esis, map[string]any{"code": "RAC", "name": fmt.Sprintf("Race %s %d", l.label, i)}).status
			})
			assert.Equal(t, []int{201, 409, 409, 409}, statuses, l.path)

			// Different codes, names that differ only in case: the table's UNIQUE is case-sensitive,
			// so only the per-table lock stops two of these from being saved together.
			for round := 0; round < 5; round++ {
				statuses = concurrently(8, func(i int) int {
					name := fmt.Sprintf("Case race %s %d", l.label, round)
					if i%2 == 1 {
						name = strings.ToUpper(name)
					}
					code := string(rune('A'+round)) + string(rune('A'+i)) + "Z"
					return api.do("POST", base, esis, map[string]any{"code": code, "name": name}).status
				})
				assert.Equal(t, []int{201, 409, 409, 409, 409, 409, 409, 409}, statuses, "%s round %d", l.path, round)
			}
		}

		// A row written outside the app with two spaces still blocks the same name with one.
		_, err := dbm.Control().Exec(ctx, `INSERT INTO pms.mst_place (place_code, place_name) VALUES ('OLD', 'Old  Town')`)
		require.NoError(t, err)
		spaced := api.do("POST", "/api/v1/places/", esis, map[string]any{"code": "OLT", "name": "old town"})
		assert.Equal(t, 409, spaced.status, spaced.body)
		assert.Equal(t, "is already used by OLD", spaced.fieldErrors()["name"])
	})

	t.Run("institutions: next code, create, duplicates, rules, list", func(t *testing.T) {
		next := api.do("GET", "/api/v1/institutions/next-code?trust_code=sat&regn_code=mys", esis, nil)
		require.Equal(t, 200, next.status, next.body)
		assert.Equal(t, "SAT-001", next.data()["inst_code"])
		assert.Equal(t, "MYS-01", next.data()["inst_reg_code"])

		body := map[string]any{
			"trust_code": "SAT", "inst_code": "sat-001", "inst_reg_code": "mys-01",
			"inst_name": " BGS  Public School ", "inst_add": "Kuvempunagar", "inst_place": "Mysore (Kuvempunagar)",
			"place_code": "MYS", "branch_code": "MYS", "from_fin_yr": curFY,
			"inst_email": "Office@BGS.test", "inst_tel": "0821 2541234",
			"inst_head_name": "Dr. Head", "inst_head_desig": "Principal", "inst_head_mobile": "+91 98450 12345", "inst_head_email": "head@bgs.test",
			"inst_acnt_name": "Accounts", "inst_acnt_mobile": "09845012346",
			"inst_liason_name": "Liaison", "inst_liason_desig": "Officer",
		}
		ok := api.do("POST", "/api/v1/institutions/validate", esis, body)
		require.Equal(t, 200, ok.status, ok.body)
		assert.Equal(t, "SAT-001:MYS-01", ok.data()["institution"].(map[string]any)["inst_full_code"])
		assert.Equal(t, float64(0), api.do("GET", "/api/v1/institutions/", esis, nil).data()["total"], "validate writes nothing")

		res := api.do("POST", "/api/v1/institutions/", esis, body)
		require.Equal(t, 201, res.status, res.body)
		d := res.data()
		assert.Equal(t, "SAT-001", d["inst_code"])
		assert.Equal(t, "SAT-001:MYS-01", d["inst_full_code"], "14 characters, built by the server")
		assert.Equal(t, "BGS Public School", d["inst_name"])
		assert.Equal(t, "Sri Adichunchanagiri Shikshana Trust", d["trust_name"], "copied from the trust by the DBA's procedure")
		assert.Equal(t, "Mysore Place", d["place_name"])
		assert.Equal(t, "Mysore Branch", d["branch_name"])
		assert.Equal(t, "9845012345", d["inst_head_mobile"], "mobiles are stored as 10 digits")
		assert.Equal(t, "9845012346", d["inst_acnt_mobile"])
		assert.Equal(t, "office@bgs.test", d["inst_email"])
		assert.NotEmpty(t, d["reg_date"], "today when not sent")
		assert.Equal(t, curFY, d["from_fin_yr"])

		next = api.do("GET", "/api/v1/institutions/next-code?trust_code=SAT&regn_code=MYS", esis, nil)
		assert.Equal(t, "SAT-002", next.data()["inst_code"])
		assert.Equal(t, "MYS-02", next.data()["inst_reg_code"])

		again := api.do("POST", "/api/v1/institutions/", esis, body)
		assert.Equal(t, 409, again.status)
		assert.Equal(t, "is already used by SAT-001", again.fieldErrors()["inst_code"])

		sameReg := clone(body)
		sameReg["inst_code"] = "SAT-002"
		taken := api.do("POST", "/api/v1/institutions/", esis, sameReg)
		assert.Equal(t, 409, taken.status)
		assert.Equal(t, "is already used by SAT-001", taken.fieldErrors()["inst_reg_code"])

		bad := api.do("POST", "/api/v1/institutions/", esis, map[string]any{
			"trust_code": "SAT", "inst_code": "ACU-005", "inst_reg_code": "ZZZ-01", "inst_name": "",
			"place_code": "XYZ", "branch_code": "XYZ", "from_fin_yr": "1999-2000", "reg_date": "2999-01-01",
			"inst_head_mobile": "12345", "inst_email": "not-an-email",
		})
		assert.Equal(t, 422, bad.status, bad.body)
		f := bad.fieldErrors()
		assert.Equal(t, "must start with the trust code SAT-", f["inst_code"])
		assert.Equal(t, "must start with an existing region code (ZZZ is not one)", f["inst_reg_code"])
		assert.Equal(t, "is required", f["inst_name"])
		assert.Equal(t, "must be an existing place", f["place_code"])
		assert.Equal(t, "must be an existing branch", f["branch_code"])
		assert.Equal(t, "must be an existing financial year", f["from_fin_yr"])
		assert.Equal(t, "cannot be in the future", f["reg_date"])
		assert.Contains(t, f, "inst_head_mobile")
		assert.Contains(t, f, "inst_email")

		nine := clone(body)
		nine["inst_code"], nine["inst_reg_code"] = "SAT-00009", "MYS-09"
		assert.Equal(t, "must be the trust code, a dash and 3 digits, e.g. SAT-001",
			api.do("POST", "/api/v1/institutions/", esis, nine).fieldErrors()["inst_code"])

		unknownTrust := clone(body)
		unknownTrust["trust_code"], unknownTrust["inst_code"], unknownTrust["inst_reg_code"] = "ZZZ", "ZZZ-001", "MYS-05"
		assert.Equal(t, "must be an existing trust", api.do("POST", "/api/v1/institutions/", esis, unknownTrust).fieldErrors()["trust_code"])

		serverOwned := clone(body)
		serverOwned["inst_full_code"] = "SAT-009:MYS-09"
		assert.Equal(t, "is not an accepted field", api.do("POST", "/api/v1/institutions/", esis, serverOwned).fieldErrors()["inst_full_code"])

		second := clone(body)
		second["trust_code"], second["inst_code"], second["inst_reg_code"] = "ACU", "ACU-001", "BGN-01"
		second["inst_name"], second["place_code"], second["branch_code"] = "AIMS", "BGN", "BGN"
		second["from_fin_yr"] = prevFY
		require.Equal(t, 201, api.do("POST", "/api/v1/institutions/", esis, second).status)

		all := api.do("GET", "/api/v1/institutions/", esis, nil).data()
		assert.Equal(t, float64(2), all["total"])
		items := all["items"].([]any)
		assert.Equal(t, "ACU-001:BGN-01", items[0].(map[string]any)["inst_full_code"], "sorted by full code")
		assert.Equal(t, float64(1), api.do("GET", "/api/v1/institutions/?trust_code=sat", esis, nil).data()["total"])
		assert.Equal(t, float64(1), api.do("GET", "/api/v1/institutions/?q=public", esis, nil).data()["total"])
		assert.Equal(t, float64(1), api.do("GET", "/api/v1/institutions/?q=acu-001", esis, nil).data()["total"])
		// % and _ are searched as themselves, not as LIKE wildcards that would match every row.
		assert.Equal(t, float64(0), api.do("GET", "/api/v1/institutions/?q=%25", esis, nil).data()["total"])
		assert.Equal(t, float64(0), api.do("GET", "/api/v1/institutions/?q=_", esis, nil).data()["total"])
		paged := api.do("GET", "/api/v1/institutions/?page=2&page_size=1", esis, nil).data()
		assert.Len(t, paged["items"], 1)
		assert.Equal(t, "SAT-001:MYS-01", paged["items"].([]any)[0].(map[string]any)["inst_full_code"])
		assert.Equal(t, 400, api.do("GET", "/api/v1/institutions/?page=0", esis, nil).status)

		one := api.do("GET", "/api/v1/institutions/sat-001", esis, nil)
		require.Equal(t, 200, one.status)
		assert.Equal(t, "Dr. Head", one.data()["inst_head_name"])
		assert.Equal(t, 404, api.do("GET", "/api/v1/institutions/SAT-999", esis, nil).status)
		assert.Equal(t, 403, api.do("GET", "/api/v1/institutions/", root, nil).status)

		statuses := concurrently(4, func(i int) int {
			b := clone(body)
			b["inst_code"], b["inst_reg_code"] = "SAT-050", fmt.Sprintf("MYS-%02d", 50+i)
			return api.do("POST", "/api/v1/institutions/", esis, b).status
		})
		assert.Equal(t, []int{201, 409, 409, 409}, statuses)
	})

	t.Run("deletes go through the DBA's checks: in use is a 409, free is a 204", func(t *testing.T) {
		del := func(path string) response { return api.do("DELETE", path, esis, nil) }
		inUse := func(path, message string) {
			t.Helper()
			res := del(path)
			assert.Equal(t, 409, res.status, "%s: %s", path, res.body)
			assert.Equal(t, message, res.errMessage(), path)
		}
		gone := func(path, getPath string) {
			t.Helper()
			res := del(path)
			require.Equal(t, 204, res.status, "%s: %s", path, res.body)
			assert.Equal(t, 404, api.do("GET", getPath, esis, nil).status, "%s is gone", getPath)
			assert.Equal(t, 404, del(path).status, "deleting again is a 404")
		}

		assert.Equal(t, 403, api.do("DELETE", "/api/v1/places/RAC", root, nil).status, "platform admin cannot delete")
		assert.Equal(t, 401, api.do("DELETE", "/api/v1/places/RAC", "", nil).status)

		// Used by institutions SAT-001:MYS-01 and ACU-001:BGN-01.
		inUse("/api/v1/places/mys", "Cannot delete! Place code is already used in Institution Master (mst_inst)")
		inUse("/api/v1/branches/MYS", "Cannot delete! Branch code is already used in Institution Master (mst_inst)")
		inUse("/api/v1/regions/MYS", "Cannot delete! Region code is already used in Institution Master (mst_inst)")
		inUse("/api/v1/trusts/SAT", "Cannot delete! Trust code is already used in Institution Master (mst_inst)")
		for _, l := range []string{"places", "branches", "regions"} {
			gone("/api/v1/"+l+"/rac", "/api/v1/"+l+"/RAC")
		}
		assert.Equal(t, 404, del("/api/v1/places/M1S").status, "a bad code never reaches the DB")
		gone("/api/v1/trusts/khe", "/api/v1/trusts/KHE")

		inUse("/api/v1/financial-years/"+curFY, "The current financial year cannot be deleted. Make another year current first.")
		inUse("/api/v1/financial-years/"+prevFY, "Cannot delete! Financial year is already used in Trust Master (mst_trust)")
		res := del("/api/v1/financial-years/" + nextFY)
		require.Equal(t, 204, res.status, res.body)
		assert.Len(t, api.do("GET", "/api/v1/financial-years/", esis, nil).list(), 2)
		assert.Equal(t, 404, del("/api/v1/financial-years/"+nextFY).status)
		assert.Equal(t, 404, del("/api/v1/financial-years/garbage").status)

		var places int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_place WHERE place_code IN ('MYS','RAC')`).Scan(&places))
		assert.Equal(t, 1, places, "MYS kept, RAC deleted")
	})

	t.Run("institutions import from .xlsx: dry run, import, re-import, bad files", func(t *testing.T) {
		// The old system's own headers, a date cell, a number cell for a mobile, and every kind of bad row.
		file := xlsxFile(t, func(f *excelize.File) {
			rows := [][]any{
				{"ACM_SL", "ACM_CODE", "ACM_REG_CODE", "ACM_FULL_CODE", "ACM_INST_NAME", "ACM_INST_PLACE", "ACM_INST_BRANCH", "ACM_INST_TRUST", "ACM_INST_REG_SHORT_CODE", "Head Mobile", "Reg Date"},
				{1, "SAT-101", "MYS-31", "SAT-101:MYS-31", "Imported One", "mysore  PLACE", "MYS", "SAT", "MYS", 9845012345, nil},
				{2, "SAT-102", "MYS-32", "SAT-102:MYS-32", "Missing Place", "Kunigal", "Mysore Branch", "SAT", "MYS", nil, nil},
				{3, "SAT-101", "MYS-33", "SAT-101:MYS-33", "Same Code Again", "Mysore Place", "MYS", "SAT", "MYS", nil, nil},
				{4, "SAT-001", "MYS-34", "SAT-001:MYS-34", "Code In Use", "Mysore Place", "MYS", "SAT", "MYS", nil, nil},
				{5, "ACU-101", "BGN-31", "ACU-101:BGN-99", "Wrong Full Code", "B G Nagara Place", "BGN", "ACU", "BGN", nil, nil},
				{6, "ZZZ-001", "MYS-35", "ZZZ-001:MYS-35", "Unknown Trust", "Mysore Place", "MYS", "ZZZ", "MYS", nil, nil},
				{},
				{8, "ACU-102", "BGN-32", "ACU-102:BGN-32", "Missing Branch", "B G Nagara Place", "Unknown Branch", "ACU", "BGN", nil, nil},
				{9, "SAT-103", "MYS-37", "SAT-103:MYS-37", "Missing Place Again", "KUNIGAL", "MYS", "SAT", "MYS", nil, nil},
				{10, "SAT-104", "MYS-36", "SAT-104:MYS-36", "Imported Two", "MYS", "Mysore Branch", "SAT", "MYS", nil, nil},
			}
			for i, r := range rows {
				cell, _ := excelize.CoordinatesToCellName(1, i+1)
				require.NoError(t, f.SetSheetRow("Sheet1", cell, &r))
			}
			date, _ := f.NewStyle(&excelize.Style{NumFmt: 14})
			require.NoError(t, f.SetCellValue("Sheet1", "K11", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)))
			require.NoError(t, f.SetCellStyle("Sheet1", "K11", "K11", date))
		})
		var before int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_inst`).Scan(&before))

		dry := api.upload("/api/v1/institutions/import?dry_run=true", esis, "inst.xlsx", file)
		require.Equal(t, 200, dry.status, dry.body)
		d := dry.data()
		assert.Equal(t, true, d["dry_run"])
		assert.Equal(t, float64(9), d["total"], "the blank row is ignored")
		assert.Equal(t, float64(2), d["ready"])
		assert.Equal(t, float64(0), d["imported"])
		assert.Equal(t, float64(7), d["skipped"])
		problems := map[float64]map[string]string{}
		for _, p := range d["problems"].([]any) {
			pm := p.(map[string]any)
			fields := map[string]string{}
			for _, e := range pm["errors"].([]any) {
				em := e.(map[string]any)
				f, _ := em["field"].(string)
				fields[f] = em["message"].(string)
			}
			problems[pm["row"].(float64)] = fields
		}
		assert.Equal(t, `"Kunigal" is not in the places list`, problems[3]["place_code"])
		assert.Equal(t, "is also in row 2 of this file", problems[4]["inst_code"])
		assert.Equal(t, "is already used by SAT-001", problems[5]["inst_code"])
		assert.Equal(t, "is ACU-101:BGN-99, but the ACM and reg codes make ACU-101:BGN-31", problems[6]["inst_full_code"])
		assert.Equal(t, "must be an existing trust", problems[7]["trust_code"])
		assert.Equal(t, `"Unknown Branch" is not in the branches list`, problems[9]["branch_code"])
		assert.Contains(t, problems, float64(10))
		assert.NotContains(t, problems, float64(2))
		assert.NotContains(t, problems, float64(11))
		assert.Equal(t, []any{map[string]any{"name": "Kunigal", "rows": float64(2)}}, d["missing_places"], "spellings that differ only in case are one place")
		assert.Equal(t, []any{map[string]any{"name": "Unknown Branch", "rows": float64(1)}}, d["missing_branches"])
		var after int
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.mst_inst`).Scan(&after))
		assert.Equal(t, before, after, "a dry run writes nothing")

		imp := api.upload("/api/v1/institutions/import", esis, "inst.xlsx", file)
		require.Equal(t, 200, imp.status, imp.body)
		assert.Equal(t, float64(2), imp.data()["imported"])
		one := api.do("GET", "/api/v1/institutions/SAT-101", esis, nil).data()
		assert.Equal(t, "SAT-101:MYS-31", one["inst_full_code"])
		assert.Equal(t, "MYS", one["place_code"], "found by name, ignoring case and spaces")
		assert.Equal(t, "mysore PLACE", one["inst_place"], "the place text becomes the locality")
		assert.Equal(t, "9845012345", one["inst_head_mobile"], "a number cell stays 10 digits")
		assert.Equal(t, curFY, one["from_fin_yr"], "the current year when the file has none")
		two := api.do("GET", "/api/v1/institutions/SAT-104", esis, nil).data()
		assert.Equal(t, "MYS", two["branch_code"], "found by its name, Mysore Branch")
		assert.Equal(t, "", two["inst_place"], "a place given by code is not copied to the locality")
		assert.Equal(t, "2025-06-01", two["reg_date"], "a date cell is read as a date")

		again := api.upload("/api/v1/institutions/import", esis, "inst.xlsx", file)
		require.Equal(t, 200, again.status, again.body)
		assert.Equal(t, float64(0), again.data()["imported"], "importing the same file twice adds nothing")

		missing := xlsxFile(t, func(f *excelize.File) {
			require.NoError(t, f.SetSheetRow("Sheet1", "A1", &[]any{"trust_code", "inst_name"}))
			require.NoError(t, f.SetSheetRow("Sheet1", "A2", &[]any{"SAT", "X"}))
		})
		res := api.upload("/api/v1/institutions/import?dry_run=true", esis, "x.xlsx", missing)
		assert.Equal(t, 422, res.status)
		assert.Contains(t, res.fieldErrors()["file"], "Missing column(s): inst_code, inst_reg_code, place, branch")

		res = api.upload("/api/v1/institutions/import?dry_run=true", esis, "x.xls", []byte("not a workbook"))
		assert.Equal(t, 422, res.status)
		assert.Contains(t, res.fieldErrors()["file"], "not an Excel .xlsx workbook")
		assert.Equal(t, 403, api.upload("/api/v1/institutions/import?dry_run=true", root, "inst.xlsx", file).status)
		assert.Equal(t, 400, api.do("POST", "/api/v1/institutions/import", esis, map[string]any{}).status, "JSON is not a file upload")

		tpl := api.raw("GET", "/api/v1/institutions/import/template", esis, "", "")
		require.Equal(t, 200, tpl.status)
		wb, err := excelize.OpenReader(strings.NewReader(tpl.body))
		require.NoError(t, err)
		header, err := wb.GetRows("Institutions")
		require.NoError(t, err)
		assert.Equal(t, []string{"trust_code", "inst_code", "inst_reg_code", "inst_name", "place", "branch"}, header[0][:6])
		helpRows, _ := wb.GetRows("Help")
		assert.Greater(t, len(helpRows), 20)
	})

	t.Run("a delete and a create of the same trust cannot interleave", func(t *testing.T) {
		require.Equal(t, 201, api.do("POST", "/api/v1/trusts/", esis, map[string]any{
			"trust_code": "DLT", "trust_desc": "Delete race trust", "wrk_st_finyr": curFY, "wrk_st_date": curStartDate}).status)
		statuses := concurrently(6, func(i int) int { return api.do("DELETE", "/api/v1/trusts/DLT", esis, nil).status })
		assert.Equal(t, []int{204, 404, 404, 404, 404, 404}, statuses)
	})

	t.Run("second tenant on its own database is isolated, and its first saves are race-free", func(t *testing.T) {
		res := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "TB", "name": "Tenant B", "db_key": "esis_b"})
		require.Equal(t, 201, res.status, res.body)
		path := fmt.Sprintf("/api/v1/admin/tenants/%d/admins", int(res.data()["id"].(float64)))
		require.Equal(t, 201, api.do("POST", path, root, map[string]any{"email": "admin@tb.test", "full_name": "TB Admin", "password": "tb-password-123"}).status)
		tb := api.login("admin@tb.test", "tb-password-123")["access_token"].(string)

		assert.Equal(t, 404, api.do("GET", "/api/v1/company/", tb, nil).status, "tenant B does not see ESIS's company")
		assert.Equal(t, "financial_year", api.do("GET", "/api/v1/setup/status", tb, nil).data()["next_step"])

		statuses := concurrently(6, func(int) int {
			return api.do("POST", "/api/v1/financial-years/", tb, map[string]any{"finyr": curFY}).status
		})
		assert.Equal(t, []int{201, 409, 409, 409, 409, 409}, statuses, "one create wins, the rest see the duplicate")

		statuses = concurrently(8, func(i int) int {
			c := clone(company)
			c["name"], c["shortname"] = fmt.Sprintf("Tenant B Co %d", i), "TB"
			return api.do("PUT", "/api/v1/company/", tb, c).status
		})
		assert.Equal(t, []int{200, 200, 200, 200, 200, 200, 200, 201}, statuses, "exactly one first save creates the row")

		var bRows, aRows int
		var aName string
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT count(*) FROM pms.company_master`).Scan(&aRows))
		require.NoError(t, dbm.Control().QueryRow(ctx, `SELECT name FROM pms.company_master WHERE sl = 1`).Scan(&aName))
		poolB, err := dbm.Tenant("esis_b")
		require.NoError(t, err)
		require.NoError(t, poolB.QueryRow(ctx, `SELECT count(*) FROM pms.company_master`).Scan(&bRows))
		assert.Equal(t, 1, aRows)
		assert.Equal(t, 1, bRows)
		assert.Equal(t, "ESIS Educational Society", aName, "ESIS's row is untouched by tenant B")
		assert.Equal(t, "ESIS Educational Society", api.do("GET", "/api/v1/company/", esis, nil).data()["name"])
	})

	t.Run("racing creates of both years always leave today's year current", func(t *testing.T) {
		res := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "TC", "name": "Tenant C", "db_key": "esis_c"})
		require.Equal(t, 201, res.status, res.body)
		path := fmt.Sprintf("/api/v1/admin/tenants/%d/admins", int(res.data()["id"].(float64)))
		require.Equal(t, 201, api.do("POST", path, root, map[string]any{"email": "admin@tc.test", "full_name": "TC Admin", "password": "tc-password-123"}).status)
		tc := api.login("admin@tc.test", "tc-password-123")["access_token"].(string)
		poolC, err := dbm.Tenant("esis_c")
		require.NoError(t, err)

		for round := 0; round < 10; round++ {
			_, err := poolC.Exec(ctx, `DELETE FROM pms.finyr_master`)
			require.NoError(t, err)
			years := []string{prevFY, curFY}
			statuses := concurrently(2, func(i int) int {
				return api.do("POST", "/api/v1/financial-years/", tc, map[string]any{"finyr": years[i]}).status
			})
			require.Equal(t, []int{201, 201}, statuses)
			var current string
			require.NoError(t, poolC.QueryRow(ctx, `SELECT string_agg(finyr, ',') FROM pms.finyr_master WHERE finyr_is_current`).Scan(&current))
			require.Equal(t, curFY, current, "round %d", round)
		}
	})

	t.Run("a tenant whose database is down gets 503, others keep working", func(t *testing.T) {
		res := api.do("POST", "/api/v1/admin/tenants", root, map[string]any{"code": "DOWN", "name": "Down", "db_key": "down"})
		require.Equal(t, 201, res.status, res.body)
		path := fmt.Sprintf("/api/v1/admin/tenants/%d/admins", int(res.data()["id"].(float64)))
		require.Equal(t, 201, api.do("POST", path, root, map[string]any{"email": "admin@down.test", "full_name": "Down", "password": "down-password-1"}).status)
		down := api.login("admin@down.test", "down-password-1")["access_token"].(string)

		got := api.do("GET", "/api/v1/company/", down, nil)
		assert.Equal(t, 503, got.status)
		assert.Equal(t, "service_unavailable", got.errCode())
		assert.Equal(t, 200, api.do("GET", "/api/v1/company/", esis, nil).status)
	})

	t.Run("wrong method is 405", func(t *testing.T) {
		assert.Equal(t, 405, api.do("DELETE", "/api/v1/company/", esis, nil).status)
	})

	t.Run("refresh tokens work once", func(t *testing.T) {
		refresh := esisTokens["refresh_token"].(string)
		res := api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh})
		require.Equal(t, 200, res.status, res.body)
		newAccess := res.data()["access_token"].(string)
		assert.Equal(t, 200, api.do("GET", "/api/v1/auth/me", newAccess, nil).status)

		reuse := api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh})
		assert.Equal(t, 401, reuse.status)

		asAccess := api.do("GET", "/api/v1/auth/me", res.data()["refresh_token"].(string), nil)
		assert.Equal(t, 401, asAccess.status, "a refresh token is not an access token")
	})

	t.Run("logout revokes the refresh token", func(t *testing.T) {
		tokens := api.login("admin@esis.in", "esis-password-123")
		refresh := tokens["refresh_token"].(string)
		assert.Equal(t, 204, api.do("POST", "/api/v1/auth/logout", "", map[string]any{"refresh_token": refresh}).status)
		assert.Equal(t, 401, api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh}).status)
	})

	t.Run("replaying an old refresh token revokes the whole session family", func(t *testing.T) {
		first := api.login("admin@esis.in", "esis-password-123")["refresh_token"].(string)
		next := api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first})
		require.Equal(t, 200, next.status)
		stolenChain := next.data()["refresh_token"].(string)

		// Past the grace window the replay is treated as theft.
		_, err := dbm.Control().Exec(ctx, `UPDATE pms.sys_refresh_token SET revoked_at = now() - interval '1 minute' WHERE revoked_at IS NOT NULL`)
		require.NoError(t, err)
		assert.Equal(t, 401, api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": first}).status)
		assert.Equal(t, 401, api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": stolenChain}).status,
			"tokens issued after the replayed one are revoked too")
	})

	t.Run("update-user changes the login and ends old sessions", func(t *testing.T) {
		old := api.login("admin@tb.test", "tb-password-123")
		oldRefresh := old["refresh_token"].(string)

		_, err := usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{Email: "admin@tb.test", FullName: "Renamed Only"})
		require.NoError(t, err)
		assert.Equal(t, 200, api.do("POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": oldRefresh}).status,
			"a name change keeps sessions")

		u, err := usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{
			Email: "ADMIN@tb.test", NewEmail: "Uncle@TB.test", FullName: "Uncle", Password: "uncle-password-1",
		})
		require.NoError(t, err)
		assert.Equal(t, "uncle@tb.test", u.Email)

		assert.Equal(t, 401, api.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "admin@tb.test", "password": "tb-password-123"}).status)
		assert.Equal(t, 401, api.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "uncle@tb.test", "password": "tb-password-123"}).status)
		fresh := api.login("uncle@tb.test", "uncle-password-1")
		assert.Equal(t, "Uncle", fresh["user"].(map[string]any)["full_name"])

		_, err = usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{Email: "uncle@tb.test", NewEmail: "admin@esis.in"})
		assert.True(t, domain.IsKind(err, domain.KindConflict), "email already taken")
		_, err = usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{Email: "uncle@tb.test"})
		assert.True(t, domain.IsKind(err, domain.KindInvalidParams), "nothing to change")
		_, err = usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{Email: "nobody@tb.test", FullName: "X"})
		assert.True(t, domain.IsKind(err, domain.KindNotFound))
		_, err = usecases.Auth.UpdateUser(ctx, domain_auth.UpdateUserInput{Email: "uncle@tb.test", Password: "short"})
		assert.True(t, domain.IsKind(err, domain.KindValidation), "password too short")
	})

	t.Run("suspending a tenant locks its users out immediately", func(t *testing.T) {
		_, err := dbm.Control().Exec(ctx, `UPDATE pms.sys_tenant SET status = 'suspended' WHERE code = 'ESIS'`)
		require.NoError(t, err)
		assert.Equal(t, 401, api.do("GET", "/api/v1/company/", esis, nil).status)
		assert.Equal(t, 403, api.do("POST", "/api/v1/auth/login", "", map[string]any{"email": "admin@esis.in", "password": "esis-password-123"}).status)
		_, err = dbm.Control().Exec(ctx, `UPDATE pms.sys_tenant SET status = 'active' WHERE code = 'ESIS'`)
		require.NoError(t, err)
	})

	t.Run("bad request bodies", func(t *testing.T) {
		res := api.raw("PUT", "/api/v1/company/", esis, "application/json", `{"name":`)
		assert.Equal(t, 400, res.status)
		res = api.raw("PUT", "/api/v1/company/", esis, "text/plain", `{}`)
		assert.Equal(t, 400, res.status)
		res = api.raw("PUT", "/api/v1/company/", esis, "application/json", `{"name": 5}`)
		assert.Equal(t, 422, res.status)
	})
}

// ---- tiny HTTP client for the test -----------------------------------------

type client struct {
	t    *testing.T
	base string
}

type response struct {
	status int
	body   string
	json   map[string]any
}

func (c *client) do(method, path, token string, body map[string]any) response {
	var raw string
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(c.t, err)
		raw = string(b)
	}
	return c.raw(method, path, token, "application/json", raw)
}

func (c *client) raw(method, path, token, contentType, body string) response {
	req, err := http.NewRequest(method, c.base+path, bytes.NewBufferString(body))
	require.NoError(c.t, err)
	if body != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode, body: string(b)}
	_ = json.Unmarshal(b, &out.json)
	return out
}

func (c *client) login(email, password string) map[string]any {
	res := c.do("POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": password})
	require.Equal(c.t, 200, res.status, res.body)
	return res.data()
}

func (r response) data() map[string]any {
	d, _ := r.json["data"].(map[string]any)
	return d
}

func (r response) list() []any {
	l, _ := r.json["data"].([]any)
	return l
}

func (r response) errBody() map[string]any {
	e, _ := r.json["error"].(map[string]any)
	return e
}

func (r response) errCode() string    { s, _ := r.errBody()["code"].(string); return s }
func (r response) errMessage() string { s, _ := r.errBody()["message"].(string); return s }

func (r response) fieldErrors() map[string]string {
	out := map[string]string{}
	fields, _ := r.errBody()["fields"].([]any)
	for _, f := range fields {
		m := f.(map[string]any)
		name, _ := m["field"].(string)
		msg, _ := m["message"].(string)
		out[name] = msg
	}
	return out
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// createTenantDatabase makes another database in the same server, loaded with the DBA's objects.
func createTenantDatabase(t *testing.T, ctx context.Context, dsn, name string) string {
	t.Helper()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close(ctx)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)

	other := strings.Replace(dsn, "/esispms?", "/"+name+"?", 1)
	conn, err := pgx.Connect(ctx, other)
	require.NoError(t, err)
	defer conn.Close(ctx)
	for _, path := range legacyScripts() {
		sql, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, string(sql))
		require.NoError(t, err, path)
	}
	return other
}

// legacyScripts are the DBA's objects plus our marked local fixes, in the order Postgres runs them.
func legacyScripts() []string {
	dir := filepath.Join("..", "..", "..", "db", "legacy")
	return []string{
		filepath.Join(dir, "001_pms_objects.sql"),
		filepath.Join(dir, "002_form5_objects.sql"),
		filepath.Join(dir, "003_local_fixes.sql"),
		filepath.Join(dir, "004_delete_objects.sql"),
	}
}

// concurrently runs fn n times at once and returns the results sorted.
func concurrently(n int, fn func(i int) int) []int {
	out := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out[i] = fn(i)
		}(i)
	}
	wg.Wait()
	sort.Ints(out)
	return out
}

// upload posts data as the multipart field "file", the way the import page does.
func (c *client) upload(path, token, filename string, data []byte) response {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filename)
	require.NoError(c.t, err)
	_, _ = part.Write(data)
	require.NoError(c.t, mw.Close())
	return c.raw("POST", path, token, mw.FormDataContentType(), buf.String())
}

// xlsxFile builds a workbook in memory.
func xlsxFile(t *testing.T, fill func(f *excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	fill(f)
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))
	return buf.Bytes()
}
