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
completed=0

say() { printf '%s\n' "$*"; }
die() { say "ERROR: $*" >&2; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

[ "$(id -u)" = 0 ] || die 'Run this installer as root on the reMarkable 2.'
[ "$(uname -m)" = armv7l ] || die 'This release supports reMarkable 2 ARMv7 only.'
for tool in systemctl sha256sum tar sed grep cp mv mkdir chmod date df awk ln head cat sleep; do has "$tool" || die "Missing required command: $tool"; done
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

runtime_found=0
for runtime in /home/root/xovi "$base/runtime/xovi"; do
    if [ -f "$runtime/xovi.so" ]; then
        [ -f "$runtime/extensions.d/qt-resource-rebuilder.so" ] || die "Existing XOVI runtime is incomplete: $runtime"
        runtime_found=1
        break
    fi
done
if [ "$runtime_found" = 0 ]; then
    dependency="$payload/dependencies/xovi-arm32.tar.gz"
    expected=$(sed -n 's/^XOVI_SHA256=//p' "$payload/device/dependencies.env")
    [ -f "$dependency" ] && [ "${#expected}" = 64 ] || die 'The XOVI dependency is missing; use the new PC installer, or transfer dependencies/ with the bundle.'
    actual=$(sha256sum "$dependency"); actual=${actual%% *}
    [ "$actual" = "$expected" ] || die 'XOVI dependency checksum mismatch; no services were changed.'
fi

mkdir -p "$base" "$base/bin" "$base/inbox" "$base/backups"
chmod 700 "$base" "$base/inbox" "$base/backups"
if [ -f "$base/ui-validation.pending" ]; then
    say 'Recovering an interrupted confirmation UI installation before retrying.'
    sh "$payload/device/install-ui.sh" recover-ui-validation || die 'Interrupted UI recovery failed.'
    systemctl start xochitl
fi
stamp=$(date -u +%Y%m%dT%H%M%SZ)-$$
transaction="$base/backups/install-$stamp"
mkdir -p "$transaction/stage"
validation_guard="/run/systemd/system/xochitl.service.d/zzzz-remarkable-quickshare-validation-$stamp.conf"

# Validate the firmware and construct the proprietary copy locally BEFORE stopping services.
# The release bundle contains no reMarkable firmware, original or modified.
"$payload/bin/kindshare" -prepare-xochitl /usr/bin/xochitl -xochitl-output "$transaction/stage/xochitl-local-import"

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
    if [ -f "$transaction/$key.absent" ]; then
        rm -f "$target"
    elif [ -f "$transaction/$key" ]; then
        cp -p "$transaction/$key" "$target.quickshare-restore-$$"
        mv "$target.quickshare-restore-$$" "$target"
    fi
}
install_file() {
    cp "$1" "$2.quickshare-$stamp.tmp"
    chmod "$3" "$2.quickshare-$stamp.tmp"
    mv "$2.quickshare-$stamp.tmp" "$2"
}
cleanup_guard() {
    rm -f "$validation_guard"
    systemctl daemon-reload || true
    systemctl reset-failed xochitl || true
}
for name in kindshare xochitl-local-import; do save "$base/bin/$name" "$name"; done
for name in launch-xochitl.sh install-ui.sh uninstall.sh VERSION original-web-interface; do save "$base/$name" "$name"; done
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
        if ! sh "$base/install-ui.sh" rollback "$transaction"; then
            # The SSH parent may already have removed the download directory.
            # If recovery itself fails, disable just our injection before the
            # previous launcher is restored and automatic restarts resume.
            for qmd in /home/root/xovi/exthome/qt-resource-rebuilder/quickshare.qmd "$base/runtime/xovi/exthome/qt-resource-rebuilder/quickshare.qmd"; do
                if [ -f "$qmd" ] && grep -q '/home/root/quickshare/' "$qmd"; then rm -f "$qmd"; fi
            done
        fi
        for name in kindshare xochitl-local-import; do restore "$base/bin/$name" "$name"; done
        for name in launch-xochitl.sh install-ui.sh uninstall.sh VERSION original-web-interface; do restore "$base/$name" "$name"; done
        restore /etc/systemd/system/remarkable-quickshare.service receiver-unit
        restore "$runtime_drop" runtime-drop
        if [ -d "$(dirname "$xovi_drop")" ]; then restore "$xovi_drop" xovi-drop; fi
        sed -i "s/^WebInterfaceEnabled=.*/WebInterfaceEnabled=$web_before/" "$conf" || true
        if [ -f "$base/ui-validation.pending" ] && [ "$(cat "$base/ui-validation.pending")" = "$transaction" ]; then rm -f "$base/ui-validation.pending" "$base/ui-validation.attempted"; fi
        cleanup_guard
        # An older receiver may auto-accept unconditionally. Never silently
        # downgrade the new confirmation requirement after a failed upgrade.
        systemctl disable "$unit" >/dev/null 2>&1 || true
        systemctl start xochitl || true
        say 'Receiver left disabled; rerun the installer after correcting the error.' >&2
        say "Rollback files retained at: $transaction" >&2
    fi
    exit "$result"
}
trap rollback EXIT
trap 'exit 130' HUP INT TERM

