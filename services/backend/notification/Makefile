.PHONY: vendor-rm
vendor-rm:
	rm -rf vendor-proto

.PHONY: vendor-proto/google/protobuf
vendor-proto/google/protobuf:
	git clone -b main --single-branch -n --depth=1 --filter=tree:0 \
		https://github.com/protocolbuffers/protobuf vendor-proto/protobuf &&\
	cd vendor-proto/protobuf &&\
	git sparse-checkout set --no-cone src/google/protobuf &&\
	git checkout
	mkdir -p vendor-proto/google
	mv vendor-proto/protobuf/src/google/protobuf vendor-proto/google
	rm -rf vendor-proto/protobuf

.PHONY: vendor-proto/validate
vendor-proto/validate:
	git clone -b main --single-branch --depth=2 --filter=tree:0 \
	https://github.com/bufbuild/protoc-gen-validate vendor-proto/tmp && \
	cd vendor-proto/tmp && \
	git sparse-checkout set --no-cone validate &&\
	git checkout
	mkdir -p vendor-proto/validate
	mv vendor-proto/tmp/validate vendor-proto/
	rm -rf vendor-proto/tmp

.PHONY: vendor-proto/google/api
vendor-proto/google/api:
	git clone -b master --single-branch -n --depth=1 --filter=tree:0 \
 		https://github.com/googleapis/googleapis vendor-proto/googleapis && \
 	cd vendor-proto/googleapis && \
	git sparse-checkout set --no-cone google/api && \
	git checkout
	mkdir -p  vendor-proto/google
	mv vendor-proto/googleapis/google/api vendor-proto/google
	rm -rf vendor-proto/googleapis

.PHONY: vendor-proto/protoc-gen-openapiv2/options
vendor-proto/protoc-gen-openapiv2/options:
	git clone -b main --single-branch -n --depth=1 --filter=tree:0 \
 		https://github.com/grpc-ecosystem/grpc-gateway vendor-proto/grpc-ecosystem && \
 	cd vendor-proto/grpc-ecosystem && \
	git sparse-checkout set --no-cone protoc-gen-openapiv2/options && \
	git checkout
	mkdir -p vendor-proto/protoc-gen-openapiv2
	mv vendor-proto/grpc-ecosystem/protoc-gen-openapiv2/options vendor-proto/protoc-gen-openapiv2
	rm -rf vendor-proto/grpc-ecosystem

.PHONY: vendor-proto
vendor-proto: vendor-rm vendor-proto/google/protobuf vendor-proto/validate vendor-proto/google/api vendor-proto/protoc-gen-openapiv2/options

# Generate this service's API: gRPC + REST gateway + Swagger, into pkg/api,
# then copy the swagger spec into internal/infra/docs for embedding.
.PHONY: generate
generate:
	protoc ./api/notification/v1/notification.proto \
		--proto_path=./api \
		--proto_path=./vendor-proto \
		--go_out=./pkg/api \
		--go_opt=paths=source_relative \
		--go-grpc_out=./pkg/api \
		--go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=./pkg/api \
		--grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=./pkg/api \
		--openapiv2_opt=logtostderr=true
	mkdir -p ./internal/infra/docs
	cp ./pkg/api/notification/v1/notification.swagger.json ./internal/infra/docs/notification.swagger.json
