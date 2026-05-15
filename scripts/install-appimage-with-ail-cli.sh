#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
    echo "Usage: $0 <app-id> <display-name> <url>" >&2
    exit 1
fi

app_id="$1"
display_name="$2"
url="$3"

temp_dir="$(mktemp -d)"
download_path="${temp_dir}/${app_id}.appimage"

cleanup() {
    rm -rf "${temp_dir}"
}

trap cleanup EXIT

if ! command -v ail-cli >/dev/null 2>&1; then
    echo "ail-cli is required to integrate AppImages. Install AppImageLauncher first." >&2
    exit 1
fi

echo "Downloading ${display_name} AppImage..."
curl -fL --retry 3 --retry-delay 2 -o "${download_path}" "${url}"
chmod 755 "${download_path}"

echo "Integrating ${display_name} with ail-cli..."
ail-cli integrate "${download_path}"

echo "${display_name} integrated with AppImageLauncher"
