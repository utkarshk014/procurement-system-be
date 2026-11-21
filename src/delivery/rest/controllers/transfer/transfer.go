// Package transfer holds what every Excel import and export handler shares: reading the uploaded
// .xlsx and sending one back.
package transfer

import (
	"errors"
	"net/http"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	"github.com/utkarshk014/procurement-system-be/src/packages/sheet"
)

// MaxImportBytes bounds an uploaded import file.
const MaxImportBytes = 5 << 20

// ReadUpload reads the .xlsx uploaded as the multipart field "file" and whether ?dry_run=true.
func ReadUpload(w http.ResponseWriter, r *http.Request) (rows [][]string, dryRun bool, err error) {
	dryRun = r.URL.Query().Get("dry_run") == "true"
	r.Body = http.MaxBytesReader(w, r.Body, MaxImportBytes+1<<20)
	if err := r.ParseMultipartForm(MaxImportBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, false, domain.InvalidParams("The file is too large (5 MB at most)")
		}
		return nil, false, domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, false, domain.InvalidParams("Upload the file as multipart/form-data in the field \"file\"")
	}
	defer file.Close()
	if header.Size > MaxImportBytes {
		return nil, false, domain.InvalidParams("The file is too large (5 MB at most)")
	}
	rows, err = sheet.ReadFirstSheet(file)
	if err != nil {
		return nil, false, domain.Validation(domain.FieldError{Field: "file", Message: err.Error()})
	}
	return rows, dryRun, nil
}

// SendXLSX sends data as a download named filename.
func SendXLSX(w http.ResponseWriter, filename string, data []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
