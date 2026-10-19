# Security

Report security issues privately using this repository's GitHub security advisory
page when available. Do not post passwords, tablet backups or personal documents.

The receiver automatically accepts supported files from reachable peers. It does
not authenticate senders as Google/Samsung contacts. Use trusted Wi-Fi networks
and stop it on untrusted networks:

```sh
systemctl stop remarkable-quickshare
```

Only completed PDF/EPUB transfers enter the import queue. Files are bounded to
100,000,000 bytes each, 32 files and 300,000,000 bytes per offer, with at most two
active sessions. The native document parser is Xochitl; format checks in the
receiver are not a guarantee that arbitrary documents are safe.

The native import API is bound to the tablet USB address, without a Wi-Fi HTTP
proxy. The daemon has restricted filesystem writes and no capabilities, but it
runs as root to access the existing tablet library. Do not expose TCP 12345,
mDNS or SSH to the Internet.

PC installers verify GitHub downloads over HTTPS, then SHA-256 checksums, and send
the package over SSH with normal host key verification. SHA-256 manifests check
integrity; they are not an independent signature if a release account is compromised.
Tablet BusyBox wget is not used to download release binaries because its TLS
certificate validation may be absent.

The local firmware adaptation is restricted to one validated OS 3.27.3.0 image.
Unknown firmware is rejected before stopping Xochitl. After an OTA change, the
launcher falls back to the original application instead of executing a stale copy.
Revalidate third-party services after firmware updates. The original executable
is never overwritten or included in release assets.
