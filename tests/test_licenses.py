"""Missing Go module metadata must never attribute the project's license to it."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipIf(os.name == 'nt', 'The isolated fake Go executable uses POSIX sh')
class LicenseCollectionTests(unittest.TestCase):
    def test_missing_module_directory_cannot_fall_back_to_working_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory)
            (folder/'LICENSE').write_text('An unrelated local project license')
            fake_go = folder/'go'
            fake_go.write_text('''#!/bin/sh
case "$1" in
  mod) exit 0 ;;
  list) printf '%s\\n' '{"Path":"golang.org/x/sync","Version":"v0.22.0"}' ;;
  *) exit 99 ;;
esac
''')
            fake_go.chmod(0o755)
            result = subprocess.run([sys.executable, str(ROOT/'scripts/collect_licenses.py'),
                                     '--go', str(fake_go), '--check'], cwd=folder,
                                    capture_output=True, text=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('Dependency is not present: golang.org/x/sync', result.stderr)


if __name__ == '__main__':
    unittest.main()
