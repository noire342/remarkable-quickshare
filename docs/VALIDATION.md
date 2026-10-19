# Validation and compatibility

The initial device baseline was validated on **18 October 2026**:

| Area | Evidence | Limit |
| --- | --- | --- |
| Hardware / OS | reMarkable 2, ARMv7, OS 3.27.3.0 | Other firmware images not supported |
| Discovery | Real unicast and multicast mDNS; Android Quick Share UI discovery | Same reachable Wi-Fi LAN required |
| Transfers | PC test sender: real UKEY2, encrypted PDF/EPUB payloads and ACK | Test sender is not Android Quick Share |
| Native import | Both formats registered; original hashes matched; native thumbnails rendered and visually checked | Tiny fixtures, not a large-file benchmark |
| EPUB | Native conversion produced a readable page | DRM books unsupported |
| Recovery | Pending queue reconciled after restart; duplicate content recognized | Device power-loss stress testing not performed |
| Startup | Tablet reboot restored XOVI, Xochitl, receiver and native API | OTA changes require revalidation |
| Idle resource use | Approximately 8 MiB receiver RSS | Snapshot, not a battery benchmark |
| Android | Galaxy S26 Ultra; Samsung Quick Share 13.9.02.31 / GMS 26.41.30 found the receiver | End-to-end Android file send was not completed: phone returned to lock screen |

The publication build was additionally checked on **19 October 2026**:

- The ARMv7 release bundle installed successfully on the same physical tablet.
  The receiver and Xochitl remained active; the native import API passed its health check.
- The installed receiver matched the packaged SHA-256. The original firmware,
  persistent identity and visible document inventory were unchanged.
- Unicast/multicast discovery and real UKEY2 encrypted PDF/EPUB transfers with ACK
  passed again against the publication receiver. Both files reached native Xochitl
  import; this sender remains a PC protocol test, not an Android end-to-end test.
- Nine isolated Linux tests passed, covering update/uninstall data preservation,
  rollback after failed startup, rejection before service changes, bundle integrity/notices,
  and UI startup with an absent default setting, a corrupt copy or a configuration write failure.
- Windows PowerShell 5.1 preserved the complete binary byte range and the exact
  remote shell argument through a fake SSH executable. The actual tablet installation
  used an authenticated SSH stream; the interactive Windows password prompt was not automated.

The publication build also upgrades the inherited DNS/crypto/network dependencies.
Source tests, CI, transport tests and physical-device evidence are distinct checks.

`govulncheck` on the Linux/ARM package graph on 19 October 2026 found no vulnerabilities
in imported packages or reachable symbols. It reports the unmaintained OpenPGP
package in `golang.org/x/crypto` at module level; this project does not import or use
OpenPGP. This is a dated scan, not a continuing security guarantee.

Live opt-in tests create their own PDF/EPUB fixtures:

```sh
python scripts/make_test_documents.py
cd kindshare
REMARKABLE_TEST_HOST=TABLET_IP REMARKABLE_TEST_IFACE=YOUR_INTERFACE go test -run '^TestDevice' -v -count=1
```

Do not run these against an unrelated tablet. Source tests and CI do not substitute
for physical-device validation. The firmware image hashes are compatibility identifiers,
not an assertion of compatibility with every image carrying the same displayed version.
