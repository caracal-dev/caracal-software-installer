#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
    echo "Usage: $0 <app-id> <display-name> <url>" >&2
    exit 1
fi

app_id="$1"
display_name="$2"
url="$3"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
temp_dir="$(mktemp -d)"
download_path="${temp_dir}/${app_id}.appimage"
install_dir="${HOME}/Applications"
installed_path="${install_dir}/${app_id}.appimage"
desktop_dir="${HOME}/.local/share/applications"
desktop_path="${desktop_dir}/${app_id}.desktop"

cleanup() {
    rm -rf "${temp_dir}"
}

trap cleanup EXIT

appimagelauncher_available() {
    command -v ail-cli >/dev/null 2>&1 && [[ -x /opt/appimagelauncher.AppDir/usr/bin/ail-cli ]]
}

resolve_bundled_icon() {
    local icon_dir
    local extension
    local candidates=()

    if [[ -n "${CARACAL_INSTALLER_APPIMAGE_ICON_DIR:-}" ]]; then
        candidates+=("${CARACAL_INSTALLER_APPIMAGE_ICON_DIR}")
    fi

    candidates+=(
        "/usr/share/caracal-software-installer/assets/images/app-desktop-icons"
        "${script_dir}/../assets/images/app-desktop-icons"
    )

    for icon_dir in "${candidates[@]}"; do
        [[ -d "${icon_dir}" ]] || continue

        for extension in png svg jpg jpeg webp; do
            if [[ -f "${icon_dir}/${app_id}.${extension}" ]]; then
                printf '%s\n' "${icon_dir}/${app_id}.${extension}"
                return
            fi
        done
    done
}

desktop_escape() {
    local value="$1"

    value="${value//\\/\\\\}"
    value="${value//$'\n'/ }"
    value="${value//$'\r'/ }"
    value="${value//\"/\\\"}"

    printf '%s' "${value}"
}

manual_install() {
    local icon="application-x-executable"
    local bundled_icon=""

    echo "Installing ${display_name} manually in ${install_dir}..."

    mkdir -p "${install_dir}" "${desktop_dir}"
    install -m755 "${download_path}" "${installed_path}"

    bundled_icon="$(resolve_bundled_icon || true)"
    if [[ -n "${bundled_icon}" ]]; then
        icon="${bundled_icon}"
        echo "Using bundled desktop icon ${bundled_icon}"
    fi

    cat >"${desktop_path}" <<EOF
[Desktop Entry]
Type=Application
Name=$(desktop_escape "${display_name}")
Comment=$(desktop_escape "${display_name}") AppImage
Exec="$(desktop_escape "${installed_path}")" %U
TryExec=$(desktop_escape "${installed_path}")
Icon=$(desktop_escape "${icon}")
Terminal=false
Categories=AudioVideo;Audio;Music;
StartupNotify=false
EOF

    chmod 644 "${desktop_path}"

    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "${desktop_dir}" >/dev/null 2>&1 || true
    fi

    echo "${display_name} installed manually at ${installed_path}"
}

echo "Downloading ${display_name} AppImage..."
curl -fL --retry 3 --retry-delay 2 -o "${download_path}" "${url}"
chmod 755 "${download_path}"

if appimagelauncher_available; then
    echo "Integrating ${display_name} with ail-cli..."
    if ail-cli integrate "${download_path}"; then
        echo "${display_name} integrated with AppImageLauncher"
        exit 0
    fi

    echo "AppImageLauncher integration failed; falling back to a manual AppImage install." >&2
else
    echo "AppImageLauncher is unavailable or incomplete; falling back to a manual AppImage install." >&2
fi

manual_install
