"""Installer transaction tests use an isolated fake tablet; no real systemd is called."""
import hashlib
import io
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
        self.runtime_units = self.root/'run/systemd/system'
        self.runtime_units.mkdir(parents=True)
        self.boot_id = self.root/'boot_id'
        self.boot_id.write_text('fixture-boot-id\n')
        self.global_xovi = self.root/'home/xovi'
        self.xovi = self.root/'home/xovi/services/xochitl.service'
        self.qrr = self.global_xovi/'exthome/qt-resource-rebuilder'
        self.conf = self.root/'home/.config/remarkable/xochitl.conf'
        self.original = self.root/'usr/bin/xochitl'
        for path in [self.units,self.xovi,self.qrr,self.global_xovi/'extensions.d',self.conf.parent,self.original.parent,self.base/'bin',self.base/'inbox']:
            path.mkdir(parents=True,exist_ok=True)
        (self.global_xovi/'xovi.so').write_text('existing owner XOVI')
        (self.global_xovi/'extensions.d/qt-resource-rebuilder.so').write_text('existing owner QRR')
        (self.global_xovi/'extensions.d/keep.so').write_text('unrelated extension')
        (self.qrr/'hashtab').write_text('existing firmware table')
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
            'date':'''#!/bin/sh
if [ "$*" = +%s ]; then
    value=1000
    [ ! -f "$TEST_ROOT/clock" ] || value=$(cat "$TEST_ROOT/clock")
    value=$((value+5))
    printf '%s\\n' "$value" > "$TEST_ROOT/clock"
    printf '%s\\n' "$value"
else /bin/date "$@"; fi
''',
            'systemctl':'''#!/bin/sh
printf "%s\\n" "$*" >> "$TEST_ROOT/systemctl.log"
case "$1" in
  show)
    case "$*" in
      *'Restart --value')
        [ "${FAIL_GUARD:-0}" = 0 ] || { echo on-failure; exit 0; }
        for guard in "$TEST_ROOT"/run/systemd/system/xochitl.service.d/zzzz-remarkable-quickshare-validation-*.conf; do
            if [ -f "$guard" ] && grep -q '^Restart=no$' "$guard"; then echo no; exit 0; fi
        done
        echo on-failure ;;
      *'ActiveState --value') if [ -f "$TEST_ROOT/xochitl-crashed" ]; then echo failed; else echo active; fi ;;
    esac ;;
  restart)
    if [ "$2" = xochitl ]; then
        found=0
        for guard in "$TEST_ROOT"/run/systemd/system/xochitl.service.d/zzzz-remarkable-quickshare-validation-*.conf; do
            if [ -f "$guard" ] && grep -q '^Restart=no$' "$guard"; then found=1; fi
        done
        [ "$found" = 1 ] || { echo 'Validation guard was missing at candidate startup' >&2; exit 1; }
        [ "${CRASH_XOCHITL:-0}" = 0 ] || : > "$TEST_ROOT/xochitl-crashed"
    fi ;;
  reset-failed) rm -f "$TEST_ROOT/xochitl-crashed" ;;
  is-active|is-enabled) exit 0 ;;
esac
''',
        }
        for name,content in commands.items():self.executable(self.fakebin/name,content)
        self.payload=self.root/'payload';(self.payload/'device').mkdir(parents=True);(self.payload/'bin').mkdir();(self.payload/'licenses').mkdir()
        mapping={
            '/home/root/quickshare':str(self.base),
            '/etc/systemd/system':str(self.units),
            '/run/systemd/system':str(self.runtime_units),
            '/home/root/xovi':str(self.global_xovi),
            '/home/root/.config/remarkable/xochitl.conf':str(self.conf),
            '/usr/bin/xochitl':str(self.original),
            '/proc/sys/kernel/random/boot_id':str(self.boot_id),
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
  -init-control)
    [ -f "$3" ] || printf '{"acceptance_mode":"ask"}\\n' > "$3"
    [ -f "$5" ] || printf 'fixture_control_token_1234567890\\n' > "$5"
    ;;
  -check-control-api) [ "${FAIL_UI:-0}" = 0 ] ;;
  *) exit 0 ;;
