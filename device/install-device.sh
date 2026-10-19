#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
umask 077

base=/home/root/quickshare
payload=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
unit=remarkable-quickshare.service
runtime_drop=/etc/systemd/system/xochitl.service.d/50-quickshare.conf
xovi_drop=/home/root/xovi/services/xochitl.service/50-quickshare.conf
conf=/home/root/.config/remarkable/xochitl.conf
changed=0
receiver_was_active=0
receiver_was_enabled=0
completed=0

say() { printf '%s\n' "$*"; }
die() { say "ERROR: $*" >&2; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

[ "$(id -u)" = 0 ] || die 'Run this installer as root on the reMarkable 2.'
[ "$(uname -m)" = armv7l ] || die 'This release supports reMarkable 2 ARMv7 only.'
for tool in systemctl sha256sum tar sed grep cp mv mkdir chmod date df awk; do has "$tool" || die "Missing required command: $tool"; done
[ -f /usr/bin/xochitl ] && [ -f "$conf" ] || die 'This does not look like a supported reMarkable installation.'
[ -f "$payload/PAYLOAD.sha256" ] || die 'Incomplete release bundle (checksum manifest missing).'
(cd "$payload" && sha256sum -c PAYLOAD.sha256) || die 'Release bundle integrity check failed.'
[ ! -L "$base" ] || die 'Refusing a symlink installation directory.'
available=$(df -Pk /home | awk 'NR==2 {print $4}')
[ "${available:-0}" -ge 131072 ] || die 'At least 128 MiB of free /home space is required.'

for drop in "$runtime_drop" "$xovi_drop"; do
    if [ -e "$drop" ] && ! grep -q '/home/root/quickshare/' "$drop"; then
        die "Refusing to replace an unrelated drop-in: $drop"
    fi
done

mkdir -p "$base" "$base/bin" "$base/inbox" "$base/backups"
chmod 700 "$base" "$base/inbox" "$base/backups"
stamp=$(date -u +%Y%m%dT%H%M%SZ)-$$
transaction="$base/backups/install-$stamp"
mkdir -p "$transaction/stage"

# Validate the firmware and construct the proprietary copy locally BEFORE stopping services.
# The release bundle contains no reMarkable firmware, original or modified.
"$payload/bin/kindshare" -prepare-xochitl /usr/bin/xochitl -xochitl-output "$transaction/stage/xochitl-local-import"

systemctl is-active --quiet "$unit" && receiver_was_active=1 || true
systemctl is-enabled --quiet "$unit" && receiver_was_enabled=1 || true
web_before=$(sed -n 's/^WebInterfaceEnabled=//p' "$conf" | head -n 1)
web_before=${web_before:-false}

save() {
    source=$1
    key=$2
    if [ -e "$source" ]; then cp -p "$source" "$transaction/$key"; else : >"$transaction/$key.absent"; fi
}
restore() {
    target=$1
    key=$2
    if [ -f "$transaction/$key.absent" ]; then rm -f "$target"; elif [ -f "$transaction/$key" ]; then cp -p "$transaction/$key" "$target"; fi
}
for name in kindshare xochitl-local-import; do save "$base/bin/$name" "$name"; done
for name in launch-xochitl.sh uninstall.sh VERSION original-web-interface; do save "$base/$name" "$name"; done
save /etc/systemd/system/remarkable-quickshare.service receiver-unit
save "$runtime_drop" runtime-drop
save "$xovi_drop" xovi-drop
cp -p "$conf" "$transaction/xochitl.conf.before"

rollback() {
    result=$?
    trap - EXIT HUP INT TERM
    if [ "$completed" != 1 ] && [ "$changed" = 1 ]; then
        say 'Installation failed; restoring the previous executable and service configuration.' >&2
        systemctl stop "$unit" 2>/dev/null || true
        systemctl stop xochitl 2>/dev/null || true
        for name in kindshare xochitl-local-import; do restore "$base/bin/$name" "$name"; done
        for name in launch-xochitl.sh uninstall.sh VERSION original-web-interface; do restore "$base/$name" "$name"; done
        restore /etc/systemd/system/remarkable-quickshare.service receiver-unit
        restore "$runtime_drop" runtime-drop
        if [ -d "$(dirname "$xovi_drop")" ]; then restore "$xovi_drop" xovi-drop; fi
        sed -i "s/^WebInterfaceEnabled=.*/WebInterfaceEnabled=$web_before/" "$conf" || true
        systemctl daemon-reload || true
        if [ "$receiver_was_enabled" = 1 ]; then systemctl enable "$unit" >/dev/null 2>&1 || true; else systemctl disable "$unit" >/dev/null 2>&1 || true; fi
        systemctl start xochitl || true
        if [ "$receiver_was_active" = 1 ]; then systemctl start "$unit" || true; fi
        say "Rollback files retained at: $transaction" >&2
    fi
    exit "$result"
}
trap rollback EXIT
trap 'exit 130' HUP INT TERM

changed=1
systemctl stop "$unit" 2>/dev/null || true
cp "$payload/bin/kindshare" "$base/bin/kindshare.new"
chmod 755 "$base/bin/kindshare.new"
mv "$base/bin/kindshare.new" "$base/bin/kindshare"
mv "$transaction/stage/xochitl-local-import" "$base/bin/xochitl-local-import"
cp "$payload/device/launch-xochitl.sh" "$base/launch-xochitl.sh"
cp "$payload/device/uninstall.sh" "$base/uninstall.sh"
chmod 755 "$base/launch-xochitl.sh" "$base/uninstall.sh"
cp "$payload/VERSION" "$base/VERSION"
if [ ! -f "$base/original-web-interface" ]; then printf '%s\n' "$web_before" >"$base/original-web-interface"; fi
mkdir -p "$base/licenses" /etc/systemd/system/xochitl.service.d
cp "$payload/LICENSE" "$payload/THIRD_PARTY_NOTICES.md" "$base/"
cp -R "$payload/licenses/." "$base/licenses/"
cp "$payload/device/remarkable-quickshare.service" /etc/systemd/system/remarkable-quickshare.service
cp "$payload/device/50-quickshare.conf" "$runtime_drop"
if [ -d "$(dirname "$xovi_drop")" ]; then cp "$payload/device/50-quickshare.conf" "$xovi_drop"; fi
chmod 644 /etc/systemd/system/remarkable-quickshare.service "$runtime_drop"
systemctl daemon-reload
systemctl restart xochitl
systemctl enable "$unit" >/dev/null
systemctl restart "$unit"

say 'Checking the receiver and native import API...'
ready=0
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do
    if "$base/bin/kindshare" -check-import-api http://10.11.99.1 >/dev/null 2>&1 && systemctl is-active --quiet "$unit"; then ready=1; break; fi
    sleep 1
done
[ "$ready" = 1 ] || die 'The native import API did not become ready; rolling back.'
completed=1
say ''
say "Installed reMarkable Quick Share $(cat "$base/VERSION")."
if [ -f "$base/identity" ]; then
    name=$(sed -n 's/^name=//p' "$base/identity")
    endpoint=$(sed -n 's/^id=//p' "$base/identity")
    say "Quick Share name: $name $endpoint"
fi
say 'Keep the tablet awake and connect both devices to the same Wi-Fi network.'
say 'PDF and EPUB files are accepted automatically and imported into My Files.'
say "Rollback backup: $transaction"
say "Uninstall: sh $base/uninstall.sh"
