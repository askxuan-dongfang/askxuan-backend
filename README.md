# askXuan-backend Go 微服务

基于 **Go 1.22 + go-zero** 微服务框架构建，注册中心 etcd，统一响应格式 `{code,message,data}`。

> 配套文档：`askXuan-docs/docs/guides/Go后端指南.md`（go-zero 入门）、`askXuan-docs/docs/standards/统一数据字典.md`（数据规范）、`askXuan-docs/.trae/specs/pivot-to-go-and-mobile-strategy/spec.md`（架构决策）

## 目录结构

按 **5 大业务域** 分组，共 19 个下游业务服务 + 1 个网关 + 1 个公共模块（21 个 Go module）。

```
askXuan-backend/
├── go.work                      # Go workspace 多模块工作区
├── Makefile                     # 便捷命令（启动/编译/测试/lint/docker）
├── .golangci.yml                # 14 个 linter 配置
├── .github/workflows/ci.yml     # GitHub Actions CI（lint/build/test/vet）
├── docker-compose.yml           # 基础设施（MySQL/Redis/RabbitMQ/MinIO/etcd）
├── docker-compose.full.yml      # 本地 Docker 全量后端服务（20 个 Go 服务）
│
├── common/                      # 公共模块（JWT/错误码/响应封装/中间件）
│   ├── response.go              # {code,message,data} 统一响应体
│   ├── errorcode.go             # 错误码常量与 BizError（40001-50299）
│   ├── jwt.go                   # JWT 签发/校验（Access 2h + Refresh 7d）
│   └── middleware/              # CORS / Auth / AdminAuth 中间件
│
├── services/                    # 业务服务（按业务域分组）
│   ├── platform/                # ① 平台域
│   │   ├── gateway-service/     #   API 网关 (8080) - 反向代理 + JWT + CORS
│   │   ├── auth-service/        #   认证服务 (8081) - login/refresh/logout + 角色权限
│   │   └── user-service/        #   用户服务 (8082) - register/profile + 地址/画像
│   │
│   ├── content/                 # ② 内容域
│   │   ├── temple-service/      #   寺院服务 (8083) - 寺院/图片/入驻/加持任务
│   │   ├── master-service/      #   法师服务 (8084) - 法师/排班/资质/加持任务
│   │   ├── booking-service/     #   预约服务 (8085) - 预约/状态流转/评价
│   │   ├── review-service/      #   评价服务 (8092) - 评价/回复/举报
│   │   └── community-service/   #   大师广场 (8099) - 帖子/评论/点赞/审核
│   │
│   ├── commerce/                # ③ 商城域
│   │   ├── product-service/     #   商品服务 (8086) - 商品/SKU/分类
│   │   ├── diy-service/         #   DIY服务 (8088) - DIY设计/材料/加持派发
│   │   ├── order-service/       #   订单服务 (8089) - 商城订单/退换货
│   │   └── payment-service/     #   支付服务 (8090) - 支付/退款/对账
│   │
│   ├── operation/               # ④ 运营域
│   │   ├── finance-service/     #   财务服务 (8091) - 结算/提现/抽成
│   │   ├── audit-service/       #   审核服务 (8093) - 内容审核/举报处理
│   │   ├── logistics-service/   #   物流服务 (8095) - 快递/运费/追踪
│   │   └── marketing-service/   #   营销服务 (8096) - 优惠券/活动/Banner
│   │
│   └── infrastructure/          # ⑤ 基础设施域
│       ├── message-service/     #   消息服务 (8094) - 站内消息 + MQ 消费
│       ├── file-service/        #   文件服务 (8097) - MinIO 上传/预签名
│       ├── ai-service/          #   AI服务 (8098) - AI问事/7技能对话
│       └── media-service/       #   媒体服务 (8100) - 上传/回调/直播房间
│
├── build/docker/Dockerfile      # 多阶段构建通用 Dockerfile
├── scripts/                     # 运维脚本
│   ├── deploy.sh                # 单服务 Docker 构建
│   ├── docker-build-all.sh      # 批量构建所有镜像
│   └── migrate.sh               # 数据库迁移
├── envs/                        # 环境配置
│   ├── dev.env                  # 开发环境
│   └── prod.env                 # 生产环境模板
└── db/init.sql                     # 数据库全量初始化
```

