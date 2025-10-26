#!/bin/bash
set -e

echo "Building tunnelled for Linux (multiple architectures)..."
echo

# Generate version in yyyy.DDmm format
VERSION=$(date +"%Y.%d%m")
BUILD_DATE=$(date)

echo "Building tunnelled v${VERSION}..."
echo "Build date: ${BUILD_DATE}"
echo

# Set common environment variable
export GOOS=linux
# Disable CGO for static binaries
export CGO_ENABLED=0

echo "Building for Linux AMD64 (Intel/AMD 64-bit)..."
export GOARCH=amd64
if go build -ldflags="-X 'tunnelled/internal/version.Version=${VERSION}' -X 'tunnelled/internal/version.BuildDate=${BUILD_DATE}'" -o tunnelled-linux-amd64 ./cmd/tunnelled; then
    echo "✓ AMD64 build successful!"
else
    echo "✗ AMD64 build failed!"
    exit 1
fi

echo
echo "Building for Linux ARM64 (ARM 64-bit, for modern servers)..."
export GOARCH=arm64
if go build -ldflags="-X 'tunnelled/internal/version.Version=${VERSION}' -X 'tunnelled/internal/version.BuildDate=${BUILD_DATE}'" -o tunnelled-linux-arm64 ./cmd/tunnelled; then
    echo "✓ ARM64 build successful!"
else
    echo "✗ ARM64 build failed!"
    exit 1
fi

echo
echo "Building for Linux ARM (32-bit, for Raspberry Pi)..."
export GOARCH=arm
if go build -ldflags="-X 'tunnelled/internal/version.Version=${VERSION}' -X 'tunnelled/internal/version.BuildDate=${BUILD_DATE}'" -o tunnelled-linux-arm ./cmd/tunnelled; then
    echo "✓ ARM build successful!"
else
    echo "✗ ARM build failed!"
    exit 1
fi

echo
echo "===== BUILD SUMMARY ====="
if [ -f tunnelled-linux-amd64 ]; then
    echo "✓ tunnelled-linux-amd64 - For Intel/AMD 64-bit servers"
fi
if [ -f tunnelled-linux-arm64 ]; then
    echo "✓ tunnelled-linux-arm64 - For ARM 64-bit servers"
fi
if [ -f tunnelled-linux-arm ]; then
    echo "✓ tunnelled-linux-arm - For ARM 32-bit devices"
fi

echo "Build complete!"

# Reset environment variables
unset GOOS
unset GOARCH

echo