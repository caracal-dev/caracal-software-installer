#!/usr/bin/env bash
set -euo pipefail

manifest_path="${HOME}/.local/share/caracal-software-installer/manifests/drumlabooh.txt"

if [[ -f "${manifest_path}" ]]; then
    while IFS= read -r target; do
        [[ -z "${target}" ]] && continue
        rm -rf "${target}"
    done < "${manifest_path}"
    rm -f "${manifest_path}"
else
    rm -rf "${HOME}/.lv2/drumlabooh.lv2" \
           "${HOME}/.lv2/drumlabooh-multi.lv2" \
           "${HOME}/.vst3/drumlabooh.vst3" \
           "${HOME}/.vst3/drumlabooh-multi.vst3" \
           "${HOME}/drum_sklad"
fi

echo "Drumlabooh removed from user-local plugin and kit paths"
