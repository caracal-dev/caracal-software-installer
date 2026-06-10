#!/usr/bin/env bash
# Installs Decent Sampler from a locally downloaded upstream tarball.
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <decent-sampler.tar.gz>" >&2
    exit 1
fi

local_archive="$1"
if [[ ! -f "${local_archive}" ]]; then
    echo "Local Decent Sampler archive not found: ${local_archive}" >&2
    exit 1
fi

workdir="$(mktemp -d)"
archive_path="${workdir}/$(basename "${local_archive}")"
trap 'rm -rf "${workdir}"' EXIT

echo "Installing Decent Sampler from ${local_archive}..."
cp -f "${local_archive}" "${archive_path}"
tar xzf "${archive_path}" -C "${workdir}"

extract_dir="$(find "${workdir}" -maxdepth 1 -mindepth 1 -type d -name 'Decent_Sampler-*' | head -n 1)"
if [[ -z "${extract_dir}" ]]; then
    echo "Decent Sampler archive did not contain the expected directory layout" >&2
    exit 1
fi

install -Dm755 "${extract_dir}/DecentSampler" "/usr/local/bin/DecentSampler"
install -Dm755 "${extract_dir}/DecentSampler.so" "/usr/local/lib64/vst/DecentSampler.so"
mkdir -p "/usr/local/lib64/vst3"
rm -rf "/usr/local/lib64/vst3/DecentSampler.vst3"
cp -a "${extract_dir}/DecentSampler.vst3" "/usr/local/lib64/vst3/"

echo "Decent Sampler installed into /usr/local"
