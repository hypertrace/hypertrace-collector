# Hotfixes for OpenTelemetry collector

All fixes and features for the upstream [open-telemetry/opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector)
should be submitted to the upstream project. However, merge and release into the upstream might take some time
therefore urgent patches can be applied to the fork [hypertrace/opentelemetry-collector](https://github.com/hypertrace/opentelemetry-collector).

Edit `go.mod` to use forked collector:

```bash
go mod edit -replace  go.opentelemetry.io/collector=github.com/hypertrace/opentelemetry-collector@d61af22c3882c312004871795f4288c09f98e372
```