esac
''')
        for name in ['LICENSE','THIRD_PARTY_NOTICES.md']:(self.payload/name).write_text('fixture license')
        (self.payload/'licenses/notice.txt').write_text('fixture dependency notice')
        (self.payload/'VERSION').write_text('v0.1.0\n')
        (self.payload/'ui').mkdir()
        (self.payload/'ui/QuickShare.qml').write_text('original test UI')
        (self.payload/'ui/quickshare.qmd').write_text(str(self.base)+'/ui/QuickShare.qml\n')
        (self.payload/'dependencies').mkdir()
        archive=self.payload/'dependencies/xovi-arm32.tar.gz'
        with tarfile.open(archive,'w:gz') as tar:
            for name in ['xovi/xovi.so','xovi/extensions.d/qt-resource-rebuilder.so']:
                data=b'fixture runtime'
                info=tarfile.TarInfo(name);info.size=len(data)
                tar.addfile(info,io.BytesIO(data))
        (self.payload/'device/dependencies.env').write_text('XOVI_SHA256='+hashlib.sha256(archive.read_bytes()).hexdigest()+'\n')
        self.checksums()
        self.env=os.environ.copy();self.env.update(PATH=str(self.fakebin)+os.pathsep+self.env['PATH'],TEST_ROOT=str(self.root))

    @staticmethod
    def executable(path,content):
        path.write_text(content);path.chmod(0o755)

    def checksums(self):
        lines=[]
        for file in sorted(self.payload.rglob('*')):
            if file.is_file() and file.name!='PAYLOAD.sha256' and 'dependencies' not in file.relative_to(self.payload).parts:
                lines.append(hashlib.sha256(file.read_bytes()).hexdigest()+'  '+file.relative_to(self.payload).as_posix()+'\n')
        (self.payload/'PAYLOAD.sha256').write_text(''.join(lines))

    def install(self,**flags):
        return subprocess.run(['sh',str(self.payload/'device/install-device.sh')],env=dict(self.env,**flags),text=True,capture_output=True)

    def assert_owner_data_untouched(self):
        self.assertEqual((self.base/'identity').read_text(),'name=Existing tablet\nid=KEEP\n')
        self.assertEqual((self.base/'inbox/keep.pdf').read_bytes(),b'pending-owner-document')
        self.assertEqual(self.original.read_text(),'original-firmware')
        self.assertIn('UnrelatedSetting=preserve',self.conf.read_text())
        if self.global_xovi.exists():
            self.assertEqual((self.global_xovi/'xovi.so').read_text(),'existing owner XOVI')
            self.assertEqual((self.global_xovi/'extensions.d/keep.so').read_text(),'unrelated extension')

    def test_update_and_uninstall_preserve_data(self):
        result=self.install();self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertEqual((self.base/'VERSION').read_text(),'v0.1.0\n')
        self.assertIn('Prepared', (self.base/'bin/xochitl-local-import').read_text().title())
        self.assertTrue((self.xovi/'50-quickshare.conf').exists())
        self.assertEqual((self.qrr/'quickshare.qmd').read_text(),str(self.base)+'/ui/QuickShare.qml\n')
        self.assertIn('fixture_control_token', (self.base/'ui/Config.js').read_text())
        self.assertFalse((self.base/'ui-validation.pending').exists())
        self.assertFalse(list(self.runtime_units.rglob('zzzz-remarkable-quickshare-validation-*.conf')))
        self.assert_owner_data_untouched()
        result=subprocess.run(['sh',str(self.base/'uninstall.sh')],env=self.env,text=True,capture_output=True)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertFalse((self.xovi/'50-quickshare.conf').exists())
        self.assertFalse((self.units/'remarkable-quickshare.service').exists())
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assert_owner_data_untouched()

    def test_crashed_candidate_is_not_restarted_and_runtime_guard_is_removed(self):
        unrelated=self.runtime_units/'xochitl.service.d/owner-custom.conf'
        unrelated.parent.mkdir(parents=True,exist_ok=True)
        unrelated.write_text('[Service]\nEnvironment=OWNER=preserve\n')
        result=self.install(CRASH_XOCHITL='1')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Xochitl stopped during candidate UI validation',result.stderr)
        log=(self.root/'systemctl.log').read_text().splitlines()
        self.assertEqual(log.count('restart xochitl'),1)
        self.assertEqual(log[-1],'start xochitl')
        self.assertIn('reset-failed xochitl',log)
        self.assertFalse(list(self.runtime_units.rglob('zzzz-remarkable-quickshare-validation-*.conf')))
        self.assertEqual(unrelated.read_text(),'[Service]\nEnvironment=OWNER=preserve\n')
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assertFalse((self.base/'ui-validation.pending').exists())
        self.assert_owner_data_untouched()

    def test_uninstall_removes_only_our_stale_validation_guards(self):
        result=self.install();self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        guards=self.runtime_units/'xochitl.service.d'
        owned=guards/'zzzz-remarkable-quickshare-validation-stale.conf'
        owned.write_text('# remarkable-quickshare candidate UI validation\n[Service]\nRestart=no\n')
        unrelated=guards/'zzzz-remarkable-quickshare-validation-owner.conf'
        unrelated.write_text('[Service]\nEnvironment=OWNER=preserve\n')
        result=subprocess.run(['sh',str(self.base/'uninstall.sh')],env=self.env,text=True,capture_output=True)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertFalse(owned.exists())
        self.assertEqual(unrelated.read_text(),'[Service]\nEnvironment=OWNER=preserve\n')
        self.assertIn('reset-failed xochitl',(self.root/'systemctl.log').read_text())
        self.assert_owner_data_untouched()

    def test_unverified_restart_guard_aborts_before_candidate_changes(self):
        result=self.install(FAIL_GUARD='1')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('Could not disable automatic Xochitl restarts',result.stderr)
        self.assertEqual((self.base/'bin/kindshare').read_text(),'previous-receiver')
        self.assertFalse(list(self.runtime_units.rglob('zzzz-remarkable-quickshare-validation-*.conf')))
        self.assert_owner_data_untouched()

    def test_failed_health_check_rolls_back(self):
        result=self.install(FAIL_HEALTH='1')
        self.assertNotEqual(result.returncode,0)
        self.assertIn('restoring the previous',result.stderr)
        self.assertEqual((self.base/'bin/kindshare').read_text(),'previous-receiver')
        self.assertEqual((self.base/'bin/xochitl-local-import').read_text(),'previous-prepared-image')
        self.assertEqual((self.base/'launch-xochitl.sh').read_text(),'previous-launcher')
        self.assertFalse((self.units/'xochitl.service.d/50-quickshare.conf').exists())
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        log=(self.root/'systemctl.log').read_text()
        self.assertIn('disable remarkable-quickshare.service',log)
        self.assertNotIn('start remarkable-quickshare.service',log.splitlines())
        self.assert_owner_data_untouched()

    def test_confirmation_ui_failure_rolls_back_and_never_enables_legacy_autoaccept(self):
        result=self.install(FAIL_UI='1')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual((self.base/'bin/kindshare').read_text(),'previous-receiver')
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assertFalse((self.base/'ui-runtime.conf').exists())
        self.assertIn('Receiver left disabled',result.stderr)
        checks=(self.root/'systemctl.log').read_text().count('show xochitl -p ActiveState --value')
        self.assertLessEqual(checks,9)
        self.assert_owner_data_untouched()

    def test_settings_and_token_survive_update(self):
        (self.base/'settings.json').write_text('{"acceptance_mode":"auto"}\n')
        (self.base/'control-token').write_text('existing_control_token\n')
        result=self.install();self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual((self.base/'settings.json').read_text(),'{"acceptance_mode":"auto"}\n')
        self.assertEqual((self.base/'control-token').read_text(),'existing_control_token\n')

    def test_existing_quickshare_ui_is_restored_after_failed_update(self):
        old=str(self.base)+'/ui/previous-ui.qml\n'
        (self.qrr/'quickshare.qmd').write_text(old)
        (self.base/'ui').mkdir()
        (self.base/'ui/QuickShare.qml').write_text('previous UI')
        result=self.install(FAIL_UI='1')
        self.assertNotEqual(result.returncode,0)
        self.assertEqual((self.qrr/'quickshare.qmd').read_text(),old)
        self.assertEqual((self.base/'ui/QuickShare.qml').read_text(),'previous UI')
        self.assert_owner_data_untouched()

    def test_missing_dependency_on_stock_tablet_is_rejected_before_service_changes(self):
        shutil.rmtree(self.global_xovi)
        (self.payload/'dependencies/xovi-arm32.tar.gz').unlink()
        result=self.install()
        self.assertNotEqual(result.returncode,0)
        self.assertIn('dependency is missing',result.stderr)
        self.assertFalse((self.root/'systemctl.log').exists())
        self.assert_owner_data_untouched()

    def test_bad_dependency_on_stock_tablet_is_rejected_before_service_changes(self):
        shutil.rmtree(self.global_xovi)
        (self.payload/'dependencies/xovi-arm32.tar.gz').write_bytes(b'bad dependency')
        result=self.install()
        self.assertNotEqual(result.returncode,0)
        self.assertIn('dependency checksum mismatch',result.stderr)
        self.assertFalse((self.root/'systemctl.log').exists())
        self.assert_owner_data_untouched()

    def test_stock_tablet_gets_owned_runtime_and_automatic_hashtab(self):
        shutil.rmtree(self.global_xovi)
        self.executable(self.original, '''#!/bin/sh
