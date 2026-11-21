package domain

import (
	"sort"
	"strings"
)

// ImportProblem is one data row that cannot be imported, with every reason. Row is the
// spreadsheet row number as Excel shows it (the header is row 1); Key and Name say which record
// it is (an item code and description, a vendor's PAN and name).
type ImportProblem struct {
	Row    int          `json:"row"`
	Key    string       `json:"key"`
	Name   string       `json:"name"`
	Errors []FieldError `json:"errors"`
}

// NameCount is a name the file uses that is not in its list yet, and how many rows use it.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"rows"`
}

// ImportReport answers an Excel import. On a dry run nothing is saved and Imported is 0;
// otherwise every ready row is saved together and the problem rows are skipped. Missing lists,
// per column (item_group, uom, …), the names that must be added to their list first.
type ImportReport struct {
	DryRun   bool                   `json:"dry_run"`
	Total    int                    `json:"total"`
	Ready    int                    `json:"ready"`
	Imported int                    `json:"imported"`
	Skipped  int                    `json:"skipped"`
	Problems []ImportProblem        `json:"problems"`
	Missing  map[string][]NameCount `json:"missing"`
}

// MissingNames counts names not found in a list, grouped ignoring case and spaces; the first
// spelling seen is the one shown.
type MissingNames map[string]*NameCount

// Add counts one more row naming text.
func (m MissingNames) Add(text string) {
	k := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if m[k] == nil {
		m[k] = &NameCount{Name: strings.Join(strings.Fields(text), " ")}
	}
	m[k].Count++
}

// Sorted returns the names, most used first.
func (m MissingNames) Sorted() []NameCount {
	out := make([]NameCount, 0, len(m))
	for _, n := range m {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
