package sql

import (
	"testing"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestRedactsWithNoSQLMatchings(t *testing.T) {
	filter := newFilter()

	attrValue := pdata.NewAttributeValueString("abc123")
	isRedacted, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.False(t, isRedacted)
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsWithSQL(t *testing.T) {
	filter := newFilter()
	attrValue := pdata.NewAttributeValueString("select password from user where name = 'dave' or name =\"bob\";")
	isRedacted, err := filter.RedactAttribute("sql.query", attrValue)
	assert.NoError(t, err)
	assert.True(t, isRedacted)
	assert.Equal(t, "select password from user where name = '***' or name =\"***\";", attrValue.StringVal())

}

func newFilter() *sqlFilter {
	return &sqlFilter{redaction.DefaultRedactor}
}
