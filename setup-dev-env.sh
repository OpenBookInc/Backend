#!/bin/bash

# =============================================================================
# Development Environment Setup Script
# =============================================================================
# This script ensures all developers have the same versions of dependencies
# installed for consistent builds across the team.
#
# Usage: ./setup-dev-env.sh
# =============================================================================

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# Version Definitions - Update these when upgrading dependencies
# =============================================================================
GO_VERSION="1.25.4"
RUST_VERSION="1.91.1"
PROTOC_VERSION="33.2"
GOOSE_VERSION="v3.26.0"
POSTGRESQL_VERSION="17"

# =============================================================================
# Helper Functions
# =============================================================================
print_header() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}============================================${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

check_command() {
    if command -v "$1" &> /dev/null; then
        return 0
    else
        return 1
    fi
}

# =============================================================================
# OS Detection
# =============================================================================
detect_os() {
    case "$(uname -s)" in
        Darwin*)    OS="macos";;
        Linux*)     OS="linux";;
        MINGW*|MSYS*|CYGWIN*) OS="windows";;
        *)          OS="unknown";;
    esac
    
    case "$(uname -m)" in
        x86_64)     ARCH="amd64";;
        arm64|aarch64) ARCH="arm64";;
        *)          ARCH="unknown";;
    esac
    
    echo "Detected OS: $OS, Architecture: $ARCH"
}

# =============================================================================
# Go Installation/Verification
# =============================================================================
setup_go() {
    print_header "Setting up Go $GO_VERSION"
    
    if check_command go; then
        CURRENT_GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+' | sed 's/go//')
        if [ "$CURRENT_GO_VERSION" == "$GO_VERSION" ]; then
            print_success "Go $GO_VERSION is already installed"
            return 0
        else
            print_warning "Go $CURRENT_GO_VERSION is installed, but $GO_VERSION is required"
        fi
    fi
    
    if [ "$OS" == "macos" ]; then
        if check_command brew; then
            print_info "Installing Go via Homebrew..."
            brew install go || brew upgrade go
            print_success "Go installed via Homebrew"
            return 0
        else
            print_error "Homebrew not found. Please install Homebrew first: https://brew.sh"
            return 1
        fi
    elif [ "$OS" == "linux" ]; then
        print_info "On Linux, download from https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
        return 1
    fi
    
    return 1
}

