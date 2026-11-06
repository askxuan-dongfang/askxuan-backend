import copy
import hashlib
import io
import json
from pathlib import Path
from types import SimpleNamespace
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import ecs_receiver as receiver
import release as builder
from retag_release import retag

A, B = 'a' * 40, 'b' * 40
ELF = b'\x7fELF\x02' + bytes(13) + b'\x3e\x00' + b'fixture'


def template():
    return {'services': {'quiet-service': {
        'container_name': 'askxuan-quiet-service', 'image': receiver.QUIET_BASE,
        'user': '1000:1000', 'entrypoint': ['/app/quiet'],
        'environment': {'QUIET_LISTEN': '0.0.0.0:8096', 'QUIET_MYSQL_ADDR': 'mysql:3306',
                        'QUIET_MYSQL_PASSWORD': 'fixture-only', 'QUIET_REDIS_ADDR': 'quiet-redis:6379',
                        'QUIET_CSRF_KEY': 'fixture-only', 'QUIET_PUBLIC_ORIGINS': 'https://example.org',
                        'QUIET_REDIS_PREFIX': 'quiet-park-prod:'},
        'ports': [{'target': 8096, 'published': '18196', 'host_ip': '127.0.0.1', 'protocol': 'tcp'}],
        'networks': {'askxuan-net': {}},
        'healthcheck': {'test': ['CMD', 'wget', '-q', '-O', '/dev/null', 'http://127.0.0.1:8096/health']},
    }}, 'networks': {'askxuan-net': {'external': True, 'name': 'askxuan-net'}}, 'volumes': {}}


def container():
    return {'Name': '/askxuan-quiet-service', 'Image': 'sha256:previous', 'Mounts': [],
            'HostConfig': {'RestartPolicy': {'Name': 'unless-stopped'}, 'PortBindings': {}},
            'Config': {'Env': ['QUIET_REDIS_PREFIX=quiet-park-prod:'], 'Entrypoint': ['/app/quiet'],
                       'Healthcheck': {'Test': ['CMD', 'wget', 'http://127.0.0.1:8096/health'],
                                       'Interval': 10_000_000_000, 'Timeout': 3_000_000_000, 'Retries': 3}},
            'NetworkSettings': {'Networks': {'askxuan-net': {}}}}


