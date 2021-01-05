curl -i -X POST "http://localhost:9411/api/v2/spans" \
-H "Content-Type: application/json" \
--data-binary "@testdata/test-trace.json"
