#!/usr/bin/env bash
# Convenient build script for lekvc C++ client

set -e

echo "Building lekvc C++ client with WebRTC APM..."

# Create build directory if it doesn't exist
mkdir -p build
cd build

# Clean previous build (optional, uncomment if needed)
# rm -rf *

# Configure and build
cmake ..
make -j$(nproc)

echo ""
echo "✅ Build successful!"
echo "Executable: $(pwd)/lekvc_client"
echo ""
echo "To run: ./lekvc_client"
echo "Or from project root: client_cpp/build/lekvc_client"

