"""Private OpenAI-compatible embedding sidecar. No prompts/documents are logged."""
import hmac
import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer
from fastembed import TextEmbedding

MODEL = 'BAAI/bge-small-zh-v1.5'
KEY = os.environ.get('AI_EMBEDDING_KEY', '')
if not KEY:
    raise RuntimeError('AI_EMBEDDING_KEY is required')
engine = TextEmbedding(model_name=MODEL, cache_dir=os.environ.get('MODEL_CACHE', '/data/models'), threads=2)

class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(15)

    def log_message(self, *_):
        pass

    def send_json(self, code, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        self.send_json(200 if self.path == '/health' else 404, {'ready': self.path == '/health', 'model': MODEL})

    def do_POST(self):
        if self.path != '/v1/embeddings':
            return self.send_json(404, {'error': 'not_found'})
        if not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + KEY):
            return self.send_json(401, {'error': 'unauthorized'})
        try:
            n = int(self.headers.get('Content-Length', '0'))
            if not 0 < n <= 65536:
                raise ValueError()
            payload = json.loads(self.rfile.read(n))
            texts = payload['input']
            if isinstance(texts, str):
                texts = [texts]
            if payload.get('model') != MODEL or not isinstance(texts, list) or not 0 < len(texts) <= 32:
                raise ValueError()
            data = []
            for index, text in enumerate(texts):
                if not isinstance(text, str) or not 0 < len(text) <= 8000:
                    raise ValueError()
                chunks = [text[i:i+350] for i in range(0, len(text), 350)]
                vectors = list(engine.embed(chunks))
                vector = sum(v * len(c) for v, c in zip(vectors, chunks)) / len(text)
                data.append({'object': 'embedding', 'index': index, 'embedding': vector.tolist()})
            self.send_json(200, {'object': 'list', 'model': MODEL, 'data': data, 'usage': {'prompt_tokens': 0, 'total_tokens': 0}})
        except (ValueError, KeyError, TypeError):
            self.send_json(400, {'error': 'invalid_input'})
        except Exception:
            self.send_json(503, {'error': 'embedding_unavailable'})

server = HTTPServer((os.environ.get('BIND', '127.0.0.1'), int(os.environ.get('PORT', '9981'))), Handler)
server.serve_forever()
