#!/bin/bash

# GIH-FTP Release Builder
# Bu script kaynak kodları BUILD ederek release paketi oluşturur
# Kaynak kodlar release paketine DAHİL EDİLMEZ

set -e

VERSION="${1:-v2.0.0}"
BUILD_DATE=$(date +"%Y-%m-%d")
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

RELEASE_DIR="releases/${VERSION}"
PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64")

echo "=================================================="
echo "   GIH-FTP Release Builder"
echo "=================================================="
echo "Version:     ${VERSION}"
echo "Build Date:  ${BUILD_DATE}"
echo "Commit:      ${COMMIT_HASH}"
echo "=================================================="
echo ""

# Temizlik
echo "🧹 Cleaning old releases..."
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

# Her platform için build
for platform in "${PLATFORMS[@]}"; do
    GOOS="${platform%/*}"
    GOARCH="${platform#*/}"

    OUTPUT_NAME="gihftp"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="gihftp.exe"
    fi

    PLATFORM_DIR="${RELEASE_DIR}/gihftp-${VERSION}-${GOOS}-${GOARCH}"

    echo "🔨 Building for ${GOOS}/${GOARCH}..."

    # Build binary
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.CommitHash=${COMMIT_HASH}" \
        -o "${PLATFORM_DIR}/${OUTPUT_NAME}" \
        .

    # Binary'ye execute permission
    chmod +x "${PLATFORM_DIR}/${OUTPUT_NAME}"

    # Sadece dokümantasyon ve config dosyalarını ekle (KAYNAK KOD YOK!)
    echo "📦 Creating release package..."
    cp README.md "${PLATFORM_DIR}/"
    cp gihftp.conf.example "${PLATFORM_DIR}/"
    cp LICENSE "${PLATFORM_DIR}/" 2>/dev/null || echo "# GIH-FTP License" > "${PLATFORM_DIR}/LICENSE"

    # Install script'lerini ekle (sadece Linux için)
    if [ "$GOOS" = "linux" ]; then
        cp install.sh "${PLATFORM_DIR}/"
        cp uninstall.sh "${PLATFORM_DIR}/"
        chmod +x "${PLATFORM_DIR}/install.sh"
        chmod +x "${PLATFORM_DIR}/uninstall.sh"

        # INSTALL.txt oluştur
        cat > "${PLATFORM_DIR}/INSTALL.txt" << 'EOF'
# GIH-FTP Installation Guide

## Quick Install (Linux)

1. Extract the archive:
   tar -xzf gihftp-*.tar.gz
   cd gihftp-*

2. Run installer as root:
   sudo ./install.sh

3. Configure:
   sudo nano /etc/gihftp.conf

4. Test:
   sudo gihftp --help

## Manual Install

1. Copy binary:
   sudo cp gihftp /usr/bin/
   sudo chmod +x /usr/bin/gihftp

2. Copy config:
   sudo cp gihftp.conf.example /etc/gihftp.conf
   sudo nano /etc/gihftp.conf

3. Run:
   gihftp --help

## Uninstall

Run:
   sudo ./uninstall.sh

Or manually:
   sudo rm /usr/bin/gihftp
   sudo rm /etc/gihftp.conf

EOF
    else
        # Mac için INSTALL.txt
        cat > "${PLATFORM_DIR}/INSTALL.txt" << 'EOF'
# GIH-FTP Installation Guide (macOS)

## Installation

1. Extract the archive:
   tar -xzf gihftp-*.tar.gz
   cd gihftp-*

2. Copy binary to PATH:
   sudo cp gihftp /usr/local/bin/
   sudo chmod +x /usr/local/bin/gihftp

3. Configure:
   cp gihftp.conf.example ~/gihftp.conf
   nano ~/gihftp.conf

4. Test:
   gihftp --help

## Uninstall

Remove binary:
   sudo rm /usr/local/bin/gihftp
   rm ~/gihftp.conf

EOF
    fi

    # CHANGELOG ekle
    cat > "${PLATFORM_DIR}/CHANGELOG.md" << EOF
# Changelog

## ${VERSION} - ${BUILD_DATE}

### New Features
- ✅ Command-line flag support
- ✅ Environment variable support (FTP_PASSWORD)
- ✅ Working directory management (--work-dir)
- ✅ Structured logging (debug/info/error)
- ✅ SSH host key verification
- ✅ TLS certificate verification
- ✅ Exit code standardization
- ✅ Modular package structure

### Security Improvements
- ✅ TLS certificate validation (default: enabled)
- ✅ SSH known_hosts support
- ✅ Trust-on-first-use (TOFU) fallback
- ✅ Environment variable password reading
- ✅ SSH key passphrase support

### Breaking Changes
- Config file is now optional (flags are preferred)
- InsecureSkipVerify is now false by default

Build: ${COMMIT_HASH}
EOF

    # Checksum oluştur
    cd "${PLATFORM_DIR}"
    sha256sum "${OUTPUT_NAME}" > "${OUTPUT_NAME}.sha256"
    cd - > /dev/null

    # Arşiv oluştur
    echo "📦 Creating archive..."
    cd "${RELEASE_DIR}"
    tar -czf "gihftp-${VERSION}-${GOOS}-${GOARCH}.tar.gz" "gihftp-${VERSION}-${GOOS}-${GOARCH}/"

    # Checksum for archive
    sha256sum "gihftp-${VERSION}-${GOOS}-${GOARCH}.tar.gz" > "gihftp-${VERSION}-${GOOS}-${GOARCH}.tar.gz.sha256"
    cd - > /dev/null

    echo "✅ ${GOOS}/${GOARCH} build complete"
    echo ""
done

# Release bilgilerini oluştur
echo "📝 Creating release info..."
cat > "${RELEASE_DIR}/RELEASE_INFO.txt" << EOF
GIH-FTP Release ${VERSION}
Built on: ${BUILD_DATE}
Commit: ${COMMIT_HASH}

Available Packages:
$(ls -lh "${RELEASE_DIR}"/*.tar.gz | awk '{print "  - " $9 " (" $5 ")"}')

Installation:
1. Download the appropriate package for your platform
2. Extract: tar -xzf gihftp-*.tar.gz
3. See INSTALL.txt in the extracted directory

Checksums:
$(cat "${RELEASE_DIR}"/*.sha256 | sed 's/^/  /')

EOF

# Release bilgilerini göster
echo "=================================================="
echo "✨ Release ${VERSION} created successfully!"
echo "=================================================="
echo ""
echo "📦 Packages:"
ls -lh "${RELEASE_DIR}"/*.tar.gz
echo ""
echo "📂 Location: ${RELEASE_DIR}"
echo ""
echo "🚀 To distribute:"
echo "   1. Upload archives from: ${RELEASE_DIR}/"
echo "   2. Provide checksums: *.sha256 files"
echo "   3. NO SOURCE CODE is included - only binaries!"
echo ""
echo "=================================================="

# Optional: Temizlik sorusu
echo ""
read -p "🗑️  Remove uncompressed directories? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    find "${RELEASE_DIR}" -maxdepth 1 -type d -name "gihftp-*" -exec rm -rf {} +
    echo "✅ Cleanup complete"
fi

echo ""
echo "✨ Done! Happy releasing! 🎉"
