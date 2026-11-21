// Package usecase_vendor is the vendor master. Every write runs our checks (all problems at
// once), then the DBA's, then one locked transaction that re-checks and calls his procedures.
// Documents go to S3 under <tenant>/vendors/<id>/, so a vendor can only point at its own files.
package usecase_vendor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_finyear "github.com/utkarshk014/procurement-system-be/src/internal/domain/finyear"
	domain_vendor "github.com/utkarshk014/procurement-system-be/src/internal/domain/vendor"
)

type vendorUsecase struct {
	repo     domain_vendor.Repository
	files    domain_vendor.FileStore // nil when S3 is not configured
	location *time.Location
	logger   *slog.Logger
	now      func() time.Time
}

// NewVendorUsecase builds the vendor master. files may be nil: uploads then answer 503.
func NewVendorUsecase(repo domain_vendor.Repository, files domain_vendor.FileStore, location *time.Location, logger *slog.Logger) domain_vendor.Usecase {
	return &vendorUsecase{repo: repo, files: files, location: location, logger: logger, now: time.Now}
}

func (uc *vendorUsecase) today() string { return uc.now().In(uc.location).Format(time.DateOnly) }

// currentFY is the financial year (1 April to 31 March) today falls in, e.g. "2026-2027".
func (uc *vendorUsecase) currentFY() string {
	t := uc.now().In(uc.location)
	y := t.Year()
	if t.Month() < time.April {
		y--
	}
	return domain_finyear.FinYrLabel(y)
}

func validID(id int64) bool { return id > 0 && id <= 1<<31-1 }

func (uc *vendorUsecase) List(ctx context.Context) ([]domain_vendor.Vendor, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	return uc.repo.List(ctx)
}

func (uc *vendorUsecase) Get(ctx context.Context, id int64) (*domain_vendor.VendorDetail, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, domain_vendor.ErrVendorNotFound
	}
	return uc.repo.Get(ctx, id)
}

func (uc *vendorUsecase) Validate(ctx context.Context, in domain_vendor.VendorInput) (*domain_vendor.Vendor, error) {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return nil, err
	}
	v, err := PrepareVendor(in, uc.today())
	if err != nil {
		return nil, err
	}
	missing, taken, err := uc.repo.Problems(ctx, v)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, domain.Validation(missing...)
	}
	if len(taken) > 0 {
		return nil, domain.Conflict("Vendor already exists", taken...)
	}
	if err := uc.repo.CheckWithFunction(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (uc *vendorUsecase) Create(ctx context.Context, in domain_vendor.VendorInput) (*domain_vendor.VendorDetail, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	v, err := PrepareVendor(in, uc.today())
	if err != nil {
		return nil, err
	}
	id, err := uc.repo.Create(ctx, v)
	if err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "vendor created", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *vendorUsecase) Update(ctx context.Context, id int64, in domain_vendor.VendorInput) (*domain_vendor.VendorDetail, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, domain_vendor.ErrVendorNotFound
	}
	v, err := PrepareVendor(in, uc.today())
	if err != nil {
		return nil, err
	}
	v.ID = id
	if err := uc.repo.Update(ctx, v); err != nil {
		return nil, err
	}
	uc.logger.InfoContext(ctx, "vendor updated", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	return uc.repo.Get(ctx, id)
}

func (uc *vendorUsecase) Delete(ctx context.Context, id int64) error {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return err
	}
	if !validID(id) {
		return domain_vendor.ErrVendorNotFound
	}
	files, err := uc.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	uc.logger.InfoContext(ctx, "vendor deleted", "tenant", p.TenantCode, "user_id", p.UserID, "id", id)
	uc.removeFiles(ctx, files...)
	return nil
}

// ---- Financials

