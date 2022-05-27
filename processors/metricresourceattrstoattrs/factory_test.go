package metricresourceattrstoattrs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)

	assert.NotNil(t, f.CreateDefaultConfig())
}
