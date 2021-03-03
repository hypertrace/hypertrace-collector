#!/bin/bash

set -e 

EXPORTED_TRACE=${1:-./exported-trace.json}

SCRIPT_PATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

curl -X POST http://localhost:9411/api/v2/spans \
  -H "Content-Type: application/json" \
  --data-binary "@${SCRIPT_PATH}/ingestion-trace.json"

while [ ! -f $EXPORTED_TRACE ]; do sleep 1; done

# We only read the last line as it is the one which is of our interest. Everytime
# the collector outputs a trace into the file, it adds a new line with JSON rather
# than creating an array of objects.
TRACE_ID=$(tail -n 1 $EXPORTED_TRACE | jq -r ".resourceSpans[0].instrumentationLibrarySpans[0].spans[0].traceId")

if [ "$TRACE_ID" == "cb5a198128c2f36138d3d48c4b72cd0e" ]; then
  echo "Trace ID has the expected value."
else
  echo "Unexpected trace ID \"$TRACE_ID\"."
  exit 1
fi
