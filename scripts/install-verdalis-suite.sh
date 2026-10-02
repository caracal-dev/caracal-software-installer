#!/usr/bin/env bash
# Installs the Verdalis Suite: ten synthesised instrument pairs (CLAP + VST3)
# from the upstream release ZIP, all user-local, no sudo required.
# Only the linux/ payload is installed: the ZIP also ships Windows bundles
# that share the same names, and a recursive copy would let them clobber
# the Linux plugins.
set -euo pipefail

VERDALIS_VERSION="0.24"
VERDALIS_URL="https://verdalis.ravetracer.de/download/174/?tmstv=1790899018"

workdir="$(mktemp -d)"
archive_name="verdalis-suite-${VERDALIS_VERSION}.zip"
archive_path="${workdir}/${archive_name}"
extract_dir="${workdir}/extract"

plugin_vst3_dir="${HOME}/.vst3"
plugin_clap_dir="${HOME}/.clap"
manifest_root="${HOME}/.local/share/caracal-software-installer/manifests"
manifest_path="${manifest_root}/verdalis-suite.txt"
manifest_tmp="${workdir}/manifest.txt"

cleanup() {
  rm -rf "${workdir}"
}

record_manifest() {
  printf '%s\n' "$1" >>"${manifest_tmp}"
}

trap cleanup EXIT

echo "Downloading Verdalis Suite ${VERDALIS_VERSION}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${VERDALIS_URL}"

echo "Extracting..."
mkdir -p "${extract_dir}"
if command -v unzip >/dev/null 2>&1; then
  unzip -q "${archive_path}" -d "${extract_dir}"
elif command -v 7z >/dev/null 2>&1; then
  7z x -y "-o${extract_dir}" "${archive_path}" >/dev/null
elif command -v bsdtar >/dev/null 2>&1; then
  bsdtar -xf "${archive_path}" -C "${extract_dir}"
else
  echo "Need one of: unzip, 7z, or bsdtar to unpack ZIP archives." >&2
  exit 1
fi

# Payload root: verdalis-suite-<version>/linux
shopt -s nullglob
payload_roots=("${extract_dir}"/verdalis-suite-*/linux)
shopt -u nullglob
if [[ ${#payload_roots[@]} -ne 1 || ! -d "${payload_roots[0]}" ]]; then
  echo "Could not locate the Verdalis Suite linux payload directory." >&2
  exit 1
fi
payload_dir="$(cd "${payload_roots[0]}" && pwd)"

mkdir -p "${manifest_root}"
: >"${manifest_tmp}"

installed_vst3=0
installed_clap=0

# VST3 bundles are directories named <Name>.vst3; presets travel inside them.
for bundle in "${payload_dir}"/*.vst3; do
  name="$(basename "${bundle}")"
  echo "Installing VST3 ${name} ..."
  mkdir -p "${plugin_vst3_dir}"
  rm -rf "${plugin_vst3_dir}/${name}"
  cp -a "${bundle}" "${plugin_vst3_dir}/"
  record_manifest "${plugin_vst3_dir}/${name}"
  installed_vst3=$((installed_vst3 + 1))
done

# Each instrument folder pairs the CLAP file with its preset folder.
for instrument_dir in "${payload_dir}"/*/; do
  name="$(basename "${instrument_dir}")"
  clap_file="${instrument_dir}${name}.clap"
  if [[ -f "${clap_file}" ]]; then
    echo "Installing CLAP ${name}.clap ..."
    mkdir -p "${plugin_clap_dir}"
    install -m755 "${clap_file}" "${plugin_clap_dir}/${name}.clap"
    record_manifest "${plugin_clap_dir}/${name}.clap"
    installed_clap=$((installed_clap + 1))
  fi
done

if [[ ${installed_vst3} -eq 0 || ${installed_clap} -eq 0 ]]; then
  echo "Expected ten VST3 bundles and ten CLAP plugins; found ${installed_vst3} and ${installed_clap}." >&2
  exit 1
fi

sort -u "${manifest_tmp}" >"${manifest_path}"
echo "  ${manifest_path}"

echo ""
echo "Verdalis Suite ${VERDALIS_VERSION} installed."
echo "  VST3: ${installed_vst3} bundles in ${plugin_vst3_dir}"
echo "  CLAP: ${installed_clap} plugins in ${plugin_clap_dir}"
echo ""
echo "Note: factory presets ship inside each VST3 bundle; the suite's preset"
echo "browser can also load the preset folders from the upstream archive."