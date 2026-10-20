#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
base=/home/root/quickshare
disable_candidate_ui() {
    for qmd in /home/root/xovi/exthome/qt-resource-rebuilder/quickshare.qmd "$base/runtime/xovi/exthome/qt-resource-rebuilder/quickshare.qmd"; do
        if [ -f "$qmd" ] && grep -q '/home/root/quickshare/' "$qmd"; then rm -f "$qmd"; fi
    done
}
recover_candidate() {
    if ! sh "$base/install-ui.sh" recover-ui-validation; then
        # A missing/corrupt recovery helper must still let the regular UI boot.
        disable_candidate_ui
        logger -t quickshare 'UI validation recovery failed; candidate injection disabled.' || true
    fi
}
# This check precedes even firmware fallback: an inherited global XOVI preload
# could otherwise load an interrupted candidate with the stock executable too.
if [ -f "$base/ui-validation.pending" ]; then
    transaction=$(cat "$base/ui-validation.pending")
    boot=$(cat /proc/sys/kernel/random/boot_id 2>/dev/null || true)
    previous_boot=$(cat "$transaction/ui-validation.boot-id" 2>/dev/null || true)
    if [ -f "$base/ui-validation.attempted" ] || [ -z "$boot" ] || [ "$boot" != "$previous_boot" ]; then
        recover_candidate
    elif ! (set -C; printf '%s\n' "$boot" >"$base/ui-validation.attempted") 2>/dev/null; then
        recover_candidate
    fi
fi
# The original application is never overwritten. An OTA version change uses stock Xochitl.
expected=87b574fc7ac1a15d502e96458a94a5279a189023bd476758e4aaca9bae13575a
actual=$(sha256sum /usr/bin/xochitl)
actual=${actual%% *}
if [ "$actual" != "$expected" ]; then
    [ ! -f "$base/ui-validation.pending" ] || recover_candidate
    disable_candidate_ui
    logger -t quickshare 'Firmware changed: using original Xochitl; local import needs revalidation.' || true
    exec /usr/bin/xochitl "$@"
fi
copy_hash=$(sha256sum "$base/bin/xochitl-local-import" 2>/dev/null || true)
copy_hash=${copy_hash%% *}
if [ "$copy_hash" != 72deb4483772142ab12d08d12f1d25dbd4e7b1f449a9ead63df103f0ac612fe4 ]; then
    [ ! -f "$base/ui-validation.pending" ] || recover_candidate
    disable_candidate_ui
    logger -t quickshare 'Local-import copy failed verification: using original Xochitl.' || true
    exec /usr/bin/xochitl "$@"
fi
conf=/home/root/.config/remarkable/xochitl.conf
if grep -q '^WebInterfaceEnabled=' "$conf"; then
    configured=0
    sed -i 's/^WebInterfaceEnabled=.*/WebInterfaceEnabled=true/' "$conf" && configured=1
else
    # A stock tablet may never have stored the default-disabled setting.
    configured=0
    sed -i '/^\[General\]$/a\WebInterfaceEnabled=true' "$conf" && configured=1
fi
if [ "$configured" != 1 ] || ! grep -q '^WebInterfaceEnabled=true$' "$conf"; then
    logger -t quickshare 'Cannot enable local import; continuing with original Xochitl.' || true
    exec /usr/bin/xochitl "$@"
fi
if ip link show dev usb0 >/dev/null 2>&1; then
    ip addr show dev usb0 | grep -q '10.11.99.1/' || ip addr add 10.11.99.1/32 dev usb0 || logger -t quickshare 'USB local address unavailable; imports will wait.'
    ip link set usb0 up || logger -t quickshare 'USB interface unavailable; continuing Xochitl startup.'
else
    logger -t quickshare 'USB interface not ready; continuing Xochitl startup.'
fi
if [ -f "$base/ui-runtime.conf" ]; then
    runtime=$(sed -n 's/^root=//p' "$base/ui-runtime.conf")
    kind=$(sed -n 's/^kind=//p' "$base/ui-runtime.conf")
    case "$kind:$runtime" in
        existing:/home/root/xovi|owned:/home/root/quickshare/runtime/xovi)
            if [ -f "$runtime/xovi.so" ] && [ -f "$runtime/extensions.d/qt-resource-rebuilder.so" ]; then
                # Only our new runtime sets its own root. Existing XOVI service
                # roots may contain other extensions and must remain intact.
                case "${LD_PRELOAD:-}" in *"$runtime/xovi.so"*) ;; *) LD_PRELOAD="$runtime/xovi.so${LD_PRELOAD:+:$LD_PRELOAD}" ;; esac
                if [ "$kind" = owned ] || [ -z "${XOVI_ROOT:-}" ]; then XOVI_ROOT=$runtime; fi
                export LD_PRELOAD XOVI_ROOT
                export QML_DISABLE_DISK_CACHE=1
            else
                logger -t quickshare 'Confirmation runtime unavailable; continuing Xochitl without the Quick Share UI.' || true
            fi
            ;;
        *) logger -t quickshare 'Invalid Quick Share runtime configuration; using existing Xochitl environment.' || true ;;
    esac
fi
exec "$base/bin/xochitl-local-import" "$@"
