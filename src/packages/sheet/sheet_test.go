package sheet

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestReadFirstSheetKeepsNumbersAndDates(t *testing.T) {
	f := excelize.NewFile()
	require.NoError(t, f.SetSheetRow("Sheet1", "A1", &[]any{"code", "mobile", "date", "text date"}))
	require.NoError(t, f.SetSheetRow("Sheet1", "A2", &[]any{"SAT-001", 9663927257, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), "01-06-2025"}))
	style, _ := f.NewStyle(&excelize.Style{NumFmt: 14})
	require.NoError(t, f.SetCellStyle("Sheet1", "C2", "C2", style))
	_, err := f.NewSheet("Second")
	require.NoError(t, err)
	require.NoError(t, f.SetCellValue("Second", "A1", "ignored"))
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))

	rows, err := ReadFirstSheet(&buf)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"SAT-001", "9663927257", "2025-06-01", "01-06-2025"}, rows[1])
}

func TestReadFirstSheetRejectsOtherFiles(t *testing.T) {
	_, err := ReadFirstSheet(strings.NewReader("ACM_CODE,ACM_REG_CODE\nSAT-001,MYS-01\n"))
	assert.ErrorIs(t, err, ErrNotXLSX, "a CSV or an old .xls is refused with a clear message")
}

func TestTemplateRoundTrip(t *testing.T) {
	data, err := Template("Institutions", []Column{
		{Header: "trust_code", Example: "SAT", Help: "Trust code", Required: true},
		{Header: "inst_head_mobile", Example: "9845012345", Help: "Mobile"},
	})
	require.NoError(t, err)
	rows, err := ReadFirstSheet(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"trust_code", "inst_head_mobile"}, {"SAT", "9845012345"}}, rows)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	help, err := f.GetRows("Help")
	require.NoError(t, err)
	assert.Equal(t, []string{"trust_code", "yes", "Trust code"}, help[1])
}
