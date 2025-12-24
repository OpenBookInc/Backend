# Directories
BIN_DIR := .bin
PROTO_DIR := ../proto
GEN_OUT_DIR := src/gen
BUILD_DIR = out/build

# Plugin paths
PROTOC_GEN_GO := $(BIN_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(BIN_DIR)/protoc-gen-go-grpc

# Go module name
MODULE := matching-clients

# Install protoc plugins locally into .bin/
.PHONY: setup
setup:
	mkdir -p "$(BIN_DIR)"
	GOBIN="$(abspath $(BIN_DIR))" go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	GOBIN="$(abspath $(BIN_DIR))" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto files
.PHONY: codegen
codegen: setup
	mkdir -p "$(GEN_OUT_DIR)" "$(GEN_OUT_DIR)/matching" "$(GEN_OUT_DIR)/gateway"
	protoc \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--proto_path=$(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=$(MODULE) \
		--go_opt=MCommon.proto=$(MODULE)/$(GEN_OUT_DIR) \
		--go_opt=MMatchingService.proto=$(MODULE)/$(GEN_OUT_DIR)/matching \
		--go_opt=MGatewayService.proto=$(MODULE)/$(GEN_OUT_DIR)/gateway \
		--go-grpc_out=. \
		--go-grpc_opt=module=$(MODULE) \
		--go-grpc_opt=MCommon.proto=$(MODULE)/$(GEN_OUT_DIR) \
		--go-grpc_opt=MMatchingService.proto=$(MODULE)/$(GEN_OUT_DIR)/matching \
		--go-grpc_opt=MGatewayService.proto=$(MODULE)/$(GEN_OUT_DIR)/gateway \
		$(PROTO_DIR)/Common.proto $(PROTO_DIR)/MatchingService.proto $(PROTO_DIR)/GatewayService.proto

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

# Find all Go files in src/apps/
APP_SOURCES := $(wildcard src/apps/*.go)
# Generate binary names from source files (remove .go extension and path)
APP_BINARIES := $(patsubst src/apps/%.go,$(BUILD_DIR)/%,$(APP_SOURCES))

# Build all applications
.PHONY: all
all: $(APP_BINARIES)

# Pattern rule to build each app
$(BUILD_DIR)/%: src/apps/%.go
	@mkdir -p "$(BUILD_DIR)"
	go build -o $@ $<
	@echo "✅ Built $@"

# Individual app targets for backwards compatibility
.PHONY: matching-engine-tester
matching-engine-tester:
	mkdir -p "$(BUILD_DIR)"
	go build -o "$(BUILD_DIR)/MatchingEngineTester" ./cmd/matching-engine-tester

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf "$(BUILD_DIR)"
	@echo "✅ Cleaned build directory"
