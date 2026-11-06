# 静心园区服务

本目录提供可独立运行的 Go 房间服务，以及专用 Redis、MySQL、Docker 和本地 HTTPS/WSS 链路。默认仅有浏览器访客历史；可显式配置现有业务 Auth 的实时核验以启用账号练习历史。房间人物始终是临时访客，未连接现有业务数据库、Redis 或生产网关。账号核验代码与 mock 测试通过，不代表真实业务账号链路已验收。

## 运行

需要 Docker Compose、OpenSSL；H5 开发页运行在本机5186端口。

```sh
./local-stack.sh up
```

脚本在本目录被 Git/Docker 忽略的 .local/ 内生成随机密码、CSRF密钥和30天自签证书。不会打印密码、修改系统证书信任、清除卷或修改其他容器。专用 Compose 项目名为 dongfang-quiet-local，独立卷为 quiet-mysql、quiet-redis。

| 入口 | 用途 |
| --- | --- |
| http://127.0.0.1:8097 | 隔离容器内Go服务的本地HTTP入口 |
| https://localhost:8446/c/quiet | nginx本地TLS入口；普通页面代理到本机H5的5186端口 |
| 127.0.0.1:3316 | 专用MySQL，数据库固定quiet_local、仅quiet用户，表为quiet_guests/quiet_practices |
| 127.0.0.1:6386 | 专用Redis，服务端房间前缀quiet-park-local: |

浏览器首次访问自签证书需要用户自行处理浏览器提示。测试只在自身TLS信任池载入该证书，不安装到系统。HTTPS访客Cookie带Secure，建议持续使用同一个协议和主机名；切换localhost/127.0.0.1会建立不同浏览器会话。

Vite的 /quiet-api 和 /quiet-ws 代理应指向8097。nginx直接将这两个路径代理到房间容器，并设置Upgrade、TLS和长连接超时；HTTPS网页其他内容仍由开发中的H5提供，这不是正式静态发布包。

```sh
./local-stack.sh deps  # 只启动独立Redis/MySQL
./local-stack.sh down  # 停止专用容器；保留数据卷和.local恢复材料
```

快速内存开发仍可在本目录运行 go run .，默认只监听127.0.0.1:8096，session的persistent=false；该模式数据重启即失效，不是MySQL持久化替代证据。容器模式固定使用内部0.0.0.0:8096，宿主端口仅绑定127.0.0.1。

## 访客身份和练习API

先GET /quiet-api/session。服务端签发256位随机HttpOnly、SameSite=Strict Cookie，只保存Cookie哈希；存在于服务端存储中的Cookie才有效。响应含csrfToken、persistent、mode、accountHistoryAvailable和roomLimit=20，不返回可供客户端指定的guestID。Cookie有效期30天；Cookie丢失、过期或被清除后无法找回旧访客历史。

- GET /quiet-api/records：无Bearer时只返回当前Cookie访客最近50条记录；账号模式见下。
- POST /quiet-api/records：JSON为date、seconds、hits、beads；必须带受允许的Origin、X-Quiet-CSRF和Idempotency-Key。日期、整数、体积和计数上限均校验；同一访客/幂等键不会重复插入，不同内容复用键返回409。
- DELETE /quiet-api/records：同样校验Origin和CSRF，只删除当前访客的历史。
- GET /quiet-api/rooms：读取当前进程持有的Redis房间目录与快照，需当前访客Cookie。

所有SQL均固定在新quiet_local库，采用参数绑定；客户端不能通过请求body或query选择访客。多标签页共享同一浏览器Cookie的访客历史，但各WebSocket分别获得临时人物ID。没有兑换积分、功德、钱款或其他业务权限。

房间bootstrap始终使用无Bearer的同源fetch，6秒AbortController超时并释放计时器；账号历史请求复用H5现有axios的续期和撤销处理，显式baseURL=''以保持/quiet-api原路径。只有persistent=true且保存获得服务器ack才可标为永久在线保存；localStorage保留的是此设备共享记录，与账号历史独立。

