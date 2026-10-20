#!/usr/bin/env python3
"""Build a redistributable ARMv7 bundle. Never include a tablet's Xochitl binary."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parents[1]

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--version',required=True)
    parser.add_argument('--go',default='go')
    parser.add_argument('--output',default=str(ROOT/'release'))
    args=parser.parse_args()
    if not re.fullmatch(r'v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?',args.version):
        raise SystemExit('Invalid version')
    output=Path(args.output).resolve();output.mkdir(parents=True,exist_ok=True)
    subprocess.run([args.go,'test','./...'],cwd=ROOT/'kindshare',check=True)
    subprocess.run([args.go,'vet','./...'],cwd=ROOT/'kindshare',check=True)
    subprocess.run([os.sys.executable,str(ROOT/'scripts/collect_licenses.py'),'--go',args.go,'--check'],check=True)
    binary=output/'kindshare'
    env=os.environ.copy();env.update(GOOS='linux',GOARCH='arm',GOARM='7',CGO_ENABLED='0',GOTOOLCHAIN='local')
    subprocess.run([args.go,'build','-buildvcs=false','-trimpath',f'-ldflags=-s -w -X main.buildVersion={args.version}','-o',str(binary),'.'],cwd=ROOT/'kindshare',env=env,check=True)
    files={'bin/kindshare':binary.read_bytes(),'VERSION':(args.version+'\n').encode()}
    for name in ['LICENSE','THIRD_PARTY_NOTICES.md']:
        files[name]=(ROOT/name).read_bytes()
    for path in (ROOT/'licenses').rglob('*'):
        if path.is_file():files[path.relative_to(ROOT).as_posix()]=path.read_bytes()
    for name in ['install-device.sh','install-ui.sh','uninstall.sh','launch-xochitl.sh','50-quickshare.conf','remarkable-quickshare.service','dependencies.env']:
        files['device/'+name]=(ROOT/'device'/name).read_bytes().replace(b'\r\n',b'\n')
    for path in (ROOT/'ui').rglob('*'):
        if path.is_file():
            if path.name == 'Config.js':
                raise SystemExit('Refusing to package private generated UI credentials')
            files[path.relative_to(ROOT).as_posix()]=path.read_bytes().replace(b'\r\n',b'\n')
    try:
        commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True,stderr=subprocess.DEVNULL).strip()
        dirty=bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True).strip())
    except subprocess.CalledProcessError:
        commit='uncommitted';dirty=True
    files['SOURCE.json']=(json.dumps({'repository':'https://github.com/noire342/remarkable-quickshare','version':args.version,'commit':commit,'working_tree_dirty':dirty,'upstream_kindshare_commit':'25800e334961bfc7e7468a85b638fc16d7a12bc5','go_version':subprocess.check_output([args.go,'version'],text=True).strip(),'proprietary_firmware_included':False},indent=2)+'\n').encode()
    files['PAYLOAD.sha256']=''.join(hashlib.sha256(content).hexdigest()+'  '+name+'\n' for name,content in sorted(files.items())).encode()
    # Explicit allowlist above: host backups, credentials, identities and firmware are unreachable.
    for name,content in files.items():
        if 'xochitl-local-import' in name or name.endswith('.original'):
            raise SystemExit('Refusing to package a proprietary firmware image')
        if name!='bin/kindshare' and content.startswith(b'\x7fELF'):
            raise SystemExit('Unexpected executable in release: '+name)
    archive=output/'remarkable-quickshare-linux-armv7.tar.gz'
    with archive.open('wb') as raw, gzip.GzipFile(filename='',fileobj=raw,mode='wb',mtime=0) as zipped, tarfile.open(fileobj=zipped,mode='w') as tar:
        for name,content in sorted(files.items()):
            info=tarfile.TarInfo(name);info.size=len(content);info.mtime=0;info.uid=0;info.gid=0
            info.mode=0o755 if name=='bin/kindshare' or name.endswith('.sh') else 0o644
            tar.addfile(info,io.BytesIO(content))
    for name in ['install.ps1','install.sh']:
        (output/name).write_bytes((ROOT/name).read_bytes().replace(b'\r\n',b'\n'))
    assets=[archive,output/'install.ps1',output/'install.sh']
    (output/'SHA256SUMS').write_text(''.join(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+path.name+'\n' for path in assets),encoding='utf-8')
    print('Built',archive)
    print('Receiver SHA256:',hashlib.sha256(files['bin/kindshare']).hexdigest())

if __name__=='__main__':main()
