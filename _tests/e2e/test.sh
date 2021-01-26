#!/bin/bash

set -e 

curl -X POST http://localhost:9411/api/v2/spans \
  -H "Content-Type: application/json" \
  --data-binary "@./ingestion-trace.json"

while [ ! -f ./exported-trace.json ]; do sleep 1; done

TRACE_ID=$(tail -n 1 ./exported-trace.json | jq -r ".resourceSpans[0].instrumentationLibrarySpans[0].spans[0].traceId")

test "$TRACE_ID" = "cb5a198128c2f36138d3d48c4b72cd0e"