## 服务清单与端口

| 业务域 | 服务 | 目录 | 端口 | 职责 |
| --- | --- | --- | --- | --- |
| 平台 | gateway-service | services/platform/gateway-service | 8080 | API 网关：路由转发 + JWT 鉴权 + CORS |
| 平台 | auth-service | services/platform/auth-service | 8081 | JWT 签发/续期/登出 + 管理台账号/角色/权限 |
| 平台 | user-service | services/platform/user-service | 8082 | 用户注册/资料/地址/画像 |
| 内容 | temple-service | services/content/temple-service | 8083 | 寺院/图片/服务/入驻/加持任务 |
| 内容 | master-service | services/content/master-service | 8084 | 法师/排班/资质/加持任务 |
| 内容 | booking-service | services/content/booking-service | 8085 | 预约/状态流转/评价/时段 |
| 内容 | review-service | services/content/review-service | 8092 | 评价/回复/举报 |
| 内容 | community-service | services/content/community-service | 8099 | 大师广场/评论/点赞/审核 |
| 商城 | product-service | services/commerce/product-service | 8086 | 商品/SKU/分类/上下架 |
| 商城 | diy-service | services/commerce/diy-service | 8088 | DIY设计/材料库/加持任务派发 |
| 商城 | order-service | services/commerce/order-service | 8089 | 商城订单/退换货 |
| 商城 | payment-service | services/commerce/payment-service | 8090 | 支付/退款/对账 |
| 运营 | finance-service | services/operation/finance-service | 8091 | 结算/提现/抽成配置 |
| 运营 | audit-service | services/operation/audit-service | 8093 | 内容审核/举报处理/敏感词 |
| 运营 | logistics-service | services/operation/logistics-service | 8095 | 快递/运费模板/物流追踪 |
| 运营 | marketing-service | services/operation/marketing-service | 8096 | 优惠券/活动/Banner/推荐位 |
| 基础设施 | message-service | services/infrastructure/message-service | 8094 | 站内消息/推送/模板 |
| 基础设施 | file-service | services/infrastructure/file-service | 8097 | MinIO 文件上传/预签名 |
| 基础设施 | ai-service | services/infrastructure/ai-service | 8098 | AI问事/7技能对话 |
| 基础设施 | media-service | services/infrastructure/media-service | 8100 | 媒体上传/处理回调/直播房间 |

## 环境准备

1. **安装 Go 1.22+**：`brew install go`
2. **配置代理**（国内必须）：`export GOPROXY=https://goproxy.cn,direct`
3. **启动基础设施**（项目根目录）：
   ```bash
   docker compose up -d
   ```
   启动 MySQL 8(3306) / Redis 7(6379) / RabbitMQ(5672) / MinIO(9000) / etcd(2379)
4. **初始化数据库**：
   ```bash
   make db-init
   ```

### 已有数据库升级

全新环境使用 `make db-init`。已有数据库按需求落地顺序执行幂等前向迁移，再刷新服务账户授权：

```bash
for migration in \
  scripts/db/20260713_belief_codes.sql \
  scripts/db/20260713_intention_hub.sql \
  scripts/db/20260713_ai_persistence.sql \
  scripts/db/20260713_diy_design_order_pricing.sql \
  scripts/db/20260713_media_live.sql \
  scripts/db/20260713_community.sql \
	  scripts/db/20260715_booking_payment_slots_grpc.sql \
  scripts/db/20260715_seed_data_consistency.sql; do
  docker exec -i askxuan-mysql mysql -uroot -proot123 < "$migration"
done
docker exec -i askxuan-mysql mysql -uroot -proot123 < scripts/db/microservice-migration.sql
```