class QuietReleaseTests(unittest.TestCase):
    def archive(self, base, scope='quiet', extra=False):
        files = {'payload/quiet/quiet': ELF}
        if extra:
            files['payload/quiet/install.sh'] = b'never execute'
        manifest = {'schema': 1, 'scope': scope, 'release': 'ci-' + scope + '-123-1-' + B[:12],
                    'history': {'backend': [B, A]}, 'components': {'quiet': {'sources': {'backend': B}, 'input': 'new'}},
                    'files': {n: hashlib.sha256(v).hexdigest() for n, v in files.items()}}
        path = base / 'release.tgz'
        with tarfile.open(path, 'w:gz') as archive:
            for name, data in {'manifest.json': json.dumps(manifest).encode(), **files}.items():
                info = tarfile.TarInfo(name)
                info.size = len(data)
                archive.addfile(info, io.BytesIO(data))
        return path, manifest

    def test_quiet_scope_extracts_one_elf_and_rejects_other_scope_or_scripts(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            path, manifest = self.archive(base)
            self.assertEqual(receiver.extract(path, base / 'good', 'quiet'), manifest)
            with self.assertRaisesRegex(ValueError, 'scope'):
                receiver.extract(path, base / 'backend', 'backend')
            self.archive(base, extra=True)
            with self.assertRaisesRegex(ValueError, 'binary payload'):
                receiver.extract(path, base / 'extra', 'quiet')

    def test_quiet_ancestry_uses_only_backend_quiet_state(self):
        manifest = {'scope': 'quiet', 'history': {'backend': [B, A]},
                    'components': {'quiet': {'sources': {'backend': B}}}}
        state = {'components': {'backend/gateway': {'sources': {'backend': 'c' * 40}},
                                'backend/quiet': {'sources': {'backend': A}}}}
        receiver.validate_versions(manifest, state)
        manifest['history']['backend'] = [B]
        with self.assertRaisesRegex(ValueError, 'backend/quiet'):
            receiver.validate_versions(manifest, state)

    def test_quiet_builder_excludes_go_work_and_every_other_service(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            module = base / 'services/content/quiet-service'
            module.mkdir(parents=True)
            def build(args, **kwargs):
                Path(args[args.index('-o') + 1]).write_bytes(ELF)
            with patch.object(builder.Path, 'cwd', return_value=base), patch.object(builder, 'git', return_value=B), \
                    patch.object(builder, 'tree_hash', return_value='quiet-input') as inputs, \
                    patch.object(builder.subprocess, 'run', side_effect=build) as run:
                builder.build('quiet', '.', base / 'release.tgz')
            self.assertEqual(run.call_count, 1)
            self.assertEqual(run.call_args.args[0][-1], '.')
            self.assertEqual(run.call_args.kwargs['cwd'], module)
            self.assertEqual(run.call_args.kwargs['env']['GOWORK'], 'off')
            inputs.assert_called_once_with(base, ['services/content/quiet-service'])
            manifest = receiver.extract(base / 'release.tgz', base / 'out', 'quiet')
            self.assertEqual(set(manifest['components']), {'quiet'})
            self.assertNotIn('contract', manifest)
            self.assertEqual(len(builder.SERVICES), 20)
            self.assertNotIn('quiet', builder.SERVICES)

    def test_runtime_template_is_fixed_root_owned_and_single_service(self):
        with patch.object(receiver, 'trusted_json', return_value=template()) as read:
            value = receiver.quiet_template('askxuan/quiet-service:ci-quiet-test')
        read.assert_called_once_with(receiver.BASE / 'runtime/quiet.json')
        self.assertEqual(value['services']['quiet-service']['image'], 'askxuan/quiet-service:ci-quiet-test')
        for change in ['extra-service', 'public-port', 'payload-base', 'empty-storage', 'volume']:
            value = template()
            d = value['services']['quiet-service']
            if change == 'extra-service': value['services']['mysql'] = {}
            if change == 'public-port': d['ports'][0]['host_ip'] = '0.0.0.0'
            if change == 'payload-base': d['image'] = 'unreviewed:latest'
            if change == 'empty-storage': del d['environment']['QUIET_MYSQL_ADDR']
            if change == 'volume': d['volumes'] = ['/var/run/docker.sock:/var/run/docker.sock']
            with self.subTest(change=change), patch.object(receiver, 'trusted_json', return_value=value), self.assertRaises(ValueError):
                receiver.quiet_template('new')
        for uid, mode in [(501, 0o100600), (0, 0o100622), (0, 0o120600)]:
            with patch.object(Path, 'lstat', return_value=SimpleNamespace(st_uid=uid, st_mode=mode)), self.assertRaisesRegex(ValueError, 'root-owned'):
                receiver.trusted_json(Path('/opt/askxuan/ci/runtime/quiet.json'))

    def fixture(self, base):
        candidate = base / 'releases' / ('ci-quiet-123-1-' + B[:12])
        (candidate / 'payload/quiet').mkdir(parents=True)
        (candidate / 'payload/quiet/quiet').write_bytes(ELF)
        manifest = {'scope': 'quiet', 'release': 'ci-quiet-123-1-' + B[:12],
                    'components': {'quiet': {'sources': {'backend': B}, 'input': 'new'}}}
        state = {'components': {'backend/gateway': {'sources': {'backend': A}, 'input': 'unchanged'}}}
        self.enterContext(patch.multiple(receiver, BASE=base, PUBLIC=base / 'no-public-required', RELEASES=base / 'web'))
        self.enterContext(patch.object(receiver.subprocess, 'run', return_value=SimpleNamespace(returncode=0)))
        self.enterContext(patch.object(receiver, 'trusted_json', side_effect=lambda p: template()))
        return candidate, manifest, state

    def test_first_deploy_changes_only_quiet_and_preserves_old_components(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            candidate, manifest, state = self.fixture(base)
            old = copy.deepcopy(state)
            with patch.object(receiver, 'quiet_container', return_value=None), patch.object(receiver, 'compose_up') as up, \
                    patch.object(receiver, 'healthy') as health, patch.object(receiver, 'smoke_quiet'), patch.object(receiver, 'smoke') as legacy:
                receiver.deploy(candidate, manifest, state)
            self.assertEqual(up.call_count, 1)
            self.assertEqual(set(json.loads(up.call_args.args[0].read_text())['services']), {'quiet-service'})
            health.assert_called_once_with(['quiet'])
            legacy.assert_not_called()
            self.assertFalse((candidate / 'rollback.json').exists())
            self.assertEqual(state['components']['backend/gateway'], old['components']['backend/gateway'])
            self.assertIn('backend/quiet', state['components'])
            journal = json.loads((candidate / 'transaction.json').read_text())
            self.assertEqual(journal['created_services'], ['quiet'])
            self.assertEqual(journal['changed_services'], ['quiet'])

    def test_first_deploy_failure_removes_only_app_and_leaves_storage_untouched(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            candidate, manifest, state = self.fixture(base)
            old = copy.deepcopy(state)
            with patch.object(receiver, 'quiet_container', side_effect=[None, container()]), \
                    patch.object(receiver, 'compose_up'), patch.object(receiver, 'healthy', side_effect=RuntimeError('unhealthy')), \
                    patch.object(receiver, 'run') as run:
                with self.assertRaisesRegex(RuntimeError, 'unhealthy'):
                    receiver.deploy(candidate, manifest, state)
            run.assert_called_once_with(['docker', 'rm', '-f', 'askxuan-quiet-service'])
            self.assertEqual(json.loads((base / 'state.json').read_text()), old)
            self.assertEqual(json.loads((candidate / 'transaction.json').read_text())['phase'], 'rolled-back')

    def test_update_health_failure_restores_previous_image_and_healthcheck(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            candidate, manifest, state = self.fixture(base)
            with patch.object(receiver, 'quiet_container', return_value=container()), patch.object(receiver, 'compose_up') as up, \
                    patch.object(receiver, 'healthy', side_effect=[RuntimeError('unhealthy'), None]), patch.object(receiver, 'run') as run:
                with self.assertRaisesRegex(RuntimeError, 'unhealthy'):
                    receiver.deploy(candidate, manifest, state)
            self.assertEqual([c.args[0].name for c in up.call_args_list], ['compose.json', 'rollback.json'])
            d = json.loads((candidate / 'rollback.json').read_text())['services']['quiet-service']
            self.assertEqual(d['image'], 'sha256:previous')
            self.assertEqual(d['healthcheck']['interval'], '10000000000ns')
            self.assertEqual(d['healthcheck']['test'][0], 'CMD')
            run.assert_not_called()

    def test_missing_base_and_docker_failure_do_not_bootstrap_runtime(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            candidate, manifest, state = self.fixture(base)
            with patch.object(receiver, 'quiet_container', return_value=None), \
                    patch.object(receiver.subprocess, 'run', return_value=SimpleNamespace(returncode=1)), patch.object(receiver, 'compose_up') as up:
                with self.assertRaisesRegex(ValueError, 'base is not provisioned'):
                    receiver.deploy(candidate, manifest, state)
            up.assert_not_called()
        with patch.object(receiver, 'run', side_effect=RuntimeError('docker unavailable')):
            with self.assertRaisesRegex(RuntimeError, 'docker unavailable'):
                receiver.quiet_container()

    def test_first_deploy_manual_rollback_removes_only_app(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            candidate, manifest, state = self.fixture(base)
            journal = {'phase': 'complete', 'created_services': ['quiet'], 'changed_services': ['quiet'],
                       'state_before': state, 'previous_public': str(base / 'no-public-required')}
            (candidate / 'transaction.json').write_text(json.dumps(journal))
            (base / 'state.json').write_text(json.dumps({'last_release': manifest['release']}))
            with patch.object(receiver, 'quiet_container', return_value=container()), patch.object(receiver, 'run') as run, \
                    patch.object(receiver, 'compose_up') as up, patch.object(receiver, 'smoke_quiet') as smoke, patch.object(receiver, 'smoke') as legacy:
                receiver.rollback_latest(manifest['release'])
            run.assert_called_once_with(['docker', 'rm', '-f', 'askxuan-quiet-service'])
            up.assert_not_called()
            smoke.assert_not_called()
            legacy.assert_not_called()
            self.assertEqual(json.loads((base / 'state.json').read_text()), state)

    def test_quiet_retry_retags_metadata_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            archive, _ = self.archive(base)
            retried = retag(archive, base / 'retry.tgz', '123', '2')
            manifest = receiver.extract(retried, base / 'out', 'quiet')
            self.assertEqual(manifest['release'], 'ci-quiet-123-2-' + B[:12])
            self.assertEqual((base / 'out/payload/quiet/quiet').read_bytes(), ELF)


if __name__ == '__main__':
    unittest.main()
