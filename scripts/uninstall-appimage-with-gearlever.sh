#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <app-id> <search-token>" >&2
    exit 1
fi

app_id="$1"
search_token="$2"

rm -f "${HOME}/AppImages/${app_id}.appimage"
find "${HOME}/.local/share/applications" -maxdepth 1 -type f \( -iname "*${app_id}*.desktop" -o -iname "*${search_token}*.desktop" \) -delete 2>/dev/null || true
find "${HOME}/.local/share/icons" -type f \( -iname "*${app_id}*" -o -iname "*${search_token}*" \) -delete 2>/dev/null || true

echo "${app_id} removed from AppImages and desktop integration paths"
