# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-proto gen-drive gen-chat gen-myaccount

# gen all
gen-all: gen-proto gen-drive gen-chat gen-myaccount

# no yet implemented
# MyAccount BFF API
gen-myaccount:
	@echo "Generating MyAccount API..."
	@mkdir -p server/bff/gen/myaccount/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/myaccount/v1/oapi-codegen.yaml \
		-o server/bff/gen/myaccount/v1/myaccount.gen.go \
		api/openapi/myaccount/v1/openapi.yaml

# protobuf code generation
gen-proto:
	buf generate
