"""Run the native knowledge components on the disposable release test runner."""
from contextlib import contextmanager
import json
from pathlib import Path
import secrets
import subprocess
import time
import urllib.request

import ecs_receiver as receiver


@contextmanager
def knowledge_fixture(directory, spec):
    runtime = directory / 'knowledge'
    runtime.mkdir()
    receiver.WEKNORA = runtime
    source = Path(__file__).resolve().parents[2] / 'deploy' / 'weknora'
    baseline = (source / 'compose.yaml').read_text().replace(',askxuan-ai-service', '')
    (runtime / 'compose.yaml').write_text(baseline)
    for name, config in spec['configs'].items():
        (runtime / name).write_text(config['content'])
    (runtime / '.env').write_text(''.join(f'{name}={secrets.token_hex(16)}\n' for name in
                                         ('DB_PASSWORD', 'REDIS_PASSWORD', 'JWT_SECRET', 'SYSTEM_AES_KEY',
                                          'AI_EMBEDDING_KEY', 'NEO4J_PASSWORD')) + 'DISABLE_REGISTRATION=false\n')
    compose = ['docker', 'compose', '-p', 'askxuan-knowledge', '-f', str(runtime / 'compose.yaml')]
    endpoint = '''
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json, sys
class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path != "/v1/embeddings":
            self.send_error(401)
            return
        value = json.loads(body)
        inputs = value["input"]
        count = 1 if isinstance(inputs, str) else len(inputs)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"object": "list", "model": value.get("model", "fixture"),
            "usage": {"prompt_tokens": count, "total_tokens": count}, "data": [
                {"object": "embedding", "index": i, "embedding": [1.0] + [0.0] * 511} for i in range(count)
            ]}).encode())
ThreadingHTTPServer(("0.0.0.0", int(sys.argv[1])), Handler).serve_forever()
'''
    server = runtime / 'fixture.py'
    server.write_text(endpoint)

    def docker(*args):
        return subprocess.check_output(['docker', *args], text=True).strip()

    def engine(path, payload=None, headers=None):
        headers = dict(headers or {})
        if isinstance(payload, dict):
            payload = json.dumps(payload).encode()
            headers['Content-Type'] = 'application/json'
        req = urllib.request.Request('http://127.0.0.1:18085/api/v1' + path, data=payload, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=20) as response:
                data = json.load(response)
        except urllib.error.HTTPError as error:
            raise RuntimeError(f'Knowledge fixture {path} returned HTTP {error.code}') from None
        assert data['success']
        return data

    def ready_document(document):
        for _ in range(45):
            state = engine('/knowledge/' + document, headers=auth)['data']['parse_status']
            if state in ('failed', 'completed'):
                assert state == 'completed', 'Fixture document failed to index'
                return
            time.sleep(2)
        raise RuntimeError('Fixture document did not index')

    assert not docker('ps', '-aq', '--filter', 'label=com.docker.compose.project=askxuan-knowledge')
    assert not docker('ps', '-aq', '--filter', 'name=^/askxuan-ai-service$')
    docker('network', 'create', 'askxuan-net')
    try:
        subprocess.run([*compose, 'up', '-d', '--no-deps', '--no-build', '--wait', '--wait-timeout', '180',
                        'postgres', 'redis', 'neo4j', 'docreader'], check=True)
        docker('run', '-d', '--name', 'knowledge-release-embedding', '--network', 'askxuan-knowledge_default',
               '--network-alias', 'embedding', '-v', str(server) + ':/fixture.py:ro', 'python:3.12-alpine',
               'python', '/fixture.py', '9981')
        subprocess.run([*compose, 'up', '-d', '--no-deps', '--no-build', '--wait', '--wait-timeout', '180', 'app'], check=True)
        credentials = {'email': 'knowledge-release@internal.example', 'username': 'Release fixture',
                       'password': secrets.token_hex(12) + 'Aa1!'}
        engine('/auth/register', credentials)
        login = engine('/auth/login', {name: credentials[name] for name in ('email', 'password')})
        token = engine(f'/tenants/{login["active_tenant"]["id"]}/api-keys', {'name': 'CI', 'full_access': True},
                       {'Authorization': 'Bearer ' + login['token']})['data']['token']
        docker('run', '-d', '--name', 'askxuan-ai-service', '--network', 'askxuan-net',
               '-e', 'AI_WEKNORA_KEY=' + token, '-e', 'AI_WEKNORA_EMBEDDING_ID=askxuan-embedding',
               '-v', str(server) + ':/fixture.py:ro', 'python:3.12-alpine', 'python', '/fixture.py', '8098')
        auth = {'X-API-Key': token}
        base = engine('/knowledge-bases', {'name': 'Release probe', 'type': 'document', 'embedding_model_id': 'askxuan-embedding',
                                          'storage_provider_config': {'provider': 'local'}}, auth)['data']['id']
        boundary = 'sample-document'
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="probe.md"\r\n'
                'Content-Type: text/markdown\r\n\r\n# Release probe\n\nKnowledge release rollback uses the previous configuration.'
                f'\r\n--{boundary}--\r\n').encode()
        item = engine('/knowledge-bases/' + base + '/knowledge/file', body,
                      {**auth, 'Content-Type': 'multipart/form-data; boundary=' + boundary})['data']
        ready_document(item['id'])
        query = {'query_text': 'Knowledge release rollback uses the previous configuration',
                 'match_count': 1, 'vector_threshold': 0.35, 'keyword_threshold': 0, 'skip_context_enrichment': True}
        search_path = '/knowledge-bases/' + base + '/hybrid-search'
        assert any(hit['knowledge_id'] == item['id'] for hit in engine(search_path, query, auth)['data'])
        original = docker('inspect', '--format', '{{.Id}}', 'askxuan-weknora')
        docreader = docker('inspect', '--format', '{{.Id}}', 'askxuan-weknora-docreader')
        yield baseline, original
        assert (runtime / 'compose.yaml').read_text() == baseline
        ready_document(item['id'])
        assert any(hit['knowledge_id'] == item['id'] for hit in engine(search_path, query, auth)['data'])
        assert docker('inspect', '--format', '{{.Id}}', 'askxuan-weknora-docreader') == docreader
    finally:
        subprocess.run(['docker', 'rm', '-f', 'askxuan-ai-service', 'knowledge-release-embedding'],
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run([*compose, 'down', '--volumes'], check=True)
        docker('network', 'rm', 'askxuan-net')
