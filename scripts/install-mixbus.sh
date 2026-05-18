#!/usr/bin/env bash
set -euo pipefail

version="11.3.0"
url="https://eu1.download.solidstatelogic.com/Mixbus%2011/Mixbus%2011.3/Mixbus-11.3.0-x86_64.tar"

workdir="$(mktemp -d)"
archive_path="${workdir}/mixbus.tar"
extract_dir="${workdir}/extract"
terminal_bin_dir="${workdir}/terminal-bin"

cleanup() {
    rm -rf "${workdir}"
}

trap cleanup EXIT

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This installer must run as root because the Mixbus .run installer writes system paths." >&2
    exit 1
fi

setup_terminal_compat() {
    mkdir -p "${terminal_bin_dir}"

    local terminal_cmd=""
    local candidate
    for candidate in ghostty konsole xterm gnome-terminal; do
        if terminal_cmd="$(command -v "${candidate}" 2>/dev/null)" && [[ -n "${terminal_cmd}" ]]; then
            break
        fi
    done

    if [[ -z "${terminal_cmd}" ]]; then
        return 0
    fi

    cat >"${terminal_bin_dir}/caracal-terminal-shim" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

terminal_cmd="${CARACAL_MIXBUS_TERMINAL:-}"
if [[ -z "${terminal_cmd}" ]]; then
    echo "CARACAL_MIXBUS_TERMINAL is not set." >&2
    exit 127
fi

args=()
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -e|-x|--execute)
            shift
            if [[ "$#" -eq 1 ]]; then
                exec "${terminal_cmd}" -e bash -lc "$1"
            fi
            exec "${terminal_cmd}" -e "$@"
            ;;
        --)
            shift
            exec "${terminal_cmd}" -e "$@"
            ;;
        --wait|--disable-factory|--hide-menubar)
            shift
            ;;
        --title|--working-directory|--app-id|--class)
            shift
            if [[ "$#" -gt 0 ]]; then
                shift
            fi
            ;;
        --title=*|--working-directory=*|--app-id=*|--class=*)
            shift
            ;;
        *)
            args+=("$1")
            shift
            ;;
    esac
done

if [[ "${#args[@]}" -eq 1 ]]; then
    exec "${terminal_cmd}" -e bash -lc "${args[0]}"
fi
if [[ "${#args[@]}" -gt 1 ]]; then
    exec "${terminal_cmd}" -e "${args[@]}"
fi

exec "${terminal_cmd}"
EOF

    chmod +x "${terminal_bin_dir}/caracal-terminal-shim"
    ln -s caracal-terminal-shim "${terminal_bin_dir}/gnome-terminal"
    ln -s caracal-terminal-shim "${terminal_bin_dir}/exterm"

    export CARACAL_MIXBUS_TERMINAL="${terminal_cmd}"
    export PATH="${terminal_bin_dir}:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"
    echo "Using ${terminal_cmd} for Mixbus terminal compatibility."
}

setup_terminal_compat

echo "Downloading Mixbus ${version}..."
curl -fL --retry 3 --retry-delay 2 -o "${archive_path}" "${url}"

mkdir -p "${extract_dir}"
tar -xf "${archive_path}" -C "${extract_dir}"

run_installer="$(find "${extract_dir}" -type f -name '*.run' | head -n 1 || true)"
if [[ -z "${run_installer}" ]]; then
    echo "Mixbus .run installer was not found in the tarball." >&2
    exit 1
fi

chmod +x "${run_installer}"
echo "Running Mixbus installer..."
"${run_installer}"

echo "Mixbus installer completed"
