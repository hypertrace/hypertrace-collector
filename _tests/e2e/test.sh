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

# Assertions declared here should be also declared in the test-config.yml

# Key base redaction
CARD_LAST_4=$(tail -n 1 $EXPORTED_TRACE | jq -r '.resourceSpans[].instrumentationLibrarySpans[].spans[].attributes[] | select (.key | contains("card.last_4")) | .value.stringValue')

if [ "$CARD_LAST_4" == "***" ]; then
  echo "Attribute card.last_4 has been redacted correctly."
else
  echo "Attribute card.last_4 hasn't been redacted correctly: \"$CARD_LAST_4\"."
  exit 1
fi

# JSON payload key based redaction
JSON_PASSWORD=$(tail -n 1 $EXPORTED_TRACE | jq -r '.resourceSpans[].instrumentationLibrarySpans[].spans[].attributes[] | select (.key | contains("http.request.body")) | .value.stringValue | select (. | contains("password"))' | jq -r '.password')

if [ "$JSON_PASSWORD" == "***" ]; then
  echo "Attribute http.request.body has been redacted correctly."
else
  echo "Attribute http.request.body hasn't been redacted correctly, password field: \"$JSON_PASSWORD\"."
  exit 1
fi
