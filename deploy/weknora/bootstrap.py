#!/usr/bin/env python3
"""Run once on ECS, loopback only. Secrets remain in owner-readable runtime files."""
import json,os,secrets,urllib.request,urllib.error,pathlib
os.umask(0o077)
root=pathlib.Path(__file__).resolve().parent
private=root/'service-account.json'
def req(path,body,token=None):
 h={'Content-Type':'application/json'}
 if token:h['Authorization']='Bearer '+token
 r=urllib.request.Request('http://127.0.0.1:18085/api/v1'+path,data=json.dumps(body).encode(),headers=h)
 with urllib.request.urlopen(r,timeout=30) as res:d=json.load(res)
 assert d.get('success'), 'WeKnora bootstrap API failed'
 return d
if not private.exists():
 credential={'email':'askxuan-knowledge@internal.example','username':'AskXuan Knowledge Service','password':secrets.token_urlsafe(18)+'Aa1!'}
 # Persist before remote mutation, making interrupted bootstrap recoverable.
 private.write_text(json.dumps(credential))
credential=json.loads(private.read_text())
if not credential.get('registered'):
 try:req('/auth/register',credential)
 except urllib.error.HTTPError as e:
  if e.code != 409:raise
 credential['registered']=True;private.write_text(json.dumps(credential))
login=req('/auth/login',{'email':credential['email'],'password':credential['password']})
tenant=login['active_tenant']['id']
if not credential.get('api_key'):
 key=req(f'/tenants/{tenant}/api-keys',{'name':'AskXuan internal knowledge adapter','full_access':True},login['token'])
 credential['api_key']=key['data']['token'];credential['tenant_id']=tenant;private.write_text(json.dumps(credential))
env=dict(line.split('=',1) for line in (root/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
(root/'ai-service.env').write_text('AI_WEKNORA_URL=http://askxuan-weknora:8080\nAI_WEKNORA_KEY='+credential['api_key']+'\nAI_WEKNORA_EMBEDDING_ID=askxuan-embedding\nAI_EMBEDDING_URL=http://askxuan-knowledge-embedding:9981/v1\nAI_EMBEDDING_MODEL=BAAI/bge-small-zh-v1.5\nAI_EMBEDDING_KEY='+env['AI_EMBEDDING_KEY']+'\n')
env['DISABLE_REGISTRATION']='true'
(root/'.env').write_text(''.join(k+'='+v+'\n' for k,v in env.items()))
print(json.dumps({'initialized':True,'tenant_id':tenant,'credentials':'server-private','registration':'close by recreating app'}))
