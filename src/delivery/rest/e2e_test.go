package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
		tcpostgres.WithInitScripts(filepath.Join("..", "..", "..", "db", "legacy", "001_pms_objects.sql")),
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

	t.Run("financial year options are current and previous", func(t *testing.T) {
		res := api.do("GET", "/api/v1/financial-years/options", esis, nil)
		require.Equal(t, 200, res.status)
		opts := res.list()
		require.Len(t, opts, 2)
		first := opts[0].(map[string]any)
		assert.Equal(t, curFY, first["finyr"])
		assert.Equal(t, curStartDate, first["finyr_st_date"])
		assert.Equal(t, "current", first["kind"])
		assert.Equal(t, prevFY, opts[1].(map[string]any)["finyr"])
	})

	t.Run("financial year rules", func(t *testing.T) {
		for in, field := range map[string]string{"abc": "finyr", "2026-2028": "finyr", "1999-2000": "finyr"} {
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
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "legacy", "001_pms_objects.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(sql))
	require.NoError(t, err)
	return other
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
