"""Exercise our QMD with upstream's parser, using original synthetic QML only."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
QMLDIFF = os.environ.get('QMLDIFF_BIN')


@unittest.skipUnless(QMLDIFF, 'Set QMLDIFF_BIN to the pinned upstream CLI')
class NativeSettingsPatchTests(unittest.TestCase):
    def run_cli(self, *args):
        result = subprocess.run([QMLDIFF, *map(str, args)], text=True,
                                capture_output=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout

    def test_settings_patch_preserves_existing_nodes(self):
        with tempfile.TemporaryDirectory() as folder:
            folder = Path(folder)
            source, output = folder/'source', folder/'output'
            fixtures = {
                'qml/device/global/Global.qml': 'QtObject { property int existingValue: 42 }',
                'qml/device/view/main/MainView.qml': 'Item { id: root; property bool existingValue: true }',
                'qml/device/view/settings/Settings.qml': '''Item {
                    id: settings
                    Item { id: settingsColumn; Repeater { id: settingsRepeater; delegate: Item {} }; Item { objectName: "existing-spacer" } }
                    Item { id: payloadLoader }
                }''',
            }
            for name, content in fixtures.items():
                path = source/name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content)
            table = folder/'hashtab'
            self.run_cli('create-hashtab', source, table, '--version', '3.27.3.0')
            self.run_cli('apply-diffs', '--hashtab', table, '--version', '3.27.3.0',
                         source, output, ROOT/'ui/quickshare.qmd')
            global_qml = (output/'qml/device/global/Global.qml').read_text()
            main_qml = (output/'qml/device/view/main/MainView.qml').read_text()
            settings_qml = (output/'qml/device/view/settings/Settings.qml').read_text()
            self.assertIn('existingValue', global_qml)
            self.assertIn('quickShareController', global_qml)
            self.assertIn('existingValue', main_qml)
            self.assertIn('quickShareHost', main_qml)
            self.assertIn('existing-spacer', settings_qml)
            self.assertIn('quickShareSettingsEntry', settings_qml)
            self.assertIn('quickShareSettingsPage', settings_qml)
            self.assertIn('SettingsPage.Wifi', settings_qml)
            self.assertIn('qrc:/ark/icons/import', settings_qml)

    def test_own_qml_parses_with_the_supported_resource_rebuilder(self):
        with tempfile.TemporaryDirectory() as folder:
            folder = Path(folder)
            changes = folder/'parse.qmd'
            changes.write_text('\n'.join(
                'AFFECT /ui/' + name + '\nTRAVERSE ' + root +
                '\nLOCATE AFTER ALL\nINSERT { property bool parserProbe: true }\nEND TRAVERSE\nEND AFFECT'
                for name, root in [('QuickShare.qml', 'Item'), ('QuickShareSettings.qml', 'Item'),
                                   ('ModeChoice.qml', 'Item'), ('ActionButton.qml', 'Button')]))
            self.run_cli('apply-diffs', ROOT, folder/'output', changes)


if __name__ == '__main__':
    unittest.main()
