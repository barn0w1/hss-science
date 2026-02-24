# OpenAPI Codegen
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: gen-all gen-drive gen-proto gen-keys

# gen all
gen-all: gen-proto gen-drive

# Drive API
gen-drive:
	@echo "Generating Drive API..."
	@mkdir -p server/bff/gen/drive/v1
	$(OAPI_CODEGEN) \
		-config api/openapi/drive/v1/oapi-codegen.yaml \
		-o server/bff/gen/drive/v1/drive.gen.go \
		api/openapi/drive/v1/drive.yaml

# protobuf code generation
gen-proto:
	buf generate

# Generate Ed25519 key pair for local development
gen-keys:
	@echo "Generating Ed25519 key pair for development..."
	openssl genpkey -algorithm Ed25519 -out dev_ed25519_private.pem
	openssl pkey -in dev_ed25519_private.pem -pubout -out dev_ed25519_public.pem
	@echo "Keys generated: dev_ed25519_private.pem, dev_ed25519_public.pem"