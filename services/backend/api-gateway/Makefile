# API Gateway / BFF — codegen.
#
# The gateway is a gRPC CLIENT to the five domain services. Its local proto copies
# (api/<svc>/v1/*.proto) are pure proto3 with no google.api.http annotations, so
# codegen needs neither vendored third-party protos nor network access — only the
# standard protoc-gen-go / protoc-gen-go-grpc plugins. This produces the message
# types and the <Svc>Client interfaces the gateway dials (and that tests fake).

SERVICES := identity poll surprise catalog notification

.PHONY: generate
generate:
	@for svc in $(SERVICES); do \
		echo "protoc $$svc..."; \
		protoc ./api/$$svc/v1/$$svc.proto \
			--proto_path=./api \
			--go_out=./pkg/api --go_opt=paths=source_relative \
			--go-grpc_out=./pkg/api --go-grpc_opt=paths=source_relative || exit 1; \
	done
	@echo "generate: done"

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test ./...

.PHONY: run
run:
	go run ./cmd/gateway
