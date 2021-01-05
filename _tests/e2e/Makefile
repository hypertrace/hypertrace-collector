run-collector:
	rm $(PWD)/trace-log.json || true
	touch $(PWD)/trace-log.json
	docker run -p 9411:9411 \
	-v $(PWD)/test-config.yml:/etc/opt/hypertrace/config.yml \
	-v $(PWD)/trace-log.json:/var/log/hypertrace-collector.json \
	hypertrace/collector:dev

test:
	./test.sh