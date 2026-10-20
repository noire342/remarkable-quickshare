#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
base=/home/root/quickshare
[ "$(id -u)" = 0 ] || { echo 'Run as root.' >&2; exit 1; }
[ -x /usr/bin/xochitl ] || { echo 'Original Xochitl is missing; refusing to change services.' >&2; exit 1; }
cleanup_validation_guards() {
    # Preserve unrelated overrides, including files using a similar name.
    for guard in /run/systemd/system/xochitl.service.d/zzzz-remarkable-quickshare-validation-*.conf; do
        if [ -f "$guard" ] && grep -q '^# remarkable-quickshare candidate UI validation$' "$guard"; then rm -f "$guard"; fi
    done
    systemctl reset-failed xochitl || true
}
# Always bring the tablet UI back if a later command fails or the session is interrupted.
restart_ui() {
    result=$?
    trap - EXIT HUP INT TERM
    cleanup_validation_guards
    systemctl daemon-reload || true
    systemctl start xochitl || true
    exit "$result"
}
trap restart_ui EXIT
trap 'exit 130' HUP INT TERM
systemctl disable --now remarkable-quickshare.service 2>/dev/null || true
systemctl stop xochitl
if [ -f "$base/ui-validation.pending" ] && [ -f "$base/install-ui.sh" ]; then
    sh "$base/install-ui.sh" recover-ui-validation || true
fi
rm -f "$base/ui-validation.pending" "$base/ui-validation.attempted"
cleanup_validation_guards
if [ -f "$base/ui-runtime.conf" ]; then
    runtime=$(sed -n 's/^root=//p' "$base/ui-runtime.conf")
    case "$runtime" in /home/root/xovi|/home/root/quickshare/runtime/xovi)
        qmd="$runtime/exthome/qt-resource-rebuilder/quickshare.qmd"
        if [ -f "$qmd" ] && grep -q '/home/root/quickshare/' "$qmd"; then rm -f "$qmd"; fi
        ;;
    esac
fi
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
