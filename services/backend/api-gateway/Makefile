# API Gateway / BFF — codegen.
#
# The gateway is a gRPC CLIENT to the five domain services. Its local proto copies
# (api/<svc>/v1/*.proto) are the REAL service protos (identical message/service/enum
# definitions, so the generated <Svc>ServiceClient stubs are wire-compatible with the
# running services). They carry google.api.http annotations + validate rules, so
# codegen resolves those imports from ./vendor-proto (populated from a service's
# vendor-proto). Only protoc-gen-go / protoc-gen-go-grpc are invoked (no gateway/
# openapiv2) — the gateway has its own hand-written REST edge.

SERVICES := identity poll surprise catalog notification

.PHONY: generate
generate:
	@for svc in $(SERVICES); do \
		echo "protoc $$svc..."; \
		protoc ./api/$$svc/v1/$$svc.proto \
			--proto_path=./api \
			--proto_path=./vendor-proto \
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
