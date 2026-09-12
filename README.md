# askXuan-backend · 产品 0.0.1

问玄东方的业务 API、消息协作与发布工具仓库。使用 **Go 1.22、go-zero v1.7.2** 和多模块 workspace；5 个业务域下有 **19 个下游服务 + 1 个网关**，加上 `common` 共 **21 个 Go module**。

当前产品版本为 **0.0.1**，见 [VERSION](VERSION)。日常后端开发使用本仓；H5、iOS、管理台和品牌资产位于同级 `askXuan-frontend`，产品说明、API 契约和运维指南位于同级 `askXuan-docs`。Go 版本、依赖版本、API 路径中的 `v1`、业务记录版本与设备上报的 `appVersion` 保持各自语义，不随产品版本一起改写。

## 从哪里开始

| 需要完成的工作 | 入口 |
| --- | --- |
| 查看当前产品能力与操作方式 | [产品手册目录](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/guides/手册目录.md) |
| 理解 Go 服务分层 | [Go 后端指南](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/guides/Go后端指南.md)；实际目录和命令以本 README、[Makefile](Makefile) 为准 |
| 核对 HTTP 接口和业务字段 | [API Reference](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/API-REFERENCE.md)、[统一数据字典](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/standards/统一数据字典.md) |
| 发布、数据库迁移与回滚 | [GitHub Actions 与 ECS 发布](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/deployment/GITHUB-ACTIONS.md)、[本仓发布工具](scripts/ci/README.md) |
| 聊天、APNs 与公网通话验收 | [聊天发布与验收边界](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/deployment/CHAT.md) |
| 消息可靠性与故障演练 | [MQ 监控与故障演练](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/guides/MQ可靠投递监控与故障演练.md) |

## 目录与数据用途

```text
askXuan-backend/
├── VERSION                      # 产品版本 0.0.1
├── go.work                      # 21 个 module 的 workspace
├── Makefile                     # 开发、编译、检查与本地栈入口
├── common/                      # JWT、错误码、响应、中间件、公共设施
├── services/                    # platform/content/commerce/operation/infrastructure
├── db/init.sql                  # 本地新环境全量初始化与种子数据
├── scripts/db/                  # 分阶段 SQL；包含迁移和演示数据调整
├── scripts/dev/                 # 本地 Docker/OpenIM 启停、配置生成与检查
├── scripts/ci/                  # CI 打包、受限上传、ECS 接收器与测试
├── scripts/ops/                 # 专项部署、配置、导入与运维工具
├── scripts/drills/              # 有副作用的受控故障演练
├── deploy/nginx/                # Nginx 对象访问与 H5 HTML 缓存片段
├── configs/openim/              # OpenIM webhook 源配置
├── build/docker/Dockerfile      # 受 Git 管理的构建定义
├── docker-compose*.yml          # 本地基础设施与 20 个 Go 服务
├── envs/ai.env.example          # 已跟踪的 AI 环境配置示例
├── .docker/etc/                 # 本地生成的容器配置
├── .local/openim/               # OpenIM 下载、运行环境、持久数据及本地备份
└── logs/                       # 宿主服务日志与 pids/ 进程记录
```

`.local/openim` 包含数据库、消息组件数据和恢复材料，不能当作缓存整体清理。`logs` 用于排障，`logs/pids` 还被启停命令读取；**当前 `make clean` 会删除整个 `logs`**，并非只删编译产物。清理前应先停止相关服务、保留所需日志和备份。`build/docker/Dockerfile` 是源码。

配置以各服务的 `etc/*.yaml`、Compose 和实际环境注入为准。仓库没有已跟踪的通用 `envs/dev.env`、`envs/prod.env` 模板；本机存在的环境文件不能直接当作可发布配置或复制到其他环境。

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
| 商城 | diy-service | services/commerce/diy-service | 8088 | DIY设计/材料库/订单/审核与加持派发 |
| 商城 | order-service | services/commerce/order-service | 8089 | 商城订单/退换货 |
| 商城 | payment-service | services/commerce/payment-service | 8090 | 支付/退款/对账 |
| 运营 | finance-service | services/operation/finance-service | 8091 | 结算/提现/抽成配置 |
| 运营 | audit-service | services/operation/audit-service | 8093 | 内容审核/举报处理/敏感词 |
| 运营 | logistics-service | services/operation/logistics-service | 8095 | 快递/运费模板/物流追踪 |
| 运营 | marketing-service | services/operation/marketing-service | 8096 | 优惠券/活动/Banner/积分奖励活动 |
| 基础设施 | message-service | services/infrastructure/message-service | 8094 | 站内消息/OpenIM 接入/聊天权限与模板 |
| 基础设施 | file-service | services/infrastructure/file-service | 8097 | MinIO 文件上传/预签名 |
| 基础设施 | ai-service | services/infrastructure/ai-service | 8098 | AI问事/模型选择/专题报告/供应商配置 |
| 基础设施 | media-service | services/infrastructure/media-service | 8100 | 媒体上传/处理回调/直播房间 |


