# Instructions

- Run collector

```bash
docker run -p 9411:9411 \
-v $(pwd)/test-config.yml:/etc/opt/hypertrace/config.yml \
-v $(pwd)/exported-trace-data.json:/var/log/hypertrace-collector.json \
hypertrace/collector:dev
```

- Run test:

```bash
./test.sh
```
