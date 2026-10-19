#!/usr/bin/env python3
"""Verify an immutable published release against a fresh build of the same tag."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import tarfile

ASSETS = {'remarkable-quickshare-linux-armv7.tar.gz', 'install.ps1', 'install.sh'}


def manifest(text):
    result = {}
    for line in text.splitlines():
        digest, name = line.split('  ', 1)
        if name in result or len(digest) != 64:
            raise ValueError('Invalid or duplicate checksum entry')
        result[name] = digest
    return result


def read_release(directory):
    checksums = manifest((directory/'SHA256SUMS').read_text())
    if set(checksums) != ASSETS:
        raise ValueError('Unexpected release asset list')
    for name, expected in checksums.items():
        if hashlib.sha256((directory/name).read_bytes()).hexdigest() != expected:
            raise ValueError('Release asset checksum mismatch: '+name)
    files = {}
    with tarfile.open(directory/'remarkable-quickshare-linux-armv7.tar.gz') as tar:
        for member in tar.getmembers():
            path = PurePosixPath(member.name)
            if not member.isfile() or path.is_absolute() or '..' in path.parts or member.name in files or member.size > 100_000_000:
                raise ValueError('Invalid bundle member: '+member.name)
            files[member.name] = tar.extractfile(member).read()
    payload = manifest(files.pop('PAYLOAD.sha256').decode())
    if set(payload) != set(files):
        raise ValueError('Unexpected payload file list')
    for name, expected in payload.items():
        if hashlib.sha256(files[name]).hexdigest() != expected:
            raise ValueError('Payload checksum mismatch: '+name)
    source = json.loads(files.pop('SOURCE.json'))
    # Go's reported build host can differ. Receiver bytes, source commit, version,
    # firmware recipe, scripts and every notice must still match exactly.
    version = source.pop('go_version').split()
    if len(version) != 4 or version[:2] != ['go', 'version']:
        raise ValueError('Invalid Go toolchain provenance')
    source['go_toolchain'] = version[2]
    return files, source


def verify(expected, published):
    expected_files, expected_source = read_release(expected)
    published_files, published_source = read_release(published)
    if expected_source != published_source:
        raise ValueError('Published source provenance differs from the requested tag')
    if expected_files != published_files:
        raise ValueError('Published executable, scripts or notices differ from the tag build')
    for name in ['install.ps1', 'install.sh']:
        if (expected/name).read_bytes() != (published/name).read_bytes():
            raise ValueError('Published host installer differs: '+name)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('expected', type=Path)
    parser.add_argument('published', type=Path)
    args = parser.parse_args()
    verify(args.expected, args.published)
    print('Existing release verified; published assets preserved unchanged.')


if __name__ == '__main__':
    main()
