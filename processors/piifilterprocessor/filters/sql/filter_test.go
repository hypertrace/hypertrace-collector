package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/hypertrace/collector/processors"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
)

func TestRedactsWithNoSQLMatchings(t *testing.T) {
	filter := newFilter()

	attrValue := pdata.NewAttributeValueString("abc123")
	parsedAttribute, newAttr, err := filter.RedactAttribute("unrelated", attrValue)
	assert.NoError(t, err)
	assert.Equal(t, &processors.ParsedAttribute{Redacted: map[string]string{}}, parsedAttribute)
	assert.Nil(t, newAttr)
	assert.Equal(t, "abc123", attrValue.StringVal())
}

func TestRedactsWithSQL(t *testing.T) {
	filter := newFilter()
	attrValue := pdata.NewAttributeValueString("select password from user where name = 'dave' or name =\"bob\";")
	parsedAttribute, newAttr, err := filter.RedactAttribute("sql.query", attrValue)
	assert.NoError(t, err)
	assert.Nil(t, newAttr)
	assert.Equal(t, &processors.ParsedAttribute{Redacted: map[string]string{"sql.query": "select password from user where name = 'dave' or name =\"bob\";"}}, parsedAttribute)
	assert.Equal(t, "select password from user where name = '***' or name =\"***\";", attrValue.StringVal())
}

func newFilter() *sqlFilter {
	return &sqlFilter{redaction.DefaultRedactor}
}
