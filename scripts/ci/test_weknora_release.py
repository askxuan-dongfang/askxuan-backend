import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import ecs_receiver as receiver


SOURCE = Path(__file__).resolve().parents[2] / 'deploy' / 'weknora'
REVISION = 'a' * 40


class WeKnoraReleaseTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.base = Path(temp.name)
        self.runtime = self.base / 'knowledge'
        self.runtime.mkdir()
        self.manifest = receiver.weknora_manifest(SOURCE)
        for name in receiver.WEKNORA_FILES:
            (self.runtime / name).write_bytes((SOURCE / name).read_bytes())
        self.compose = self.runtime / 'compose.yaml'
        self.current_compose = (SOURCE / 'compose.yaml').read_text().replace(',askxuan-ai-service', '')
        self.compose.write_text(self.current_compose)
        self.candidate = self.base / 'releases' / 'ci-backend-11-1-aaaaaaaaaaaa'
        self.candidate.mkdir(parents=True)
        public = self.base / 'public'
        public.mkdir()
        state = patch.multiple(receiver, BASE=self.base, WEKNORA=self.runtime, PUBLIC=public,
                               RELEASES=self.base / 'web')
        state.start()
        self.addCleanup(state.stop)

    def inspect(self, args, **kwargs):
        if args[1:2] == ['exec']:
            return b'0'
        if args[1:3] == ['image', 'inspect']:
            result = [{'RepoDigests': [self.manifest['fields'][args[-1]]]}]
        elif args[-1] == 'askxuan-diy-service':
            result = [{'Image': 'prior-diy'}]
        else:
            service = 'app' if args[-1] == 'askxuan-weknora' else 'docreader'
            result = [{'Image': service, 'Config': {
                'Env': ['SSRF_WHITELIST=' + receiver.weknora_compose(self.compose.read_text())[0]['hosts']],
                'Labels': {'com.docker.compose.service': service, 'com.docker.compose.project': 'askxuan-knowledge',
                           'com.docker.compose.project.config_files': str(self.compose)},
            }}]
        return json.dumps(result).encode()

    def test_manifest_rejects_mutable_images_new_egress_and_unsafe_files(self):
        receiver.validate_weknora(self.manifest)
        for key, name, value in [
            ('fields', 'app', 'wechatopenai/weknora-app:latest'),
            ('fields', 'hosts', 'embedding,rerank,askxuan-ai-service,example.org'),
            ('configs', '../env', {'content': 'sample', 'sha256': '1' * 64}),
            ('configs', 'config.yaml', {'content': 'sample', 'sha256': '1' * 64}),
        ]:
            with self.subTest(name=name):
                spec = copy.deepcopy(self.manifest)
                spec[key][name] = value
                with self.assertRaises(ValueError):
                    receiver.validate_weknora(spec)

    def test_running_pins_and_config_need_no_restarts(self):
        self.compose.write_bytes((SOURCE / 'compose.yaml').read_bytes())
        with patch.object(receiver, 'run', side_effect=self.inspect):
            self.assertEqual(receiver.prepare_weknora(self.manifest, self.candidate), ({}, []))
        self.assertFalse((self.candidate / 'weknora.rollback.json').exists())

    def test_infrastructure_drift_stops_before_inspection_or_write(self):
        self.compose.write_text(self.current_compose.replace('mem_limit: 2g', 'mem_limit: 4g', 1))
        with patch.object(receiver, 'run') as run, self.assertRaisesRegex(ValueError, 'layout changed'):
            receiver.prepare_weknora(self.manifest, self.candidate)
        run.assert_not_called()

    def test_release_and_operator_rollback_restore_exact_weknora_configs(self):
        release = {'release': self.candidate.name, 'scope': 'backend', 'weknora': self.manifest,
                   'history': {'backend': [REVISION]}, 'components': {}}
        with patch.object(receiver, 'run', side_effect=self.inspect), patch.object(receiver, 'smoke'), \
             patch.object(receiver, 'smoke_weknora', return_value={'retrieval': True}), \
             patch.object(receiver, 'compose_up') as compose:
            receiver.deploy(self.candidate, release, {}, registry=self.base)
            self.assertEqual(self.compose.read_bytes(), (SOURCE / 'compose.yaml').read_bytes())
            current = json.loads((self.base / 'state.json').read_text())
            self.assertEqual(current['components']['backend/weknora']['fields'], self.manifest['fields'])
            receiver.rollback_latest(self.candidate.name)
        self.assertEqual(compose.call_count, 2)
        self.assertEqual([call.kwargs['services'] for call in compose.call_args_list], [['app'], ['app']])
        self.assertEqual(self.compose.read_text(), self.current_compose)
        self.assertEqual(json.loads((self.base / 'state.json').read_text()), {})

    def test_failed_joint_smoke_restores_backend_and_weknora(self):
        binary = '1' * 64
        release = {'release': self.candidate.name, 'scope': 'backend', 'weknora': self.manifest,
                   'history': {'backend': [REVISION]}, 'files': {'payload/diy/diy': binary},
                   'components': {'diy': {'input': 'changed', 'image': 'verified-diy'}}}

        def inspect(args, **kwargs):
            if args[1:3] == ['image', 'inspect'] and args[-1] == 'verified-diy':
                return binary.encode()
            return self.inspect(args, **kwargs)

        with patch.object(receiver, 'run', side_effect=inspect), patch.object(receiver, 'healthy'), \
             patch.object(receiver, 'compose_definition', return_value=({'image': 'verified-diy'}, {}, {})), \
             patch.object(receiver, 'smoke'), patch.object(receiver.subprocess, 'run'), \
             patch.object(receiver, 'compose_up') as compose, \
             patch.object(receiver, 'smoke_weknora', side_effect=RuntimeError('probe failed')):
            with self.assertRaisesRegex(RuntimeError, 'probe failed'):
                receiver.deploy(self.candidate, release, {}, registry=self.base)
        self.assertEqual([call.args[0].name for call in compose.call_args_list],
                         ['compose.json', 'compose.yaml', 'compose.yaml', 'rollback.json'])
        self.assertEqual(self.compose.read_text(), self.current_compose)
        self.assertEqual(json.loads((self.candidate / 'transaction.json').read_text())['phase'], 'rolled-back')
