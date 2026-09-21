// LOCAL ONLY. Authentication fixture, never deploy this proxy.
// New experience + journal endpoints are served by the real Go service + MySQL.
import http from 'node:http';
const server = http.createServer(async (req,res)=>{
 if(req.url==='/api/__test_signin'){
  res.setHeader('Content-Type','text/html; charset=utf-8');
  return res.end(`<html lang="zh"><meta charset="utf-8"><h1>AI 体验 · 本地隔离验收</h1><p>使用隔离身份 95101。新技能与手记连接本地 Go 服务和 MySQL；这里不验证生产登录与真实模型质量。</p><button onclick="localStorage.setItem('h5_token','fixture-95101');localStorage.setItem('h5-auth',JSON.stringify({state:{token:'fixture-95101',role:'customer',displayName:'隔离验收用户',userId:95101},version:0}));location.href='/c/ai'">进入隔离验收</button></html>`);
 }
 if(req.url.startsWith('/api/v1/ai/')){
  let body='';for await(const chunk of req)body+=chunk;
  const headers={'Content-Type':'application/json','X-User-Id':'95101','X-User-Type':'user'};
  if(req.headers.authorization!=='Bearer fixture-95101'){res.setHeader('Content-Type','application/json');return res.end(JSON.stringify({code:40301,message:'local fixture sign-in required'}))}
  try {const upstream=await fetch('http://127.0.0.1:18098'+req.url,{method:req.method,headers,...(body?{body}:{})});res.statusCode=upstream.status;res.setHeader('Content-Type',upstream.headers.get('content-type')||'application/json');res.end(await upstream.text())}catch{res.statusCode=502;res.end('local AI service unavailable')}return;
 }
 res.setHeader('Content-Type','application/json');res.end(JSON.stringify({code:0,data:{list:[],total:0}}));
});
server.listen(18198,'127.0.0.1',()=>console.log('Local fixture proxy 18198'));
