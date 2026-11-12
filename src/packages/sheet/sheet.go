// Package sheet reads and writes the simple one-table .xlsx files used for imports.
package sheet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ErrNotXLSX is returned for anything that is not an .xlsx workbook (including old .xls files).
var ErrNotXLSX = errors.New("the file is not an Excel .xlsx workbook; in Excel use File, Save As, Excel Workbook (.xlsx)")

// ReadFirstSheet returns every row of the workbook's first sheet as text. Number cells come back
// as plain digits (a mobile stays 9663927257, not 9.66E+09) and date cells as YYYY-MM-DD.
func ReadFirstSheet(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, ErrNotXLSX
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("the workbook has no sheets")
	}
	name := sheets[0]
	rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("could not read sheet %q: %w", name, err)
	}
	// Raw values lose date formatting: turn date-formatted number cells back into dates.
	for ri, row := range rows {
		for ci, v := range row {
			if v == "" || !looksNumeric(v) {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
			if isDateCell(f, name, cell) {
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					if t, err := excelize.ExcelDateToTime(n, false); err == nil {
						rows[ri][ci] = t.Format(time.DateOnly)
					}
				}
			}
		}
	}
	return rows, nil
}

func looksNumeric(v string) bool {
	_, err := strconv.ParseFloat(v, 64)
	return err == nil
}

func isDateCell(f *excelize.File, sheetName, cell string) bool {
	styleID, err := f.GetCellStyle(sheetName, cell)
	if err != nil || styleID == 0 {
		return false
	}
	style, err := f.GetStyle(styleID)
	if err != nil || style == nil {
		return false
	}
	// Excel's built-in date formats are 14-22 and 45-47; custom ones contain d, m and y.
	if (style.NumFmt >= 14 && style.NumFmt <= 22) || (style.NumFmt >= 45 && style.NumFmt <= 47) {
		return true
	}
	if style.CustomNumFmt != nil {
		f := strings.ToLower(*style.CustomNumFmt)
		return strings.Contains(f, "d") && strings.Contains(f, "y")
	}
	return false
}

// Column is one column of a template: its header, an example value and a line of help.
type Column struct {
	Header, Example, Help string
	Required              bool
}

// Template builds a workbook with the headers and one example row on the first sheet, and a
// "Help" sheet that explains each column.
func Template(sheetName string, cols []Column) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}
	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	text, _ := f.NewStyle(&excelize.Style{NumFmt: 49}) // "@": keep codes and mobiles as text
	for i, c := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, c.Header); err != nil {
			return nil, err
		}
		ex, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheetName, ex, c.Example); err != nil {
			return nil, err
		}
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheetName, col, col, float64(max(14, len(c.Header)+4)))
		_ = f.SetColStyle(sheetName, col, text)
	}
	last, _ := excelize.CoordinatesToCellName(len(cols), 1)
	_ = f.SetCellStyle(sheetName, "A1", last, bold)
	_ = f.SetPanes(sheetName, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})

	if _, err := f.NewSheet("Help"); err != nil {
		return nil, err
	}
	_ = f.SetSheetRow("Help", "A1", &[]any{"Column", "Required", "What to put in it"})
	_ = f.SetCellStyle("Help", "A1", "C1", bold)
	for i, c := range cols {
		req := ""
		if c.Required {
			req = "yes"
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = f.SetSheetRow("Help", cell, &[]any{c.Header, req, c.Help})
	}
	_ = f.SetColWidth("Help", "A", "A", 22)
	_ = f.SetColWidth("Help", "C", "C", 70)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
