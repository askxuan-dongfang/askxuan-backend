#!/usr/bin/env python3
"""Local gateway regression for private evidence size and content boundaries."""
from pathlib import Path
import secrets,json,urllib.request,urllib.error
root=Path(__file__).resolve().parent
ns={};exec(compile((root/'verify.py').read_text().split("name='httpqa_'")[0],str(root/'verify.py'),'exec'),ns)
ns['BASE']='http://127.0.0.1:58084/api/v1/auth/'
post,ok=ns['post'],ns['ok']
name='upload_'+secrets.token_hex(6);email=name+'@example.test'
ok(post('email/code',dict(domain='admin',purpose='master_register',email=email)))
session=ok(post('work/register',dict(kind='master',email=email,username=name,password='Local-upload-'+secrets.token_urlsafe(18),code=ns['code_from_mail'](email),agreementVersion='2026-09-15')))
token=session['accessToken'];origin=ns['BASE']+'onboarding/'
def upload(data):
 b='test'+secrets.token_hex(10)
 payload=('--'+b+'\r\nContent-Disposition: form-data; name="file"; filename="size-test.pdf"\r\nContent-Type: application/pdf\r\n\r\n').encode()+data+('\r\n--'+b+'--\r\n').encode()
 req=urllib.request.Request(origin+'evidence',data=payload,headers={'Authorization':'Bearer '+token,'Content-Type':'multipart/form-data; boundary='+b})
 try:
  with urllib.request.urlopen(req) as r:return r.status,json.load(r)
 except urllib.error.HTTPError as e:
  try:value=json.load(e)
  except Exception:value={}
  return e.code,value
checks=[]
size=5*1024*1024
status,value=upload(b'%PDF-1.4\n'+b'0'*(size-9));ev=ok(value);assert status==200;checks.append('5MB evidence accepted through real gateway')
req=urllib.request.Request(origin+'evidence?id='+ev['id'],headers={'Authorization':'Bearer '+token})
with urllib.request.urlopen(req) as r:
 assert len(r.read())==size and r.headers['Cache-Control']=='no-store' and r.headers['X-Content-Type-Options']=='nosniff'
checks.append('private download preserves size and security headers')
status,value=upload(b'%PDF-1.4\n'+b'0'*(size-8));assert status!=200 or value.get('code')!=0;checks.append('over 5MB rejected')
status,value=upload(b'<html>not a PDF</html>');assert status!=200 or value.get('code')!=0;checks.append('forged PDF MIME rejected')
print(json.dumps({'passed':True,'checks':checks},ensure_ascii=False))
