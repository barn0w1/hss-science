# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-proto gen-drive gen-chat gen-myaccount

# gen all
gen-all: gen-proto gen-drive gen-chat gen-myaccount

# Drive API
gen-drive:
	@echo "Generating Drive API..."
	@mkdir -p server/bff/gen/drive/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/drive/v1/oapi-codegen.yaml \
		-o server/bff/gen/drive/v1/drive.gen.go \
		api/openapi/drive/v1/drive.yaml

gen-chat:
	@echo "Generating Chat API..."
	@mkdir -p server/bff/gen/chat/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/chat/v1/oapi-codegen.yaml \
		-o server/bff/gen/chat/v1/chat.gen.go \
		api/openapi/chat/v1/chat.yaml

# protobuf code generation
gen-proto:
	buf generate

# MyAccount BFF API
gen-myaccount:
	@echo "Generating MyAccount API..."
	@mkdir -p server/bff/gen/myaccount/v1
	$(OAPI_CODEGEN) \
		-generate types \
		-package myaccountv1 \
		-o server/bff/gen/myaccount/v1/types.gen.go \
		api/openapi/myaccount/v1/openapi.yaml