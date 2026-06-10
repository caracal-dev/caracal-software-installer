#!/usr/bin/env bash
# Installs Renoise from a locally downloaded upstream tarball.
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <renoise.tar.gz>" >&2
    exit 1
fi

local_archive="$1"
if [[ ! -f "${local_archive}" ]]; then
    echo "Local Renoise archive not found: ${local_archive}" >&2
    exit 1
fi

workdir="$(mktemp -d)"
archive_path="${workdir}/$(basename "${local_archive}")"
extract_root="${workdir}/extract"

cleanup() {
    rm -rf "${workdir}"
}

trap cleanup EXIT

echo "Installing Renoise from ${local_archive}..."
cp -f "${local_archive}" "${archive_path}"
mkdir -p "${extract_root}"
tar xf "${archive_path}" -C "${extract_root}"

extract_dir="$(find "${extract_root}" -maxdepth 2 -type f -name renoise -printf '%h\n' | head -n 1 || true)"
if [[ -z "${extract_dir}" ]]; then
    echo "Renoise executable was not found in the archive." >&2
    exit 1
fi

mkdir -p /opt/renoise
cp -r "${extract_dir}/Resources" /opt/renoise/
install -m755 "${extract_dir}/renoise" /opt/renoise/renoise

mkdir -p /usr/local/bin
cat > /usr/local/bin/renoise <<'EOF'
#!/usr/bin/env bash
exec /opt/renoise/renoise "$@"
EOF
chmod 755 /usr/local/bin/renoise

mkdir -p /usr/local/share/applications \
         /usr/local/share/icons/hicolor/{48x48,64x64,128x128}/apps \
         /usr/local/share/mime/packages \
         /usr/local/share/man/man1 \
         /usr/local/share/man/man5

install -m644 "${extract_dir}/Installer/renoise.desktop" /usr/local/share/applications/renoise.desktop
install -m644 "${extract_dir}/Installer/renoise-48.png" /usr/local/share/icons/hicolor/48x48/apps/renoise.png
install -m644 "${extract_dir}/Installer/renoise-64.png" /usr/local/share/icons/hicolor/64x64/apps/renoise.png
install -m644 "${extract_dir}/Installer/renoise-128.png" /usr/local/share/icons/hicolor/128x128/apps/renoise.png
install -m644 "${extract_dir}/Installer/renoise.xml" /usr/local/share/mime/packages/renoise.xml
install -m644 "${extract_dir}/Installer/renoise.1.gz" /usr/local/share/man/man1/renoise.1.gz
install -m644 "${extract_dir}/Installer/renoise-pattern-effects.5.gz" /usr/local/share/man/man5/renoise-pattern-effects.5.gz

sed -i 's|Exec=renoise|Exec=/opt/renoise/renoise|g' /usr/local/share/applications/renoise.desktop

echo "Renoise installed to /opt/renoise"
echo "Run 'renoise' to launch, or purchase a license at renoise.com to unlock full functionality."
