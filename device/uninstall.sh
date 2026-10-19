#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
base=/home/root/quickshare
[ "$(id -u)" = 0 ] || { echo 'Run as root.' >&2; exit 1; }
[ -x /usr/bin/xochitl ] || { echo 'Original Xochitl is missing; refusing to change services.' >&2; exit 1; }
# Always bring the tablet UI back if a later command fails or the session is interrupted.
restart_ui() {
    result=$?
    trap - EXIT HUP INT TERM
    systemctl daemon-reload || true
    systemctl start xochitl || true
    exit "$result"
}
trap restart_ui EXIT
trap 'exit 130' HUP INT TERM
systemctl disable --now remarkable-quickshare.service 2>/dev/null || true
systemctl stop xochitl
for file in /etc/systemd/system/xochitl.service.d/50-quickshare.conf /home/root/xovi/services/xochitl.service/50-quickshare.conf; do
    if [ -f "$file" ] && grep -q '/home/root/quickshare/' "$file"; then rm -f "$file"; fi
done
if [ -f /etc/systemd/system/remarkable-quickshare.service ] && grep -q '/home/root/quickshare/' /etc/systemd/system/remarkable-quickshare.service; then
    rm -f /etc/systemd/system/remarkable-quickshare.service
fi
if [ -f "$base/original-web-interface" ]; then
    old=$(cat "$base/original-web-interface")
    case "$old" in true|false) sed -i "s/^WebInterfaceEnabled=.*/WebInterfaceEnabled=$old/" /home/root/.config/remarkable/xochitl.conf ;; esac
fi
systemctl daemon-reload
systemctl start xochitl
trap - EXIT HUP INT TERM
echo 'Quick Share disabled. Original Xochitl restored.'
echo 'Imported documents, identity, pending files and rollback backups are preserved.'
echo 'The inactive program files remain in /home/root/quickshare for recovery or reinstall.'
