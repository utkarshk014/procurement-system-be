package usecase_codelist

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_codelist "github.com/utkarshk014/procurement-system-be/src/internal/domain/codelist"
)

// ---- fakes ------------------------------------------------------------------

type fakeRepo struct {
	kind     domain_codelist.Kind
	rows     map[string]domain_codelist.Entry
	function domain_codelist.FunctionResult
}

func newRepo() *fakeRepo {
	return &fakeRepo{kind: domain_codelist.Place, rows: map[string]domain_codelist.Entry{},
		function: domain_codelist.FunctionResult{Valid: true, Message: domain_codelist.FunctionSuccess}}
}

func (f *fakeRepo) List(context.Context) ([]domain_codelist.Entry, error) {
	out := []domain_codelist.Entry{}
	for _, e := range f.rows {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, code string) (*domain_codelist.Entry, error) {
	e, ok := f.rows[code]
	if !ok {
		return nil, f.kind.NotFound()
	}
	return &e, nil
}
func (f *fakeRepo) NameTaken(_ context.Context, name, except string) (string, bool, error) {
	for code, e := range f.rows {
		if code != except && strings.EqualFold(e.Name, name) {
			return code, true, nil
		}
	}
	return "", false, nil
}
func (f *fakeRepo) CheckWithFunction(context.Context, *domain_codelist.Entry) (*domain_codelist.FunctionResult, error) {
	r := f.function
	return &r, nil
}
func (f *fakeRepo) Create(_ context.Context, e *domain_codelist.Entry) error {
	if _, ok := f.rows[e.Code]; ok {
		return f.kind.CodeTaken()
	}
	f.rows[e.Code] = *e
	return nil
}

func tenantCtx() context.Context {
	return domain.WithPrincipal(context.Background(), &domain.Principal{
		UserID: 7, Role: domain.RoleTenantAdmin, TenantID: 1, TenantCode: "ESIS", DBKey: "esis",
	})
}

func newUC(repo *fakeRepo) domain_codelist.Usecase {
	return NewCodeListUsecase(domain_codelist.Place, repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func fieldsOf(err error) map[string]string {
	out := map[string]string{}
	if ae, ok := domain.AsAppError(err); ok {
		for _, f := range ae.Fields {
			out[f.Field] = f.Message
		}
	}
	return out
}

// ---- tests ------------------------------------------------------------------

func TestCreateNormalisesAndSaves(t *testing.T) {
	repo := newRepo()
	got, err := newUC(repo).Create(tenantCtx(), domain_codelist.Input{Code: " mys ", Name: "  Mysore   City "})
	require.NoError(t, err)
	assert.Equal(t, "MYS", got.Code)
	assert.Equal(t, "Mysore City", got.Name, "trimmed, single spaces")
}

func TestCreateReportsEveryFieldProblemAtOnce(t *testing.T) {
	_, err := newUC(newRepo()).Create(tenantCtx(), domain_codelist.Input{Code: "M1S", Name: strings.Repeat("n", 251)})
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	f := fieldsOf(err)
	assert.Equal(t, "must be exactly 3 letters, e.g. MYS", f["code"])
	assert.Equal(t, "must be at most 250 characters", f["name"])

	f = fieldsOf(func() error { _, err := newUC(newRepo()).Create(tenantCtx(), domain_codelist.Input{}); return err }())
	assert.Equal(t, "is required", f["code"])
	assert.Equal(t, "is required", f["name"])
}

func TestValidateFindsDuplicatesWithoutWriting(t *testing.T) {
	repo := newRepo()
	repo.rows["MYS"] = domain_codelist.Entry{Code: "MYS", Name: "Mysore"}

	_, err := newUC(repo).Validate(tenantCtx(), domain_codelist.Input{Code: "mys", Name: "Other"})
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "is already taken", fieldsOf(err)["code"])

	_, err = newUC(repo).Validate(tenantCtx(), domain_codelist.Input{Code: "MYZ", Name: "MYSORE"})
	assert.True(t, domain.IsKind(err, domain.KindConflict))
	assert.Equal(t, "is already used by MYS", fieldsOf(err)["name"])

	got, err := newUC(repo).Validate(tenantCtx(), domain_codelist.Input{Code: "BGN", Name: "B G Nagara"})
	require.NoError(t, err)
	assert.Equal(t, "BGN", got.Code)
	assert.Len(t, repo.rows, 1, "validate writes nothing")
}

func TestValidateMapsTheDBAsMessage(t *testing.T) {
	repo := newRepo()
	repo.function = domain_codelist.FunctionResult{Valid: false, Message: "Place name cannot be empty"}
	_, err := newUC(repo).Validate(tenantCtx(), domain_codelist.Input{Code: "MYS", Name: "Mysore"})
	assert.True(t, domain.IsKind(err, domain.KindValidation))
	assert.Equal(t, "Place name cannot be empty", fieldsOf(err)["name"])
}

func TestGetUpperCasesAndRejectsBadCodes(t *testing.T) {
	repo := newRepo()
	repo.rows["MYS"] = domain_codelist.Entry{Code: "MYS", Name: "Mysore"}
	got, err := newUC(repo).Get(tenantCtx(), " mys ")
	require.NoError(t, err)
	assert.Equal(t, "Mysore", got.Name)
	_, err = newUC(repo).Get(tenantCtx(), "M1S")
	assert.True(t, domain.IsKind(err, domain.KindNotFound))
}

func TestNeedsATenantAdmin(t *testing.T) {
	_, err := newUC(newRepo()).List(context.Background())
	assert.Error(t, err)
	_, err = newUC(newRepo()).Create(context.Background(), domain_codelist.Input{Code: "MYS", Name: "Mysore"})
	assert.Error(t, err)
}

func (f *fakeRepo) Delete(_ context.Context, code string) error {
	if _, ok := f.rows[code]; !ok {
		return f.kind.NotFound()
	}
	delete(f.rows, code)
	return nil
}

func TestDeleteNormalisesTheCode(t *testing.T) {
	repo := newRepo()
	repo.rows["MYS"] = domain_codelist.Entry{Code: "MYS", Name: "Mysore"}
	require.NoError(t, newUC(repo).Delete(tenantCtx(), " mys "))
	assert.Empty(t, repo.rows)
	assert.True(t, domain.IsKind(newUC(repo).Delete(tenantCtx(), "MYS"), domain.KindNotFound))
	assert.True(t, domain.IsKind(newUC(repo).Delete(tenantCtx(), "M1S"), domain.KindNotFound))
	assert.Error(t, newUC(repo).Delete(context.Background(), "MYS"), "tenant admins only")
}
