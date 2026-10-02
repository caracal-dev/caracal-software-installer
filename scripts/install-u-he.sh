#!/usr/bin/env bash
# Installs u-he products from the upstream Linux tarballs/ZIPs.
#
# u-he archives are not standard plugin archives: the payload is a product
# data directory (Data/, Presets/, <Product>[-build].64.so, PDFs) that the
# upstream install.sh copies to ~/.u-he/<Product> and then wires up:
#   - a synthesized VST3 bundle at ~/.vst3/u-he/<Product>.vst3 whose
#     Contents/x86_64-linux/<Product>.so is a symlink into the data directory,
#   - a VST2 symlink at ~/.vst/u-he/<Product>.64.so,
#   - product PDFs symlinked into the bundle's Resources/Documentation.
# The freeware line ships no CLAP target, so none is created.
#
# Usage: install-u-he.sh <plugin-id> <archive-url> [Product]...
# Every product directory in the archive is installed unless specific
# product names are passed, in which case only those are installed.
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <plugin-id> <archive-url> [Product]..." >&2
  exit 1
fi

plugin_id="$1"
url="$2"
shift 2
requested_products=("$@")

workdir="$(mktemp -d)"
archive_name="$(basename "${url%%\?*}")"
archive_path="${workdir}/${archive_name}"
extract_dir="${workdir}/extract"

uhe_data_root="${HOME}/.u-he"
vst_dir="${HOME}/.vst/u-he"
vst3_dir="${HOME}/.vst3/u-he"
manifest_root="${HOME}/.local/share/caracal-software-installer/manifests"
manifest_path="${manifest_root}/${plugin_id}.txt"
manifest_tmp="${workdir}/manifest.txt"

cleanup() {
  rm -rf "${workdir}"
}

record_manifest() {
  printf '%s\n' "$1" >>"${manifest_tmp}"
}

trap cleanup EXIT

echo "Downloading ${plugin_id}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${url}"

echo "Extracting..."
mkdir -p "${extract_dir}"
lower_archive="$(printf '%s' "${archive_name}" | tr '[:upper:]' '[:lower:]')"
case "${lower_archive}" in
*.zip)
  if command -v unzip >/dev/null 2>&1; then
    unzip -q "${archive_path}" -d "${extract_dir}"
  elif command -v bsdtar >/dev/null 2>&1; then
    bsdtar -xf "${archive_path}" -C "${extract_dir}"
  else
    echo "Need unzip or bsdtar to unpack ZIP archives." >&2
    exit 1
  fi
  ;;
*.tar.xz | *.tar.gz | *.tgz | *.tar.bz2 | *.tar)
  if command -v tar >/dev/null 2>&1; then
    tar -xf "${archive_path}" -C "${extract_dir}"
  elif command -v bsdtar >/dev/null 2>&1; then
    bsdtar -xf "${archive_path}" -C "${extract_dir}"
  else
    echo "Need tar or bsdtar to unpack ${archive_name}." >&2
    exit 1
  fi
  ;;
*)
  echo "Unsupported archive format for ${archive_name}." >&2
  exit 1
  ;;
esac

mkdir -p "${manifest_root}"
: >"${manifest_tmp}"

# Product directories: any directory containing a "<Name>*.64.so" binary.
# The product name is the directory name with a trailing build number
# stripped ("Podolski-12092" -> "Podolski"); the binary filename is kept
# as-is since it may carry the build number ("Podolski-12092.64.so").
declare -A product_sos=()
while IFS= read -r -d '' so_file; do
  so_dir="$(dirname "${so_file}")"
  product_sos["$(cd "${so_dir}" && pwd)"]="$(basename "${so_file}")"
done < <(find "${extract_dir}" -type f -name '*.64.so' -print0)

declare -A products=()
for so_dir in "${!product_sos[@]}"; do
  raw_name="$(basename "${so_dir}")"
  products["${so_dir}"]="$(printf '%s' "${raw_name}" | sed -E 's/[- ][0-9]{2,}$//')"
done

if [[ ${#products[@]} -eq 0 ]]; then
  echo "No u-he product directories found in ${archive_name}." >&2
  exit 1
fi

installed_any=0
for so_dir in "${!products[@]}"; do
  product="${products[${so_dir}]}"
  so_name="${product_sos[${so_dir}]}"

  # When the caller names products explicitly, skip anything else so one
  # card cannot claim unrelated payloads (e.g. paid bundles in a free pack).
  if [[ ${#requested_products[@]} -gt 0 ]]; then
    keep=0
    for want in "${requested_products[@]}"; do
      want_norm="$(printf '%s' "${want}" | tr -d ' -_' | tr '[:upper:]' '[:lower:]')"
      prod_norm="$(printf '%s' "${product}" | tr -d ' -_' | tr '[:upper:]' '[:lower:]')"
      if [[ "${prod_norm}" == "${want_norm}" ]]; then
        keep=1
        break
      fi
    done
    if [[ ${keep} -eq 0 ]]; then
      echo "Skipping ${product} (not requested for ${plugin_id})."
      continue
    fi
  fi

  echo "Installing ${product}..."
  mkdir -p "${uhe_data_root}"
  rm -rf "${uhe_data_root}/${product}"
  cp -a "${so_dir}" "${uhe_data_root}/${product}"
  record_manifest "${uhe_data_root}/${product}"

  mkdir -p "${vst3_dir}/${product}.vst3/Contents/x86_64-linux/"
  mkdir -p "${vst3_dir}/${product}.vst3/Contents/Resources/Documentation/"
  ln -sfn "${uhe_data_root}/${product}/${so_name}" \
    "${vst3_dir}/${product}.vst3/Contents/x86_64-linux/${product}.so"
  find "${uhe_data_root}/${product}" -maxdepth 1 -name '*.pdf' \
    -exec ln -sfn {} "${vst3_dir}/${product}.vst3/Contents/Resources/Documentation/" \;
  record_manifest "${vst3_dir}/${product}.vst3"

  mkdir -p "${vst_dir}"
  ln -sfn "${uhe_data_root}/${product}/${so_name}" "${vst_dir}/${product}.64.so"
  record_manifest "${vst_dir}/${product}.64.so"

  installed_any=$((installed_any + 1))
done

if [[ ${installed_any} -eq 0 ]]; then
  echo "None of the requested products (${requested_products[*]}) were found in ${archive_name}." >&2
  exit 1
fi

sort -u "${manifest_tmp}" >"${manifest_path}"
echo "  ${manifest_path}"

echo ""
echo "${plugin_id} installed."
echo "  Data: ${uhe_data_root}/<Product>"
echo "  VST3: ${vst3_dir}/<Product>.vst3"
echo "  VST2: ${vst_dir}/<Product>.64.so"
echo ""
echo "Note: u-he freeware ships VST3 and VST2 targets only (no CLAP)."