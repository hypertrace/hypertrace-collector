#!/bin/bash

set -e 

EXPORTED_TRACE=${1:-./exported-trace.json}

SCRIPT_PATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

curl -X POST http://localhost:9411/api/v2/spans \
  -H "Content-Type: application/json" \
  --data-binary "@${SCRIPT_PATH}/ingestion-trace.json"

while [ ! -f $EXPORTED_TRACE ]; do sleep 1; done

TRACE_ID=$(tail -n 1 $EXPORTED_TRACE | jq -r ".resourceSpans[0].instrumentationLibrarySpans[0].spans[0].traceId")

test "$TRACE_ID" = "cb5a198128c2f36138d3d48c4b72cd0e"