func (uc *vendorUsecase) prepareFinancial(ctx context.Context, vendorID int64, in domain_vendor.FinancialInput) (*domain_vendor.Financial, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(vendorID) {
		return nil, domain_vendor.ErrVendorNotFound
	}
	f, err := PrepareFinancial(in, uc.today(), uc.currentFY())
	if err != nil {
		return nil, err
	}
	if f.ITRFile != "" && !strings.HasPrefix(f.ITRFile, filePrefix(p, vendorID)) {
		return nil, domain.Validation(domain.FieldError{Field: "fin_itr_s3_bucket_reference", Message: "is not a file of this vendor"})
	}
	return f, nil
}

func (uc *vendorUsecase) CreateFinancial(ctx context.Context, vendorID int64, in domain_vendor.FinancialInput) (*domain_vendor.Financial, error) {
	f, err := uc.prepareFinancial(ctx, vendorID, in)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.CreateFinancial(ctx, vendorID, f); err != nil {
		return nil, err
	}
	return uc.repo.GetFinancial(ctx, vendorID, f.FinYear)
}

func (uc *vendorUsecase) UpdateFinancial(ctx context.Context, vendorID int64, finYear string, in domain_vendor.FinancialInput) (*domain_vendor.Financial, error) {
	in.FinYear = finYear
	f, err := uc.prepareFinancial(ctx, vendorID, in)
	if err != nil {
		if fe, ok := domain.AsAppError(err); ok && fe.Kind == domain.KindValidation && len(fe.Fields) > 0 && fe.Fields[0].Field == "fin_year" {
			return nil, domain_vendor.ErrFinancialNotFound // a year in the path that cannot exist
		}
		return nil, err
	}
	old, err := uc.repo.UpdateFinancial(ctx, vendorID, f)
	if err != nil {
		return nil, err
	}
	if old != "" && old != f.ITRFile {
		uc.removeFiles(ctx, old)
	}
	return uc.repo.GetFinancial(ctx, vendorID, f.FinYear)
}

func (uc *vendorUsecase) DeleteFinancial(ctx context.Context, vendorID int64, finYear string) error {
	if _, err := domain.TenantPrincipal(ctx); err != nil {
		return err
	}
	if !validID(vendorID) {
		return domain_vendor.ErrVendorNotFound
	}
	finYear = strings.TrimSpace(finYear)
	if len(finYear) != 9 {
		return domain_vendor.ErrFinancialNotFound
	}
	old, err := uc.repo.DeleteFinancial(ctx, vendorID, finYear)
	if err != nil {
		return err
	}
	uc.removeFiles(ctx, old)
	return nil
}

// ---- Certificates

func (uc *vendorUsecase) SaveCertificates(ctx context.Context, vendorID int64, in domain_vendor.Certificates) (domain_vendor.Certificates, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(vendorID) {
		return nil, domain_vendor.ErrVendorNotFound
	}
	c, err := PrepareCertificates(in)
	if err != nil {
		return nil, err
	}
	prefix := filePrefix(p, vendorID)
	var bad []domain.FieldError
	for _, slot := range domain_vendor.CertificateSlots {
		if f := c[slot].File; f != "" && !strings.HasPrefix(f, prefix) {
			bad = append(bad, domain.FieldError{Field: slot + ".file", Message: "is not a file of this vendor"})
		}
	}
	if len(bad) > 0 {
		return nil, domain.Validation(bad...)
	}
	oldFiles, err := uc.repo.SaveCertificates(ctx, vendorID, c)
	if err != nil {
		return nil, err
	}
	kept := map[string]bool{}
	for _, f := range c.Files() {
		kept[f] = true
	}
	var gone []string
	for _, f := range oldFiles {
		if !kept[f] {
			gone = append(gone, f)
		}
	}
	uc.removeFiles(ctx, gone...)
	uc.logger.InfoContext(ctx, "vendor certificates saved", "tenant", p.TenantCode, "user_id", p.UserID, "id", vendorID)
	return uc.repo.GetCertificates(ctx, vendorID)
}

// ---- Files

