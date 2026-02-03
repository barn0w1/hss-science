# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-accounts gen-drive

# gen all
gen-all: gen-accounts gen-drive

# Accounts API
gen-accounts:
	@echo "Generating Accounts API..."
	@mkdir -p server/gateway/gen/accounts/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/accounts/v1/oapi-codegen.yaml \
		-o server/gateway/gen/accounts/v1/accounts.gen.go \
		api/openapi/accounts/v1/accounts.yaml

# Drive API
gen-drive:
	@echo "Generating Drive API..."
	@mkdir -p server/gateway/gen/drive/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/drive/v1/oapi-codegen.yaml \
		-o server/gateway/gen/drive/v1/drive.gen.go \
		api/openapi/drive/v1/drive.yaml