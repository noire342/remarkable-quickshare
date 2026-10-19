# Architecture

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
