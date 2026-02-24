# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-drive

# gen all
gen-all: gen-drive

# Drive API
gen-drive:
	@echo "Generating Drive API..."
	@mkdir -p server/bff/gen/drive/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/drive/v1/oapi-codegen.yaml \
		-o server/bff/gen/drive/v1/drive.gen.go \
		api/openapi/drive/v1/drive.yaml

# protobuf code generation
.PHONY: gen-proto

gen-proto:
	buf generate