[ ! -e "$validation_guard" ] || die "Refusing to replace an existing validation guard: $validation_guard"
changed=1
mkdir -p /run/systemd/system/xochitl.service.d
printf '%s\n' "$validation_guard" >"$transaction/ui-validation.guard"
printf '# remarkable-quickshare candidate UI validation\n[Service]\nRestart=no\n' >"$validation_guard"
systemctl daemon-reload
[ "$(systemctl show xochitl -p Restart --value)" = no ] || die 'Could not disable automatic Xochitl restarts during UI validation.'
systemctl reset-failed xochitl
systemctl stop "$unit" 2>/dev/null || true
systemctl stop xochitl
cp "$payload/bin/kindshare" "$base/bin/kindshare.new"
chmod 755 "$base/bin/kindshare.new"
mv "$base/bin/kindshare.new" "$base/bin/kindshare"
mv "$transaction/stage/xochitl-local-import" "$base/bin/xochitl-local-import"
install_file "$payload/device/launch-xochitl.sh" "$base/launch-xochitl.sh" 755
install_file "$payload/device/install-ui.sh" "$base/install-ui.sh" 755
install_file "$payload/device/uninstall.sh" "$base/uninstall.sh" 755
cp "$payload/VERSION" "$base/VERSION"
if [ ! -f "$base/original-web-interface" ]; then printf '%s\n' "$web_before" >"$base/original-web-interface"; fi
mkdir -p "$base/licenses" /etc/systemd/system/xochitl.service.d
cp "$payload/LICENSE" "$payload/THIRD_PARTY_NOTICES.md" "$base/"
cp -R "$payload/licenses/." "$base/licenses/"
"$base/bin/kindshare" -init-control -settings "$base/settings.json" -control-token "$base/control-token"
install_file "$payload/device/remarkable-quickshare.service" /etc/systemd/system/remarkable-quickshare.service 644
# Select the recovery-aware launcher persistently BEFORE any candidate QMD can
# be activated, including when XOVI remounts its runtime drop-in directory.
install_file "$payload/device/50-quickshare.conf" "$runtime_drop" 644
if [ -d "$(dirname "$xovi_drop")" ]; then install_file "$payload/device/50-quickshare.conf" "$xovi_drop" 644; fi
cat /proc/sys/kernel/random/boot_id >"$transaction/ui-validation.boot-id"
printf '%s\n' "$transaction" >"$base/ui-validation.pending.new"
rm -f "$base/ui-validation.attempted"
mv "$base/ui-validation.pending.new" "$base/ui-validation.pending"
sh "$base/install-ui.sh" "$payload" "$transaction"
systemctl daemon-reload
systemctl restart xochitl
systemctl enable "$unit" >/dev/null
systemctl restart "$unit"

say 'Checking the receiver, native import API and confirmation UI...'
ready=0
deadline=$(( $(date +%s) + 45 ))
while [ "$(date +%s)" -lt "$deadline" ]; do
    state=$(systemctl show xochitl -p ActiveState --value)
    case "$state" in failed|inactive) die 'Xochitl stopped during candidate UI validation; rolling back.' ;; esac
    if "$base/bin/kindshare" -check-import-api http://10.11.99.1 >/dev/null 2>&1 && "$base/bin/kindshare" -check-control-api -settings "$base/settings.json" -control-token "$base/control-token" >/dev/null 2>&1 && systemctl is-active --quiet "$unit"; then ready=1; break; fi
    sleep 1
done
[ "$ready" = 1 ] || die 'The import API or confirmation UI did not become ready; rolling back.'
sh "$base/install-ui.sh" finish-ui-validation "$transaction"
cleanup_guard
completed=1
say ''
say "Installed reMarkable Quick Share $(cat "$base/VERSION")."
if [ -f "$base/identity" ]; then
    name=$(sed -n 's/^name=//p' "$base/identity")
    endpoint=$(sed -n 's/^id=//p' "$base/identity")
    say "Quick Share name: $name $endpoint"
fi
say 'Keep the tablet awake and connect both devices to the same Wi-Fi network.'
say 'Choose Ask every time or Accept automatically under Settings > Quick Share.'
say 'New installations and first upgrades default to Ask every time.'
say "Rollback backup: $transaction"
say "Uninstall: sh $base/uninstall.sh"
