#!/usr/bin/env python3
"""Give a retried deployment job its own immutable release ID.

Re-running only a failed deploy job reuses the successful build artifact. The
artifact's original run attempt must not collide with the previous transaction.
"""
import io
import json
import os
from pathlib import Path
import re
import sys
import tarfile


def retag(source, target, run_id, attempt):
    if not run_id.isdecimal() or not attempt.isdecimal():
        raise ValueError('Expected numeric GitHub run and attempt')
    with tarfile.open(source, 'r:gz') as archive:
        manifest = json.load(archive.extractfile('manifest.json'))
        match = re.fullmatch(r'ci-(backend|web|h5)-(\d+)-(\d+)-([0-9a-f]{12})', manifest['release'])
        if not match or match[2] != run_id:
            raise ValueError('Artifact does not belong to this workflow run')
        release = 'ci-%s-%s-%s-%s' % (match[1], run_id, attempt, match[4])
        if release == manifest['release']:
            return source
        manifest['release'] = release
        data = json.dumps(manifest, sort_keys=True).encode()
        with tarfile.open(target, 'w:gz') as output:
            for member in archive:
                if member.name == 'manifest.json':
                    member.size = len(data)
                    output.addfile(member, io.BytesIO(data))
                else:
                    output.addfile(member, archive.extractfile(member) if member.isfile() else None)
    return target


if __name__ == '__main__':
    print(retag(Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve(),
                os.environ['GITHUB_RUN_ID'], os.environ['GITHUB_RUN_ATTEMPT']))
