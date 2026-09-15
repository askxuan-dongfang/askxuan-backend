#!/usr/bin/env python3
"""Real local HTTP/Mailpit regression. Never target public hosts or real recipients."""
from pathlib import Path
import json,os,secrets,time
root=Path(__file__).resolve().parent
ns={};exec(compile((root/'verify.py').read_text().split("name='httpqa_'")[0],str(root/'verify.py'),'exec'),ns)
ns['BASE']='http://127.0.0.1:58084/api/v1/auth/'
post,request,ok,proof=ns['post'],ns['request'],ns['ok'],ns['proof']
origin='http://127.0.0.1:58084/api/v1/auth/onboarding/'
checks=[]
def yes(name,value):
    if not value:raise AssertionError(name)
    checks.append(name)
def call(path,body=None,token=None):return request(origin+path,body,token)[1]
password='Local-onboarding-'+secrets.token_urlsafe(16)
accounts={}
for kind in ['master','temple']:
    username='http_'+kind+'_'+secrets.token_hex(4);email=username+'@example.test'
    ok(post('email/code',dict(domain='admin',email=email,purpose=kind+'_register')))
    code=ns['code_from_mail'](email)
    session=ok(post('work/register',dict(kind=kind,email=email,username=username,password=password,code=code,agreementVersion='2026-09-15')))
    token=session['accessToken'];accounts[kind]={'username':username,'email':email,'password':password,'session':session}
    a=ok(call('application',token=token));yes(kind+' draft available',a['status']=='draft')
    yes(kind+' cannot review',call('reviews',token=token).get('code')!=0)
    yes(kind+' cannot allocate managed accounts',call('managed',token=token).get('code')!=0)
    yes(kind+' cannot use workbench',request('http://127.0.0.1:58084/api/v1/admin/masters/workspace',token=token)[1].get('code')!=0)
    yes(kind+' cannot submit incomplete profile',call('application',{'profile':{},'revision':a['revision'],'submit':True},token).get('code')!=0)
    # An actual private PDF upload goes through the authenticated HTTP boundary.
    import urllib.request
    boundary='qa'+secrets.token_hex(10)
    payload=('--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="local-fixture.pdf"\r\nContent-Type: application/pdf\r\n\r\n%PDF-1.4\n% local fixture, no personal documents\n%%EOF\r\n--'+boundary+'--\r\n').encode()
    req=urllib.request.Request(origin+'evidence',data=payload,headers={'Authorization':'Bearer '+token,'Content-Type':'multipart/form-data; boundary='+boundary})
    with urllib.request.urlopen(req) as r:ev=ok(json.load(r))
    p={'name':'本地'+('大师' if kind=='master' else '寺院')+'验收','legalName':'测试经办人','contact':'仅本地测试','region':'本地验收','address':'本地测试地址','registrationNumber':'LOCAL-FIXTURE','belief':'han_buddhism','sect':'禅宗','position':'文化交流','description':'隔离环境自动验收记录','evidenceIds':[ev['id']]}
    a=ok(call('application',{'profile':p,'revision':a['revision'],'submit':False},token));yes(kind+' draft saved',a['profile']['name']==p['name'])
    a=ok(call('application',{'profile':p,'revision':a['revision'],'submit':True},token));yes(kind+' submitted',a['status']=='submitted')
    yes(kind+' submitted cannot mutate',call('application',{'profile':p,'revision':a['revision'],'submit':False},token).get('code')!=0)
    accounts[kind]['application']=a
# Use the explicitly local platform fixture from the earlier auth QA setup.
admin=ok(post('admin/login',{'account':'admin.qa@example.test','password':'Local-Admin-Identity-2026!'}))
accounts['reviewer']={'username':'admin.qa@example.test','password':'Local-Admin-Identity-2026!','session':admin}
queue=ok(call('reviews?status=submitted',token=admin['accessToken']))
yes('reviewer sees submitted applications',all(any(row['id']==accounts[k]['application']['id'] for row in queue['list']) for k in ['master','temple']))
a=accounts['master']['application']
yes('foreign evidence denied',call('evidence?id='+a['profile']['evidenceIds'][0],token=accounts['temple']['session']['accessToken']).get('code')!=0)
# Leave master submitted for real browser review. Approve temple to test managed scope.
a=accounts['temple']['application'];ok(call('review',{'id':a['id'],'revision':a['revision'],'approve':True,'note':'本地 HTTP 核验'},admin['accessToken']))
yes('duplicate review rejected',call('review',{'id':a['id'],'revision':a['revision'],'approve':True},admin['accessToken']).get('code')!=0)
yes('old applicant token revoked',call('application',token=accounts['temple']['session']['accessToken']).get('code')!=0)
accounts['temple']['session']=ok(post('admin/login',{'account':accounts['temple']['email'],'password':password}))
yes('approved temple enters scoped account management',call('managed',token=accounts['temple']['session']['accessToken']).get('code')==0)
yes('temple cannot review',call('reviews',token=accounts['temple']['session']['accessToken']).get('code')!=0)
base=root.parents[3]
secretfile=base/'private/onboarding-http-accounts.json'
fd=os.open(secretfile,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
with os.fdopen(fd,'w') as f:json.dump(accounts,f)
report={'passed':True,'checks':checks,'scope':'real local gateway/auth HTTP and Mailpit; CAPTCHA fixture proof; no production mutations','pending_browser_master_review':accounts['master']['application']['id']}
(base/'onboarding-http-verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(report,ensure_ascii=False))
