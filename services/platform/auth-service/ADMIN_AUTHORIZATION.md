# 平台自定义角色与实时授权

本轮为兼容增量。`platform_super` 保留超级管理员权限；寺院、师傅、商城和客服内置角色保留旧作用域。自定义角色固定 `authz_scope=platform`，创建后没有任何平台权限；不根据角色名称、客户端请求头或旧的十个 provider 权限编码推断权限。

## 数据与发布顺序

1. 显式执行 `scripts/db/20261022_admin_rbac.sql`。新增 `role.authz_scope`、`role.authz_version`、`admin_account.authz_version`；仅新增 permission 目录，**不为任何角色赋权，不改变账号角色、业务绑定或凭据**。该 SQL 不由旧初始化脚本自动执行。
2. 执行独立审计迁移 `scripts/db/20261022_admin_operation_log.sql`，先发布 auth、audit 与受共同鉴权代码影响的服务，再发布 gateway、前端。
3. audit 的 `Auth.AccessSecret` 使用现有 JWT 密钥的受保护配置映射。gateway 优先使用可选 `Security.AuthTarget`、`Security.AuditTarget`（内部直连地址），否则从现有 `/api/v1/auth`、`/api/v1/admin/audit` 的静态 Upstream.Target 解析；缺失任一目标拒绝启动。`Security.TimeoutMs` 默认 3000。

最小数据库增量授权（不包含服务原有业务权限）：迁移部署账号需要 auth 两张表 ALTER、permission INSERT 和 role scope UPDATE；运行时 auth 需要 `role` SELECT/INSERT/UPDATE、`permission` SELECT、`role_permission` SELECT/INSERT/DELETE、`admin_account` SELECT/INSERT/UPDATE，以及原有身份、寺院绑定和会话功能的既有权限。audit 仅新增 `operation_log` SELECT/INSERT/UPDATE。gateway 不获得数据库授权。示例授权范围应落到现有服务数据库用户，不能直接用通配数据库账号替代。

回退时先回退前端/gateway，再回退 auth/common 代码，保留新增列、权限目录和历史审计表。不要删除审计数据，不恢复旧会话，不执行账户或凭据批量回写；再次前滚可重复执行增量 SQL。

## API 与会话

- `GET /api/v1/auth/access`：验签、SID/epoch、实时账号启用状态、provider 绑定资格、账号和角色版本。返回本人安全授权快照，不返回登录名、邮箱、密码、密钥或令牌。
- `GET /api/v1/admin/auth/permissions`：返回可授能力和不可委派目录（`assignable=false`），不把旧 provider 权限作为平台能力。
- `GET /api/v1/admin/auth/roles/:id/permissions`：`roleId/revision/permissionCodes/editable`。
- `PUT /api/v1/admin/auth/roles/:id/permissions`：请求 `revision/permissionCodes`，仅已验签且实时有效的超级管理员；角色行锁与版本 CAS、权限映射替换在同一事务中。内置角色不可编辑授权；未知、重复、敏感能力或缺少对应查看权限的管理授权拒绝。版本冲突返回 40916，不覆盖别人修改。
- 账号角色/绑定/启停、入驻转角色和账号激活/撤回路径同步递增账号版本。最后一个启用超级管理员不能被停用或改为其他角色（40917）；事务锁定固定 super 角色行，避免并发绕过。
- Access/refresh 都携带签名角色ID、账号版本、角色版本、scope；仅 access 携带 capability。refresh 不携带角色列表、权限列表或客户端自报范围，续期重新读取身份和授权。
- gateway 每个已验签 human admin 请求（包括文件下载与 `/auth/access`）直接调用 auth 自省；不经过 gateway，不缓存旧授权，失败关闭。refresh 也比较相同版本，撤权后旧 refresh 无法恢复权限。原 SID/epoch 与密码撤销保留。旧内置版本零令牌仅在实时账号/角色版本仍为零且访问角色匹配时兼容；自定义角色不能使用旧无版本令牌。

## 可委派范围

权威方法与路径为 `common/adminpolicy.Routes` 的有限静态清单；新 API 不会因共享前缀自动被授予。每个资源有 `.read` 与 `.write`；管理能力保存时必须同时选择查看能力。

| 资源代码 | 真实边界 |
| --- | --- |
| `platform.temples` | 平台寺院档案只读、运营状态；入驻及历史初审终审仍仅超级管理员 |
| `platform.masters` | 平台师傅档案只读、运营状态、咨询和服务标签；创建、身份宗派资料修改、资质审核仍仅超级管理员 |
| `platform.users` | 信众列表、详情、启停 |
| `platform.products` | 商品、分类与SKU的既有管理 API |
| `platform.orders` | 商城订单、退货查看、发货与退货审核/收货；退款执行不委派 |
| `platform.diy` | 材料、设计审核、DIY订单及历史附加服务；创作返现、提现不委派 |
| `platform.points` | 积分商品、订单及报表；管理 API 的既有业务校验继续生效 |
| `platform.logistics` | 物流目录、模板与轨迹 API；未接入真实轨迹时同步只返回明确不可用，不生成假轨迹 |
| `platform.finance` | 既有结算/提现审核与收支汇总、配置；未接入真实出款的处理接口仍明确拒绝，不能凭此授权完成真实出款 |
| `platform.marketing` | 广告、活动、优惠券和积分活动奖品 |
| `platform.moderation` | 举报查询/处理、敏感词查询/新增/删除；通用审核队列（含寺院、师傅资料）、社区评价及素材审核仍仅超级管理员 |
| `platform.operations.read` | 真实操作日志只读 |

