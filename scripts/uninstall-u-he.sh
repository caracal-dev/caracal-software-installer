#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 [<plugin-id>] <Product>..." >&2
    exit 1
fi

plugin_id=""
if [[ "$1" != *","* && "$#" -ge 2 && ! -f "$1" ]]; then
    plugin_id="$1"
    shift
fi

manifest_path=""
if [[ -n "${plugin_id}" ]]; then
    manifest_path="${HOME}/.local/share/caracal-software-installer/manifests/${plugin_id}.txt"
fi

if [[ -n "${manifest_path}" && -f "${manifest_path}" ]]; then
    while IFS= read -r target; do
        [[ -z "${target}" ]] && continue
        rm -rf "${target}"
    done < "${manifest_path}"
    rm -f "${manifest_path}"
    echo "u-he products removed using recorded manifest."
    exit 0
fi

for product in "$@"; do
    product_norm="$(printf '%s' "${product}" | tr -d ' -_' | tr '[:upper:]' '[:lower:]')"
    if [[ -d "${HOME}/.u-he" ]]; then
        for dir in "${HOME}/.u-he"/*/; do
            name="$(basename "${dir}")"
            name_norm="$(printf '%s' "${name}" | tr -d ' -_' | tr '[:upper:]' '[:lower:]')"
            if [[ "${name_norm}" == "${product_norm}" ]]; then
                rm -rf "${dir}"
                rm -rf "${HOME}/.vst3/u-he/${name}.vst3"
                rm -f "${HOME}/.vst/u-he/${name}.64.so"
            fi
        done
    fi
done

echo "u-he products removed."