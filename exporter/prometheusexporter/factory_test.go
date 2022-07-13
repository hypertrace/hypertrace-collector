// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Endpoint = ""
	exp, err := createMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg)
	require.Equal(t, errBlankPrometheusAddress, err)
	require.Nil(t, exp)
}

func TestCreateMetricsExporterExportHelperError(t *testing.T) {
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	cfg.Endpoint = "http://localhost:8889"

	set := componenttest.NewNopExporterCreateSettings()
	set.Logger = nil

	// Should give us an exporterhelper.errNilLogger
	exp, err := createMetricsExporter(context.Background(), set, cfg)

	assert.Nil(t, exp)
	assert.Error(t, err)
}
