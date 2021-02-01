# E2E tests

This test suite launches a collector, sends a trace and inspect the output trace to match the expected data.

## Run collector with test config

In terminal 1:

```bash
make run-collector
// or make run-docker-collector
```

In terminal 2

```bash
make test
```
