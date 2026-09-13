import io
import hashlib
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import ecs_receiver as receiver
import release as builder
from retag_release import retag

A, B = 'a' * 40, 'b' * 40


class ReceiverTests(unittest.TestCase):
    def test_compose_preserves_container_shell_and_environment_dollars(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'compose.json'
            value = {'services': {'diy-service': {'command': ['/bin/sh', '-c', '/app/${BINARY} -f etc/${BINARY}.yaml'],
                                                 'environment': {'BINARY': 'diy', 'EXAMPLE': 'literal$VALUE${NAME:-default}'}}}}
            receiver.atomic_compose(path, value)
            stored = json.loads(path.read_text())['services']['diy-service']
            self.assertEqual(stored['command'][2], '/app/$${BINARY} -f etc/$${BINARY}.yaml')
            self.assertEqual(stored['environment']['EXAMPLE'], 'literal$$VALUE$${NAME:-default}')
            self.assertEqual(value['services']['diy-service']['environment']['EXAMPLE'], 'literal$VALUE${NAME:-default}')

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
        (old / 'shop/assets').mkdir(parents=True)
        (old / 'shop/index.html').write_text('shop old')
        (old / 'shop/assets/obsolete.js').write_text('old standalone bundle')
        (old / 'temple').mkdir()
        (old / 'temple/index.html').write_text('temple old')
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
            self.assertEqual((public / 'shop/index.html').read_text(), 'shop old')
            self.assertEqual((public / 'temple/index.html').read_text(), 'temple old')
            self.assertEqual((old / 'index.html').read_text(), 'h5 old')

    def unified_fixture(self, base):
        releases, old, public, candidate = self.web_fixture(base)
        for name in ('admin', 'temple'):
            (candidate / 'payload' / name).mkdir()
            (candidate / 'payload' / name / 'index.html').write_text(name + ' new')
        legacy = candidate / 'payload/admin/legacy/shop'
        legacy.mkdir(parents=True)
        (legacy / 'index.html').write_text('self-contained compatibility entry')
        manifest = {'release': 'ci-web-test-1', 'scope': 'web',
                    'components': {name: {'sources': {'frontend': B}} for name in ('admin', 'temple')}}
        state = {'components': {'web/' + name: {'sources': {'frontend': A}}
                                for name in ('admin', 'shop', 'temple')}}
        return releases, old, public, candidate, manifest, state

    def test_unified_deploy_replaces_standalone_shop_and_records_admin_ownership(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            releases, old, public, candidate, manifest, state = self.unified_fixture(base)
            with patch.multiple(receiver, BASE=base, PUBLIC=public, RELEASES=releases), \
                 patch.object(receiver, 'run'), patch.object(receiver, 'smoke'), patch.object(receiver, 'smoke_web'):
                receiver.deploy(candidate, manifest, state)
            self.assertEqual((public / 'index.html').read_text(), 'h5 old')
            self.assertEqual((public / 'temple/index.html').read_text(), 'temple new')
            self.assertEqual((public / 'shop/index.html').read_bytes(),
                             (public / 'admin/legacy/shop/index.html').read_bytes())
            self.assertEqual([p.name for p in (public / 'shop').iterdir()], ['index.html'])
            self.assertTrue((old / 'shop/assets/obsolete.js').is_file())
            stored = json.loads((base / 'state.json').read_text())
            self.assertNotIn('web/shop', stored['components'])
            self.assertEqual(stored['aliases']['web/shop']['owner'], 'web/admin')

    def test_unified_failed_smoke_restores_standalone_shop_and_state(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            releases, old, public, candidate, manifest, state = self.unified_fixture(base)
            with patch.multiple(receiver, BASE=base, PUBLIC=public, RELEASES=releases), \
                 patch.object(receiver, 'run'), patch.object(receiver, 'smoke'), \
                 patch.object(receiver, 'smoke_web', side_effect=RuntimeError('legacy route mismatch')):
                with self.assertRaisesRegex(RuntimeError, 'legacy route'):
                    receiver.deploy(candidate, manifest, state)
            self.assertEqual(public.resolve(), old.resolve())
            self.assertTrue((public / 'shop/assets/obsolete.js').is_file())
            self.assertEqual(json.loads((base / 'state.json').read_text()), state)
            self.assertEqual(json.loads((candidate / 'transaction.json').read_text())['phase'], 'rolled-back')

    def test_web_archive_requires_two_components_and_owned_legacy_entry(self):
        for names, include_legacy, expected in [
            (('admin', 'temple'), True, None),
            (('admin', 'temple'), False, 'legacy shop entry'),
            (('admin', 'shop', 'temple'), True, 'scope'),
        ]:
            with self.subTest(names=names, legacy=include_legacy), tempfile.TemporaryDirectory() as tmp:
                base = Path(tmp)
                files = {'payload/' + name + '/index.html': name.encode() for name in names}
                if include_legacy:
                    files['payload/admin/legacy/shop/index.html'] = b'compatibility entry'
                manifest = {'schema': 1, 'release': 'ci-web-test-1', 'scope': 'web',
                            'components': {name: {} for name in names},
                            'files': {path: hashlib.sha256(data).hexdigest() for path, data in files.items()}}
                members = [('manifest.json', json.dumps(manifest).encode(), tarfile.REGTYPE)]
                members += [(path, data, tarfile.REGTYPE) for path, data in files.items()]
                archive = self.archive(base, members)
                if expected:
                    with self.assertRaisesRegex(ValueError, expected):
                        receiver.extract(archive, base / 'out', 'web')
                else:
                    self.assertEqual(receiver.extract(archive, base / 'out', 'web'), manifest)

    def test_builder_packages_only_unified_admin_and_temple(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            for app in ('web-platform-admin', 'web-temple-admin'):
                dist = base / 'apps' / app / 'dist'
                dist.mkdir(parents=True)
                (dist / 'index.html').write_text(app)
            legacy = base / 'apps/web-platform-admin/dist/legacy/shop'
            legacy.mkdir(parents=True)
            (legacy / 'index.html').write_text('compatibility')
            with patch.object(builder, 'git', return_value=B), patch.object(builder.subprocess, 'run') as build:
                builder.build('web', base, base / 'release.tgz')
            self.assertEqual({c.kwargs['cwd'].name for c in build.call_args_list},
                             {'web-platform-admin', 'web-temple-admin'})
            with tarfile.open(base / 'release.tgz') as archive:
                manifest = json.load(archive.extractfile('manifest.json'))
            self.assertEqual(set(manifest['components']), {'admin', 'temple'})
            self.assertIn('payload/admin/legacy/shop/index.html', manifest['files'])
            self.assertFalse(any(p.startswith('payload/shop/') for p in manifest['files']))

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

    def test_retry_retags_only_metadata_without_changing_payload(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            manifest = {'release': 'ci-h5-123-1-' + A[:12], 'files': {'payload/h5/index.html': 'checksum'}}
            archive = self.archive(base, [('manifest.json', json.dumps(manifest).encode(), tarfile.REGTYPE),
                                          ('payload/h5/index.html', b'unchanged', tarfile.REGTYPE)])
            self.assertEqual(retag(archive, base / 'retry.tgz', '123', '1'), archive)
            retried = retag(archive, base / 'retry.tgz', '123', '2')
            with tarfile.open(retried) as tar:
                updated = json.load(tar.extractfile('manifest.json'))
                self.assertEqual(updated['release'], 'ci-h5-123-2-' + A[:12])
                self.assertEqual(updated['files'], manifest['files'])
                self.assertEqual(tar.extractfile('payload/h5/index.html').read(), b'unchanged')
            with self.assertRaisesRegex(ValueError, 'does not belong'):
                retag(archive, base / 'bad.tgz', '124', '2')


if __name__ == '__main__':
    unittest.main()
