package repository_auth

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateKeepsValidUTF8(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	got := truncate("ab€€€", 3)
	assert.Equal(t, "ab€", got)
	assert.True(t, utf8.ValidString(got))
}
