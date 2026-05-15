#!/usr/bin/env bash
set -euo pipefail

version="11.3.0"
url="https://eu1.download.solidstatelogic.com/Mixbus%2011/Mixbus%2011.3/Mixbus-11.3.0-x86_64.tar"

workdir="$(mktemp -d)"
archive_path="${workdir}/mixbus.tar"
extract_dir="${workdir}/extract"

cleanup() {
    rm -rf "${workdir}"
}

trap cleanup EXIT

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This installer must run as root because the Mixbus .run installer writes system paths." >&2
    exit 1
fi

echo "Downloading Mixbus ${version}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${url}"

mkdir -p "${extract_dir}"
tar -xf "${archive_path}" -C "${extract_dir}"

run_installer="$(find "${extract_dir}" -type f -name '*.run' | head -n 1 || true)"
if [[ -z "${run_installer}" ]]; then
    echo "Mixbus .run installer was not found in the tarball." >&2
    exit 1
fi

chmod +x "${run_installer}"
echo "Running Mixbus installer..."
"${run_installer}"

echo "Mixbus installer completed"