`20260713_belief_codes.sql` 会把默认库中的寺院和法师存量数据同步到服务分库，再补充一级流派字段。`20260715_booking_payment_slots_grpc.sql` 会迁移结构化时段、预约计价/支付快照和支付幂等键；`20260715_seed_data_consistency.sql` 会统一演示用户 ID、修复旧 DIY 金额、补齐寺院服务目录及缺失唯一索引。以上脚本均可重复执行。

## 闭环测试

```bash
# 前置：启动基础设施 + 初始化数据库 + 启动所有服务
docker compose up -d
make db-init
make start-all

# MVP-1 预约祈福闭环（10 步）：注册→登录→寺院/法师→预约→消息→管理台确认
bash scripts/test-mvp1-closed-loop.sh

# 预约权威计价、模拟支付、容量防超卖与 gRPC/响应丢失恢复（10 项）
RUN_DISRUPTION=1 bash scripts/test-booking-payment-closed-loop.sh

# MVP-2 DIY 定制闭环（16 步）：材料→设计→DIY订单→支付→加持→发货→物流→完成
bash scripts/test-mvp2-diy-closed-loop.sh

# MVP-2 商城交易闭环（11 步）：商品→订单→支付→发货→物流→确认收货
bash scripts/test-mvp2-trade-closed-loop.sh

# MVP-3 评价闭环（9 步）：评价创建→列表→详情→回复→举报→处理
bash scripts/test-mvp3-review-closed-loop.sh

# MVP-3 营销闭环（11 步）：Banner/活动/优惠券/推荐位全链路
bash scripts/test-mvp3-marketing-closed-loop.sh

# MVP-3 财务闭环（10 步）：总览/结算/提现/抽成配置/报表
bash scripts/test-mvp3-finance-closed-loop.sh

# MVP-3 审核闭环（10 步）：审核队列/通过/驳回/举报/敏感词/统计
bash scripts/test-mvp3-audit-closed-loop.sh

# App 改进：信仰流派专题、筛选和管理闭环
bash scripts/test-app5-belief-closed-loop.sh

# App 改进：诉求聚合商品/寺院服务混排闭环
bash scripts/test-app6-intention-closed-loop.sh

# App 改进：AI 默认会话、所有权、异步回复和重启恢复闭环
bash scripts/test-app1-ai-closed-loop.sh

# App 改进：设计广场服务端计价、事务、支付后作者收益闭环
bash scripts/test-mvp2-diy-closed-loop.sh

# App 改进：媒体/直播基础闭环
bash scripts/test-mvp4-media-live-closed-loop.sh

# App 改进：大师广场发布、审核、互动闭环
bash scripts/test-mvp6-community-closed-loop.sh
```

> 六项 App 改进均使用 MySQL 持久化。AI、媒体、社区闭环脚本还覆盖服务重启、真实 MinIO 对象与审核可见性；脚本失败应按真实回归处理，不使用内存重置规避。

## 快速启动

所有 `make` 命令均在 `askXuan-backend/` 根目录执行。

### Docker 一键启动（推荐）

完整后端栈包含两组：

- `askxuan`：MySQL / Redis / RabbitMQ / MinIO / etcd + 20 个 askXuan Go 服务
- `open-im-server-383`：OpenIM 的 MongoDB / Redis / Kafka / MinIO / etcd / Web Front / Admin Front，以及本机 OpenIM 服务进程

```bash
# 推荐：一键启动完整后端栈（先做端口预检）
make stack-up

# 停止完整后端栈（保留 Docker volume 和 OpenIM 本地数据）
make stack-down

# 健康检查
make stack-check

# 只启动 askXuan 这一组：中间件 + 首次初始化数据库 + 20 个后端服务
make docker-up

# 查看容器状态
make docker-ps

# 查看全部日志，或只看网关日志
make docker-logs
make docker-logs SVC=gateway-service

# 停止全部容器（保留数据卷）
make docker-down
```