if [ -n "${QMLDIFF_HASHTAB_CREATE:-}" ]; then
    printf 'generated table' > "$QMLDIFF_HASHTAB_CREATE"
    printf '[qmldiff]: Hashtab saved to %s\\n' "$QMLDIFF_HASHTAB_CREATE"
fi
''')
        result=self.install()
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        owned=self.base/'runtime/xovi'
        self.assertTrue((owned/'xovi.so').is_file())
        self.assertTrue((owned/'exthome/qt-resource-rebuilder/quickshare.qmd').is_file())
        self.assertIn('kind=owned',(self.base/'ui-runtime.conf').read_text())
        self.assertFalse(self.global_xovi.exists())

    def test_new_owned_runtime_is_removed_after_failed_ui_check(self):
        shutil.rmtree(self.global_xovi)
        self.executable(self.original, '''#!/bin/sh
printf 'generated table' > "$QMLDIFF_HASHTAB_CREATE"
printf '[qmldiff]: Hashtab saved to %s\\n' "$QMLDIFF_HASHTAB_CREATE"
''')
        result=self.install(FAIL_UI='1')
        self.assertNotEqual(result.returncode,0)
        self.assertFalse((self.base/'runtime/xovi').exists())
        self.assertFalse(self.global_xovi.exists())

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

    def run_launcher(self, config, *, corrupt_copy=False, config_failure=False):
        self.conf.write_text(config)
        self.executable(self.original, '#!/bin/sh\necho stock-ui\n')
        self.executable(self.base/'bin/xochitl-local-import', '#!/bin/sh\necho prepared-ui\n')
        self.executable(self.fakebin/'logger', '#!/bin/sh\nexit 0\n')
        self.executable(self.fakebin/'ip', '#!/bin/sh\nexit 1\n')
        self.executable(self.fakebin/'sha256sum', '''#!/bin/sh
