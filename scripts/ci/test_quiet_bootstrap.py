import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import ecs_receiver as receiver

spec = importlib.util.spec_from_file_location('quiet_bootstrap', Path(__file__).parents[1] / 'ops/bootstrap-quiet-runtime.py')
ops = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ops)
BASE_IMAGE = 'alpine@sha256:' + 'a' * 64


class QuietBootstrapTests(unittest.TestCase):
    def commands(self, args, data=None, timeout=60):
        if args == ops.MYSQL:
            return '0\n0' if data.startswith(b'SELECT') else ''
        if args[:3] in (['docker', 'ps', '-aq'], ['docker', 'volume', 'ls'], ['docker', 'image', 'ls']):
            return ''
        if args[:3] == ['docker', 'network', 'inspect']:
            return 'askxuan-net'
        if args[:3] == ['docker', 'inspect', '--format']:
            return 'true' if args[3] == '{{.State.Running}}' else 'sha256:' + 'b' * 64
        if args[:3] == ['docker', 'image', 'inspect']:
            return 'sha256:' + 'a' * 64
        if args[:3] == ['docker', 'run', '-d']:
            return 'c' * 64
        if args[:4] == ['docker', 'exec', '-i', ops.REDIS]:
            return 'PONG'
        return ''

    def fixture(self, base):
        self.enterContext(patch.object(ops, 'BASE', base))
        self.enterContext(patch.object(ops, 'root_path'))
        return self.enterContext(patch.object(ops, 'command', side_effect=self.commands))

    def test_default_plan_only_reads_and_never_generates_secrets(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            (base / 'publish.lock').touch()
            command = self.fixture(base)
            output = io.StringIO()
            # main's restrictive umask is restored for the remainder of the test process.
            previous = os.umask(0o077)
            try:
                with patch.object(ops.os, 'geteuid', return_value=0), \
                        patch.object(ops.secrets, 'token_hex') as random, \
                        patch('sys.argv', ['bootstrap', '--base-image', BASE_IMAGE]), contextlib.redirect_stdout(output):
                    ops.main()
            finally:
                os.umask(previous)
            self.assertEqual(json.loads(output.getvalue())['status'], 'plan-only')
            self.assertEqual(set(p.name for p in base.iterdir()), {'publish.lock'})
            random.assert_not_called()
            for call in command.call_args_list:
                args = call.args[0]
                self.assertNotEqual(args[:2], ['docker', 'build'])
                self.assertNotEqual(args[:2], ['docker', 'run'])
                if args == ops.MYSQL:
                    self.assertTrue(call.args[1].startswith(b'SELECT'))

    def test_apply_keeps_credentials_private_and_matches_receiver_contract(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            command = self.fixture(base)
            plan = ops.preflight(BASE_IMAGE)
            result = ops.apply(plan)
            receipt = json.loads(Path(result['receipt']).read_text())
            backup = Path(result['receipt']).parent
            secret = json.loads((backup / 'recovery-secrets.json').read_text())
            self.assertEqual(receipt['phase'], 'complete')
            for value in secret.values():
                self.assertNotIn(value, json.dumps(receipt))
                self.assertNotIn(value, json.dumps(result))
            for file in [backup / 'recovery-secrets.json', backup / 'quiet.json', base / 'runtime/quiet.json']:
                self.assertEqual(file.stat().st_mode & 0o777, 0o600)
            value = json.loads((base / 'runtime/quiet.json').read_text())
            with patch.object(receiver, 'trusted_json', return_value=value):
                receiver.quiet_template('askxuan/quiet-service:verified')
            sql = next(call.args[1].decode() for call in command.call_args_list if call.args[0] == ops.MYSQL and call.args[1].startswith(b'CREATE'))
            self.assertIn('GRANT CREATE, SELECT, INSERT, UPDATE, DELETE ON quiet_local.*', sql)
            self.assertNotIn('DROP ', sql)
            self.assertNotIn('ON *.*', sql)
            redis = next(call.args[0] for call in command.call_args_list if call.args[0][:3] == ['docker', 'run', '-d'])
            self.assertNotIn('-p', redis)
            self.assertNotIn('--publish', redis)
            self.assertIn('sha256:' + 'b' * 64, redis)
            self.assertNotIn('askxuan-redis', redis)
            dockerfile = (backup / 'runtime.Dockerfile').read_text()
            self.assertEqual(dockerfile.splitlines()[0], 'FROM ' + BASE_IMAGE)
            self.assertNotIn('COPY', dockerfile)
            self.assertNotIn('apk', dockerfile)

    def test_partial_mysql_failure_stops_without_cleanup_or_secret_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            command = self.fixture(base)
            plan = ops.preflight(BASE_IMAGE)
            def fail(args, data=None, timeout=60):
                if args == ops.MYSQL and data.startswith(b'CREATE'):
                    raise RuntimeError('sensitive command output must not escape')
                return self.commands(args, data, timeout)
            command.side_effect = fail
            with self.assertRaises(ops.Refused) as error:
                ops.apply(plan)
            self.assertNotIn('sensitive command output', str(error.exception))
            receipt = json.loads(next((base / 'backups').glob('quiet-bootstrap-*/receipt.json')).read_text())
            self.assertEqual(receipt['phase'], 'failed')
            self.assertEqual(receipt['failedDuring'], 'creating-mysql-database-and-user')
            self.assertEqual(receipt['lastCompleted'], 'runtime-base-ready')
            self.assertFalse((base / 'runtime/quiet.json').exists())
            for call in command.call_args_list:
                self.assertFalse(any(word in call.args[0] for word in ['rm', 'down', 'remove']))
                self.assertNotEqual(call.args[0][:3], ['docker', 'volume', 'create'])

    def test_existing_database_resources_or_unfinished_release_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            command = self.fixture(base)
            def existing_database(args, data=None, timeout=60):
                return '1\n0' if args == ops.MYSQL else self.commands(args, data, timeout)
            command.side_effect = existing_database
            with self.assertRaisesRegex(ops.Refused, 'already exists'):
                ops.preflight(BASE_IMAGE)
            command.side_effect = self.commands
            (base / 'runtime').mkdir()
            (base / 'runtime/quiet.json').write_text('{}')
            with self.assertRaisesRegex(ops.Refused, 'template already exists'):
                ops.preflight(BASE_IMAGE)
            (base / 'runtime/quiet.json').unlink()
            journal = base / 'releases/test/transaction.json'
            journal.parent.mkdir(parents=True)
            journal.write_text(json.dumps({'phase': 'switching-backend'}))
            command.reset_mock()
            with self.assertRaisesRegex(ops.Refused, 'unfinished'):
                ops.preflight(BASE_IMAGE)
            command.assert_not_called()

    def test_command_errors_hide_stderr_arguments_and_stdin(self):
        from types import SimpleNamespace
        with patch.object(ops.subprocess, 'run', return_value=SimpleNamespace(returncode=1, stdout=b'', stderr=b'secret-value')):
            with self.assertRaises(ops.Refused) as error:
                ops.command(['fixture', 'secret-value'], b'secret-value')
        self.assertNotIn('secret-value', str(error.exception))


if __name__ == '__main__':
    unittest.main()
