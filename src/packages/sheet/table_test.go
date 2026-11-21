package sheet

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMatchesHeadersAndSkipsBlankRows(t *testing.T) {
	fields := []Field{{Header: "item_code", Aliases: []string{"code"}, Required: true}, {Header: "uom", Aliases: []string{"unit"}}}
	rows, err := Parse([][]string{{"Code", "Unit", "other"}, {" A1 ", "Nos"}, {"", ""}, {"B2"}}, fields, "items", 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, 2, rows[0].Number)
	assert.Equal(t, "A1", rows[0].Get("item_code"))
	assert.Equal(t, 4, rows[1].Number)
	assert.Equal(t, "", rows[1].Get("uom"), "a short row reads as empty")

	_, err = Parse([][]string{{"unit"}, {"x"}}, fields, "items", 10)
	assert.EqualError(t, err, "Missing column(s): item_code. Download the template to see every column.")
	_, err = Parse([][]string{{"code"}}, fields, "items", 10)
	assert.EqualError(t, err, "The sheet has a header but no items under it.")
	_, err = Parse([][]string{{"code"}, {"a"}, {"b"}}, fields, "items", 1)
	assert.Contains(t, err.Error(), "more than 1 items")
}

func TestBoolNumberDate(t *testing.T) {
	for in, want := range map[string]bool{"Yes": true, "y": true, "1": true, "TRUE": true, "no": false, "N": false, "0": false} {
		v, ok := Bool(in, !want)
		assert.True(t, ok, in)
		assert.Equal(t, want, v, in)
	}
	v, ok := Bool("", true)
	assert.True(t, ok && v, "empty is the default")
	_, ok = Bool("maybe", false)
	assert.False(t, ok)

	n, ok := Number(" 1,25,000.50 ")
	require.True(t, ok)
	assert.Equal(t, 125000.5, *n)
	n, ok = Number("18%")
	require.True(t, ok)
	assert.Equal(t, 18.0, *n)
	n, ok = Number("")
	assert.True(t, ok)
	assert.Nil(t, n)
	_, ok = Number("ten")
	assert.False(t, ok)

	assert.Equal(t, "2020-04-01", Date("01/04/2020"))
	assert.Equal(t, "2020-04-01", Date("01-04-2020"))
	assert.Equal(t, "2020-04-01", Date("2020-04-01"))
	assert.Equal(t, "April", Date("April"))
}

func TestExportRoundTrips(t *testing.T) {
	data, err := Export("Items", []Field{{Header: "item_code"}, {Header: "mobile"}}, [][]string{{"007", "9845012345"}})
	require.NoError(t, err)
	rows, err := ReadFirstSheet(bytesReader(data))
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"item_code", "mobile"}, {"007", "9845012345"}}, rows, "codes keep leading zeros")
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func TestHeaderKeyPercent(t *testing.T) {
	assert.Equal(t, "gstpercent", HeaderKey("GST %"))
	assert.Equal(t, HeaderKey("gst_percent"), HeaderKey("GST %"))
	assert.Equal(t, "itemcode", HeaderKey(" Item-Code "))
}
