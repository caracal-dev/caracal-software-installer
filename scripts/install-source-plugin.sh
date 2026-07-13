#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <index-id> <display-name>" >&2
    exit 1
fi

index_id="$1"
display_name="$2"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
url="$("${script_dir}/download-index" get "${index_id}" "url")"
version="$("${script_dir}/download-index" get-optional "${index_id}" "version")"

workdir="$(mktemp -d)"
archive_path="${workdir}/$(basename "${url%%\?*}")"
extract_dir="${workdir}/extract"
prefix="${HOME}/.local"

cleanup() {
    rm -rf "${workdir}"
}

require_command() {
    local command_name="$1"
    if ! command -v "${command_name}" >/dev/null 2>&1; then
        echo "Required command not found: ${command_name}" >&2
        exit 1
    fi
}

extract_archive() {
    local archive="$1"
    local destination="$2"

    mkdir -p "${destination}"
    if command -v tar >/dev/null 2>&1; then
        tar -xf "${archive}" -C "${destination}"
        return
    fi

    if command -v bsdtar >/dev/null 2>&1; then
        bsdtar -xf "${archive}" -C "${destination}"
        return
    fi

    echo "Need tar or bsdtar to unpack ${archive}." >&2
    exit 1
}

trap cleanup EXIT

require_command curl
require_command make

echo "Downloading ${display_name}${version:+ ${version}}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${url}"
extract_archive "${archive_path}" "${extract_dir}"

source_dir="$(find "${extract_dir}" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
if [[ -z "${source_dir}" ]]; then
    echo "Could not determine extracted source directory for ${display_name}." >&2
    exit 1
fi

cd "${source_dir}"
mkdir -p "${prefix}"

if [[ -x "./configure" ]]; then
    ./configure --prefix="${prefix}"
elif [[ -f "CMakeLists.txt" ]]; then
    require_command cmake
    cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="${prefix}"
else
    echo "Unsupported source layout for ${display_name}; expected ./configure or CMakeLists.txt." >&2
    exit 1
fi

build_dir="."
if [[ -d build ]]; then
    build_dir="build"
fi

make -C "${build_dir}" -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 1)"
make -C "${build_dir}" install

echo "${display_name} installed into ${prefix}"
echo "Check ${HOME}/.local/bin and ${HOME}/.local/lib for the installed payload."