## 可选账号练习历史

`QUIET_AUTH_URL` 默认为空；配置值只允许Auth服务的HTTP(S) origin，不含路径、query或凭据。服务端固定请求其`GET /api/v1/auth/access`，3秒超时、不跟随重定向、不记录或回显Bearer。容器需能访问该地址，例如开发机已运行Auth时由维护者选择容器可达的开发地址；Compose原样透传该环境变量，不自动连接生产。

本轮本地运行配置的origin取自既有H5开发业务API，先经无令牌的系统默认TLS验证，再仅保存到被忽略的`.local/stack.env`；未复制JWT密钥或真实用户令牌，未修改远端。源码默认值仍为空。回退时在该本地文件将`QUIET_AUTH_URL`置空，再执行`docker compose --env-file .local/stack.env up -d --no-deps --wait room`，仅重建room运行容器且保留所有卷；浏览器完整刷新后重新读取模式。

- 未配置：session明确返回`mode=guest-local, accountHistoryAvailable=false`，即使浏览器有登录令牌也不称账号身份；H5明确按浏览器访客通道保存。直接向records发送Bearer返回503，不在服务端静默降级。
- 已配置、无Bearer：仍为`guest-local`。已配置且Bearer通过实时核验才返回`account-local`。每次账号记录的读、写、删都会重新核验，要求正数userId、clientId=customer且roles恰为[customer]。Auth现有实现检查签名、有效期、SID、账号启用状态及账号版本。
- Auth标准HTTP200中的40101/40102/40103/40105会转换为HTTP401并保留业务码，供既有前端刷新/撤销逻辑处理；角色错误403，依赖异常、非标准响应或超时503。任何失败均不会改写成访客记录。账号请求的访客Cookie失效返回403/code40310，重新建立浏览器会话而不误注销业务账号。
- 记录owner仅由服务端计算`SHA256("quiet:customer:"+canonicalAuthOrigin+":"+verifiedUserId)`；origin规范化大小写、默认端口与末尾斜线，切换到不同Auth authority时相同数字ID也不会复用历史。沿用现有quiet_practices.guest_hash字段及幂等/查询索引，不新增业务账号schema或外键。访客owner仍是随机Cookie的哈希；客户端body、query、X-User-Id均不能选owner。相同authority的账号跨浏览器使用同一历史，访客历史不自动迁入账号。当前没有真实账号记录，无需迁移旧测试owner。
- H5按当前身份清理账号会话缓存并取消在途请求；token正常刷新保留同账号上下文，退出、切账号和跨标签页storage变化使旧结果失效。账号历史的读/删只作用当前验证身份，WS仍仅用访客Cookie，各标签页临时人物不改为业务用户。

UI中的设备共享副本可由同浏览器使用者查看，不是按账号隔离的副本；账号历史隔离发生在服务端。真实Auth的无效测试Bearer已只读验证返回40102且不降级，合法账号练习历史尚未实测。未实现自动访客转账号迁移或账号人物/房间票据。

## 房间、园区和协议

一个Go进程维护多个权威房间，自动将新连接分配到未满的房间；每房20是代码上限，不是压测承诺。Redis真实记录owner、人数、完整快照、更新时间和60秒TTL，活跃房间每20秒续期；旧进程停机后状态过期。没有实现跨进程迁房、分布式权威接管或公网扩容。

WebSocket路径为 /quiet-ws，必须先取得有效访客Cookie；允许本地http的5173/5186和https的8446对应localhost/127.0.0.1 Origin。welcome包含id、players、capacity、roomId；后续为state（players、roomId）或error（message）。玩家字段为id、name、x、z、yaw、action、seat、offeringHall。