超级管理员专有：账号/角色授权、标准目录及首页分类展示、AI配置/密钥、入驻审核、全平台服务监管、成长体系、创作返现及提现、数据备份、通用审核队列、社区评价及素材/短视频管理、退款执行、独立模拟打款入口。内置 provider 的私有 `/admin/temples/*`、`/admin/masters/*` 不属于自定义平台权限；不会因授予平台同名档案能力获得寺院或师傅身份。

## 审计故障边界

gateway 仅审计已验签 human admin 写请求；先由内部 audit-service 持久化 pending 后执行业务。写前记录失败阻止业务；写后最终审计失败仍保留原业务响应及 `X-Admin-Audit-Pending:true`，前端提示已提交、不要重复提交。无资金操作自动重试。HTTP200 的非零业务码记录 failed/denied；无法确认业务码用 `unknown` 和 `businessCode=-1`，不能伪造成功或失败。

审计字段只包含随机 requestId、数字 actorId、角色代码、静态动作/路径模板、安全限长业务ID与结果；不收集 request/response body、query、登录名、邮箱、密码、密钥、token、cookie。内部 start/finish 使用 issuer/client 均为 gateway-service 的签名 service token，公开 gateway 无该路由代理。

## 本地验证责任

核心单元测试覆盖有限方法边界、签名scope/能力与伪造请求头、provider隔离、敏感模块不可委派、Access及refresh版本撤销、权限校验、审计写前阻断/写后保留原响应及pending、业务失败/拒绝、脱敏、断连后的最终审计。

数据库 CAS、最后超级管理员并发保护、真实 HTTP 登录/授权/撤权、配置合并与发布、页面行为验收需由任务整体隔离集成与 ECS 验收记录分别证明。源码或单元测试不等于线上验收通过。

持久事务集成测试：设置 `AUTHORIZATION_TEST_DSN` 后执行 `go test -race ./services/platform/auth-service/internal/logic -run TestAuthorizationMySQLTransactions`。该测试仅接受独立空库 `askxuan_admin_test_auth_tx`，拒绝已有数据，创建必要最小表，仅清理其固定测试行；覆盖同修订并发授权、部分失败回滚、两个超级管理员并发停用。未设置环境变量时明确跳过，不应写成真实 MySQL 已通过。

示意最小增量 GRANT（由部署操作者替换私网服务账户；不是创建新账号脚本）：

```sql
GRANT SELECT, INSERT, UPDATE ON askxuan_auth.role TO '<auth-user>'@'<private-host>';
GRANT SELECT ON askxuan_auth.permission TO '<auth-user>'@'<private-host>';
GRANT SELECT, INSERT, DELETE ON askxuan_auth.role_permission TO '<auth-user>'@'<private-host>';
GRANT SELECT, INSERT, UPDATE ON askxuan_auth.admin_account TO '<auth-user>'@'<private-host>';
GRANT SELECT, INSERT, UPDATE ON askxuan_audit.operation_log TO '<audit-user>'@'<private-host>';
```

这些权限只说明本轮新增授权链路使用的表。身份、账号注册和 provider 绑定流程的原有最小权限需要保留；不要用上面片段覆盖其完整既有授权。

若回退到不支持版本检查的旧后端，必须在撤掉版本守卫前通过既有 SID/epoch 机制撤销受影响管理员会话，避免被撤销的旧 Access/refresh 在旧代码下重新有效。保留新列本身不能代替会话撤销。该回退步骤不在本轮正常发布中自动执行，也不影响普通信众会话。

## 消费者会话的实时状态（2026-10-22）

- 上线前先执行 `scripts/db/20261022_user_session_revision.sql`。用户状态每次实际变化与 `user.authz_version + 1` 同一语句提交；重复设置相同状态不递增。不改用户 ID、凭据、订单或资产。
- 信众 access / refresh 都签入 `accountVersion`。`GET /api/v1/auth/access` 兼容现有管理端返回结构，并支持 `user/customer` 身份：每次直接读取真实用户状态和版本，不缓存。账号恢复不会让旧 access / refresh 复活；登录也在签发前复查版本，防止延迟登录跨过封禁边界。
- 网关对全部带 user/admin 令牌的请求执行实时自省，包括资料、订单、上传、下载、IM 续签以及公开内容的个性化读取；不带令牌的公开读取与合法内部服务调用保持原边界。40105 表示会话已永久失效，客户端清除凭据、不再次刷新。数据库、Redis 或认证依赖故障时拒绝请求，返回依赖不可用，不把临时故障伪装成账号封禁。
- 未结束的人类请求每 3 秒重验，单次查询总超时 2 秒，最迟 5 秒取消请求上下文并关闭反向代理流。短请求结束即停止监测，没有额外重验查询。已发送的 SSE / 下载不追加 JSON 错误，已经完成的操作不回滚。
- `/auth/im-token` 即使直接访问认证服务也检查真实 SID 和账号状态。OpenIM 使用独立凭据，现有外部连接的发送资格由业务消息入口与回调验证；不能把主站会话撤销声称为所有外部连接均已强退。
- `platform_service` 仅供内部 `userType=service` 的签名令牌使用，不能通过人工账号登录、续期或自省建立人类会话。

验收分层：真实 MySQL / Redis SID 与版本测试、真实反向代理 SSE 终止测试均在专用隔离环境执行；浏览器及目标 ECS 验收由发布记录另行列明。
