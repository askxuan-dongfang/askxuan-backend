# askXuan-backend Go 微服务

基于 **Go 1.22 + go-zero** 微服务框架构建，注册中心 etcd，统一响应格式 `{code,message,data}`。

> 配套文档：`askXuan-docs/docs/guides/Go后端指南.md`（go-zero 入门）、`askXuan-docs/docs/standards/统一数据字典.md`（数据规范）、`askXuan-docs/.trae/specs/pivot-to-go-and-mobile-strategy/spec.md`（架构决策）

## 目录结构

按 **5 大业务域** 分组，共 18 个服务 + 1 个网关 + 1 个公共模块（19 个 Go module）。

```
askXuan-backend/
├── go.work                      # Go workspace 多模块工作区
├── Makefile                     # 便捷命令（启动/编译/测试/lint/docker）
├── .golangci.yml                # 14 个 linter 配置
├── .github/workflows/ci.yml     # GitHub Actions CI（lint/build/test/vet）
├── docker-compose.yml           # 基础设施（MySQL/Redis/RabbitMQ/MinIO/etcd/Zookeeper/Kafka/MongoDB）
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
│   │   └── review-service/      #   评价服务 (8092) - 评价/回复/举报
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
│       └── ai-service/          #   AI服务 (8098) - AI问事/7技能对话
│
├── build/docker/Dockerfile      # 多阶段构建通用 Dockerfile
├── scripts/                     # 运维脚本
│   ├── deploy.sh                # 单服务 Docker 构建
│   ├── docker-build-all.sh      # 批量构建所有镜像
│   └── migrate.sh               # 数据库迁移
├── envs/                        # 环境配置
│   ├── dev.env                  # 开发环境
│   └── prod.env                 # 生产环境模板
└── db/init.sql                     # 数据库初始化（69 表 / 16 域）
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

## 环境准备

1. **安装 Go 1.22+**：`brew install go`
2. **配置代理**（国内必须）：`export GOPROXY=https://goproxy.cn,direct`
3. **启动基础设施**（项目根目录）：
   ```bash
   docker compose up -d
   ```
   启动 MySQL 8(3306) / Redis 7(6379) / RabbitMQ(5672) / MinIO(9000) / etcd(2379) / Zookeeper(2181) / Kafka(9092) / MongoDB(27017)
4. **初始化数据库**：
   ```bash
   make db-init
   ```

## 闭环测试

```bash
# 执行 MVP-1 预约祈福闭环测试
docker compose up -d
make db-init
make start-all
bash scripts/test-mvp1-closed-loop.sh
```

## 快速启动

所有 `make` 命令均在 `askXuan-backend/` 根目录执行。

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
curl http://localhost:8080/api/v1/user/profile \
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
# 创建预约（需带 token）
curl -X POST http://localhost:8080/api/v1/booking \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"userId":"1001","templeId":"T001","masterId":"M001","serviceId":"S001","bookingDate":"2026-07-10","timeSlot":"09:00-10:00","meritMoney":200,"meritMoneyTier":"大额"}'
# 创建成功后，message-service 会收到 booking.events 事件并生成站内消息

# 状态流转：pending → confirmed
curl -X PUT http://localhost:8080/api/v1/booking/B20260630001/status \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"status":"confirmed"}'

# 查询站内消息
curl "http://localhost:8080/api/v1/message?userId=1001"
```

### 4. 文件上传（file + MinIO）

```bash
# 获取预签名上传 URL（前端直传）
curl "http://localhost:8080/api/v1/file/presigned?fileName=test.jpg&objectType=temples"

# 后端代传上传
curl -X POST http://localhost:8080/api/v1/file/upload \
  -H "Authorization: Bearer <token>" -F "file=@/path/to/image.jpg"
```

## 关键设计说明

### 统一响应格式
所有接口返回 `{code:0, message:"success", data:...}`，错误时 `code` 非 0。封装在 `common/response.go` 的 `Ok` / `JsonError`。错误码范围 `40001-50299`。

### JWT 鉴权链路
- **auth-service** 签发 Access(2h) + Refresh(7d) Token
- **gateway-service** 全局鉴权中间件校验 Access Token，校验通过后将 `userId/mobile/userType` 注入 `X-User-Id` 等请求头透传下游
- **白名单**：`/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/user/register`
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

| 手机号 | 验证码 | 密码 | 角色 |
| --- | --- | --- | --- |
| 13800138000 | 1234 | 123456 | C 端用户（善信居士） |
| 13800138001 | 1234 | 123456 | 法师（智海法师） |
| 13800138002 | 1234 | 123456 | 平台管理员 |

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
