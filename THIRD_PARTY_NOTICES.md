# Third-party notices

The original reMarkable integration, importer, installers, release tooling and
project documentation are Copyright (c) 2026 Lorenzo Garrone (noire342), under
the [MIT license](LICENSE). This does not replace any third-party license below.

## KindShare

The receiver is a modified version of [unrealJune/kindshare](https://github.com/unrealJune/kindshare),
based on commit `25800e334961bfc7e7468a85b638fc16d7a12bc5`.

Copyright (c) 2026 June Philip. Licensed under MIT. The original notice and full
license are retained in [kindshare/LICENSE](kindshare/LICENSE) and
[licenses/kindshare-MIT.txt](licenses/kindshare-MIT.txt).

Changes include the reMarkable defaults, persistent import queue, validation and
resource bounds, native Xochitl import, firmware preparation and health commands,
suspend recovery and release integration. Files derived from KindShare retain
that attribution; the project does not claim exclusive authorship of the receiver.

## Google protocol definitions

`proto/*.proto` and the corresponding generated `kindshare/pb/**/*.pb.go` retain
their Google LLC copyright and **Apache-2.0** headers. They were obtained from
the pinned KindShare source, which credits NearDrop's collected Chromium/Google
definitions. They are **not MIT or BSD code** merely because the surrounding
receiver is MIT. The full [Apache-2.0 license](licenses/Apache-2.0.txt) is included.
The generated definitions are retained without local modifications.

## Local USB web interface adaptation

The local string-substitution recipe is based on
[rM-self-serve/webinterface-onboot](https://github.com/rM-self-serve/webinterface-onboot),
Copyright (c) 2023 rM-self-serve, under MIT. Its full notice is retained in
[licenses/webinterface-onboot-MIT.txt](licenses/webinterface-onboot-MIT.txt).
Our implementation validates a known firmware hash and creates a copy locally;
it does not distribute the original or modified Xochitl executable.

## Go modules and runtime

The exact resolved module versions, license identifiers and paths to their full
license/notice/patent texts are listed in
[licenses/go-modules.json](licenses/go-modules.json). This covers the module
graph, including build/test dependencies; not every listed module is linked
into the receiver. MIT, BSD-2-Clause and BSD-3-Clause notices are retained in
`licenses/go-modules/`. The Go runtime's BSD-3-Clause license is retained in
[licenses/Go-BSD-3-Clause.txt](licenses/Go-BSD-3-Clause.txt).

Release bundles carry this notice, the project MIT license and the complete
`licenses/` directory. `scripts/collect_licenses.py --check` verifies that the
reviewed module graph and copied license texts match the build environment.

## Research and interoperability credits

- [grishka/NearDrop](https://github.com/grishka/NearDrop): protocol documentation
  and collection of protocol definitions, credited by KindShare.
- [Martichou/rquickshare](https://github.com/Martichou/rquickshare): implementation
  research credited by KindShare.
- [google/ukey2](https://github.com/google/ukey2): handshake specification.

These research credits are not a claim that another project's license covers
our code or that those projects endorse this port.

## Not included

Xochitl, reMarkable firmware, XOVI, KOReader and InkShelf are not distributed by
this project. In particular, Xochitl is proprietary software provided by
reMarkable. The installer uses the copy already present on the user's own
supported tablet; the project MIT license grants no rights to that firmware.
Product and protocol names belong to their respective owners. This is an
independent, unofficial project, not affiliated with Google, Samsung or reMarkable.
