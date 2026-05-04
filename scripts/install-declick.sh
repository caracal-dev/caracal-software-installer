#!/usr/bin/env bash
set -euo pipefail

readonly DECLICK_URL="https://home.snafu.de/wahlm/dl8hbs/declick-0.6.5.tar.gz"

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

archive_path="${workdir}/declick.tar.gz"
extract_dir="${workdir}/extract"

require_command() {
    local command_name="$1"
    if ! command -v "${command_name}" >/dev/null 2>&1; then
        echo "Required command not found: ${command_name}" >&2
        exit 1
    fi
}

require_command curl
require_command tar
require_command make

mkdir -p "${extract_dir}"

echo "Downloading Declick source..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${DECLICK_URL}"
tar -xf "${archive_path}" -C "${extract_dir}"

source_dir="$(find "${extract_dir}" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
if [[ -z "${source_dir}" ]]; then
    echo "Could not determine extracted source directory for Declick." >&2
    exit 1
fi

cd "${source_dir}"
make all
make install

echo "Declick installed to /usr/local/bin/declick"