- move：x/z/yaw；园区范围x[-42,42]、z[-50,34]。服务端按5单位/秒补充移动额度，最多积累1.5单位；客户端需发送连续走动步骤，不能一次传送。
- action：idle/walk/greet/bow；坐着结束动作回到sit。
- sit：seat为0..29；原6个中心坐垫索引不变，另外每殿2座。服务端校验1.2单位接近距离与占位互斥。
- stand：释放坐垫。
- offer：hall须为12殿静态白名单、未坐且距门前approach不超过2.5单位；只广播greet与offeringHall、朝向殿内。移动、起身或切其他动作会清除。不是付费、真实供奉服务或功德记录。

殿堂ID/坐标与H5 quietPlaces.ts一致：han-main、han-guanyin、han-wenshu、tibetan-buddha、tibetan-guanyin、tibetan-tara、theravada-buddha、theravada-reclining、dao-sanqing、dao-caishen、dao-zhenwu、mazu-tianhou。

断开连接即移除人物并释放座位，重连获得新的临时人物ID；浏览器Cookie身份与短期房间人物身份分开。每条WS消息上限1KiB，操作额度每秒补充30次、最多40次；未知字段、无效数值、越界、抢占和速度异常均拒绝。慢客户端仅保留最新快照，ping和写入均有超时。

## 检查

```sh
go test -race ./...
./local-stack.sh deps
./local-stack.sh test
QUIET_TEST_WSS=1 ./local-stack.sh test
```

普通测试覆盖真实双客户端同步、21连接自动分为20+1、Origin/身份/CSRF拒绝、座位竞争、殿堂距离、非法输入、速度频率限制和幂等保存。集成测试实际连接独立MySQL/Redis，验证SQL连接及应用重建后记录保留、两个Cookie的读删隔离、Redis owner/人数/快照/TTL；WSS测试实际穿过专用nginx并用测试进程TLS信任池校验证书，完成访客、WSS和记录CRUD链路。

这些检查证明本地集成行为；不证明公网、现有账号、移动真机或20人持续性能验收。凭据、数据库卷与证书属于运行和恢复材料，不应加入源码包或归档截图。

## 连续同步检查

`QUIET_TEST_SOAK=1 GOWORK=off go test -run '^TestLocalTwentyClientSoak$' -v -timeout 3m` 在独立随机loopback监听器和内存hub中运行20客户端90秒，不接公共预览或数据库。停止移动后要求所有客户端看到全部玩家最终姿态；中间状态允许被最新快照合并，缺失统计及仅已观察样本的延迟会明确输出。默认普通测试跳过90秒负载，但会运行收敛谓词测试。该结果不等同20设备、WSS／持久化或公网容量验收。

## 生产 beta 配置

独立 CI 使用 `GOWORK=off` 构建 quiet scope，不更改原20服务的 workspace 或输入指纹。生产只接收 CI 编译的 Linux amd64 二进制；首装运行层、数据库与反向代理由审阅后的运维脚本配置，见 `scripts/ci/README.md`。

- `QUIET_PUBLIC_ORIGINS` 显式填写以逗号分隔的精确 HTTPS origin，拒绝路径、凭据和通配符；不配置时只保留开发入口。
- `QUIET_REDIS_PREFIX` 默认 `quiet-park-local:`；生产固定 `quiet-park-prod:`，指向独立、有密码、无宿主端口的 Redis。MySQL使用新 `quiet_local` 库和仅有该库权限的 quiet 用户。
- `QUIET_AUTH_URL` 生产使用已核验的 Auth origin；HTTP(S)证书校验不放宽。协议沿用 guest-local/account-local mode值以保持已有客户端兼容，local字样不表示数据存在浏览器。
- 公网仅经同域 Nginx 到 loopback18196；跨域拒绝。beta防滥用阈值为每IP40/全园400条WS，消息仍按连接限频；记录POST每IP20次/分钟、全园60次/分钟，突发30/60，超额429且客户端保留设备副本。阈值由固定key跨站点别名共享，不代表400用户性能已验收。`/quiet-health`只允许loopback，避免公开存储Ping。

本地测试/模拟账号通过与真实生产账号、设备、容量验收分开记录。持久历史不会自动删除，持续增长需按正常磁盘运维管理；没有自动访客合并或跨进程房间接管。
