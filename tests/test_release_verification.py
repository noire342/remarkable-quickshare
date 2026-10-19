import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('release_verification', ROOT/'scripts/verify_published_release.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ReleaseVerificationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.expected = Path(self.temp.name)/'expected'
        self.published = Path(self.temp.name)/'published'
        self.make_release(self.expected)

    def make_release(self, directory, *, host='linux/amd64', receiver=b'receiver', commit='same-tag-commit'):
        directory.mkdir(exist_ok=True)
        files = {'bin/kindshare': receiver, 'LICENSE': b'license retained',
                 'SOURCE.json': json.dumps({'commit': commit, 'version': 'v0.1.0', 'go_version': 'go version go1.27.2 '+host}).encode()}
        files['PAYLOAD.sha256'] = ''.join(hashlib.sha256(data).hexdigest()+'  '+name+'\n' for name, data in sorted(files.items())).encode()
        with tarfile.open(directory/'remarkable-quickshare-linux-armv7.tar.gz', 'w:gz') as tar:
            for name, data in files.items():
                info = tarfile.TarInfo(name); info.size = len(data)
                tar.addfile(info, io.BytesIO(data))
        (directory/'install.ps1').write_bytes(b'Windows installer')
        (directory/'install.sh').write_bytes(b'POSIX installer')
        (directory/'SHA256SUMS').write_text(''.join(hashlib.sha256((directory/name).read_bytes()).hexdigest()+'  '+name+'\n' for name in sorted(module.ASSETS)))

    def test_same_release_from_another_build_host_is_accepted(self):
        self.make_release(self.published, host='windows/amd64')
        module.verify(self.expected, self.published)

    def test_different_receiver_is_rejected_even_with_valid_checksums(self):
        self.make_release(self.published, receiver=b'another receiver')
        with self.assertRaisesRegex(ValueError, 'executable'):
            module.verify(self.expected, self.published)

    def test_different_source_commit_is_rejected(self):
        self.make_release(self.published, commit='another commit')
        with self.assertRaisesRegex(ValueError, 'provenance'):
            module.verify(self.expected, self.published)

    def test_corrupt_asset_is_rejected(self):
        self.make_release(self.published)
        (self.published/'install.ps1').write_bytes(b'corrupt')
        with self.assertRaisesRegex(ValueError, 'checksum'):
            module.verify(self.expected, self.published)
