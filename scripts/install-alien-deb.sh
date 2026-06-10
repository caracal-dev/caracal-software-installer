#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "Usage: $0 <package-id> <display-name> <deb-url> [local-deb]" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "This installer must run as root because it installs a system RPM." >&2
  exit 1
fi

package_id="$1"
display_name="$2"
deb_url="$3"
local_deb="${4:-}"

state_root="/var/lib/caracal-software-installer/alien"
state_path="${state_root}/${package_id}.package"
workdir="$(mktemp -d)"
deb_name="$(basename "${deb_url%%\?*}")"
if [[ -n "${local_deb}" ]]; then
  deb_name="$(basename "${local_deb}")"
fi
deb_path="${workdir}/${deb_name}"
rpm_path=""
rpm_name=""

cleanup() {
  rm -rf "${workdir}"
}

require_command() {
  local command="$1"
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "Need ${command} to install ${display_name}." >&2
    exit 1
  fi
}

route_plugins() {
  if ! command -v ujust >/dev/null 2>&1; then
    echo "ujust was not found; skipping route-plugins refresh."
    return
  fi

  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    local target_home=""
    target_home="$(getent passwd "${SUDO_USER}" | cut -d: -f6 || true)"
    if [[ -n "${target_home}" ]]; then
      sudo -u "${SUDO_USER}" HOME="${target_home}" ujust route-plugins || true
      return
    fi
  fi

  ujust route-plugins || true
}

install_rpm() {
  local rpm="$1"

  if command -v rpm-ostree >/dev/null 2>&1 && [[ -d /run/ostree-booted ]]; then
    if rpm-ostree install --apply-live -y "${rpm}"; then
      return
    fi

    echo "Live rpm-ostree install failed; staging ${display_name} for the next boot."
    rpm-ostree install -y "${rpm}"
    echo "${display_name} was staged by rpm-ostree. Reboot before using the installed plugin files."
    return
  fi

  if command -v dnf >/dev/null 2>&1; then
    dnf install -y "${rpm}"
    return
  fi

  if command -v rpm >/dev/null 2>&1; then
    rpm -Uvh "${rpm}"
    return
  fi

  echo "Need rpm-ostree, dnf, or rpm to install ${display_name}." >&2
  exit 1
}

trap cleanup EXIT

if [[ -z "${local_deb}" ]]; then
  require_command curl
fi
require_command alien
require_command rpm

if [[ -n "${local_deb}" ]]; then
  if [[ ! -f "${local_deb}" ]]; then
    echo "Local Debian package not found: ${local_deb}" >&2
    exit 1
  fi
  echo "Installing ${display_name} from ${local_deb}..."
  cp -f "${local_deb}" "${deb_path}"
else
  echo "Downloading ${display_name}..."
  curl -fL --retry 3 --retry-delay 2 -o "${deb_path}" "${deb_url}"
fi

echo "Converting ${deb_name} to RPM with alien..."
(
  cd "${workdir}"
  alien --to-rpm --scripts --keep-version "${deb_path}"
)

rpm_path="$(find "${workdir}" -maxdepth 1 -type f -name '*.rpm' | head -n 1)"
if [[ -z "${rpm_path}" ]]; then
  echo "Alien did not create an RPM for ${display_name}." >&2
  exit 1
fi

rpm_name="$(rpm -qp --queryformat '%{NAME}' "${rpm_path}")"
if [[ -z "${rpm_name}" ]]; then
  echo "Could not read package name from ${rpm_path}." >&2
  exit 1
fi

echo "Installing ${rpm_name} from converted RPM..."
install_rpm "${rpm_path}"

mkdir -p "${state_root}"
printf '%s\n' "${rpm_name}" >"${state_path}"

route_plugins

echo "${display_name} installed as RPM package ${rpm_name}."
echo "State recorded in ${state_path}."
