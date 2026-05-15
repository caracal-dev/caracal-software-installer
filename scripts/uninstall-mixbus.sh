#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This uninstaller must run as root because Mixbus installs system paths." >&2
    exit 1
fi

rm -rf /opt/Mixbus-11* /opt/Mixbus\ 11* /opt/mixbus-11*
rm -f /usr/local/bin/mixbus11 /usr/local/bin/mixbus
find /usr/local/share/applications /usr/share/applications -maxdepth 1 -type f -iname '*mixbus*.desktop' -delete 2>/dev/null || true
find /usr/local/share/icons /usr/share/icons -type f -iname '*mixbus*' -delete 2>/dev/null || true

echo "Mixbus removed from common system installation paths"
