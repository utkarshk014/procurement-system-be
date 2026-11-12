package domain_institution

import "github.com/utkarshk014/procurement-system-be/src/internal/domain"

// MaxImportRows bounds one import file (the client has 571 institutions today).
const MaxImportRows = 5000

// ImportProblem is one data row that cannot be imported, with every reason. Row is the
// spreadsheet row number as Excel shows it (the header is row 1).
type ImportProblem struct {
	Row    int                 `json:"row"`
	Code   string              `json:"inst_code"`
	Name   string              `json:"inst_name"`
	Errors []domain.FieldError `json:"errors"`
}

// NameCount is a place or branch named in the file that is not in its list yet, and how many
// rows name it.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"rows"`
}

// ImportReport is the answer to POST /institutions/import. On a dry run nothing is saved and
// Imported is 0; otherwise every ready row is saved together and the problem rows are skipped.
type ImportReport struct {
	DryRun          bool            `json:"dry_run"`
	Total           int             `json:"total"`
	Ready           int             `json:"ready"`
	Imported        int             `json:"imported"`
	Skipped         int             `json:"skipped"`
	Problems        []ImportProblem `json:"problems"`
	MissingPlaces   []NameCount     `json:"missing_places"`
	MissingBranches []NameCount     `json:"missing_branches"`
	DefaultYear     string          `json:"default_from_fin_yr"`
}

// ImportColumn is one column the import understands: the canonical header (used in the
// template) and the other headers it accepts, such as the old system's ACM_* names.
type ImportColumn struct {
	Header   string
	Aliases  []string
	Required bool
	Help     string
}

// ImportColumns is the import file layout. Headers are matched ignoring case, spaces and
// punctuation, so "Inst Code", "inst_code" and "INSTCODE" are the same column.
var ImportColumns = []ImportColumn{
	{Header: "trust_code", Aliases: []string{"trust", "acm_inst_trust"}, Required: true, Help: "Trust code, e.g. SAT"},
	{Header: "inst_code", Aliases: []string{"acm_code", "code"}, Required: true, Help: "ACM code: trust code, dash, 3 digits, e.g. SAT-001"},
	{Header: "inst_reg_code", Aliases: []string{"acm_reg_code", "reg_code"}, Required: true, Help: "Reg code: region code, dash, 2 digits, e.g. MYS-01"},
	{Header: "inst_name", Aliases: []string{"acm_inst_name", "name", "institution", "institution_name"}, Required: true, Help: "Institution name"},
	{Header: "place", Aliases: []string{"place_code", "place_name", "acm_inst_place"}, Required: true, Help: "Place code or name from the places list"},
	{Header: "branch", Aliases: []string{"branch_code", "branch_name", "acm_inst_branch"}, Required: true, Help: "Branch code or name from the branches list"},
	{Header: "from_fin_yr", Aliases: []string{"start_year", "finyr"}, Help: "Start year, e.g. 2026-2027 (empty: the current year)"},
	{Header: "reg_date", Aliases: []string{"registered_on"}, Help: "Registered on, YYYY-MM-DD (empty: today)"},
	{Header: "inst_add", Aliases: []string{"address"}, Help: "Address"},
	{Header: "inst_place", Aliases: []string{"locality"}, Help: "Locality, free text (empty: the place column's text)"},
	{Header: "inst_tel", Aliases: []string{"tel", "telephone"}, Help: "Institution telephone"},
	{Header: "inst_email", Aliases: []string{"email"}, Help: "Institution email"},
	{Header: "inst_head_name", Aliases: []string{"head_name"}, Help: "Head of institution"},
	{Header: "inst_head_desig", Aliases: []string{"head_designation"}, Help: "Head's designation"},
	{Header: "inst_head_mobile", Aliases: []string{"head_mobile"}, Help: "Head's mobile, 10 digits"},
	{Header: "inst_head_email", Aliases: []string{"head_email"}, Help: "Head's email"},
	{Header: "inst_acnt_name", Aliases: []string{"accountant", "accountant_name"}, Help: "Accountant"},
	{Header: "inst_acnt_mobile", Aliases: []string{"accountant_mobile"}, Help: "Accountant's mobile, 10 digits"},
	{Header: "inst_acnt_email", Aliases: []string{"accountant_email"}, Help: "Accountant's email"},
	{Header: "inst_liason_name", Aliases: []string{"inst_liaison_name", "liaison_name"}, Help: "Liaison officer"},
	{Header: "inst_liason_desig", Aliases: []string{"inst_liaison_desig", "liaison_designation"}, Help: "Liaison officer's designation"},
	{Header: "inst_liason_mobile", Aliases: []string{"inst_liaison_mobile", "liaison_mobile"}, Help: "Liaison officer's mobile, 10 digits"},
	{Header: "inst_liason_email", Aliases: []string{"inst_liaison_email", "liaison_email"}, Help: "Liaison officer's email"},
	// Checked when present, never stored: the old system's full code and region short code.
	{Header: "inst_full_code", Aliases: []string{"acm_full_code", "full_code"}, Help: "Optional check: must equal inst_code:inst_reg_code"},
	{Header: "regn_code", Aliases: []string{"acm_inst_reg_short_code", "region"}, Help: "Optional check: must equal the reg code's first 3 letters"},
}
