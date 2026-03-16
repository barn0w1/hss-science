# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-proto gen-drive gen-chat gen-myaccount generate-bff

# gen all
gen-all: gen-proto gen-drive gen-chat gen-myaccount

# no yet implemented
# MyAccount BFF API
gen-myaccount: generate-bff

generate-bff:
	@echo "Generating MyAccount BFF types..."
	@mkdir -p server/services/myaccount-bff/internal/handler
	cd server && go tool oapi-codegen -generate types \
		-package handler \
		-o services/myaccount-bff/internal/handler/api_gen.go \
		../api/openapi/myaccount/v1/myaccount.yaml

# protobuf code generation
gen-proto:
	buf generate