# =============================================================================
# Rust Installation/Verification
# =============================================================================
setup_rust() {
    print_header "Setting up Rust $RUST_VERSION"
    
    if check_command rustc; then
        CURRENT_RUST_VERSION=$(rustc --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
        if [ "$CURRENT_RUST_VERSION" == "$RUST_VERSION" ]; then
            print_success "Rust $RUST_VERSION is already installed"
            return 0
        else
            print_warning "Rust $CURRENT_RUST_VERSION is installed, expected $RUST_VERSION"
            print_info "Updating Rust toolchain..."
            rustup install "$RUST_VERSION"
            rustup default "$RUST_VERSION"
            print_success "Rust updated to $RUST_VERSION"
            return 0
        fi
    fi
    
    if [ "$OS" == "macos" ]; then
        if check_command brew; then
            print_info "Installing rustup via Homebrew..."
            brew install rustup-init
            rustup-init -y --default-toolchain "$RUST_VERSION"
            source "$HOME/.cargo/env"
            print_success "Rust $RUST_VERSION installed via Homebrew + rustup"
            return 0
        fi
    fi
    
    # Fallback to rustup installer
    print_info "Installing Rust via rustup..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain "$RUST_VERSION"
    source "$HOME/.cargo/env"
    print_success "Rust $RUST_VERSION installed"
}

# =============================================================================
# Protocol Buffers Installation
# =============================================================================
setup_protoc() {
    print_header "Setting up Protocol Buffers (protoc) $PROTOC_VERSION"
    
    if check_command protoc; then
        CURRENT_PROTOC_VERSION=$(protoc --version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?')
        if [ "$CURRENT_PROTOC_VERSION" == "$PROTOC_VERSION" ]; then
            print_success "protoc $PROTOC_VERSION is already installed"
            return 0
        else
            print_warning "protoc $CURRENT_PROTOC_VERSION is installed, but $PROTOC_VERSION is required"
        fi
    fi
    
    if [ "$OS" == "macos" ]; then
        if check_command brew; then
            print_info "Installing protobuf via Homebrew..."
            brew install protobuf || brew upgrade protobuf
            print_success "protoc installed via Homebrew"
            print_warning "Homebrew may install a different version than $PROTOC_VERSION"
            return 0
        else
            print_error "Homebrew not found. Please install Homebrew first: https://brew.sh"
            return 1
        fi
    elif [ "$OS" == "linux" ]; then
        PROTOC_ZIP="protoc-${PROTOC_VERSION}-linux-x86_64.zip"
        if [ "$ARCH" == "arm64" ]; then
            PROTOC_ZIP="protoc-${PROTOC_VERSION}-linux-aarch_64.zip"
        fi
        
        PROTOC_URL="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${PROTOC_ZIP}"
        
        print_info "Downloading from $PROTOC_URL"
        
        TMP_DIR=$(mktemp -d)
        curl -Lo "$TMP_DIR/protoc.zip" "$PROTOC_URL"
        unzip -o "$TMP_DIR/protoc.zip" -d "$HOME/.local"
        rm -rf "$TMP_DIR"
        
        # Add to PATH if not already there
        if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
            export PATH="$HOME/.local/bin:$PATH"
        fi
        
        print_success "protoc $PROTOC_VERSION installed to ~/.local/bin"
    fi
}

# =============================================================================
# Go Protobuf Plugins (versions determined by go.mod)
# =============================================================================
setup_protoc_go_plugins() {
    print_header "Setting up Go protobuf plugins"
    
    # Versions are tied to google.golang.org/protobuf and google.golang.org/grpc in go.mod
    print_info "Installing protoc-gen-go (version from go.mod)..."
    go install "google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    
    print_info "Installing protoc-gen-go-grpc (version from go.mod)..."
    go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    
    # Ensure GOPATH/bin is in PATH
    GOBIN=$(go env GOBIN)
    if [ -z "$GOBIN" ]; then
        GOBIN="$(go env GOPATH)/bin"
    fi
    
    if [[ ":$PATH:" != *":$GOBIN:"* ]]; then
        print_warning "GOPATH/bin is not in PATH. Add this to your shell profile:"
        print_info "export PATH=\"\$PATH:$GOBIN\""
    fi
    
    print_success "Go protobuf plugins installed"
}

# =============================================================================
# Goose (Database Migrations)
# =============================================================================
setup_goose() {
    print_header "Setting up Goose $GOOSE_VERSION"
    
    if check_command goose; then
        print_success "Goose is already installed: $(goose --version 2>&1 | head -1)"
        return 0
    fi
    
    if [ "$OS" == "macos" ]; then
        if check_command brew; then
            print_info "Installing goose via Homebrew..."
            brew install goose
            print_success "Goose installed via Homebrew"
            return 0
        else
            print_error "Homebrew not found. Please install Homebrew first: https://brew.sh"
            return 1
        fi
    else
        # Fallback for non-macOS
        print_info "Installing goose@${GOOSE_VERSION} via go install..."
        go install "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"
        
        if check_command goose; then
            print_success "Goose installed: $(goose --version 2>&1 | head -1)"
        else
            GOBIN=$(go env GOBIN)
            if [ -z "$GOBIN" ]; then
                GOBIN="$(go env GOPATH)/bin"
            fi
            print_warning "Goose installed but not in PATH. Binary is at: $GOBIN/goose"
        fi
    fi
}

# =============================================================================
# PostgreSQL Client Tools (psql)
# =============================================================================
setup_postgresql() {
    print_header "Setting up PostgreSQL $POSTGRESQL_VERSION client tools"

    # Check PATH first, then keg-only locations
    if check_command psql; then
        CURRENT_PSQL_VERSION=$(psql --version | grep -oE '[0-9]+' | head -1)
        print_success "psql is already installed (version $CURRENT_PSQL_VERSION) and in PATH"
        return 0
    fi

    # Check keg-only Homebrew location
    BREW_PSQL="/opt/homebrew/opt/postgresql@${POSTGRESQL_VERSION}/bin/psql"
    if [ -x "$BREW_PSQL" ]; then
        print_success "psql found at $BREW_PSQL (keg-only, not in PATH)"
        print_warning "To add to PATH, add this to your shell profile:"
        print_info "export PATH=\"/opt/homebrew/opt/postgresql@${POSTGRESQL_VERSION}/bin:\$PATH\""
        return 0
    fi

    if [ "$OS" == "macos" ]; then
        if check_command brew; then
            print_info "Installing postgresql@${POSTGRESQL_VERSION} via Homebrew..."
            brew install "postgresql@${POSTGRESQL_VERSION}"
            print_success "PostgreSQL $POSTGRESQL_VERSION installed via Homebrew"
            print_warning "This is a keg-only formula. To add to PATH:"
            print_info "export PATH=\"/opt/homebrew/opt/postgresql@${POSTGRESQL_VERSION}/bin:\$PATH\""
            return 0
        else
            print_error "Homebrew not found. Please install Homebrew first: https://brew.sh"
            return 1
        fi
    elif [ "$OS" == "linux" ]; then
        print_info "On Linux, install via: sudo apt-get install postgresql-client-${POSTGRESQL_VERSION}"
        return 1
    fi

    return 1
}

# =============================================================================
# Verify Installation
# =============================================================================
verify_installation() {
    print_header "Verifying Installation"
    
    echo ""
    echo "Tool Versions:"
    echo "----------------------------------------"
    
    if check_command go; then
        echo "Go:      $(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+')"
    else
        print_error "Go: NOT INSTALLED"
    fi
    
    if check_command rustc; then
        echo "Rust:    $(rustc --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"
    else
        print_error "Rust: NOT INSTALLED"
    fi
    
    if check_command cargo; then
        echo "Cargo:   $(cargo --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"
    else
        print_error "Cargo: NOT INSTALLED"
    fi
    
    if check_command protoc; then
        echo "protoc:  $(protoc --version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?')"
    else
        print_error "protoc: NOT INSTALLED"
    fi
    
    if check_command protoc-gen-go; then
        echo "protoc-gen-go: installed"
    else
        print_warning "protoc-gen-go: not in PATH (may be in local .bin)"
    fi
    
    if check_command protoc-gen-go-grpc; then
        echo "protoc-gen-go-grpc: installed"
    else
        print_warning "protoc-gen-go-grpc: not in PATH (may be in local .bin)"
    fi
    
    if check_command goose; then
        echo "goose:   $(goose --version 2>&1 | head -1)"
    else
        print_warning "goose: not in PATH"
    fi

    if check_command psql; then
        echo "psql:    $(psql --version | grep -oE '[0-9]+\.[0-9]+')"
    elif [ -x "/opt/homebrew/opt/postgresql@${POSTGRESQL_VERSION}/bin/psql" ]; then
        echo "psql:    $(/opt/homebrew/opt/postgresql@${POSTGRESQL_VERSION}/bin/psql --version | grep -oE '[0-9]+\.[0-9]+') (keg-only, not in PATH)"
    else
        print_warning "psql: NOT INSTALLED"
    fi

    echo "----------------------------------------"
    echo ""
}

# =============================================================================
# Print Version Reference
# =============================================================================
print_version_reference() {
    print_header "Version Reference"
    
    echo ""
    echo "Required Versions for this Project:"
    echo "----------------------------------------"
    echo "Go:                    $GO_VERSION"
    echo "Rust:                  $RUST_VERSION"
    echo "protoc:                $PROTOC_VERSION"
    echo "goose:                 $GOOSE_VERSION"
    echo "postgresql:            $POSTGRESQL_VERSION"
    echo "protoc-gen-go:         (installed via go install)"
    echo "protoc-gen-go-grpc:    (installed via go install)"
    echo "----------------------------------------"
    echo ""
    echo "Note: Go/Rust package dependencies are managed by go.mod and Cargo.toml"
    echo "Run 'go mod download' and 'cargo fetch' to install them."
    echo ""
}

# =============================================================================
# Main Script
# =============================================================================
main() {
    echo ""
    echo "🚀 Development Environment Setup Script"
    echo "========================================"
    echo ""
    
    detect_os
    
    # Parse arguments
    SKIP_INSTALL=false
    VERIFY_ONLY=false
    
    for arg in "$@"; do
        case $arg in
            --verify)
                VERIFY_ONLY=true
                ;;
            --versions)
                print_version_reference
                exit 0
                ;;
            --help)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --verify    Only verify installations, don't install anything"
                echo "  --versions  Print required versions and exit"
                echo "  --help      Show this help message"
                exit 0
                ;;
        esac
    done
    
    if [ "$VERIFY_ONLY" = true ]; then
        verify_installation
        print_version_reference
        exit 0
    fi
    
    # Run setup
    setup_go || print_warning "Go setup incomplete - please install manually"
    setup_rust
    setup_protoc
    setup_protoc_go_plugins
    setup_goose
    setup_postgresql

    verify_installation
    print_version_reference
    
    print_success "Development environment setup complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Ensure all tools are in your PATH"
    echo "  2. Run 'go mod download' from the repo root to fetch Go dependencies (uses go.work)"
    echo "  3. Run 'cd matching_server && cargo fetch' to fetch Rust dependencies"
    echo ""
}

main "$@"
