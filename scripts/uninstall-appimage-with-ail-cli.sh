#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <app-id> <search-token>" >&2
    exit 1
fi

app_id="$1"
search_token="$2"

appimagelauncher_available() {
    command -v ail-cli >/dev/null 2>&1 && [[ -x /opt/appimagelauncher.AppDir/usr/bin/ail-cli ]]
}

declare -a appimage_paths=()

add_path() {
    local candidate="$1"

    [[ -n "${candidate}" && -f "${candidate}" ]] || return 0

    local existing
    for existing in "${appimage_paths[@]}"; do
        [[ "${existing}" == "${candidate}" ]] && return 0
    done

    appimage_paths+=("${candidate}")
}

for directory in "${HOME}/Applications" "${HOME}/AppImages"; do
    add_path "${directory}/${app_id}.appimage"
    add_path "${directory}/${app_id}.AppImage"

    while IFS= read -r -d '' candidate; do
        add_path "${candidate}"
    done < <(find "${directory}" -maxdepth 1 -type f \( -iname "*${app_id}*.appimage" -o -iname "*${search_token}*.appimage" \) -print0 2>/dev/null || true)
done

while IFS= read -r -d '' desktop_file; do
    while IFS= read -r candidate; do
        add_path "${candidate}"
    done < <(grep -Eo '/[^" ]+\.[Aa]pp[Ii]mage' "${desktop_file}" || true)
done < <(find "${HOME}/.local/share/applications" -maxdepth 1 -type f \( -iname "*${app_id}*.desktop" -o -iname "*${search_token}*.desktop" \) -print0 2>/dev/null || true)

if [[ ${#appimage_paths[@]} -eq 0 ]]; then
    echo "No integrated ${app_id} AppImage found; cleaning stale desktop integration paths"
else
    for appimage_path in "${appimage_paths[@]}"; do
        if appimagelauncher_available; then
            echo "Unintegrating ${appimage_path} with ail-cli..."
            ail-cli unintegrate "${appimage_path}" || true
        else
            echo "AppImageLauncher is unavailable or incomplete; removing ${appimage_path} manually."
        fi

        rm -f "${appimage_path}"
    done
fi

find "${HOME}/.local/share/applications" -maxdepth 1 -type f \( -iname "*${app_id}*.desktop" -o -iname "*${search_token}*.desktop" \) -delete 2>/dev/null || true
find "${HOME}/.local/share/icons" -type f \( -iname "*${app_id}*" -o -iname "*${search_token}*" \) -delete 2>/dev/null || true

echo "${app_id} removed from AppImageLauncher integration paths"
