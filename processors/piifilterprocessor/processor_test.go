package piifilterprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewProcessor(t *testing.T) {
	_, err := newPIIFilterProcessor(zap.NewNop(), &Config{
		ComplexData: []PiiComplexData{
			{Key: "test_attribute", Type: "unknown"},
		},
	})
	assert.Error(t, err)
	assert.Equal(t, "unknown type \"unknown\" for structured data", err.Error())
}
