# 双轨大师认证与寺院入驻

本功能依赖邮箱密码认证迁移 `scripts/db/20260915_email_identity.sql`。C 端普通用户注册保持原流程；独立大师从大师端注册、认证，寺院经办人从寺院管理台注册、入驻。寺院纳管大师由寺院分配账号，在同一大师端激活与登录。

## 入口与身份

| 入口 | 开通流程 | 权限 |
| --- | --- | --- |
| H5 `/m/login`、原生大师登录页 | 独立大师注册 → 申请 → 平台审核 | `master_applicant` → `master`，`manage_by=platform` |
| 寺院管理台 `/login` | 经办人注册 → `/onboarding` 机构申请 → 平台审核 | `temple_applicant` → `temple_admin`，绑定新寺院 |
| 寺院管理台 `/masters/accounts` | 本寺已认证纳管档案 → 分配用户名、本人邮箱 | 生成禁用账号，无默认密码 |
| 大师端“寺院分配账号激活” | 邮箱验证 → 本人设置密码 → 登录 | 启用原分配账号，保留 `manage_by=temple` 和寺院绑定 |
| 平台 `/onboarding/review` | 材料审查 → 通过或退回说明 | 仅 `platform_super` |

审核前只能访问本人申请。提交后不可编辑，退回后可补充；草稿和审核都使用版本检查。审批在同一数据库事务创建业务档案、绑定角色、记录审核快照，重复审批不能开通第二个身份。审核通过、账号停用均使旧会话失效。恢复账号不恢复旧会话，未激活账号恢复后仍须邮箱激活。

寺院纳管档案沿用原寺院建档及资质审核流程。旧“寺院审核／法师审核”处理存量档案审核；新增“认证与入驻审核”处理申请人开通，不能互相代替。原 `user` / `admin` 身份域与 ID 保留，相同邮箱不触发历史账号自动合并。

## 接口

公开接口都验证一次性图片验证码：

- `POST /auth/email/code`：`domain=admin`，purpose 为 `master_register`、`temple_register` 或 `activate`。
- `POST /auth/work/register`：`kind=master|temple`，email、username、password、code、agreementVersion、captchaId、captchaCode；返回申请人会话。
- `POST /auth/work/activate`：email、password、code、agreementVersion、captchaId、captchaCode；成功后从通用大师登录入口登录。

以下路径均在 `/api/v1/auth/onboarding` 下，服务端验证 JWT、会话和实时角色；寺院操作还验证实际寺院成员关系：

- `GET/POST /application`：本人申请；POST 为 profile、revision、submit。
- `GET /history?id=`：本人或平台审核员，返回事件与材料快照。
- `POST /evidence`：multipart file；单件上限 5MB，仅 PDF/PNG/JPEG，通过内容检测；最多 8 件随申请提交，账号累计上传 24 件。
- `GET /evidence?id=`：本人或平台审核员，私密下载、nosniff、no-store。
- `GET /reviews?status=&page=`、`POST /review`：平台超管；审核输入 id、revision、approve、note，退回必填原因。
- `GET /managed`、`GET /managed/candidates`：本寺分配记录与可分配大师。
- `POST /managed`：masterCode、email、username；不得给独立大师或外寺大师分配。
- `POST /managed/revoke`、`POST /managed/restore`：id，停用／恢复原账号。

证明文件存放在 `onboarding_evidence` 私有数据库记录中，公开目录不返回文件。删除表单中的材料仅从当前申请移除，历史审核快照保留；不自动清除历史证明。

## 迁移与部署

1. 备份现有身份、业务库与服务版本，确认前置邮箱身份迁移已经执行；执行 `scripts/db/20260915_dual_track_onboarding.sql`。该脚本可重复执行，只新增申请人角色和四张表，不改写既有身份或订单。
2. 认证服务需要访问同一 MySQL 实例中的身份、寺院及大师库，以保证审核开通事务原子性。除原身份权限外，增加以下最小表级权限：
   - `askxuan_auth.onboarding_application/onboarding_event/onboarding_evidence/master_invitation`：SELECT、INSERT、UPDATE（证据和事件实际仅新增/查询）。
   - `askxuan_auth.admin_account/auth_identity`：SELECT、INSERT、UPDATE；role：SELECT。
   - `askxuan_master.master/master_profile_ext`：SELECT、INSERT；`askxuan_temple.temple/temple_admin`：SELECT、INSERT。
   - 数据库授权由部署环境确定账号，不把 root 或测试凭据用于生产。
3. 网关、认证服务必须共用已配置 Redis 会话存储。上传入口反向代理限制至少覆盖 5MB 文件和 multipart 开销，MySQL max_allowed_packet 至少 8MB。私密材料数据库应使用现有备份与访问控制流程。
4. 同步发布认证服务、网关、大师 H5、寺院管理台、平台管理台；原生大师端发布对应新版本。C 端和存量管理员沿用前置邮箱认证版本。先迁移旧账号本人邮箱，不能将旧默认密码重新开放。
5. 沿用部署环境 SMTP 配置，真实注册前替换本地联调协议为运营确认版本；验证管理员权限、本人注册、审核、纳管激活及停用会话撤销后开放入口。

不要通过删除新表回滚用户数据。需要回滚时关闭新注册入口，保留新身份及材料，恢复成套兼容服务；已批准身份仍按正常大师／寺院账号管理。

## 本地验收

专用 Docker 栈沿用本目录 compose。向测试数据库执行 `onboarding-schema.sql`，该文件补齐认证测试所需的最小业务表，**仅用于本地，不是生产迁移**。运行 auth 58083、gateway 58084，网关 auth 上游指向 58083。Mailpit SMTP 51025 / HTTP 58025。客户端代理目标 58084。

    IDENTITY_TEST_MYSQL='root:local-identity-fixture-only@tcp(127.0.0.1:53306)/askxuan_auth?charset=utf8mb4&parseTime=true' IDENTITY_TEST_REDIS=127.0.0.1:56379 go test ./common/identity/...
    python3 scripts/dev/email-auth/verify-onboarding.py
    python3 scripts/dev/email-auth/verify-onboarding-upload.py
    make test-ci

HTTP 回归使用专用 Redis CAPTCHA 凭据与 Mailpit，不给真实收件人发信。隔离目录内的私有账号文件仅供本地 UI QA，权限 0600，不进入 Git。真实浏览器另行测试图片验证码、表单、上传、审核、激活。测试栈未运行订单、履约、IM 等业务服务，不能据此宣称全业务或线上验收完成；原生模拟器构建也不等于真机端到端验收。
