package main

import (
	"fmt"
	"log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/ballastextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/multierr"

	"github.com/hypertrace/collector/processors/metricremover"
	"github.com/hypertrace/collector/processors/metricresourceattrstoattrs"
	"github.com/hypertrace/collector/processors/ratelimiter"
	"github.com/hypertrace/collector/processors/spancounter"
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

	if err := run(otelcol.CollectorSettings{BuildInfo: info, Factories: factories}); err != nil {
		log.Fatal(err)
	}
}

func components() (otelcol.Factories, error) {
	factories, err := defaultComponents()
	if err != nil {
		return otelcol.Factories{}, err
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

	prf := prometheusreceiver.NewFactory()
	factories.Receivers[prf.Type()] = prf

	tidpf := tenantidprocessor.NewFactory()
	factories.Processors[tidpf.Type()] = tidpf

	rlpf := ratelimiter.NewFactory()
	factories.Processors[rlpf.Type()] = rlpf

	mrata := metricresourceattrstoattrs.NewFactory()
	factories.Processors[mrata.Type()] = mrata

	mr := metricremover.NewFactory()
	factories.Processors[mr.Type()] = mr

	routingProcessor := routingprocessor.NewFactory()
	factories.Processors[routingProcessor.Type()] = routingProcessor

	sc := spancounter.NewFactory()
	factories.Processors[sc.Type()] = sc

	fp := filterprocessor.NewFactory()
	factories.Processors[fp.Type()] = fp

	fef := fileexporter.NewFactory()
	factories.Exporters[fef.Type()] = fef

	kef := kafkaexporter.NewFactory()
	factories.Exporters[kef.Type()] = kef

	pef := prometheusexporter.NewFactory()
	factories.Exporters[pef.Type()] = pef

	return factories, nil
}

func run(settings otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(settings)
	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("collector server run finished with error: %w", err)
	}

	return nil
}

func registerMetricViews() error {
	views := tenantidprocessor.MetricViews()
	views = append(views, spancounter.MetricViews()...)
	views = append(views, ratelimiter.MetricViews()...)
	return view.Register(views...)
}

// defaultComponents() is defined here since service/defaultcomponents pkg was
// removed in the otel collector repo.
func defaultComponents() (otelcol.Factories, error) {
	var errs error

	extensions, err := extension.MakeFactoryMap(
		zpagesextension.NewFactory(),
		ballastextension.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	receivers, err := receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	exporters, err := exporter.MakeFactoryMap(
		loggingexporter.NewFactory(),
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	processors, err := processor.MakeFactoryMap(
		batchprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
	)
	errs = multierr.Append(errs, err)

	factories := otelcol.Factories{
		Extensions: extensions,
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
	}

	return factories, errs
}
