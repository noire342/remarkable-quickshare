# Architecture

The following import pipeline describes the published v0.1.0 baseline. The
v0.2.0 development build inserts the approval gate described below before
creating a transfer directory or accepting file content.

```text
Android / Quick Share sender
  -> mDNS discovery (_FC9F5ED42C8A._tcp)
  -> TCP 12345 / UKEY2 P-256 / AES-CBC + HMAC
  -> private transfer directory / *.partial
  -> size, offset and PDF/EPUB checks / fsync / atomic rename
  -> one persistent import worker
  -> native Xochitl HTTP upload at 10.11.99.1
  -> original hash reconciliation / receipt / queue cleanup
  -> My Files, native EPUB rendering and thumbnails
```

The receiver is a locally modified KindShare daemon. It has no runtime Python,
Alpine, KOReader or Bluetooth dependency. Wi-Fi interface changes rebind mDNS;
wall-clock gaps detect suspend/resume. Identity is persisted separately from binaries.

The importer streams fixed-length multipart bodies. Xochitl requires quoted
Content-Disposition attributes. A matching SHA-256 of an immutable native original
confirms registration even when EPUB metadata changes the visible title. Pending
files survive import errors and process restarts. Original files and metadata are
read for reconciliation; only Xochitl writes library records during normal operation.

The original Xochitl executable remains in `/usr/bin/xochitl`. A known-image-only
command prepares a separate copy on the device. It uses the MIT-licensed
webinterface-onboot recipe, verifies both original and prepared hashes, and refuses
unknown versions. Releases contain the preparer code, not reMarkable executables.

The launcher verifies both hashes again at startup and falls back to the stock
executable on mismatch. A systemd drop-in selects it. If XOVI manages a temporary
drop-in mount, the installer also writes the same drop-in into XOVI's persistent
source directory. Other XOVI extensions are preserved.

The PC downloader verifies HTTPS and SHA-256, then streams the archive through
one SSH connection. The tablet validates the payload manifest and firmware before
stopping services. Installation backups capture the previous executables and
service configuration. An EXIT trap restores them on failure. The queue, identity
and document library are never replaced by installation or update.

The public source and binary bundle use explicit allowlists. Private device backups,
credentials, host keys, per-device status, test logs and Xochitl copies are excluded.

## Development approval gate

UKEY2 already derives the connection authentication material. The approval layer
formats its Quick Share-compatible four-digit code and snapshots the saved
`acceptance_mode` (`ask` or `auto`) when a valid offer arrives. In `ask` mode,
one request can wait for a tablet decision for 60 seconds. The connection reader
continues handling keepalives and disconnection while approval is pending.
Creating file output and the content-level ACCEPT response follow approval;
transport handshake acceptance alone does not authorize content. Encrypted
message sequence allocation and writes are serialized.

The original QML UI is loaded through XOVI's resource rebuilder and talks to the
Go process through an authenticated loopback HTTP control API. Settings, UI
readiness and the pending offer are exposed to that local UI; decisions identify
one live request. Long polling avoids continuous e-ink redraws. The control
bearer token is generated on the tablet and is absent from public source/assets.
The saved mode is separate from device identity. No live pending approval is
restored after a process restart.

An existing compatible XOVI installation is reused. When absent, the development
installer obtains the pinned upstream ARM32 archive and sets up the required
runtime without enabling its unrelated inactive extensions. Runtime dependency
provenance is separate from the static receiver's Go module inventory.
