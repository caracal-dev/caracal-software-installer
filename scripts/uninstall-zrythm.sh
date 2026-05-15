#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "This uninstaller must run as root because it removes /opt and /usr/local files." >&2
    exit 1
fi

rm -f /opt/zrythm
rm -rf /opt/zrythm-trial-1.0.0
rm -f /usr/local/share/applications/org.zrythm.Zrythm-installer.desktop
rm -f /usr/local/share/icons/hicolor/scalable/apps/org.zrythm.Zrythm.svg
rm -f /usr/local/share/man/man1/zrythm.1
rm -f /usr/local/share/metainfo/org.zrythm.Zrythm-installer.appdata.xml
rm -f /usr/local/share/mime/packages/org.zrythm.Zrythm-mime.xml

echo "Zrythm removed from /opt and /usr/local integration paths"
