# On-tablet confirmation — v0.2.0 development

This feature is in development. The public installation commands still select
v0.1.0; v0.2.0 will be published only after the required physical-device and
Android validation. Support remains limited to the recognized reMarkable 2
OS 3.27.3.0 firmware image.

## Reception modes

In **Settings → Quick Share**, select one of two modes:

Quick Share sits directly below **Wi-Fi** in the existing Settings sidebar.
It uses the native settings frame and radio controls; Help remains the last entry.

- **Ask every time / Chiedi sempre** — the default for a new installation or
  migration from v0.1.0. A popup shows the sender, offered files, total size and
  temporary four-digit connection code. Compare the code with the phone, then
  choose **Accept / Accetta** or **Decline / Rifiuta**.
- **Accept automatically / Accetta automaticamente** — the earlier behavior:
  supported files from any reachable sender are accepted without a popup. This
  does not authenticate Google/Samsung contacts or remember a trusted phone.

The mode is saved separately from the tablet's Quick Share identity and survives
restarts and updates. It applies to new offers immediately. Changing to automatic
mode does not approve a request already waiting for a decision.

Only one manual request can wait at a time. It expires after 60 seconds and is
rejected on sender cancellation, disconnection or unavailable UI. File contents
are not saved before approval. Explicitly selected automatic mode continues
working without the confirmation interface.

The code is derived from the current encrypted connection, not a permanent PIN
you choose or type into Android. It is useful only when compared with the sender's
display. The sender's advertised name is untrusted text.

## Installation and dependencies

The development installer reuses compatible XOVI installations and preserves
their other extensions. When XOVI is absent it obtains a pinned ARM32 upstream
archive, verifies its SHA-256 on the PC, and prepares the required XOVI/resource
rebuilder runtime. The tablet may need to be unlocked during QML table setup.
No unrelated inactive extensions are enabled. Xochitl itself remains proprietary;
its original executable is neither replaced nor included in release assets.

Candidate UI validation temporarily disables Xochitl's automatic restart policy.
A failed start restores the previous UI and leaves the receiver disabled. A
persistent transaction marker also recovers an interrupted installation before
a second candidate start, including after a reboot. Existing extensions remain
in place.

XOVI, qt-resource-rebuilder and QMLDiff keep their upstream licenses. Their
version-specific source and license details are in
[the third-party notices](../THIRD_PARTY_NOTICES.md#xovi-confirmation-interface-v020-development)
and [provenance inventory](../licenses/xovi-provenance.json). The MIT project
license does not replace those licenses.

## Local control interface

The QML interface uses `127.0.0.1:12346` on the tablet with a generated bearer
token. It reads state, saves the mode and submits a decision for one live request.
It is not accessible as a Wi-Fi settings page. Do not publish the token or generated
private UI configuration.

The normal Quick Share protocol and PDF/EPUB import remain unchanged after the
offer has been approved. Existing identity, completed queue entries, receipts
and library documents are retained during migration.
