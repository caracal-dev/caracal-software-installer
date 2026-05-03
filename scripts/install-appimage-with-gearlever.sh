#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
    echo "Usage: $0 <app-id> <display-name> <url>" >&2
    exit 1
fi

app_id="$1"
display_name="$2"
url="$3"

gearlever_flatpak_id="it.mijorus.gearlever"
install_root="${HOME}/AppImages"
appimage_path="${install_root}/${app_id}.appimage"
temp_dir="$(mktemp -d)"
download_path="${temp_dir}/${app_id}.appimage"

cleanup() {
    rm -rf "${temp_dir}"
}

trap cleanup EXIT

if ! command -v flatpak >/dev/null 2>&1; then
    echo "flatpak is required to integrate AppImages with Gear Lever." >&2
    exit 1
fi

if ! flatpak info "${gearlever_flatpak_id}" >/dev/null 2>&1; then
    echo "Gear Lever is not installed. Install ${gearlever_flatpak_id} first." >&2
    exit 1
fi

echo "Downloading ${display_name} AppImage..."
curl -fL --retry 3 --retry-delay 2 -o "${download_path}" "${url}"

mkdir -p "${install_root}"
install -m755 "${download_path}" "${appimage_path}"

echo "Integrating ${display_name} with Gear Lever..."
flatpak run "${gearlever_flatpak_id}" --integrate "${appimage_path}"

echo "${display_name} installed to ${appimage_path}"
