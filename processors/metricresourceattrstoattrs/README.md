# metricresourceattrstoattrs processor

The purpose of this processor is to copy over the resource attributes to the metric data point attributes with the exception of `job` and `instance` if present when `service.name` and `service.instance.id` are present. This is because the prometheus exporter, while scraping, adds them again and this might cause a `duplicate label names` error to be thrown back.

This is caused by the change added in https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/9115. More specifically https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/12cc610f93429fbd9dec71c5f486d266844f11c2/exporter/prometheusexporter/collector.go#L96


It similar to setting prometheus exporter's resource_to_telemetry_conversion enabled config to true with the above caveat.
```
prometheus:
  endpoint: "0.0.0.0:8889"
  resource_to_telemetry_conversion:
    enabled: true
```

Filed an issue with otel collector contrib: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10374