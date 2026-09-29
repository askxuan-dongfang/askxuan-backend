#!/usr/bin/env python3
"""Release a tested AI binary, preserving live configuration. Run on ECS as root."""
import fcntl,json,os,pathlib,subprocess,time,urllib.request,sys,importlib.util,importlib.machinery
os.umask(0o077)
release,sha,input_hash=sys.argv[1:]
assert release.replace('-','').isalnum() and len(sha)==40 and len(input_hash)==64
base=pathlib.Path('/opt/askxuan');backup=base/'backups'/release;candidate=base/'runtime'/release
backup.mkdir(exist_ok=True);candidate.mkdir(exist_ok=True)
lock=open(base/'ci/publish.lock','a');fcntl.flock(lock,fcntl.LOCK_EX)
def run(args):return subprocess.check_output(args,stderr=subprocess.STDOUT)
loader=importlib.machinery.SourceFileLoader('receiver','/usr/local/sbin/askxuan-ci-receiver');spec=importlib.util.spec_from_loader(loader.name,loader);receiver=importlib.util.module_from_spec(spec);loader.exec_module(receiver)
c=json.loads(run(['docker','inspect','askxuan-ai-service']))[0]
(backup/'inspect-before.json').write_text(json.dumps(c));(backup/'ci-state-before.json').write_bytes((base/'ci/state.json').read_bytes())
assert (base/'backups/weknora-20260930/MIGRATED').exists()
image='askxuan/ai-service:'+release
base_tag='askxuan/ai-service:weknora-runtime-base'
run(['docker','tag',c['Image'],base_tag])
(candidate/'Dockerfile').write_text('FROM '+base_tag+'\nCOPY --chown=1000:1000 --chmod=755 ai /app/ai\n')
with open(candidate/'build.log','wb') as log:subprocess.run(['docker','build','-t',image,str(candidate)],stdout=log,stderr=log,check=True)
d,nets,vols=receiver.compose_definition(c,c['Image'])
compose=lambda definition:{'services':{'ai-service':definition},'networks':nets,'volumes':vols}
rollback=backup/'compose.rollback.json';receiver.atomic_compose(rollback,compose(d))
d['environment'].update(dict(x.split('=',1) for x in (base/'runtime/weknora-20260930/deploy/weknora/ai-service.env').read_text().splitlines() if '=' in x));d['image']=image
if d.get('mem_limit'):d['mem_limit']=max(int(d['mem_limit']),512*1024*1024)
current=candidate/'compose.json';receiver.atomic_compose(current,compose(d))
try:
 run(['docker','compose','-p','askxuan','-f',str(current),'up','-d','--no-deps','--no-build','ai-service'])
 for i in range(45):
  status=json.loads(run(['docker','inspect','askxuan-ai-service']))[0]['State']
  if status.get('Health',{}).get('Status')=='healthy':break
  time.sleep(2)
 else:raise RuntimeError('AI health timeout')
 with urllib.request.urlopen('http://127.0.0.1:8080/api/v1/health',timeout=10) as r:assert r.status==200
except Exception:
 run(['docker','compose','-p','askxuan','-f',str(rollback),'up','-d','--no-deps','--no-build','ai-service']);print('AI rolled back');raise
state=json.loads((base/'ci/state.json').read_text());state['components']['backend/ai']={'input':input_hash,'sources':{'backend':sha},'release':release};state['last_release']=release
receiver.atomic_json(base/'ci/state.json',state)
(candidate/'release.txt').write_text('ai_service='+sha+'\nrelease='+release+'\nweknora=v0.8.2\n');(candidate/'DEPLOYED').write_text('health passed; business smoke pending\n')
print(json.dumps({'deployed':release,'sha':sha,'health':'healthy','rollback':str(rollback)}))
