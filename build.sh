#!/bin/bash
# Build script for GoBGP PureLB fork
# This script injects version information (git commit) into the binaries

set -e

# Get git commit hash
COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags for version injection
LDFLAGS="-X github.com/osrg/gobgp/v4/internal/pkg/version.COMMIT=${COMMIT}"

echo "Building GoBGP PureLB fork..."
echo "  Commit: ${COMMIT}"
echo "  Build Date: ${BUILD_DATE}"
echo ""

# Build gobgpd
echo "Building gobgpd..."
go build -ldflags "${LDFLAGS}" -o gobgpd ./cmd/gobgpd
echo "  ✓ gobgpd built successfully"

# Build gobgp
echo "Building gobgp..."
go build -ldflags "${LDFLAGS}" -o gobgp ./cmd/gobgp
echo "  ✓ gobgp built successfully"

echo ""
echo "Build complete!"
echo ""
echo "Version information:"
./gobgpd --version