完整栈端口已错开：askXuan 使用 `3306/6379/5672/15672/9000/9001/2379/2380/8080-8100/9088`（服务端口未连续占满）；
OpenIM 使用 `37017/16379/12379/12380/19094/10005/19090/11001/11002/10001/10002`。
`make stack-preflight` 会在启动前检查端口占用；端口已被对应容器占用视为正常，未知进程占用会直接报错。

`make docker-up` 会先生成 `.docker/etc/*` 容器配置，把本机配置里的
`localhost/127.0.0.1` 改成 Docker 网络内的 `mysql/redis/rabbitmq/minio/etcd`
以及各服务名，避免容器重启后服务互相找不到。数据库只在首次检测不到
`askxuan.temple` 表时初始化；后续重启保留 Docker volume 数据，需要清库时手动执行 `make db-reset`。

askXuan 容器访问 OpenIM REST API 走 `host.docker.internal:10002`，避免把两套 compose 网络强行合并造成容器名和端口冲突。

```bash
# 拉取依赖
make tidy

# 启动单个服务
make start-gateway
make start-auth
make start-temple
# ... 其他同理（见 make help）

# 并发启动所有服务（后台运行，日志在 logs/）
make start-all
tail -f logs/gateway.log

# 停止所有服务
make stop-all

# 编译所有服务
make build

# 运行单元测试
make test

# 代码检查
make vet
make lint

# 格式化
make fmt
```

## Docker 构建

```bash
# 构建单个服务
make docker-build SVC=auth-service TAG=v1.0.0

# 构建所有服务
make docker-build-all TAG=v1.0.0

# 或使用脚本
./scripts/deploy.sh auth-service v1.0.0
./scripts/docker-build-all.sh v1.0.0
```

## 联调验证

### 1. 登录闭环（auth + gateway + user）

```bash
# 登录（手机号 13800138000，验证码 1234 或 密码 123456）
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800138000","code":"1234"}'

# 返回 {code:0, data:{accessToken, refreshToken, userInfo}}
# 用返回的 accessToken 访问受保护接口：
curl http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer <accessToken>"
```

### 2. 浏览闭环（temple / master，无需鉴权）

```bash
# 寺院列表（按宗派筛选）
curl "http://localhost:8080/api/v1/temples?sect=禅宗&page=1&size=20"
# 法师列表
curl "http://localhost:8080/api/v1/masters?type=佛教"
# 寺院详情
curl http://localhost:8080/api/v1/temples/T001
```

### 3. 预约闭环（booking + message 经 RabbitMQ 联动）

```bash
# 先查询指定日期的权威价格和剩余时段
curl "http://localhost:8080/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=2026-08-10"

# 创建预约（需带 token；userId 和价格由服务端确定）
curl -X POST http://localhost:8080/api/v1/bookings \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"requestId":"client-uuid","templeId":"T001","masterId":"M001","serviceId":"S001","bookingDate":"2026-08-10","slotCode":"slot-1","meritMoney":200,"meritMoneyTier":"大额"}'
# 本地 mock 支付成功后预约进入 pending，message-service 收到 booking.events 并生成站内消息

# 用户只可取消自己的预约，寺院/法师确认使用管理端接口
curl -X PUT http://localhost:8080/api/v1/bookings/B20260630001/status \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"status":"cancelled"}'

# 查询站内消息
curl "http://localhost:8080/api/v1/messages?userId=1001"
```

### 4. 文件上传（file + MinIO）

```bash
# 获取预签名上传 URL（前端直传）
curl "http://localhost:8080/api/v1/files/presigned?fileName=test.jpg&objectType=temples"

# 后端代传上传
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer <token>" -F "file=@/path/to/image.jpg"
```

## 关键设计说明

### 统一响应格式
所有接口返回 `{code:0, message:"success", data:...}`，错误时 `code` 非 0。封装在 `common/response.go` 的 `Ok` / `JsonError`。错误码范围 `40001-50299`。

