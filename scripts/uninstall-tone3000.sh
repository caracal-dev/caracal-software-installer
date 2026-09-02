#!/usr/bin/env bash
# Uninstalls TONE3000 (VST3, LV2, CLAP, standalone, desktop entry, icon, presets).
set -euo pipefail

standalone_dir="${HOME}/.local/bin"
desktop_dir="${HOME}/.local/share/applications"
icon_dir="${HOME}/.local/share/icons/hicolor"
factory_presets_dir="${XDG_CONFIG_HOME:-${HOME}/.config}/TONE3000/Presets/Factory"
manifest_root="${HOME}/.local/share/caracal-software-installer/manifests"
manifest_path="${manifest_root}/tone-3000.txt"

# Prefer manifest for exact removal; fall back to known paths.
if [[ -f "${manifest_path}" ]]; then
  while IFS= read -r target; do
    [[ -z "${target}" ]] && continue
    rm -rf "${target}"
  done < "${manifest_path}"
  rm -f "${manifest_path}"
else
  rm -rf "${HOME}/.vst3/TONE3000.vst3" 2>/dev/null || true
  rm -rf "${HOME}/.lv2/TONE3000.lv2" 2>/dev/null || true
  rm -f  "${HOME}/.clap/TONE3000.clap" 2>/dev/null || true
  rm -f  "${standalone_dir}/TONE3000" 2>/dev/null || true
  rm -f  "${desktop_dir}/tone3000.desktop" 2>/dev/null || true
  rm -f  "${icon_dir}/512x512/apps/tone3000.png" 2>/dev/null || true
  rm -f  "${icon_dir}/256x256/apps/tone3000.png" 2>/dev/null || true
  rm -f  "${icon_dir}/128x128/apps/tone3000.png" 2>/dev/null || true
  rm -f  "${icon_dir}/64x64/apps/tone3000.png" 2>/dev/null || true
  rm -f  "${icon_dir}/48x48/apps/tone3000.png" 2>/dev/null || true
  rm -f  "${icon_dir}/32x32/apps/tone3000.png" 2>/dev/null || true
fi

# Factory presets — remove only the ones shipped with the tarball (cleanup
# was already done by manifest if present, but handle the case where the
# manifest never recorded them).
rm -f "${factory_presets_dir}"/*.t3kpreset 2>/dev/null || true
rmdir "${factory_presets_dir}" 2>/dev/null || true

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "${desktop_dir}" 2>/dev/null || true
fi

echo "TONE3000 uninstalled."