case "$1" in
  */usr/bin/xochitl) echo '87b574fc7ac1a15d502e96458a94a5279a189023bd476758e4aaca9bae13575a  original' ;;
  *) echo "${COPY_HASH:-72deb4483772142ab12d08d12f1d25dbd4e7b1f449a9ead63df103f0ac612fe4}  prepared" ;;
esac
''')
        if config_failure:
            self.executable(self.fakebin/'sed', '#!/bin/sh\nexit 1\n')
        env=dict(self.env)
        if corrupt_copy:env['COPY_HASH']='invalid'
        return subprocess.run(['sh',str(self.payload/'device/launch-xochitl.sh')],env=env,text=True,capture_output=True)

    def test_launcher_handles_absent_default_and_missing_usb_interface(self):
        result=self.run_launcher('[General]\nUnrelatedSetting=preserve\n')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'prepared-ui')
        self.assertIn('WebInterfaceEnabled=true',self.conf.read_text())
        self.assertIn('UnrelatedSetting=preserve',self.conf.read_text())

    def test_launcher_uses_stock_ui_if_copy_is_corrupt(self):
        result=self.run_launcher('[General]\nWebInterfaceEnabled=false\n',corrupt_copy=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'stock-ui')

    def test_launcher_uses_stock_ui_if_configuration_cannot_be_saved(self):
        result=self.run_launcher('[General]\nWebInterfaceEnabled=false\n',config_failure=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'stock-ui')

    def stage_interrupted_candidate(self, *, attempted=True, another_boot=False):
        transaction=self.base/'backups/install-20261020T120000Z-123'
        transaction.mkdir(parents=True)
        (transaction/'ui-qrr.path').write_text(str(self.qrr)+'\n')
        (transaction/'ui-qmd.absent').touch()
        (transaction/'ui.absent').touch()
        (transaction/'ui-runtime.conf.absent').touch()
        (transaction/'ui-validation.boot-id').write_text('previous-boot\n' if another_boot else self.boot_id.read_text())
        (self.base/'ui-validation.pending').write_text(str(transaction)+'\n')
        if attempted:(self.base/'ui-validation.attempted').write_text('attempted\n')
        (self.qrr/'quickshare.qmd').write_text(str(self.base)+'/ui/broken-candidate.qml\n')
        shutil.copy(self.payload/'device/install-ui.sh',self.base/'install-ui.sh')
        return transaction

    def test_second_launcher_attempt_recovers_candidate_before_firmware_fallback(self):
        transaction=self.stage_interrupted_candidate()
        result=self.run_launcher('[General]\nWebInterfaceEnabled=false\n',corrupt_copy=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'stock-ui')
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assertFalse((self.base/'ui-validation.pending').exists())
        self.assertEqual((self.base/'ui-validation.failed').read_text(),str(transaction)+'\n')
        self.assertEqual((self.global_xovi/'extensions.d/keep.so').read_text(),'unrelated extension')

    def test_reboot_recovers_unattempted_interrupted_candidate(self):
        self.stage_interrupted_candidate(attempted=False,another_boot=True)
        result=self.run_launcher('[General]\nWebInterfaceEnabled=false\n')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'prepared-ui')
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assertFalse((self.base/'ui-validation.pending').exists())

    def test_candidate_gets_only_one_launcher_attempt_until_health_confirms(self):
        self.stage_interrupted_candidate(attempted=False)
        first=self.run_launcher('[General]\nWebInterfaceEnabled=false\n')
        self.assertEqual(first.returncode,0,first.stderr)
        self.assertTrue((self.qrr/'quickshare.qmd').exists())
        self.assertTrue((self.base/'ui-validation.attempted').exists())
        second=self.run_launcher('[General]\nWebInterfaceEnabled=false\n')
        self.assertEqual(second.returncode,0,second.stderr)
        self.assertFalse((self.qrr/'quickshare.qmd').exists())
        self.assertFalse((self.base/'ui-validation.pending').exists())


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


@unittest.skipIf(os.name == 'nt', 'POSIX host transport is checked in Linux CI')
class HostTransportTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='quickshare-host-test-')
        self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        self.fakebin=self.root/'fakebin';self.fakebin.mkdir()
        self.fixture=self.root/'fixtures';self.fixture.mkdir()
        self.dependency=bytes(range(256))*16
        (self.fixture/'xovi-arm32.tar.gz').write_bytes(self.dependency)
        manifest=('XOVI_VERSION=fixture\nXOVI_URL=https://github.com/asivery/rm-xovi-extensions/releases/download/fixture/xovi-arm32.tar.gz\nXOVI_SHA256='+hashlib.sha256(self.dependency).hexdigest()+'\n').encode()
        with tarfile.open(self.fixture/'remarkable-quickshare-linux-armv7.tar.gz','w:gz') as tar:
            info=tarfile.TarInfo('device/dependencies.env');info.size=len(manifest)
            tar.addfile(info,io.BytesIO(manifest))
        self.receiver=(self.fixture/'remarkable-quickshare-linux-armv7.tar.gz').read_bytes()
        (self.fixture/'SHA256SUMS').write_text(hashlib.sha256(self.receiver).hexdigest()+'  remarkable-quickshare-linux-armv7.tar.gz\n')
        self.executable(self.fakebin/'curl', '''#!/usr/bin/env python3
