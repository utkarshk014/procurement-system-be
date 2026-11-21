package sheet

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// Field is one column of an import/export file: its header (used in the template and the
// export), other headers it accepts, and a line of help.
type Field struct {
	Header   string
	Aliases  []string
	Required bool
	Example  string
	Help     string
}

// HeaderKey makes "Item Code", "item_code" and "ITEMCODE" the same key; "%" reads as
// "percent", so "GST %" is gst_percent and not the GST yes/no column.
func HeaderKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '%':
			b.WriteString("percent")
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NameKey compares names ignoring case and runs of spaces, like the masters' duplicate checks.
func NameKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// Row is one data row. Number is the row number Excel shows (the header is row 1).
type Row struct {
	Number int
	cells  []string
	index  map[string]int
}

// Get returns the trimmed cell under header ("" when the column or cell is missing).
func (r Row) Get(header string) string {
	i, ok := r.index[header]
	if !ok || i >= len(r.cells) {
		return ""
	}
	return strings.TrimSpace(r.cells[i])
}

// Has reports whether the file has the column at all.
func (r Row) Has(header string) bool { _, ok := r.index[header]; return ok }

// Parse finds the columns in the header row and returns the non-blank data rows. what is the
// plural shown in messages ("items"). The error is a sentence for the user.
func Parse(rows [][]string, fields []Field, what string, maxRows int) ([]Row, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("The sheet is empty. The first row must be the column headers.")
	}
	aliases := map[string]string{}
	for _, f := range fields {
		aliases[HeaderKey(f.Header)] = f.Header
		for _, a := range f.Aliases {
			aliases[HeaderKey(a)] = f.Header
		}
	}
	index := map[string]int{}
	for i, h := range rows[0] {
		if canon, ok := aliases[HeaderKey(h)]; ok {
			if _, seen := index[canon]; !seen {
				index[canon] = i
			}
		}
	}
	var missing []string
	for _, f := range fields {
		if _, ok := index[f.Header]; f.Required && !ok {
			missing = append(missing, f.Header)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("Missing column(s): %s. Download the template to see every column.", strings.Join(missing, ", "))
	}
	var out []Row
	for i, r := range rows[1:] {
		if strings.TrimSpace(strings.Join(r, "")) != "" {
			out = append(out, Row{Number: i + 2, cells: r, index: index})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("The sheet has a header but no %s under it.", what)
	}
	if len(out) > maxRows {
		return nil, fmt.Errorf("The sheet has more than %s %s; split it into smaller files.", strconv.FormatInt(int64(maxRows), 10), what)
	}
	return out, nil
}

// Bool reads yes/no style cells. Empty gives def; anything unreadable gives ok=false.
func Bool(s string, def bool) (value, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return def, true
	case "yes", "y", "true", "t", "1":
		return true, true
	case "no", "n", "false", "f", "0":
		return false, true
	}
	return false, false
}

// Number reads a number cell ("1,25,000.50" and "18%" work). Empty gives nil.
func Number(s string) (value *float64, ok bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return nil, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, false
	}
	return &v, true
}

// YesNo writes a boolean the way Bool reads it.
func YesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// TemplateOf builds the import template for fields (see Template).
func TemplateOf(sheetName string, fields []Field) ([]byte, error) {
	cols := make([]Column, len(fields))
	for i, f := range fields {
		cols[i] = Column{Header: f.Header, Example: f.Example, Help: f.Help, Required: f.Required}
	}
	return Template(sheetName, cols)
}

// Export writes one sheet with the fields' headers and the rows under them, every cell as text
// so codes and mobiles keep their digits. The file can be edited and imported again.
func Export(sheetName string, fields []Field, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}
	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	text, _ := f.NewStyle(&excelize.Style{NumFmt: 49})
	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return nil, err
	}
	for i, fl := range fields {
		_ = sw.SetColWidth(i+1, i+1, float64(max(14, len(fl.Header)+4)))
	}
	_ = sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	header := make([]any, len(fields))
	for i, fl := range fields {
		header[i] = excelize.Cell{StyleID: bold, Value: fl.Header}
	}
	if err := sw.SetRow("A1", header); err != nil {
		return nil, err
	}
	for ri, r := range rows {
		cells := make([]any, len(r))
		for i, v := range r {
			cells[i] = excelize.Cell{StyleID: text, Value: v}
		}
		cell, _ := excelize.CoordinatesToCellName(1, ri+2)
		if err := sw.SetRow(cell, cells); err != nil {
			return nil, err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Date accepts YYYY-MM-DD, DD-MM-YYYY and DD/MM/YYYY and returns YYYY-MM-DD; anything else is
// passed through for the usual date check to reject.
func Date(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.DateOnly, "02-01-2006", "02/01/2006", "2/1/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.DateOnly)
		}
	}
	return s
}
