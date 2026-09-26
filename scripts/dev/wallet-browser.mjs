// LOCAL ONLY: JWT fixtures for read-only wallet QA, connected to isolated Go/MySQL test servers.
import http from 'node:http';
import {createHmac} from 'node:crypto';
const token=(role)=>{const enc=x=>Buffer.from(JSON.stringify(x)).toString('base64url');const value=enc({alg:'HS256',typ:'JWT'})+'.'+enc({userId:91001,userType:role==='customer'?'user':'admin',roles:[role],clientId:role==='temple_admin'?'temple-admin':role,masterId:901,templeCode:'T901',templeId:901,type:'access',exp:Math.floor(Date.now()/1000)+3600});return value+'.'+createHmac('sha256','local-wallet-fixture-only').update(value).digest('base64url')};
http.createServer(async(req,res)=>{
 const url=new URL(req.url,'http://localhost');
 if(url.pathname==='/api/__wallet_signin'){
  const role=['customer','master','temple_admin'].includes(url.searchParams.get('role'))?url.searchParams.get('role'):'customer';const jwt=token(role);
  const payload=JSON.stringify({state:{token:jwt,role:role,userId:91001,displayName:'隔离验收账户',masterId:901},version:0});
  res.setHeader('Content-Type','text/html;charset=utf-8');return res.end(`<h1>钱包 · 本地隔离验收</h1><p>使用本机测试库，不连接生产资金。</p><button onclick='localStorage.setItem("h5_token",${JSON.stringify(jwt)});localStorage.setItem("h5-auth",${JSON.stringify(payload)});localStorage.setItem("df_temple_admin_token",${JSON.stringify(jwt)});localStorage.setItem("df_temple_admin_user",JSON.stringify({id:91001,name:"测试寺院",roles:["temple_admin"]}));location.href="${role==='customer'?'/c/wallet':role==='master'?'/m/earnings':'/temple/wallet'}"'>进入 ${role} 验收</button>`);
 }
 const port=url.pathname.startsWith('/api/v1/payments/wallet')?18191:url.pathname.startsWith('/api/v1/finance/wallet/')?18192:0;
 res.setHeader('Content-Type','application/json');
 if(port){try{const upstream=await fetch('http://127.0.0.1:'+port+req.url,{headers:{Authorization:req.headers.authorization||''}});res.statusCode=upstream.status;return res.end(await upstream.text())}catch{res.statusCode=502;return res.end(JSON.stringify({code:500,message:'本地测试服务未就绪'}))}}
 res.end(JSON.stringify({code:0,data:{list:[],total:0,unreadCount:0}}));
}).listen(18190,'127.0.0.1',()=>console.log('Local wallet fixture 18190'));
