# Directories
BIN_DIR := .bin
PROTO_DIR := proto
GEN_OUT_DIR := src/gen
BUILD_DIR = out/build

# Plugin paths
PROTOC_GEN_GO := $(BIN_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(BIN_DIR)/protoc-gen-go-grpc

# Install protoc plugins locally into .bin/
.PHONY: setup
setup:
	mkdir -p "$(BIN_DIR)"
	GOBIN="$(abspath $(BIN_DIR))" go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	GOBIN="$(abspath $(BIN_DIR))" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto files
.PHONY: codegen
codegen: setup
	mkdir -p "$(GEN_OUT_DIR)"
	protoc \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_OUT_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto

# Check if generated code is up-to-date, useful for CI/CD pipelines
.PHONY: check-codegen
check-codegen: codegen
	@if ! git diff --quiet -- "$(GEN_OUT_DIR)"; then \
		echo "❌ Generated code is not up to date. Run 'make codegen' and commit the changes."; \
		git --no-pager diff -- "$(GEN_OUT_DIR)"; \
		exit 1; \
	else \
		echo "✅ Generated code is up to date."; \
	fi

# Compile the MatchingServer client
.PHONY: matching-server-client
matching-server-client:
	mkdir -p "$(BUILD_DIR)"
	go build -o "$(BUILD_DIR)/matching-server-client" ./src/apps/MatchingServerClient.go
