#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <package-id> <display-name>" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "This uninstaller must run as root because it removes a system RPM." >&2
  exit 1
fi

package_id="$1"
display_name="$2"
state_path="/var/lib/caracal-software-installer/alien/${package_id}.package"

if [[ ! -f "${state_path}" ]]; then
  echo "No recorded alien-installed RPM package for ${display_name} at ${state_path}." >&2
  exit 1
fi

rpm_name="$(head -n 1 "${state_path}")"
if [[ -z "${rpm_name}" ]]; then
  echo "Recorded package state for ${display_name} is empty." >&2
  exit 1
fi

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

remove_rpm() {
  local name="$1"

  if command -v rpm-ostree >/dev/null 2>&1 && [[ -d /run/ostree-booted ]]; then
    if rpm-ostree uninstall --apply-live -y "${name}"; then
      return
    fi

    echo "Live rpm-ostree uninstall failed; staging ${display_name} removal for the next boot."
    rpm-ostree uninstall -y "${name}"
    echo "${display_name} removal was staged by rpm-ostree. Reboot to finish removing plugin files."
    return
  fi

  if command -v dnf >/dev/null 2>&1; then
    dnf remove -y "${name}"
    return
  fi

  if command -v rpm >/dev/null 2>&1; then
    rpm -e "${name}"
    return
  fi

  echo "Need rpm-ostree, dnf, or rpm to uninstall ${display_name}." >&2
  exit 1
}

echo "Removing ${display_name} RPM package ${rpm_name}..."
remove_rpm "${rpm_name}"
rm -f "${state_path}"

route_plugins

echo "${display_name} removed."
