#!/bin/bash
# Script to regenerate protobuf files for GoBGP
# This script ensures proper paths and generates both .pb.go and _grpc.pb.go files

set -e  # Exit on error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Add GOPATH/bin to PATH if it's not already there
if [ -n "$GOPATH" ]; then
    export PATH="$GOPATH/bin:$PATH"
elif [ -d "$HOME/go/bin" ]; then
    export PATH="$HOME/go/bin:$PATH"
fi

echo "=== GoBGP Protocol Buffer Generation ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "ERROR: protoc not found. Please install protobuf compiler:"
    echo "  Debian/Ubuntu: sudo apt-get install protobuf-compiler"
    echo "  macOS: brew install protobuf"
    echo "  or download from https://github.com/protocolbuffers/protobuf/releases"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "ERROR: protoc-gen-go not found. Installing..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "ERROR: protoc-gen-go-grpc not found. Installing..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo "Installed tools:"
echo "  protoc version: $(protoc --version)"
echo "  protoc-gen-go: $(which protoc-gen-go)"
echo "  protoc-gen-go-grpc: $(which protoc-gen-go-grpc)"
echo ""

cd "$PROJECT_ROOT"

# Create api directory if it doesn't exist
mkdir -p api

echo "Generating protobuf files..."

# The key is to:
# 1. Run from project root
# 2. Use proto/ as the proto_path so imports like "api/common.proto" work
# 3. Generate into the current directory (.) which will create api/*.pb.go

protoc \
  --proto_path=proto \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  proto/api/*.proto

echo ""
echo "Generated files:"
ls -lh api/*.pb.go | awk '{print "  " $9 " (" $5 ")"}'

echo ""
echo "=== Protocol Buffer Generation Complete ==="
echo ""
echo "Files generated in: $PROJECT_ROOT/api/"
echo ""
echo "To rebuild gobgpd and gobgp:"
echo "  go build -o gobgpd ./cmd/gobgpd"
echo "  go build -o gobgp ./cmd/gobgp"
