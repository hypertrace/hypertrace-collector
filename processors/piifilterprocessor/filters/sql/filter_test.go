package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func TestRedactsWithNoSQLMatchings(t *testing.T) {
	filter := newFilter()

	attrValue := pdata.NewAttributeValueString("abc123")
	redacted, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(redacted.Redacted))
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsWithSQL(t *testing.T) {
	filter := newFilter()
	attrValue := pdata.NewAttributeValueString("select password from user where name = 'dave' or name =\"bob\";")
	parsedAttribute, err := filter.RedactAttribute("sql.query", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"sql.query": "select password from user where name = 'dave' or name =\"bob\";"}, parsedAttribute.Redacted)
	assert.Equal(t, "select password from user where name = '***' or name =\"***\";", attrValue.StringVal())

}

func newFilter() *sqlFilter {
	return &sqlFilter{redaction.DefaultRedactor}
}
