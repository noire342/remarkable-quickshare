#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
base=/home/root/quickshare
# The original application is never overwritten. An OTA version change uses stock Xochitl.
expected=87b574fc7ac1a15d502e96458a94a5279a189023bd476758e4aaca9bae13575a
actual=$(sha256sum /usr/bin/xochitl)
actual=${actual%% *}
if [ "$actual" != "$expected" ]; then
    logger -t quickshare 'Firmware changed: using original Xochitl; local import needs revalidation.' || true
    exec /usr/bin/xochitl "$@"
fi
copy_hash=$(sha256sum "$base/bin/xochitl-local-import" 2>/dev/null || true)
copy_hash=${copy_hash%% *}
if [ "$copy_hash" != 72deb4483772142ab12d08d12f1d25dbd4e7b1f449a9ead63df103f0ac612fe4 ]; then
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
exec "$base/bin/xochitl-local-import" "$@"
