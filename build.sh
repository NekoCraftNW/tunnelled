#!/bin/bash

# Generate version in yyyy.DDmm format
VERSION=$(date +"%Y.%d%m")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags with version injection
LDFLAGS="-X 'tunnelled/internal/version.Version=${VERSION}' -X 'tunnelled/internal/version.BuildDate=${BUILD_DATE}'"

echo "Building tunnelled v${VERSION}..."
echo "Build date: ${BUILD_DATE}"
echo

# Build for current platform
echo "Building for current platform..."

ACTUAL_OS=$(go env GOOS)
ACTUAL_ARCH=$(go env GOARCH)

BINARY_NAME="tunnelled-${ACTUAL_OS}-${ACTUAL_ARCH}"

CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o ${BINARY_NAME} ./cmd/tunnelled
if [ $? -eq 0 ]; then
    echo "✓ Build successful!"
    echo "✓ Binary created: $BINARY_NAME"
    echo
    echo "Run with: ./${BINARY_NAME} --version"
else
    echo "✗ Build failed!"
    exit 1
fi

echo "Build complete!"