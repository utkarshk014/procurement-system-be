package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDistinctTenantDatabases(t *testing.T) {
	assert.NoError(t, CheckDistinctTenantDatabases(map[string]string{
		"a": "postgres://u:p@db1:5432/esis_a",
		"b": "postgres://u:p@db1:5432/esis_b",
		"c": "postgres://u:p@db2:5432/esis_a",
	}))

	// Same host, port and database under another key (even with different options) mixes tenants.
	err := CheckDistinctTenantDatabases(map[string]string{
		"a":     "postgres://u:p@db1:5432/esis_a?sslmode=disable",
		"alias": "postgres://other:x@db1:5432/esis_a?application_name=x",
	})
	assert.ErrorContains(t, err, "point at the same database db1:5432/esis_a")

	assert.Error(t, CheckDistinctTenantDatabases(map[string]string{"bad": "::not a url::"}))
}
