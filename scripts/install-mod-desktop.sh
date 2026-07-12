#!/usr/bin/env bash
# Installs MOD Desktop to /opt/mod-desktop (writable on atomic Fedora via /var/opt).
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This installer must run as root because it writes to /opt and /usr/local." >&2
    exit 1
fi

MOD_DESKTOP_VERSION="0.0.12"
MOD_DESKTOP_ARCHIVE="/tmp/mod-desktop-${MOD_DESKTOP_VERSION}.tar.xz"
MOD_DESKTOP_EXTRACT_DIR="/tmp/mod-desktop-extract"
MOD_DESKTOP_APP_DIR="/opt/mod-desktop"
MOD_DESKTOP_WRAPPER="/usr/local/bin/mod-desktop"
MOD_DESKTOP_DESKTOP_FILE="/usr/local/share/applications/mod-desktop.desktop"
MOD_DESKTOP_ICON_DIR="/usr/local/share/icons/hicolor/scalable/apps"
MOD_DESKTOP_ICON_FILE="${MOD_DESKTOP_ICON_DIR}/mod-desktop.svg"
MOD_DESKTOP_URL="https://github.com/moddevices/mod-desktop/releases/download/${MOD_DESKTOP_VERSION}/mod-desktop-${MOD_DESKTOP_VERSION}-linux-x86_64.tar.xz"

cleanup() {
    rm -rf "${MOD_DESKTOP_ARCHIVE}" "${MOD_DESKTOP_EXTRACT_DIR}"
}

trap cleanup EXIT

echo "Downloading MOD Desktop ${MOD_DESKTOP_VERSION}..."
curl -fL --retry 3 --retry-delay 2 -o "${MOD_DESKTOP_ARCHIVE}" "${MOD_DESKTOP_URL}"

mkdir -p "${MOD_DESKTOP_EXTRACT_DIR}"
tar -xJf "${MOD_DESKTOP_ARCHIVE}" -C "${MOD_DESKTOP_EXTRACT_DIR}"

# The tarball expands to mod-desktop-VERSION-linux-x86_64/ which contains
# the mod-desktop/ app folder, an upstream mod-desktop.desktop, and the
# upstream mod-desktop.run launcher wrapper.
shopt -s nullglob
payload_dirs=("${MOD_DESKTOP_EXTRACT_DIR}/mod-desktop-"*/mod-desktop)
shopt -u nullglob

if [[ ${#payload_dirs[@]} -ne 1 || ! -d "${payload_dirs[0]}" ]]; then
    echo "MOD Desktop archive did not contain the expected mod-desktop/ payload." >&2
    exit 1
fi

payload_dir="${payload_dirs[0]}"

mkdir -p /opt
rm -rf "${MOD_DESKTOP_APP_DIR}"
cp -a "${payload_dir}" "${MOD_DESKTOP_APP_DIR}"

# Ensure the bundled launcher, jackd, jack libs, and the main mod-desktop
# binary stay executable after copy.
chmod -R a+rX "${MOD_DESKTOP_APP_DIR}"

# The upstream mod-desktop.run script does:
#   cd "$(dirname $0)/mod-desktop"
#   exec "$(pwd)/mod-desktop"
# which only works when mod-desktop.run sits next to the mod-desktop/ folder.
# We replicate that behavior via a small wrapper in /usr/local/bin so the
# wrapper path stays stable even if the install prefix changes.
cat > "${MOD_DESKTOP_WRAPPER}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
APP_DIR="/opt/mod-desktop"
cd "${APP_DIR}"
exec "${APP_DIR}/mod-desktop" "$@"
EOF
chmod 755 "${MOD_DESKTOP_WRAPPER}"

mkdir -p /usr/local/share/applications "${MOD_DESKTOP_ICON_DIR}"

cat > "${MOD_DESKTOP_DESKTOP_FILE}" <<'EOF'
[Desktop Entry]
Name=MOD Desktop
GenericName=MOD Desktop
Comment=Modular pedalboard environment reimagined for desktop
Exec=/usr/local/bin/mod-desktop
Icon=mod-desktop
Terminal=false
Type=Application
Categories=AudioVideo;Audio;Music;X-AudioEditing;Qt;
StartupNotify=true
EOF

# Drop a simple SVG icon when one is provided as a Caracal asset so the entry
# shows up with a branded icon. Falls back to the system audio icon otherwise.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
asset_candidates=(
    "/usr/share/caracal-software-installer/assets/images/mod-desktop.svg"
    "/usr/share/caracal-software-installer/assets/images/mod-desktop.png"
    "${script_dir}/../assets/images/mod-desktop.svg"
    "${script_dir}/../assets/images/mod-desktop.png"
    "${MOD_DESKTOP_APP_DIR}/html/favicon.ico"
)

icon_source=""
for candidate in "${asset_candidates[@]}"; do
    if [[ -f "${candidate}" ]]; then
        icon_source="${candidate}"
        break
    fi
done

if [[ -n "${icon_source}" ]]; then
    case "${icon_source}" in
        *.svg)
            install -m644 "${icon_source}" "${MOD_DESKTOP_ICON_FILE}"
            ;;
        *.png)
            mkdir -p /usr/local/share/icons/hicolor/256x256/apps
            install -m644 "${icon_source}" /usr/local/share/icons/hicolor/256x256/apps/mod-desktop.png
            ;;
        *.ico)
            # The upstream favicon is an .ico; rasterize via convert if available,
            # otherwise leave the desktop entry pointing at the system audio icon.
            if command -v convert >/dev/null 2>&1; then
                mkdir -p /usr/local/share/icons/hicolor/256x256/apps
                convert "${icon_source}" /usr/local/share/icons/hicolor/256x256/apps/mod-desktop.png || true
            else
                sed -i 's|^Icon=mod-desktop$|Icon=audio|' "${MOD_DESKTOP_DESKTOP_FILE}"
            fi
            ;;
    esac
else
    sed -i 's|^Icon=mod-desktop$|Icon=audio|' "${MOD_DESKTOP_DESKTOP_FILE}"
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/local/share/applications >/dev/null 2>&1 || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t -f /usr/local/share/icons/hicolor >/dev/null 2>&1 || true
fi

if command -v kbuildsycoca6 >/dev/null 2>&1; then
    kbuildsycoca6 >/dev/null 2>&1 || true
fi

echo "MOD Desktop installed to ${MOD_DESKTOP_APP_DIR}"
echo "Launcher: ${MOD_DESKTOP_WRAPPER}"
echo "Desktop entry: ${MOD_DESKTOP_DESKTOP_FILE}"
