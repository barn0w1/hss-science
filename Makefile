# Configuration

PROTO_DIR := proto
GEN_GO_DIR := server/gen
OPENAPI_DIR := web/packages/api/openapi

PROTOC := protoc
GO_BIN := $(HOME)/go/bin

PROTOC_GEN_GO := $(GO_BIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(GO_BIN)/protoc-gen-go-grpc
PROTOC_GEN_OPENAPI := $(GO_BIN)/protoc-gen-openapiv2

# Tooling

.PHONY: tools
tools: $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(PROTOC_GEN_OPENAPI)

$(PROTOC_GEN_GO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

$(PROTOC_GEN_GO_GRPC):
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

$(PROTOC_GEN_OPENAPI):
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

# Proto generation

.PHONY: proto
proto: tools proto-go proto-openapi

.PHONY: proto-go
proto-go:
	mkdir -p $(GEN_GO_DIR)
	$(PROTOC) -I $(PROTO_DIR) \
	  --go_out=$(GEN_GO_DIR) --go_opt=paths=source_relative \
	  --go-grpc_out=$(GEN_GO_DIR) --go-grpc_opt=paths=source_relative \
	  $(PROTO_DIR)/public/accounts/v1/accounts.proto \
	  $(PROTO_DIR)/internal/accounts/v1/accounts_internal.proto

.PHONY: proto-openapi
proto-openapi:
	mkdir -p $(OPENAPI_DIR)
	$(PROTOC) -I $(PROTO_DIR) \
	  --openapiv2_out=$(OPENAPI_DIR) \
	  $(PROTO_DIR)/public/accounts/v1/accounts.proto
