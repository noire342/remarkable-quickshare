#!/bin/sh
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
set -eu
repo=noire342/remarkable-quickshare
host=${1:-}
version=${QUICKSHARE_VERSION:-v0.2.0}
port=${QUICKSHARE_SSH_PORT:-22}
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo 'Invalid release version.' >&2; exit 1 ;; esac
case "$version" in *[!a-zA-Z0-9.-]*) echo 'Invalid release version.' >&2; exit 1 ;; esac
case "$port" in ''|*[!0-9]*) echo 'Invalid SSH port.' >&2; exit 1 ;; esac
if [ -z "$host" ]; then
    printf 'reMarkable IP address (Enter for USB: 10.11.99.1): ' >/dev/tty
    IFS= read -r host </dev/tty
    host=${host:-10.11.99.1}
fi
case "$host" in ''|-*|*[!a-zA-Z0-9.:-]*) echo 'Invalid tablet address.' >&2; exit 1 ;; esac
for tool in curl ssh mktemp tar; do command -v "$tool" >/dev/null 2>&1 || { echo "Missing $tool; run this from a Mac or Linux PC." >&2; exit 1; }; done
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT HUP INT TERM
name=remarkable-quickshare-linux-armv7.tar.gz
url="https://github.com/$repo/releases/download/$version"
curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error "$url/$name" -o "$temp/$name"
curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error "$url/SHA256SUMS" -o "$temp/SHA256SUMS"
expected=$(awk -v name="$name" '$2==name {print $1}' "$temp/SHA256SUMS")
[ "${#expected}" = 64 ] || { echo 'Missing release checksum.' >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$temp/$name"); else actual=$(shasum -a 256 "$temp/$name"); fi
actual=${actual%% *}
[ "$actual" = "$expected" ] || { echo 'Checksum mismatch; nothing installed.' >&2; exit 1; }
tar -xzf "$temp/$name" -C "$temp" device/dependencies.env
xovi_url=$(sed -n 's/^XOVI_URL=//p' "$temp/device/dependencies.env")
xovi_sha=$(sed -n 's/^XOVI_SHA256=//p' "$temp/device/dependencies.env")
case "$xovi_url" in https://github.com/asivery/rm-xovi-extensions/releases/download/*/xovi-arm32.tar.gz) ;; *) echo 'Invalid XOVI URL.' >&2; exit 1 ;; esac
printf '%s\n' "$xovi_sha" | grep -Eq '^[0-9a-f]{64}$' || { echo 'Invalid XOVI checksum.' >&2; exit 1; }
mkdir -p "$temp/transport/dependencies"
echo 'Downloading the pinned XOVI runtime...'
curl --fail --location --proto '=https' --tlsv1.2 --silent --show-error "$xovi_url" -o "$temp/transport/dependencies/xovi-arm32.tar.gz"
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$temp/transport/dependencies/xovi-arm32.tar.gz"); else actual=$(shasum -a 256 "$temp/transport/dependencies/xovi-arm32.tar.gz"); fi
actual=${actual%% *}
[ "$actual" = "$xovi_sha" ] || { echo 'XOVI checksum mismatch; nothing installed.' >&2; exit 1; }
cp "$temp/$name" "$temp/transport/receiver.tar.gz"
tar -cf "$temp/ssh-install.tar" -C "$temp/transport" receiver.tar.gz dependencies/xovi-arm32.tar.gz
echo "Verified $version. Confirm the host key and enter the tablet SSH password."
ssh -p "$port" -o StrictHostKeyChecking=ask "root@$host" 'set -eu; umask 077; d=$(mktemp -d /home/root/.quickshare-install.XXXXXX); trap '\''rm -rf "$d"'\'' EXIT HUP INT TERM; tar -xf - -C "$d"; mkdir "$d/payload"; tar -xzf "$d/receiver.tar.gz" -C "$d/payload"; mkdir -p "$d/payload/dependencies"; mv "$d/dependencies/xovi-arm32.tar.gz" "$d/payload/dependencies/"; sh "$d/payload/device/install-device.sh"' <"$temp/ssh-install.tar"
