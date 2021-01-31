run-collector:
	@rm $(PWD)/exported-trace.json || true
	@touch $(PWD)/exported-trace.json
	go run ../../cmd/collector/* --config ./test-config.yml

run-docker-collector:
	@rm $(PWD)/exported-trace.json || true
	@touch $(PWD)/exported-trace.json
	docker run -p 9411:9411 \
	-v $(pwd)/test-config.yml:/etc/opt/hypertrace/config.yml \
	-v $(pwd)/exported-trace.json:/usr/local/bin/hypertrace/exported-trace.json \
	hypertrace/hypertrace-collector:dev

test:
	@./test.sh