import os,sys
from pathlib import Path
args=sys.argv[1:]
url=next(arg for arg in args if arg.startswith('https://'))
output=Path(args[args.index('-o')+1])
name=url.rsplit('/',1)[-1]
output.write_bytes((Path(os.environ['TEST_ROOT'])/'fixtures'/name).read_bytes())
''')
        self.executable(self.fakebin/'ssh', '''#!/usr/bin/env python3
import json,os,sys
from pathlib import Path
root=Path(os.environ['TEST_ROOT'])
(root/'ssh-args.json').write_text(json.dumps(sys.argv[1:]))
(root/'received.tar').write_bytes(sys.stdin.buffer.read())
''')
        self.script=self.root/'install.sh';self.script.write_text((ROOT/'install.sh').read_text())
        self.env=os.environ.copy();self.env.update(PATH=str(self.fakebin)+os.pathsep+self.env['PATH'],TEST_ROOT=str(self.root),QUICKSHARE_VERSION='v0.2.0')

    @staticmethod
    def executable(path,content):
        path.write_text(content);path.chmod(0o755)

    def test_verified_receiver_and_dependency_share_one_binary_ssh_stream(self):
        result=subprocess.run(['sh',str(self.script),'192.0.2.1'],env=self.env,text=True,capture_output=True)
        self.assertEqual(result.returncode,0,result.stderr)
        with tarfile.open(self.root/'received.tar') as tar:
            self.assertEqual(set(tar.getnames()),{'receiver.tar.gz','dependencies/xovi-arm32.tar.gz'})
            self.assertEqual(tar.extractfile('receiver.tar.gz').read(),self.receiver)
            self.assertEqual(tar.extractfile('dependencies/xovi-arm32.tar.gz').read(),self.dependency)
        args=json.loads((self.root/'ssh-args.json').read_text())
        self.assertEqual(args[:5],['-p','22','-o','StrictHostKeyChecking=ask','root@192.0.2.1'])
        self.assertIn('tar -xf - -C "$d"',args[5])

    def test_corrupt_dependency_never_starts_ssh(self):
        (self.fixture/'xovi-arm32.tar.gz').write_bytes(b'corrupt dependency')
        result=subprocess.run(['sh',str(self.script),'192.0.2.1'],env=self.env,text=True,capture_output=True)
        self.assertNotEqual(result.returncode,0)
        self.assertIn('XOVI checksum mismatch',result.stderr)
        self.assertFalse((self.root/'ssh-args.json').exists())


if __name__=='__main__':unittest.main()
