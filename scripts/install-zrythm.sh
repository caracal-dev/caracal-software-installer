#!/usr/bin/env bash
set -euo pipefail

version="1.0.0"
pkgname="zrythm-trial-${version}"
url="https://github.com/zrythm/zrythm/releases/download/v${version}/zrythm-trial-${version}-installer.zip"

workdir="$(mktemp -d)"
archive_path="${workdir}/zrythm-installer.zip"
extract_dir="${workdir}/extract"
installer_dir="${extract_dir}/${pkgname}-installer"
payload_dir="${installer_dir}/opt/${pkgname}"
installed_prefix="/opt/${pkgname}"
symlink="/opt/zrythm"

cleanup() {
    rm -rf "${workdir}"
}

extract_zip() {
    local archive="$1"
    local destination="$2"

    mkdir -p "${destination}"
    if command -v unzip >/dev/null 2>&1; then
        unzip -q "${archive}" -d "${destination}"
        return
    fi

    if command -v bsdtar >/dev/null 2>&1; then
        bsdtar -xf "${archive}" -C "${destination}"
        return
    fi

    echo "Need unzip or bsdtar to unpack the Zrythm installer ZIP." >&2
    exit 1
}

trap cleanup EXIT

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This installer must run as root because it writes to /opt and /usr/local." >&2
    exit 1
fi

echo "Downloading Zrythm ${version} trial..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${url}"
extract_zip "${archive_path}" "${extract_dir}"

if [[ ! -d "${payload_dir}" ]]; then
    echo "Zrythm payload was not found in the installer ZIP." >&2
    exit 1
fi

rm -rf "${installed_prefix}"
mkdir -p /opt
cp -a "${payload_dir}" "${installed_prefix}"
ln -sfn "${installed_prefix}" "${symlink}"

mkdir -p /usr/local/share/applications \
         /usr/local/share/icons/hicolor/scalable/apps \
         /usr/local/share/man/man1 \
         /usr/local/share/metainfo \
         /usr/local/share/mime/packages

install -m644 "${payload_dir}/share/applications/org.zrythm.Zrythm.desktop" /usr/local/share/applications/org.zrythm.Zrythm-installer.desktop
sed -i "s|Exec=${installed_prefix}/bin/zrythm_launch|Exec=${symlink}/bin/zrythm_launch|g" /usr/local/share/applications/org.zrythm.Zrythm-installer.desktop
install -m644 "${payload_dir}/share/icons/hicolor/scalable/apps/org.zrythm.Zrythm.svg" /usr/local/share/icons/hicolor/scalable/apps/org.zrythm.Zrythm.svg
install -m644 "${payload_dir}/share/man/man1/zrythm.1" /usr/local/share/man/man1/zrythm.1
install -m644 "${payload_dir}/share/metainfo/org.zrythm.Zrythm.appdata.xml" /usr/local/share/metainfo/org.zrythm.Zrythm-installer.appdata.xml
install -m644 "${payload_dir}/share/mime/packages/org.zrythm.Zrythm-mime.xml" /usr/local/share/mime/packages/org.zrythm.Zrythm-mime.xml

chmod -R a+rX "${installed_prefix}"

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/local/share/applications >/dev/null 2>&1 || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t -f /usr/local/share/icons/hicolor >/dev/null 2>&1 || true
fi

echo "Zrythm installed to ${installed_prefix}"