// filePrefix is where a vendor's files live: <tenant>/vendors/<id>/.
func filePrefix(p *domain.Principal, vendorID int64) string {
	return strings.ToLower(p.TenantCode) + "/vendors/" + strconv.FormatInt(vendorID, 10) + "/"
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// safeName keeps a readable, URL-safe version of an uploaded file's name.
func safeName(name string) string {
	base := path.Base(strings.ReplaceAll(name, `\`, "/"))
	base = strings.TrimSuffix(base, path.Ext(base)) // the stored extension comes from the content
	base = strings.Trim(unsafeName.ReplaceAllString(base, "-"), "-.")
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-.")
	}
	if base == "" {
		base = "file"
	}
	return base
}

func (uc *vendorUsecase) UploadFile(ctx context.Context, vendorID int64, name string, size int64, body io.Reader) (*domain_vendor.UploadedFile, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if !validID(vendorID) {
		return nil, domain_vendor.ErrVendorNotFound
	}
	if uc.files == nil {
		return nil, domain.Unavailable("File storage is not set up")
	}
	if size <= 0 {
		return nil, domain.Validation(domain.FieldError{Field: "file", Message: "is empty"})
	}
	if size > domain_vendor.MaxFileBytes {
		return nil, domain.Validation(domain.FieldError{Field: "file", Message: "is too large (10 MB at most)"})
	}
	exists, err := uc.repo.Exists(ctx, vendorID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain_vendor.ErrVendorNotFound
	}

	// The type comes from the file's first bytes, never from its name or the browser.
	head := make([]byte, 512)
	n, err := io.ReadFull(body, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, domain.InvalidParams("Could not read the uploaded file")
	}
	head = head[:n]
	contentType := http.DetectContentType(head)
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	ext, ok := domain_vendor.AllowedFileTypes[contentType]
	if !ok {
		return nil, domain.Validation(domain.FieldError{Field: "file", Message: "must be a PDF, JPG or PNG"})
	}

	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return nil, domain.Internal("Could not name the file", err)
	}
	key := filePrefix(p, vendorID) + hex.EncodeToString(random) + "-" + safeName(name) + ext
	if err := uc.files.Put(ctx, key, contentType, size, io.MultiReader(strings.NewReader(string(head)), body)); err != nil {
		return nil, domain.Internal("Could not store the file", err)
	}
	uc.logger.InfoContext(ctx, "vendor file uploaded", "tenant", p.TenantCode, "user_id", p.UserID, "id", vendorID, "key", key, "size", size)
	return &domain_vendor.UploadedFile{Key: key, Name: path.Base(key), Size: size, ContentType: contentType}, nil
}

// fileURLTTL is how long a download link works.
const fileURLTTL = 10 * time.Minute

func (uc *vendorUsecase) FileURL(ctx context.Context, vendorID int64, key string) (string, error) {
	p, err := domain.TenantPrincipal(ctx)
	if err != nil {
		return "", err
	}
	if !validID(vendorID) {
		return "", domain_vendor.ErrVendorNotFound
	}
	if uc.files == nil {
		return "", domain.Unavailable("File storage is not set up")
	}
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, filePrefix(p, vendorID)) || strings.Contains(key, "..") {
		return "", domain.NotFound("File not found")
	}
	return uc.files.URL(ctx, key, downloadName(key), fileURLTTL)
}

// downloadName is the name a stored file downloads as: its key without the folder and the
// random part ("esis/vendors/3/9f86d081884c7d65-GST-certificate.pdf" → "GST-certificate.pdf").
func downloadName(key string) string {
	name := path.Base(key)
	if i := strings.IndexByte(name, '-'); i == 16 && len(name) > 17 {
		return name[17:]
	}
	return name
}

// removeFiles deletes files no row points at any more. A failure is only logged: the database
// change has already been saved, and a leftover file harms nothing.
func (uc *vendorUsecase) removeFiles(ctx context.Context, keys ...string) {
	if uc.files == nil {
		return
	}
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := uc.files.Delete(context.WithoutCancel(ctx), k); err != nil {
			uc.logger.WarnContext(ctx, "could not delete vendor file", "key", k, "error", err)
		}
	}
}
