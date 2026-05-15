#!/usr/bin/env bash
set -euo pipefail

drumlabooh_version="12.2.0"
kits_version_url="https://raw.githubusercontent.com/psemiletov/drum_sklad/refs/heads/main/version.txt"

workdir="$(mktemp -d)"
manifest_root="${HOME}/.local/share/caracal-software-installer/manifests"
manifest_path="${manifest_root}/drumlabooh.txt"
manifest_tmp="${workdir}/manifest.txt"

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

    echo "Need unzip or bsdtar to unpack Drumlabooh ZIP archives." >&2
    exit 1
}

download_and_extract() {
    local url="$1"
    local archive_name="$2"
    local destination="$3"
    local extract_dir="${workdir}/${archive_name%.zip}"

    echo "Downloading ${archive_name}..."
    curl -fL --retry 3 --retry-delay 2 -o "${workdir}/${archive_name}" "${url}"
    extract_zip "${workdir}/${archive_name}" "${extract_dir}"
    mkdir -p "${destination}"
    cp -a "${extract_dir}/." "${destination}/"
}

record_existing() {
    local target="$1"
    [[ -e "${target}" ]] && printf '%s\n' "${target}" >>"${manifest_tmp}"
}

trap cleanup EXIT

if [[ "$(id -u)" -eq 0 ]]; then
    echo "Please run as a regular user; Drumlabooh installs into the current user's plugin directories." >&2
    exit 1
fi

kits_version="$(curl -fsSL "${kits_version_url}" | tr -d '[:space:]')"
if [[ -z "${kits_version}" ]]; then
    echo "Could not determine drum_sklad kit version." >&2
    exit 1
fi

mkdir -p "${HOME}/.lv2" "${HOME}/.vst3" "${manifest_root}"
: >"${manifest_tmp}"

download_and_extract "https://github.com/psemiletov/drumlabooh/releases/download/${drumlabooh_version}/drumlabooh.lv2.zip" "drumlabooh.lv2.zip" "${HOME}/.lv2"
download_and_extract "https://github.com/psemiletov/drumlabooh/releases/download/${drumlabooh_version}/drumlabooh-multi.lv2.zip" "drumlabooh-multi.lv2.zip" "${HOME}/.lv2"
download_and_extract "https://github.com/psemiletov/drumlabooh/releases/download/${drumlabooh_version}/drumlabooh.vst3.zip" "drumlabooh.vst3.zip" "${HOME}/.vst3"
download_and_extract "https://github.com/psemiletov/drumlabooh/releases/download/${drumlabooh_version}/drumlabooh-multi.vst3.zip" "drumlabooh-multi.vst3.zip" "${HOME}/.vst3"

kits_extract="${workdir}/drum_sklad"
echo "Downloading drum_sklad ${kits_version}..."
curl -fL --retry 3 --retry-delay 2 -o "${workdir}/drum_sklad.zip" "https://github.com/psemiletov/drum_sklad/archive/refs/tags/${kits_version}.zip"
extract_zip "${workdir}/drum_sklad.zip" "${kits_extract}"
rm -rf "${HOME}/drum_sklad"
kit_source="$(find "${kits_extract}" -maxdepth 1 -type d -name 'drum_sklad-*' | head -n 1 || true)"
if [[ -z "${kit_source}" ]]; then
    echo "drum_sklad payload was not found." >&2
    exit 1
fi
cp -a "${kit_source}" "${HOME}/drum_sklad"

record_existing "${HOME}/.lv2/drumlabooh.lv2"
record_existing "${HOME}/.lv2/drumlabooh-multi.lv2"
record_existing "${HOME}/.vst3/drumlabooh.vst3"
record_existing "${HOME}/.vst3/drumlabooh-multi.vst3"
record_existing "${HOME}/drum_sklad"
sort -u "${manifest_tmp}" >"${manifest_path}"

echo "Drumlabooh installed into user-local LV2, VST3, and drum_sklad paths"
