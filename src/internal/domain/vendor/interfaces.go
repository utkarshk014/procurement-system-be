package domain_vendor

import (
	"context"
	"io"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

// Usecase is the vendor master of the caller's tenant.
type Usecase interface {
	List(ctx context.Context) ([]Vendor, error)
	Get(ctx context.Context, id int64) (*VendorDetail, error)
	// Validate is a dry run of Create.
	Validate(ctx context.Context, in VendorInput) (*Vendor, error)
	Create(ctx context.Context, in VendorInput) (*VendorDetail, error)
	Update(ctx context.Context, id int64, in VendorInput) (*VendorDetail, error)
	// Delete removes the vendor with its groups, financials and certificates, and their files.
	Delete(ctx context.Context, id int64) error

	CreateFinancial(ctx context.Context, vendorID int64, in FinancialInput) (*Financial, error)
	UpdateFinancial(ctx context.Context, vendorID int64, finYear string, in FinancialInput) (*Financial, error)
	DeleteFinancial(ctx context.Context, vendorID int64, finYear string) error

	// SaveCertificates stores the vendor's certificates (one row, created on the first save).
	SaveCertificates(ctx context.Context, vendorID int64, in Certificates) (Certificates, error)

	// UploadFile stores a document for the vendor and returns its key.
	UploadFile(ctx context.Context, vendorID int64, name string, size int64, body io.Reader) (*UploadedFile, error)
	// FileURL returns a short-lived link to download one of the vendor's files.
	FileURL(ctx context.Context, vendorID int64, key string) (string, error)
}

// Repository talks to the DBA's vendor tables and procedures.
type Repository interface {
	List(ctx context.Context) ([]Vendor, error)
	Get(ctx context.Context, id int64) (*VendorDetail, error)
	Exists(ctx context.Context, id int64) (bool, error)
	// Problems returns a field error for each group that does not exist, and a 409 field error
	// for a name, PAN or GSTIN another vendor (not v.ID) already uses.
	Problems(ctx context.Context, v *Vendor) (missing, taken []domain.FieldError, err error)
	// CheckWithFunction runs the DBA's create check on a new vendor; nil when it passes.
	CheckWithFunction(ctx context.Context, v *Vendor) error
	// Create re-checks and inserts the vendor and its groups under one lock, returning the id.
	Create(ctx context.Context, v *Vendor) (int64, error)
	// Update re-checks, updates the vendor and replaces its groups under the same lock.
	Update(ctx context.Context, v *Vendor) error
	// Delete calls the DBA's delete and returns the S3 keys the vendor's rows pointed at.
	Delete(ctx context.Context, id int64) (files []string, err error)

	// CreateFinancial adds a year. UpdateFinancial changes one and returns the file it pointed
	// at before. DeleteFinancial removes one and returns its file.
	CreateFinancial(ctx context.Context, vendorID int64, f *Financial) error
	UpdateFinancial(ctx context.Context, vendorID int64, f *Financial) (oldFile string, err error)
	DeleteFinancial(ctx context.Context, vendorID int64, finYear string) (oldFile string, err error)
	GetFinancial(ctx context.Context, vendorID int64, finYear string) (*Financial, error)

	// SaveCertificates inserts or updates the row and returns the files it pointed at before.
	SaveCertificates(ctx context.Context, vendorID int64, c Certificates) (oldFiles []string, err error)
	GetCertificates(ctx context.Context, vendorID int64) (Certificates, error)
}

// FileStore keeps vendor documents (S3). Keys are opaque paths chosen by the use case.
type FileStore interface {
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	Delete(ctx context.Context, key string) error
	URL(ctx context.Context, key, downloadName string, ttl time.Duration) (string, error)
}