以上为 HTTP 端口，RPC 另见 [Makefile](Makefile) 中 `RPC_PORTS` 和各服务配置。每个服务独立维护 `go.mod`；[go.work](go.work) 聚合全部模块，服务通过 `replace` 引用本地 `common`。

## 本地启动

需要 Go 1.22+、Docker 与 Compose；网络需要时可配置 `GOPROXY`。以下命令均在本仓根目录执行，使用本地开发配置。

```bash
make help

# 本地完整栈：OpenIM + askXuan 中间件 + 20 个 Go 服务
make stack-preflight
make stack-up
make stack-check

# 状态与日志
make stack-ps
make docker-logs SVC=gateway-service

# 停止完整栈，保留数据卷与 OpenIM 本地数据
make stack-down
```

[stack-up.sh](scripts/dev/stack-up.sh) 会先停止已有的宿主 askXuan 进程，再启动 OpenIM 和 Docker 后端。只需要 askXuan Docker 服务时使用 `make docker-up`；`make docker-down` 保留数据卷。OpenIM 的 REST/WebSocket 以及独立组件端口与 askXuan 分开，完整映射和端口占用检查以 [stack-preflight.sh](scripts/dev/stack-preflight.sh) 为准。

`make docker-up` 调用 [docker-up-all.sh](scripts/dev/docker-up-all.sh)：生成 `.docker/etc` 容器网络配置；首次缺少 `askxuan_temple.temple` 时执行初始化；随后按文件名顺序执行 `scripts/db/20*.sql`，再重建服务容器。这是本地栈流程，包含演示数据调整，不能照搬为生产数据库升级命令。

需要宿主 Go 调试时，先准备本地中间件和数据库，再按需启动服务：

```bash
make start-gateway
# 在其他终端启动所需服务，例如 make start-auth、make start-user

# 或编译并后台启动全部宿主服务，日志输出到 logs/
make start-all
make stop-all
```

宿主 `start-all`/`start-core` 会处理服务端口上的进程，使用前核对本机占用；不要与同端口 Docker 服务同时启动。聊天调试还需要 OpenIM；相关配置和启动入口在 [scripts/dev/openim-up.sh](scripts/dev/openim-up.sh)。

## 数据库、构建与验证

全新本地环境可用 `make db-init` 初始化；已有环境升级前先核对目标 SQL 和数据。`make db-reset` 删除业务库，`scripts/migrate.sh up` 执行的是全量初始化，不是按版本跟踪的生产迁移器。生产迁移遵循部署指南中的备份、审阅和配置合约流程。

```bash
make build
make test-ci
make vet
make lint-ci
python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v

# 与同级文档仓对照接口，包含数据驱动的路由注册
node ../askXuan-docs/scripts/audit-api-contracts.mjs .
```

`make test-ci` 会在任一模块失败时返回非零状态，适合验收。`make test`、`make vet` 和 `make lint` 的现有循环会打印失败后继续，不能只凭命令退出成功判断全部通过；检查模块输出。Lint 配置在 [.golangci.yml](.golangci.yml)，当前 Actions 未运行独立 lint job。

接口静态核对优先使用文档仓的 `audit-api-contracts.mjs`。本仓旧 `scripts/audit-api-reference.mjs` 仅扫描部分固定文件和字面量路由，会漏掉积分、专题报告等注册方式；其“文档过期”输出需回到注册入口确认，不能据此删除 API 文档。这两类检查都不替代真实接口请求验证。

业务联调脚本位于 [scripts](scripts)，运行前读取对应脚本的环境变量、数据库和服务要求：

