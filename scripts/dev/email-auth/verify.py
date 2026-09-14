#!/usr/bin/env python3
"""HTTP/SMTP smoke test for ONLY the fixed loopback fixture in this directory.
No test bypass is added to the service. CAPTCHA proof is seeded into disposable
Redis; browser visual CAPTCHA acceptance is verified separately.
"""
import hashlib,hmac,json,re,secrets,socket,time,urllib.request,urllib.error
BASE='http://127.0.0.1:58081/api/v1/auth/'
GATE='http://127.0.0.1:58080'
SECRET=b'isolated-email-auth-fixture-signing-key-not-production'
def digest(v):return hmac.new(SECRET,v.encode(),hashlib.sha256).hexdigest()
def redis(*parts):
    data=b'*'+str(len(parts)).encode()+b'\r\n'
    for p in parts:
        p=str(p).encode();data+=b'$'+str(len(p)).encode()+b'\r\n'+p+b'\r\n'
    with socket.create_connection(('127.0.0.1',56379),timeout=5) as s:
        s.sendall(data);result=s.recv(4096)
        assert not result.startswith(b'-'),result
        return result
def proof():
    ident=secrets.token_hex(24);code='48261'
    redis('SET','identity:captcha:'+digest(ident),digest(ident+':'+code),'EX',180)
    return dict(captchaId=ident,captchaCode=code)
def request(url,body=None,token=None):
    headers={'Content-Type':'application/json'}
    if token:headers['Authorization']='Bearer '+token
    req=urllib.request.Request(url,data=json.dumps(body).encode() if body is not None else None,headers=headers)
    try:
        with urllib.request.urlopen(req,timeout=25) as r:return r.status,json.load(r)
    except urllib.error.HTTPError as e:
        raw=e.read()
        try:return e.code,json.loads(raw)
        except ValueError:return e.code,{}
def post(path,body):return request(BASE+path,{**body,**proof()})[1]
def ok(result):assert result.get('code')==0,result;return result.get('data',{})
def code_from_mail(email):
    for _ in range(30):
        messages=request('http://127.0.0.1:58025/api/v1/messages')[1]['messages']
        for m in messages:
            if any(x['Address']==email for x in m['To']):
                mail=request('http://127.0.0.1:58025/api/v1/message/'+m['ID'])[1]
                return re.search(r'验证码为 (\d{6})',mail['Text']).group(1)
        time.sleep(.1)
    raise AssertionError('SMTP message missing')
name='httpqa_'+secrets.token_hex(6);email=name+'@example.test';password='Fixture-HTTP-Password-2026!'
ok(request(BASE+'options')[1]);bad=post('admin/login',dict(account=name,password='1234'));assert bad['code']!=0
ok(post('email/code',dict(email=email,purpose='register',domain='user')))
code=code_from_mail(email)
result=ok(post('email/register',dict(email=email,username=name,password=password,code=code,agreementVersion='2026-09-15')))
access,refresh=result['accessToken'],result['refreshToken']
assert request(GATE+'/api/v1/users/me',token=access)[0]==404,'valid session rejected before upstream'
ok(request(BASE+'refresh',dict(refreshToken=refresh))[1])
assert post('email/register',dict(email=email,username=name,password=password,code=code,agreementVersion='2026-09-15'))['code']!=0
ok(post('login',dict(account=email,password=password)))
assert post('login',dict(account=name,password='1234'))['code']!=0
# Clear only this generated fixture email cooldown to avoid a one-minute test sleep.
redis('DEL','identity:limit:email-minute:'+digest(email))
ok(post('email/code',dict(email=email,purpose='reset',domain='user')))
code=code_from_mail(email);next_password='Fixture-HTTP-Changed-2026!'
ok(post('password/reset',dict(email=email,password=next_password,code=code,domain='user')))
assert request(GATE+'/api/v1/users/me',token=access)[1].get('code') in [40101,40102,40103]
assert request(BASE+'refresh',dict(refreshToken=refresh))[1]['code']!=0
assert post('login',dict(account=name,password=password))['code']!=0
session=ok(post('login',dict(account=name,password=next_password)))
ok(request(BASE+'logout',dict(accessToken=session['accessToken']))[1])
assert request(BASE+'refresh',dict(refreshToken=session['refreshToken']))[1]['code']!=0
print(json.dumps(dict(passed=True,userId=result['userInfo']['userId'],checks=['SMTP registration','email and username login','demo password rejected','OTP replay rejected','gateway access revocation','refresh revocation','password reset','logout']),ensure_ascii=False))
