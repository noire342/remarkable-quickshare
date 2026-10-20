# Validation and compatibility

## v0.2.0 development status

The on-tablet confirmation/settings update is in development, **not released**.
The v0.1.0 evidence below is not evidence that the new popup, default manual mode
or automatic XOVI bootstrap has passed physical-device validation.

Release acceptance requires tablet checks for the popup while reading/writing,
mode persistence, accept/reject/timeout/cancellation, missing-UI behavior,
existing-XOVI reuse, dependency bootstrap and recovery, plus native PDF and EPUB
import. It also requires a real Android-originated send and matching connection
codes. A PC protocol sender alone does not satisfy that Android check. The user
later confirmed an Android PDF transfer and matching codes; EPUB and the
remaining physical checks below are still required before publication.

Development checks completed on **20 October 2026**:

- Go tests and vet pass, including Google PIN vectors, encrypted approval flows,
  early-content rejection, cancellation, expiry and concurrent encrypted writes.
- Thirty Python tests pass: twenty-four installer/transport cases, four release
  verification cases and two QMLDiff parser/patch cases. The latter use original
  synthetic fixtures; no Xochitl resources are distributed with the tests.
- The pinned QMLDiff parser also applied the patch offline to private resources
  from the exact supported firmware. Windows PowerShell 5.1 preserved the verified
  dependency/bundle transport and exact remote command through a fake SSH client.
- The development package installed on the physical tablet using its existing
  XOVI installation. Its original executable, identity and document inventory
  were unchanged during installation. The native import API and authenticated
  UI health checks passed; automatic restarts were disabled during validation
  and the temporary guard was removed after success.
- The user confirmed the native Settings page, radio selection in both
  directions, the icon, placement directly below Wi-Fi and Help remaining last.
  The saved manual setting survived subsequent candidate updates.
- An independent PC protocol probe displayed a four-digit code matching the
  sender's UKEY2 result and canceled successfully. A second simultaneous manual
  offer was rejected while a real sender's request was pending.
- The user confirmed a successful real **Windows Quick Share PDF** send through
  the popup; the receiver acknowledged the complete file and native import
  registered it in Xochitl.
- The user additionally confirmed a successful **Android Quick Share PDF** send
  and explicitly confirmed that the four-digit codes matched on both devices.
  Android EPUB reception has not yet been confirmed.

An early draft QMD caused Xochitl startup failures. The corrected patch was
validated offline before reinstalling; the installer now has restart suppression
and persistent recovery guards with regression coverage. No further physical
testing was performed after the user asked to end those tests.

Still required before release: Android EPUB reception and native conversion,
confirmation while reading/writing, the remaining physical rejection/expiry and
missing-UI scenarios, an owned-runtime bootstrap, final reboot and uninstall
checks. Automated coverage of these paths is not physical-device evidence.

## Published v0.1.0 baseline

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
  remote shell argument through a fake SSH executable. A complete Windows installation
  then passed with the real OpenSSH client, host-key verification and interactive password
  authentication. This used the local release assets; public download is a separate check.
- Four release-verification tests check that existing published assets can be verified
  without replacing them, while rejecting a different executable, source commit or checksum.

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
