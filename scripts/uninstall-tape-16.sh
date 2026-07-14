#!/usr/bin/env bash
set -euo pipefail

echo "Removing TAPE 16..."

# Kill running TAPE 16 processes
pkill -f 'TAPE 16|tape_daw|TapeDAW|tape_midi_host|tape_plugin_scan_helper' >/dev/null 2>&1 || true

# Remove main app directory
rm -rf /opt/tape-16

# Remove wrappers and utility scripts
rm -f \
    /usr/local/bin/tape16 \
    /usr/local/bin/tape16-stable \
    /usr/local/bin/tape16-lowlatency \
    /usr/local/bin/tape16-preflight \
    /usr/local/bin/tape16-create-desktop-copy \
    /usr/local/bin/tape16-system-check \
    /usr/local/bin/tape16-reset-defaults \
    /usr/local/bin/TAPE16 \
    /usr/local/bin/tape-16

# Remove desktop entries
rm -f \
    /usr/local/share/applications/tape16.desktop \
    /usr/local/share/applications/tape16-lowlatency.desktop \
    /usr/local/share/applications/TAPE16.desktop \
    /usr/local/share/applications/tape-16.desktop

# Remove icons
find /usr/local/share/icons -name '*tape16*' -delete 2>/dev/null || true
find /usr/local/share/icons -name '*TAPE16*' -delete 2>/dev/null || true
find /usr/local/share/icons -name '*tape-16*' -delete 2>/dev/null || true

# Remove metainfo
rm -f /usr/local/share/metainfo/com.jackpaterson.tapedaw.metainfo.xml 2>/dev/null || true

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/local/share/applications
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/local/share/icons/hicolor 2>/dev/null || true
fi

echo "TAPE 16 removed."