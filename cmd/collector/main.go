package main

import (
	"fmt"
	"log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/defaultcomponents"

	"github.com/hypertrace/collector/processors/tenantidprocessor"
)

func main() {
	if err := registerMetricViews(); err != nil {
		log.Fatal(err)
	}

	factories, err := components()
	if err != nil {
		log.Fatalf("failed to build default components: %v", err)
	}

	info := component.BuildInfo{
		Command:     "collector",
		Description: "Hypertrace Collector",
		Version:     BuildVersion,
	}

	if err := run(service.CollectorSettings{BuildInfo: info, Factories: factories}); err != nil {
		log.Fatal(err)
	}
}

func components() (component.Factories, error) {
	factories, err := defaultcomponents.Components()
	if err != nil {
		return component.Factories{}, err
	}

	hcf := healthcheckextension.NewFactory()
	factories.Extensions[hcf.Type()] = hcf

	ppf := pprofextension.NewFactory()
	factories.Extensions[ppf.Type()] = ppf

	zrf := zipkinreceiver.NewFactory()
	factories.Receivers[zrf.Type()] = zrf

	ocrf := opencensusreceiver.NewFactory()
	factories.Receivers[ocrf.Type()] = ocrf

	jrf := jaegerreceiver.NewFactory()
	factories.Receivers[jrf.Type()] = jrf

	tidpf := tenantidprocessor.NewFactory()
	factories.Processors[tidpf.Type()] = tidpf

	fef := fileexporter.NewFactory()
	factories.Exporters[fef.Type()] = fef

	kef := kafkaexporter.NewFactory()
	factories.Exporters[kef.Type()] = kef

	pef := prometheusexporter.NewFactory()
	factories.Exporters[pef.Type()] = pef

	return factories, nil
}

func run(settings service.CollectorSettings) error {
	cmd := service.NewCommand(settings)
	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("collector server run finished with error: %w", err)
	}

	return nil
}

func registerMetricViews() error {
	views := tenantidprocessor.MetricViews()
	return view.Register(views...)
}
