#!/usr/bin/env bash
# Removes MOD Desktop from /opt and the /usr/local wrappers and desktop entry.
set -euo pipefail

rm -rf /opt/mod-desktop
rm -f /usr/local/bin/mod-desktop
rm -f /usr/local/share/applications/mod-desktop.desktop
rm -f /usr/local/share/icons/hicolor/scalable/apps/mod-desktop.svg
rm -f /usr/local/share/icons/hicolor/256x256/apps/mod-desktop.png

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/local/share/applications >/dev/null 2>&1 || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t -f /usr/local/share/icons/hicolor >/dev/null 2>&1 || true
fi

if command -v kbuildsycoca6 >/dev/null 2>&1; then
    kbuildsycoca6 >/dev/null 2>&1 || true
fi

echo "MOD Desktop removed."