| 验证范围 | 入口 |
| --- | --- |
| 注册与预约 | [注册](scripts/test-customer-registration.sh)、[预约计价/模拟支付/容量](scripts/test-booking-payment-closed-loop.sh) |
| DIY 与商城履约 | [DIY](scripts/test-mvp2-diy-closed-loop.sh)、[结算](scripts/test-shop-checkout-closed-loop.sh)、[履约](scripts/test-commerce-fulfillment.sh) |
| 积分商城与奖励活动 | [积分商城](scripts/test-points-mall.sh)、[奖励活动](scripts/test-free-rewards.sh) |
| AI 与专题报告 | [Agent](scripts/test-ai-agent-v2-closed-loop.sh)、[图像输入](scripts/test-ai-vision-closed-loop.sh)、[专题报告](scripts/test-ai-topic-reports.sh) |
| 私聊与社区 | [咨询聊天](scripts/test-consultation-chat-closed-loop.sh)、[OpenIM](scripts/test-openim-chat.sh)、[社区](scripts/test-mvp6-community-closed-loop.sh) |
| 媒体与直播基础 | [媒体/直播脚本](scripts/test-mvp4-media-live-closed-loop.sh) |

脚本可能创建数据、启动测试数据库或重启服务，不是只读健康探针。需要故障注入的用例须单独确认测试环境。脚本通过不代表当前生产真实支付、APNs 或公网音视频已经验收；当前转盘/奖池参与会消耗积分，`test-free-rewards.sh` 的文件名不代表免费参与。具体规则以服务和产品手册为准，现金、积分、体验商品与功德记录分别核对。

## API 联调约定

统一响应为 `{code,message,data}`，业务成功使用 `code: 0`。HTTP 200 不等于业务成功；错误定义见 [common/errorcode.go](common/errorcode.go)，响应封装见 [common/response.go](common/response.go)。

通过网关访问业务 API；公开浏览范围由网关配置和 [鉴权中间件](services/platform/gateway-service/internal/middleware/auth.go) 共同决定。受保护接口使用登录返回的 access token；网关校验后透传用户、角色和寺院/法师身份。40101/40102/40103 表示会话相关错误，40104 为登录凭据错误，40105 为 refresh token 失效；权限错误另行处理。客户端续期或返回登录的行为见产品手册。

```bash
curl http://127.0.0.1:8080/api/v1/health
curl http://127.0.0.1:8080/api/v1/users/profile \
  -H 'Authorization: Bearer <accessToken>'
```

测试账号和权限取决于当前环境的种子数据及实际分配，本 README 不维护固定可登录账号表。具体请求字段、状态流转和作用域以 API Reference、对应 handler/logic/model 及目标环境返回为准。

## CI 与 ECS 运维

[当前工作流](.github/workflows/ci.yml) 对 `main`、`develop` 的 push/PR 执行构建、测试与 vet。构建产出 `ecs-release`；仅 `main` 的非 PR 运行且 `ECS_DEPLOY_ENABLED=true` 时进入 production 部署，部署依赖 build/test/vet 完成。工作流也支持手动触发。

日常发布使用 [scripts/ci](scripts/ci/README.md)：CI 编译并记录源码 SHA、祖先和逐文件哈希；受限 SSH 接收器校验原包、串行切换变化组件、做健康检查和失败回滚。生产发布以 CI 原包和接收器状态为依据；Git 推送、CI 结果、实际发布回执和业务验收分别核对。

`./scripts/deploy.sh <service> <tag>` 与 `make docker-build` 只在本地构建镜像，脚本输出的 `docker push` 只是提示，不会自动发布 ECS。`scripts/ops` 保留专项迁移和救援入口，使用前遵循统一发布锁、配置合约及回滚流程。SQL/运行配置变化不能靠放宽合约或执行全量初始化来通过部署。

常用运维入口：`make monitor-runtime` 检查网关、RabbitMQ、容器与 outbox；`make drill-mq-outbox` 是会中断消息组件的测试演练。接收器安装、生产状态路径及恢复步骤见文档仓部署指南。

## 品牌资产与 Nginx 安装清单

正式标识、浅深变体和生成器位于前端 [packages/brand](https://github.com/askxuan-dongfang/askxuan-frontend/tree/master/packages/brand)，由 H5、iOS 和管理端接入。该目录是当前品牌资产入口，后端无需另存一套 Logo。

后端的 [deploy/nginx](deploy/nginx) 和 [refresh-h5-html-cache.sh](scripts/ops/refresh-h5-html-cache.sh) 管理静态站点的部分 Nginx 配置。当前片段同时包含 HTML 缓存规则，以及 `/manifest.webmanifest`、`/master.webmanifest` 两个角色的安装清单规则：明确返回 `application/manifest+json`，要求重新验证缓存，缺失文件返回 404。安装脚本备份原配置，仅迁移已知的旧清单规则，遇到自定义冲突时拒绝覆盖；通过 `nginx -t` 后 reload，错误时回滚。配置安装和业务服务发布是独立操作，执行流程见[部署文档](https://github.com/askxuan-dongfang/askxuan-docs/blob/master/docs/deployment/GITHUB-ACTIONS.md)。
