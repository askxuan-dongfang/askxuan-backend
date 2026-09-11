import io
import hashlib
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import ecs_receiver as receiver

A, B = 'a' * 40, 'b' * 40


class ReceiverTests(unittest.TestCase):
    def archive(self, base, members):
        path = base / 'archive.tgz'
        with tarfile.open(path, 'w:gz') as tar:
            for name, content, kind in members:
                info = tarfile.TarInfo(name)
                info.type = kind
                info.size = len(content) if kind == tarfile.REGTYPE else 0
                info.linkname = '/etc/passwd' if kind == tarfile.SYMTYPE else ''
                tar.addfile(info, io.BytesIO(content) if info.size else None)
        return path

    def test_rejects_traversal_links_and_duplicate_files(self):
        for members in [
            [('../escaped', b'bad', tarfile.REGTYPE)],
            [('payload/h5/link', b'', tarfile.SYMTYPE)],
            [('manifest.json', b'{}', tarfile.REGTYPE)] * 2,
        ]:
            with self.subTest(members=members), tempfile.TemporaryDirectory() as tmp:
                base = Path(tmp)
                with self.assertRaises(ValueError):
                    receiver.extract(self.archive(base, members), base / 'out', 'h5')
                self.assertFalse((base.parent / 'escaped').exists())

    def test_stale_related_frontend_and_h5_are_both_rejected(self):
        state = {'components': {'web/h5': {'sources': {'frontend': B, 'h5': B}}}}
        manifest = {'scope': 'h5', 'components': {'h5': {'sources': {'frontend': A, 'h5': B}}},
                    'history': {'frontend': [A], 'h5': [B, A]}}
        with self.assertRaisesRegex(ValueError, 'Stale'):
            receiver.validate_versions(manifest, state)

        manifest['components']['h5']['sources']['frontend'] = B
        manifest['history']['frontend'] = [B, A]
        receiver.validate_versions(manifest, state)
        manifest['components']['h5']['sources']['h5'] = A
        manifest['history']['h5'] = [A]
        with self.assertRaisesRegex(ValueError, 'Stale'):
            receiver.validate_versions(manifest, state)

    def test_checksum_and_scope_are_checked_before_deploy(self):
        manifest = {'schema': 1, 'scope': 'h5', 'release': 'ci-h5-test-1',
                    'components': {'h5': {}}, 'files': {'payload/h5/index.html': hashlib.sha256(b'good').hexdigest()}}
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            archive = self.archive(base, [('manifest.json', json.dumps(manifest).encode(), tarfile.REGTYPE),
                                          ('payload/h5/index.html', b'tampered', tarfile.REGTYPE)])
            with self.assertRaisesRegex(ValueError, 'checksum'):
                receiver.extract(archive, base / 'first', 'h5')
            with self.assertRaisesRegex(ValueError, 'scope'):
                receiver.extract(archive, base / 'second', 'web')

    def web_fixture(self, base):
        releases = base / 'web'
        old = releases / 'old/public'
        (old / 'admin').mkdir(parents=True)
        (old / 'admin/index.html').write_text('admin old')
        (old / 'index.html').write_text('h5 old')
        public = base / 'public'
        public.symlink_to(old)
        candidate = base / 'candidate'
        (candidate / 'payload/h5').mkdir(parents=True)
        (candidate / 'payload/h5/index.html').write_text('h5 new')
        return releases, old, public, candidate

    def test_h5_deploy_preserves_admin_and_immutable_previous_release(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            releases, old, public, candidate = self.web_fixture(base)
            manifest = {'release': 'ci-h5-test-1', 'scope': 'h5', 'components': {'h5': {'sources': {'h5': B, 'frontend': B}}}}
            with patch.multiple(receiver, BASE=base, PUBLIC=public, RELEASES=releases), patch.object(receiver, 'run'), patch.object(receiver, 'smoke'), patch.object(receiver, 'smoke_web'):
                receiver.deploy(candidate, manifest, {})
            self.assertEqual((public / 'index.html').read_text(), 'h5 new')
            self.assertEqual((public / 'admin/index.html').read_text(), 'admin old')
            self.assertEqual((old / 'index.html').read_text(), 'h5 old')

    def test_failed_smoke_restores_public_and_provenance(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            releases, old, public, candidate = self.web_fixture(base)
            manifest = {'release': 'ci-h5-test-1', 'scope': 'h5', 'components': {'h5': {'sources': {'h5': B, 'frontend': B}}}}
            state = {'components': {'web/h5': {'sources': {'h5': A, 'frontend': A}}}}
            with patch.multiple(receiver, BASE=base, PUBLIC=public, RELEASES=releases), patch.object(receiver, 'run'), patch.object(receiver, 'smoke_web'), patch.object(receiver, 'smoke', side_effect=RuntimeError('smoke failed')):
                with self.assertRaises(RuntimeError):
                    receiver.deploy(candidate, manifest, state)
            self.assertEqual(public.resolve(), old.resolve())
            self.assertEqual(json.loads((base / 'state.json').read_text()), state)
            self.assertEqual(json.loads((candidate / 'transaction.json').read_text())['phase'], 'rolled-back')

    def test_failed_backend_health_restores_previous_compose(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            releases, old, public, candidate = self.web_fixture(base)
            (candidate / 'payload/diy').mkdir()
            (candidate / 'payload/diy/diy').write_bytes(b'binary')
            manifest = {'release': 'ci-backend-test-1', 'scope': 'backend', 'components': {'diy': {'sources': {'backend': B}, 'input': 'new'}}}
            inspect = [{'Image': 'sha256:old', 'Name': '/askxuan-diy-service', 'HostConfig': {}, 'Config': {}, 'Mounts': [], 'NetworkSettings': {}}]
            with patch.multiple(receiver, BASE=base, PUBLIC=public, RELEASES=releases), \
                 patch.object(receiver, 'run', return_value=json.dumps(inspect).encode()), \
                 patch.object(receiver, 'compose_definition', return_value=({'image': 'new'}, {}, {})), \
                 patch.object(receiver.subprocess, 'run'), patch.object(receiver, 'compose_up') as up, \
                 patch.object(receiver, 'healthy', side_effect=[RuntimeError('unhealthy'), None]):
                with self.assertRaises(RuntimeError):
                    receiver.deploy(candidate, manifest, {})
            self.assertEqual([c.args[0].name for c in up.call_args_list], ['compose.json', 'rollback.json'])
            self.assertEqual(json.loads((candidate / 'rollback.json').read_text())['services']['diy-service']['image'], 'sha256:old')

    def test_manual_rollback_refuses_to_overwrite_a_later_release(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            (base / 'state.json').write_text(json.dumps({'last_release': 'ci-web-later-2'}))
            with patch.object(receiver, 'BASE', base), self.assertRaisesRegex(ValueError, 'latest global'):
                receiver.rollback_latest('ci-h5-older-1')


if __name__ == '__main__':
    unittest.main()
