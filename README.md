# reMarkable Quick Share

Receive **PDF and EPUB** files from Android Quick Share over Wi-Fi and import
them automatically into the reMarkable library. No cable, Bluetooth adapter,
cloud account or extra Android app is required for reception.

Built for **reMarkable 2, OS 3.27.3.0**. Other models and firmware images are
rejected by the installer. This is an unofficial community project.

Based on [KindShare by June Philip](https://github.com/unrealJune/kindshare),
with a reMarkable importer, persistent queue and easy SSH installers.

## Install from Windows

Open **PowerShell** on your PC and paste:

```powershell
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/noire342/remarkable-quickshare/v0.1.0/install.ps1')))
```

Enter the tablet's Wi-Fi IP address, confirm its SSH host key, then enter its
root SSH password. USB networking also works with the default `10.11.99.1`.
The password is available on the tablet under Help → Copyrights and licenses.
Only the Windows **OpenSSH Client** is needed; no Python, Go or package manager.

Prefer inspecting the script first? Download `install.ps1` from
[Releases](https://github.com/noire342/remarkable-quickshare/releases), review it,
then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -HostAddress TABLET_IP
```

The execution policy override applies only to that process. The script verifies
the release over HTTPS and SHA-256, sends it through one SSH connection, creates
the firmware adaptation **locally on your tablet**, and enables automatic start.
It checks the native import API and restores the previous installation on failure.

## Install from macOS or Linux

```sh
curl --fail --location --proto '=https' --tlsv1.2 \
  https://raw.githubusercontent.com/noire342/remarkable-quickshare/v0.1.0/install.sh \
  -o install-quickshare.sh
sh install-quickshare.sh TABLET_IP
```

To run directly on the tablet, first transfer the verified release bundle over
SSH/USB, extract it into a temporary folder, and run `sh device/install-device.sh`
from that extracted bundle. The installer does not download unsigned firmware or
use the tablet's limited BusyBox wget for HTTPS verification.

## Use

1. Keep the tablet awake and connect it and your phone to the same Wi-Fi network.
2. Select a PDF or EPUB on Android → **Share → Quick Share**.
3. Choose **reMarkable 2 XXXX**, where the four-character suffix identifies your tablet.
4. The file appears in **My Files**. Xochitl handles EPUB conversion, thumbnails
   and normal library synchronization.

Files are accepted **automatically** from reachable senders. Use a trusted network.
The tablet cannot receive while asleep; this app preserves normal standby.
Receiving only: sending from the tablet is not implemented. Limits are 100 MB
(100,000,000 bytes) per file, 32 files / 300 MB per offer. DRM EPUBs are unsupported.

## Update or uninstall

Run the Windows installer again to update to the latest published release. The
device identity, queued files, receipts and existing documents are preserved.

```powershell
.\install.ps1 -HostAddress TABLET_IP -Version v0.1.0  # select a release
.\install.ps1 -HostAddress TABLET_IP -Uninstall       # disable and restore stock Xochitl
```

Or uninstall on the tablet:

```sh
sh /home/root/quickshare/uninstall.sh
```

Uninstall preserves imported documents, identity, queued files and recovery
backups. Inactive program files remain available for recovery or reinstall.

## What is installed

- One static ARMv7 Go receiver in `/home/root/quickshare/bin/`.
- A persistent identity and atomic, validated transfer queue.
- Native HTTP import through Xochitl, with SHA-256 reconciliation to avoid
  duplicate imports after interruption. The receiver does not write library metadata.
- A `systemd` service with bounded sessions, restricted filesystem writes, low
  priority, network recovery and automatic restart.
- A separate local Xochitl copy with the USB interface adaptation documented by
  [webinterface-onboot](https://github.com/rM-self-serve/webinterface-onboot).
  **`/usr/bin/xochitl` is never overwritten. No firmware binary is distributed.**
- A hash-guarded launcher and install rollback backups. Existing XOVI integrations
  are detected; no XOVI or KOReader installation is required by the receiver.

**Firmware updates require revalidation.** The launcher falls back to stock
Xochitl if its original firmware hash changes. OTA updates may also remove
third-party service files. Do not apply the adaptation blindly to another version.

## Troubleshooting

```sh
systemctl status remarkable-quickshare
journalctl -u remarkable-quickshare -n 50 --no-pager
cat /tmp/kindshare-status.json
systemctl restart remarkable-quickshare
```

If Android does not see the tablet, keep the send screen open for 20–30 seconds,
check that Wi-Fi stays connected, and avoid guest/client-isolated networks.
Some Quick Share versions temporarily drop their infrastructure Wi-Fi connection;
see [KindShare's troubleshooting notes](https://github.com/unrealJune/kindshare/blob/main/docs/TROUBLESHOOTING.md).
Being far from the tablet is fine while both devices remain on the same reachable LAN.

Status counters reset with the process. Persistent import receipts live in
`/home/root/quickshare/inbox/receipts/`. Transfer success acknowledges reception;
native import can complete later if Xochitl is temporarily busy.

## Build and validation

Go **1.27.2**, Python 3.10+ for release packaging; no Python on the tablet:

```sh
cd kindshare
go test ./...
go vet ./...
cd ..
python scripts/collect_licenses.py --check
python -m unittest discover -s tests -v
python scripts/build_release.py --version v0.1.0
```

The ARMv7 build is static (`CGO_ENABLED=0`) and includes full third-party notices.
Builds and releases are automated by GitHub Actions, pinned to action commit SHAs.

See [validation and compatibility](docs/VALIDATION.md),
[architecture](docs/ARCHITECTURE.md), [Italian quick start](docs/IT.md), and
[security notes](SECURITY.md). Tests, CI and device validation are reported separately.

## License and credits

Our integration, installers and original additions are **MIT**, Copyright (c) 2026
Lorenzo Garrone (noire342). KindShare's MIT copyright and license are preserved.
Google protocol definitions and generated bindings retain **Apache-2.0**; Go
dependencies retain their MIT/BSD notices. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
and [licenses/](licenses/).

Xochitl is proprietary reMarkable software and is not covered by our MIT license.
This project is not affiliated with reMarkable, Google or Samsung.
