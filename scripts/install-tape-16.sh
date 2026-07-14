#!/usr/bin/env bash
# Installs TAPE 16 from a local archive to /opt/tape-16.
# Usage: install-tape-16.sh [path-to-tape16-zip]
# If no path is provided, prompts the user to download from the product page first.
set -euo pipefail

LOCAL_ZIP="${1:-}"
EXTRACT_DIR="/tmp/tape-16-extract"

cleanup() {
    rm -rf "${EXTRACT_DIR}"
}

trap cleanup EXIT

if [ -z "${LOCAL_ZIP}" ] || [ ! -f "${LOCAL_ZIP}" ]; then
    echo "Error: No TAPE 16 archive found." >&2
    echo "" >&2
    echo "To install TAPE 16:" >&2
    echo "  1. Download the Linux ZIP from https://emrmusicgroup.com/tape16/" >&2
    echo "  2. Right-click the downloaded file and choose 'Install with Caracal'" >&2
    echo "     or run: sudo bash $0 /path/to/TAPE-16-Linux-Release.zip" >&2
    exit 1
fi

echo "Installing TAPE 16 from ${LOCAL_ZIP}..."

# Detect display server -- prefer Wayland on modern Atomic Fedora
if [ "${XDG_SESSION_TYPE:-}" = "wayland" ] || [ -n "${WAYLAND_DISPLAY:-}" ]; then
    DEB_DIR="TAPE 16 Wayland Deb"
else
    DEB_DIR="TAPE 16 X11 Deb"
fi

mkdir -p "${EXTRACT_DIR}"
unzip -q "${LOCAL_ZIP}" -d "${EXTRACT_DIR}" "${DEB_DIR}/INSTALL-TAPE16.deb" 2>/dev/null || {
    # If the specific path doesn't match, try extracting the full archive
    unzip -q "${LOCAL_ZIP}" -d "${EXTRACT_DIR}"
}

DEB_PATH="${EXTRACT_DIR}/${DEB_DIR}/INSTALL-TAPE16.deb"

if [ ! -f "${DEB_PATH}" ]; then
    # Try to find any .deb in the extracted structure
    DEB_PATH="$(find "${EXTRACT_DIR}" -name 'INSTALL-TAPE16.deb' -type f | head -1)" || true
fi

if [ -z "${DEB_PATH}" ] || [ ! -f "${DEB_PATH}" ]; then
    echo "Error: Could not find INSTALL-TAPE16.deb in the archive." >&2
    ls -R "${EXTRACT_DIR}" >&2
    exit 1
fi

DEB_EXTRACT="/tmp/tape-16-deb-extract"
mkdir -p "${DEB_EXTRACT}"
dpkg-deb -x "${DEB_PATH}" "${DEB_EXTRACT}"

# Install to /opt/tape-16
rm -rf /opt/tape-16
if [ -d "${DEB_EXTRACT}/opt/tape16" ]; then
    cp -a "${DEB_EXTRACT}/opt/tape16" /opt/tape-16
elif [ -d "${DEB_EXTRACT}/opt/TAPE16" ]; then
    cp -a "${DEB_EXTRACT}/opt/TAPE16" /opt/tape-16
elif [ -d "${DEB_EXTRACT}/opt/TAPE 16" ]; then
    cp -a "${DEB_EXTRACT}/opt/TAPE 16" /opt/tape-16
else
    echo "Error: Could not find TAPE 16 app payload in the extracted package." >&2
    ls "${DEB_EXTRACT}/opt/" >&2
    exit 1
fi

# Install desktop integration to /usr/local
mkdir -p /usr/local/bin /usr/local/share/applications /usr/local/share/icons/hicolor/256x256/apps /usr/local/share/icons/hicolor/1024x1024/apps

if [ -d "${DEB_EXTRACT}/usr/share/applications" ]; then
    cp -a "${DEB_EXTRACT}/usr/share/applications/tape16.desktop" /usr/local/share/applications/ 2>/dev/null || true
    cp -a "${DEB_EXTRACT}/usr/share/applications/tape16-lowlatency.desktop" /usr/local/share/applications/ 2>/dev/null || true
    cp -a "${DEB_EXTRACT}/usr/share/applications/TAPE16.desktop" /usr/local/share/applications/ 2>/dev/null || true
    cp -a "${DEB_EXTRACT}/usr/share/applications/tape-16.desktop" /usr/local/share/applications/ 2>/dev/null || true
fi

if [ -d "${DEB_EXTRACT}/usr/share/icons" ]; then
    cp -a "${DEB_EXTRACT}/usr/share/icons/." /usr/local/share/icons/
fi

if [ -d "${DEB_EXTRACT}/usr/share/metainfo" ]; then
    mkdir -p /usr/local/share/metainfo
    cp -a "${DEB_EXTRACT}/usr/share/metainfo/." /usr/local/share/metainfo/
fi

# Fix desktop entries to point to /opt/tape-16 and /usr/local paths
for desktop in /usr/local/share/applications/*tape*.desktop /usr/local/share/applications/TAPE*.desktop; do
    [ -f "${desktop}" ] || continue
    sed -i \
        -e 's|/opt/tape16|/opt/tape-16|g' \
        -e 's|/opt/TAPE16|/opt/tape-16|g' \
        -e 's|/opt/TAPE 16|/opt/tape-16|g' \
        -e 's|/usr/bin/tape16|/usr/local/bin/tape16|g' \
        -e 's|Icon=/usr/share/|Icon=/usr/local/share/|g' \
        "${desktop}"
done

# Create wrapper script
cat > /usr/local/bin/tape16 <<'WRAPPER'
#!/usr/bin/env bash
exec /opt/tape-16/tape16 "$@"
WRAPPER
chmod 755 /usr/local/bin/tape16

for wrapper in tape16-stable tape16-lowlatency; do
    if [ -f "${DEB_EXTRACT}/usr/bin/${wrapper}" ]; then
        cp -a "${DEB_EXTRACT}/usr/bin/${wrapper}" /usr/local/bin/
        sed -i 's|/opt/tape16|/opt/tape-16|g' /usr/local/bin/"${wrapper}"
    fi
done

for util in tape16-preflight tape16-create-desktop-copy tape16-system-check tape16-reset-defaults; do
    if [ -f "${DEB_EXTRACT}/usr/bin/${util}" ]; then
        cp -a "${DEB_EXTRACT}/usr/bin/${util}" /usr/local/bin/
    fi
done

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/local/share/applications
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/local/share/icons/hicolor 2>/dev/null || true
fi

echo "TAPE 16 installed to /opt/tape-16"