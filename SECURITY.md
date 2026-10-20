# Security

Report security issues privately using this repository's GitHub security advisory
page when available. Do not post passwords, tablet backups or personal documents.

The published v0.1.0 receiver automatically accepts supported files from reachable
peers. It does not authenticate senders as Google/Samsung contacts. Use trusted Wi-Fi networks
and stop it on untrusted networks:

```sh
systemctl stop remarkable-quickshare
```

The v0.2.0 development build adds **Ask every time** and **Accept automatically**
under the tablet's Quick Share settings. Missing or invalid settings select
**Ask every time**, including migration from v0.1.0. The choice survives updates
and restarts. Automatic mode is an explicit choice to accept supported offers
from all reachable senders; it is not a list of trusted phones.

In manual mode, the temporary four-digit code identifies the current encrypted
connection. Compare it with the code on the sending device before accepting.
The displayed sender name is supplied by the peer and is not proof of identity.
This is not a fixed password, account authentication or permanent pairing.
Only one manual offer can wait at a time. An unavailable interface, cancellation,
disconnect or 60-second decision timeout rejects the offer. File contents are not
written before approval. Changing the global mode does not approve an offer that
was already waiting for confirmation.

The development UI control API listens only on `127.0.0.1:12346`. It requires a
per-installation bearer token held in a root-readable private file. Do not publish
that token or the generated private UI configuration. Approval is tied to the
current request ID; stale decisions cannot authorize a new connection. The UI
heartbeat is required in manual mode; explicitly selected automatic mode can
continue without the popup. The control API is not a Wi-Fi administration page.

These development security behaviors must be tested separately from v0.1.0's
existing device evidence; see [validation](docs/VALIDATION.md).

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