### JWT 鉴权链路
- **auth-service** 签发 Access(2h) + Refresh(7d) Token
- **gateway-service** 全局鉴权中间件校验 Access Token，校验通过后将 `userId/mobile/userType` 注入 `X-User-Id` 等请求头透传下游
- **白名单**：`/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/users/register`
- **登出**：将 Access Token 写入 Redis 黑名单（key `jwt:blacklist:<token>`，TTL=剩余有效期）
- **JWT Claims** 包含：`userId`、`mobile`、`userType`、`roles`、`clientId`、`templeId`、`masterId`、`type`（access/refresh）；标准字段 `sub` 用于标记 token 类型（access/refresh），`exp`/`iat` 由框架填充

### 预约状态流转
`pending → confirmed → in_progress → completed → reviewed`，任意中间态可 `→ cancelled`。终态为 `reviewed` 和 `cancelled`。流转校验在 `booking-service/internal/model/booking.go` 的 `CanTransit`。

### 支付状态流转
`pending → paid → refunded`，终态为 `refunded`。流转校验在 `payment-service/internal/model/payment.go` 的 `CanPaymentTransit`。

### RabbitMQ 事件（9 个 fanout 交换机）
所有交换机均有生产者和消费者，确保异步链路闭环。采用 fanout 模式，按业务域一个交换机聚合该域所有事件：
- `booking.events` — 预约通知与状态变更（消费者：message-service、finance-service）
- `blessing.events` — 加持任务派单/接单/完成（消费者：master-service、diy-service、temple-service、message-service）
- `order.events` — 商城订单状态变更（消费者：finance-service、message-service）
- `payment.events` — 支付结果通知（消费者：order-service、finance-service、message-service）
- `logistics.events` — 物流同步（消费者：message-service）
- `review.events` — 评价通知（消费者：temple-service、message-service）
- `audit.events` — 审核结果（消费者：message-service）
- `finance.events` — 提现审核结果（消费者：message-service）
- `ai.events` — AI 推理完成（消费者：ai-service 自消费）

## Mock 账号

### 管理台账号（用户名 + 密码）

| 用户名 | 密码 | 角色 |
| --- | --- | --- |
| admin | 123456 | 平台管理员 |
| lingyin_admin | 123456 | 寺院管理员（灵隐寺） |

### C 端 / 法师账号（手机号 + 验证码）

| 手机号 | 验证码 | 密码 | 角色 |
| --- | --- | --- | --- |
| 13900000001 | 1234 | 123456 | C 端用户 |
| 13900000002 | 1234 | 123456 | C 端用户 |
| 13800138001 | 1234 | 123456 | 法师（智海法师） |
| 13800138002 | 1234 | 123456 | 平台管理员（手机号） |
| 13800138000 | 1234 | 123456 | C 端用户（善信居士） |

## 依赖说明

每个服务独立 `go.mod`，通过 `go.work` 聚合。`github.com/askxuan/common` 通过 `replace` 指向本地 `../../../common`（服务位于 `services/<域>/<服务>/` 三级目录）。

核心依赖：`go-zero v1.7.2`、`golang-jwt/v5`、`rabbitmq/amqp091-go`、`minio-go/v7`。

## 工程化能力

| 能力 | 文件 | 说明 |
| --- | --- | --- |
| 代码规范 | `.golangci.yml` | 14 个 linter（errcheck/govet/staticcheck/goimports 等） |
| CI/CD | `.github/workflows/ci.yml` | lint + build + test + vet 四阶段 |
| 容器化 | `build/docker/Dockerfile` | 多阶段构建，参数化 SERVICE/BINARY |
| 运维脚本 | `scripts/*.sh` | 部署 / 批量构建 / 数据库迁移 |
| 环境隔离 | `envs/{dev,prod}.env` | 开发/生产环境配置分离 |
| 单元测试 | `*_test.go` | common / booking / payment 状态机测试 |
