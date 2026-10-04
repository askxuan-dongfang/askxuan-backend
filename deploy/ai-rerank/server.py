"""Private, bounded Chinese/English reranking. Documents and secrets are never logged."""
import hmac, json, os, math
from http.server import BaseHTTPRequestHandler, HTTPServer
from fastembed.rerank.cross_encoder import TextCrossEncoder
MODEL = 'BAAI/bge-reranker-base'
KEY = os.environ['AI_EMBEDDING_KEY']
engine = TextCrossEncoder(model_name=MODEL, cache_dir=os.environ.get('MODEL_CACHE','/data/models'), threads=2, specific_model_path=os.environ.get('MODEL_PATH') or None)
class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(15)
    def log_message(self,*_): pass
    def reply(self,status,data):
        raw=json.dumps(data,ensure_ascii=False).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
    def do_GET(self): self.reply(200 if self.path=='/health' else 404,{'model':MODEL,'ready':self.path=='/health'})
    def do_POST(self):
        if self.path not in ('/v1/rerank','/rerank'): return self.reply(404,{'error':'not_found'})
        if not hmac.compare_digest(self.headers.get('Authorization',''),'Bearer '+KEY): return self.reply(401,{'error':'unauthorized'})
        try:
            n=int(self.headers.get('Content-Length','0'))
            if not 0<n<=262144: raise ValueError()
            p=json.loads(self.rfile.read(n));q=p['query'];docs=p['documents']
            if p.get('model')!=MODEL or not isinstance(q,str) or not 0<len(q)<=4000 or not isinstance(docs,list) or not 0<len(docs)<=40: raise ValueError()
            if any(not isinstance(d,str) or not 0<len(d)<=16000 for d in docs): raise ValueError()
            scores=list(engine.rerank(q,docs,batch_size=1));results=[{'index':i,'relevance_score':1/(1+math.exp(-max(-60,min(60,float(s)))))} for i,s in enumerate(scores)]
            self.reply(200,{'results':sorted(results,key=lambda r:r['relevance_score'],reverse=True)})
        except (ValueError,KeyError,TypeError): self.reply(400,{'error':'invalid_input'})
        except Exception: self.reply(503,{'error':'rerank_unavailable'})
HTTPServer((os.environ.get('BIND','127.0.0.1'),int(os.environ.get('PORT','9981'))),Handler).serve_forever()
