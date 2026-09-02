#!/usr/bin/env bash
# Installs TONE3000: VST3, LV2, CLAP plugins, standalone app, factory presets,
# desktop entry, and icon — all user-local, no sudo required.
set -euo pipefail

TONE3000_VERSION="0.0.2"
TONE3000_URL="https://github.com/tone-3000/tone3000-plugin/releases/download/v${TONE3000_VERSION}/TONE3000-v${TONE3000_VERSION}-linux-x64.tar.gz"

workdir="$(mktemp -d)"
archive_name="TONE3000-v${TONE3000_VERSION}-linux-x64.tar.gz"
archive_path="${workdir}/${archive_name}"
extract_dir="${workdir}/extract"

plugin_vst3_dir="${HOME}/.vst3"
plugin_lv2_dir="${HOME}/.lv2"
plugin_clap_dir="${HOME}/.clap"
standalone_dir="${HOME}/.local/bin"
desktop_dir="${HOME}/.local/share/applications"
icon_dir="${HOME}/.local/share/icons/hicolor"
factory_presets_dir="${XDG_CONFIG_HOME:-${HOME}/.config}/TONE3000/Presets/Factory"
manifest_root="${HOME}/.local/share/caracal-software-installer/manifests"
manifest_path="${manifest_root}/tone-3000.txt"
manifest_tmp="${workdir}/manifest.txt"

cleanup() {
  rm -rf "${workdir}"
}

record_manifest() {
  local target="$1"
  printf '%s\n' "${target}" >>"${manifest_tmp}"
}

trap cleanup EXIT

echo "Downloading TONE3000 ${TONE3000_VERSION}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${TONE3000_URL}"

echo "Extracting..."
mkdir -p "${extract_dir}"
tar -xzf "${archive_path}" -C "${extract_dir}"

# The tarball extracts to a folder named TONE3000-<arch>-<toolchain>/ containing
# TONE3000.vst3, TONE3000.lv2, TONE3000.clap, TONE3000, tone3000.png, etc.
shopt -s nullglob
payload_dirs=("${extract_dir}/TONE3000-"*/)
shopt -u nullglob
if [[ ${#payload_dirs[@]} -ne 1 ]]; then
  echo "Could not locate the extracted TONE3000 payload directory." >&2
  exit 1
fi
payload_dir="${payload_dirs[0]}"

mkdir -p "${manifest_root}"
: >"${manifest_tmp}"

# VST3
echo "Installing VST3 to ${plugin_vst3_dir} ..."
if [[ -d "${payload_dir}/TONE3000.vst3" ]]; then
  mkdir -p "${plugin_vst3_dir}"
  rm -rf "${plugin_vst3_dir}/TONE3000.vst3"
  cp -a "${payload_dir}/TONE3000.vst3" "${plugin_vst3_dir}/"
  record_manifest "${plugin_vst3_dir}/TONE3000.vst3"
fi

# LV2
echo "Installing LV2 to ${plugin_lv2_dir} ..."
if [[ -d "${payload_dir}/TONE3000.lv2" ]]; then
  mkdir -p "${plugin_lv2_dir}"
  rm -rf "${plugin_lv2_dir}/TONE3000.lv2"
  cp -a "${payload_dir}/TONE3000.lv2" "${plugin_lv2_dir}/"
  record_manifest "${plugin_lv2_dir}/TONE3000.lv2"
fi

# CLAP
echo "Installing CLAP to ${plugin_clap_dir} ..."
if [[ -f "${payload_dir}/TONE3000.clap" ]]; then
  mkdir -p "${plugin_clap_dir}"
  install -m 644 "${payload_dir}/TONE3000.clap" "${plugin_clap_dir}/TONE3000.clap"
  record_manifest "${plugin_clap_dir}/TONE3000.clap"
fi

# Standalone binary
echo "Installing standalone app to ${standalone_dir} ..."
if [[ -f "${payload_dir}/TONE3000" ]]; then
  mkdir -p "${standalone_dir}"
  install -m 755 "${payload_dir}/TONE3000" "${standalone_dir}/TONE3000"
  record_manifest "${standalone_dir}/TONE3000"
fi

# Factory presets
if compgen -G "${payload_dir}/factory-presets/*.t3kpreset" >/dev/null; then
  echo "Installing factory presets to ${factory_presets_dir} ..."
  mkdir -p "${factory_presets_dir}"
  rm -f "${factory_presets_dir}"/*.t3kpreset
  cp "${payload_dir}/factory-presets"/*.t3kpreset "${factory_presets_dir}/"
  # Don't record each preset; the directory is tracked as a whole.
fi

# Desktop entry + icon
desktop_entry="${desktop_dir}/tone3000.desktop"
icon_file="${icon_dir}/512x512/apps/tone3000.png"
if [[ -f "${payload_dir}/tone3000.png" ]]; then
  echo "Installing desktop entry and icon ..."
  mkdir -p "$(dirname "${desktop_entry}")" "$(dirname "${icon_file}")"
  install -m 644 "${payload_dir}/tone3000.png" "${icon_file}"
  record_manifest "${icon_file}"
  cat >"${desktop_entry}" <<EOF
[Desktop Entry]
Type=Application
Name=TONE3000
Comment=Neural Amp Modeler plugin and standalone player
Exec=${standalone_dir}/TONE3000
Icon=tone3000
Categories=Audio;AudioVideo;
Terminal=false
EOF
  record_manifest "${desktop_entry}"
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "${desktop_dir}" 2>/dev/null || true
  fi
fi

sort -u "${manifest_tmp}" >"${manifest_path}"
echo "  ${manifest_path}"

echo ""
echo "TONE3000 ${TONE3000_VERSION} installed."
echo "  Plugins:  VST3, LV2, CLAP"
echo "  Standalone: ${standalone_dir}/TONE3000"
[[ -f "${desktop_entry}" ]] && echo "  Desktop entry: ${desktop_entry}"
echo ""
echo "Note: TONE3000 requires WebKitGTK at runtime for the plugin UI."
echo "If the plugin window renders black, install webkit2gtk-4.1 via your package manager."