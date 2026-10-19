"""Installer transaction tests use an isolated fake tablet; no real systemd is called."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipIf(os.name == 'nt', 'POSIX fake-tablet tests run in Linux CI')
class InstallTransactionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='quickshare-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.base = self.root/'home/quickshare'
        self.units = self.root/'etc/systemd/system'
        self.xovi = self.root/'home/xovi/services/xochitl.service'
        self.conf = self.root/'home/.config/remarkable/xochitl.conf'
        self.original = self.root/'usr/bin/xochitl'
        for path in [self.units,self.xovi,self.conf.parent,self.original.parent,self.base/'bin',self.base/'inbox']:
            path.mkdir(parents=True,exist_ok=True)
        self.original.write_text('original-firmware')
        self.original.chmod(0o755)
        self.conf.write_text('[General]\nWebInterfaceEnabled=false\nUnrelatedSetting=preserve\n')
        (self.base/'identity').write_text('name=Existing tablet\nid=KEEP\n')
        (self.base/'inbox/keep.pdf').write_bytes(b'pending-owner-document')
        (self.base/'bin/kindshare').write_text('previous-receiver')
        (self.base/'bin/xochitl-local-import').write_text('previous-prepared-image')
        (self.base/'launch-xochitl.sh').write_text('previous-launcher')
        self.fakebin=self.root/'fakebin';self.fakebin.mkdir()
        commands={
            'id':'#!/bin/sh\necho 0\n',
            'uname':'#!/bin/sh\necho armv7l\n',
            'df':'#!/bin/sh\nprintf "Filesystem 1024-blocks Used Available Capacity Mounted on\\nvolume 999999 1 999998 1%% /home\\n"\n',
            'sleep':'#!/bin/sh\nexit 0\n',
            'systemctl':'#!/bin/sh\nprintf "%s\\n" "$*" >> "$TEST_ROOT/systemctl.log"\ncase "$1" in is-active|is-enabled) exit 0;; esac\n',
        }
        for name,content in commands.items():self.executable(self.fakebin/name,content)
        self.payload=self.root/'payload';(self.payload/'device').mkdir(parents=True);(self.payload/'bin').mkdir();(self.payload/'licenses').mkdir()
        mapping={
            '/home/root/quickshare':str(self.base),
            '/etc/systemd/system':str(self.units),
            '/home/root/xovi/services/xochitl.service':str(self.xovi),
            '/home/root/.config/remarkable/xochitl.conf':str(self.conf),
            '/usr/bin/xochitl':str(self.original),
        }
        for source in (ROOT/'device').iterdir():
            if source.is_file():
                text=source.read_text()
                for before,after in mapping.items():text=text.replace(before,after)
                (self.payload/'device'/source.name).write_text(text)
        self.executable(self.payload/'bin/kindshare', '''#!/bin/sh
case "$1" in
  -prepare-xochitl)
    [ "${FAIL_PREPARE:-0}" = 0 ] || exit 1
    printf 'new-prepared-image' > "$4"
    chmod 755 "$4"
    ;;
  -check-import-api) [ "${FAIL_HEALTH:-0}" = 0 ] ;;
  *) exit 0 ;;
esac
''')
        for name in ['LICENSE','THIRD_PARTY_NOTICES.md']:(self.payload/name).write_text('fixture license')
        (self.payload/'licenses/notice.txt').write_text('fixture dependency notice')
        (self.payload/'VERSION').write_text('v0.1.0\n')
        self.checksums()
        self.env=os.environ.copy();self.env.update(PATH=str(self.fakebin)+os.pathsep+self.env['PATH'],TEST_ROOT=str(self.root))

    @staticmethod
    def executable(path,content):
        path.write_text(content);path.chmod(0o755)

    def checksums(self):
        lines=[]
        for file in sorted(self.payload.rglob('*')):
            if file.is_file() and file.name!='PAYLOAD.sha256':
                lines.append(hashlib.sha256(file.read_bytes()).hexdigest()+'  '+file.relative_to(self.payload).as_posix()+'\n')
        (self.payload/'PAYLOAD.sha256').write_text(''.join(lines))

    def install(self,**flags):
        return subprocess.run(['sh',str(self.payload/'device/install-device.sh')],env=dict(self.env,**flags),text=True,capture_output=True)

    def assert_owner_data_untouched(self):
        self.assertEqual((self.base/'identity').read_text(),'name=Existing tablet\nid=KEEP\n')
        self.assertEqual((self.base/'inbox/keep.pdf').read_bytes(),b'pending-owner-document')
        self.assertEqual(self.original.read_text(),'original-firmware')
        self.assertIn('UnrelatedSetting=preserve',self.conf.read_text())

    def test_update_and_uninstall_preserve_data(self):
        result=self.install();self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertEqual((self.base/'VERSION').read_text(),'v0.1.0\n')
        self.assertIn('Prepared', (self.base/'bin/xochitl-local-import').read_text().title())
        self.assertTrue((self.xovi/'50-quickshare.conf').exists())
        self.assert_owner_data_untouched()
        result=subprocess.run(['sh',str(self.base/'uninstall.sh')],env=self.env,text=True,capture_output=True)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertFalse((self.xovi/'50-quickshare.conf').exists())
        self.assertFalse((self.units/'remarkable-quickshare.service').exists())
        self.assert_owner_data_untouched()

    def test_failed_health_check_rolls_back(self):
        result=self.install(FAIL_HEALTH='1')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('restoring the previous',result.stderr)
        self.assertEqual((self.base/'bin/kindshare').read_text(),'previous-receiver')
        self.assertEqual((self.base/'bin/xochitl-local-import').read_text(),'previous-prepared-image')
        self.assertEqual((self.base/'launch-xochitl.sh').read_text(),'previous-launcher')
        self.assertFalse((self.units/'xochitl.service.d/50-quickshare.conf').exists())
        self.assert_owner_data_untouched()

    def test_unknown_firmware_fails_before_service_changes(self):
        result=self.install(FAIL_PREPARE='1')
        self.assertNotEqual(result.returncode,0)
        log=self.root/'systemctl.log'
        self.assertFalse(log.exists())
        self.assert_owner_data_untouched()

    def test_corrupt_bundle_fails_before_service_changes(self):
        (self.payload/'VERSION').write_text('tampered')
        result=self.install();self.assertNotEqual(result.returncode,0)
        self.assertIn('integrity check failed',result.stderr)
        self.assertFalse((self.root/'systemctl.log').exists())
        self.assert_owner_data_untouched()


class DistributionTests(unittest.TestCase):
    def test_protocol_licenses_and_original_attribution_retained(self):
        self.assertIn('June Philip',(ROOT/'kindshare/LICENSE').read_text())
        self.assertIn('Lorenzo Garrone',(ROOT/'LICENSE').read_text())
        for file in (ROOT/'kindshare/pb').rglob('*.go'):
            self.assertIn('Licensed under the Apache License, Version 2.0',file.read_text())
        modules=json.loads((ROOT/'licenses/go-modules.json').read_text())
        self.assertTrue(modules)
        for module in modules:
            for notice in module['notices']:self.assertTrue((ROOT/notice).is_file(),notice)

    def test_release_bundle_contains_notices_and_no_firmware(self):
        archive=ROOT/'release/remarkable-quickshare-linux-armv7.tar.gz'
        if not archive.exists():self.skipTest('release bundle is checked after packaging')
        with tarfile.open(archive) as tar:
            names=tar.getnames()
            self.assertIn('LICENSE',names);self.assertIn('THIRD_PARTY_NOTICES.md',names);self.assertIn('licenses/Apache-2.0.txt',names)
            self.assertFalse(any('xochitl-local-import' in name or 'backups/' in name or name.endswith('.original') for name in names))
            self.assertFalse(any(item.issym() or item.islnk() or name.startswith('/') or '..' in Path(name).parts for name,item in ((m.name,m) for m in tar.getmembers())))
            source=json.load(tar.extractfile('SOURCE.json'));self.assertFalse(source['proprietary_firmware_included'])
            checksums=tar.extractfile('PAYLOAD.sha256').read().decode()
            for line in checksums.splitlines():
                expected,name=line.split('  ',1)
                self.assertEqual(hashlib.sha256(tar.extractfile(name).read()).hexdigest(),expected,name)


if __name__=='__main__':unittest.main()
