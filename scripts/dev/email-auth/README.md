# 邮箱与密码认证

实现范围：H5 信众／师傅、iOS 信众／师傅、统一管理台、寺院管理台、认证／用户服务与网关。短信尚未接入，`smsEnabled=false`；前端不展示不可用的短信入口。

## 接口契约

响应保持 `{code,message,data}`。以下接口公开，不携带旧登录令牌；其他接口仍经网关鉴权。

| 方法与 `/api/v1/auth` 下路径 | 输入或结果 |
| --- | --- |
| GET `/options` | `passwordEnabled,emailEnabled,smsEnabled,agreementVersion` |
| GET `/captcha` | `id,image`（PNG data URL）,`expiresIn=180`；不返回答案 |
| POST `/login` | `account`（邮箱或用户名）,`password,captchaId,captchaCode` |
| POST `/admin/login` | 同上；平台、寺院、商城、师傅角色及绑定关系从原管理账号读取 |
| POST `/email/code` | `email,purpose`（register/reset）,`domain`（user/admin）,图片验证码；返回 `retryAfter=60,message` |
| POST `/email/register` | `email,username,password,code,agreementVersion,captchaId,captchaCode`；仅 user，成功即返回会话 |
| POST `/password/reset` | `email,password,code,domain,captchaId,captchaCode`；成功撤销该身份全部会话 |
| POST `/refresh` | `refreshToken`；返回新访问令牌，会话必须仍有效 |
| POST `/logout` | `accessToken` 或 Authorization Bearer；撤销对应会话及其刷新能力 |

用户名 4–32 位小写字母、数字、下划线，以字母开头。密码 12–128 个字符（最多 512 字节），Argon2id 独立随机盐。邮箱验证码 6 位、5 分钟、一次性，最多 5 次尝试；图片验证码提交一次即消耗。发送邮件后前端刷新图片，最终提交需填写新图。新验证码不继承旧码的错误次数。

未知找回邮箱与重复注册邮箱使用一致发送提示。按 IP、账户和邮箱限流。Redis 仅保存验证码 HMAC，不保存明文。认证服务关闭请求正文及 SQL 参数日志。表单禁用重复提交；注册协议不是预勾选。

## SMTP 配置

只在服务端配置下列变量，禁止放进 VITE、iOS Info.plist、Git 或前端包。

| 变量 | 用途 |
| --- | --- |
| `AUTH_SMTP_HOST` | SMTP 服务商主机 |
| `AUTH_SMTP_PORT` | 默认 587 / STARTTLS；465 使用隐式 TLS |
| `AUTH_SMTP_USERNAME` / `AUTH_SMTP_PASSWORD` | SMTP 凭据，通过部署密钥或受限环境文件注入 |
| `AUTH_SMTP_FROM` | 服务商已允许的发信地址，可带显示名 |
| `AUTH_SMTP_LOCAL_TEST` | 仅本地 Mailpit 设为 true，且仅 loopback 地址允许不加密；生产不得启用 |
| `AUTH_REDIS_HOST` / `AUTH_REDIS_PASSWORD` | 网关使用，与认证服务 Redis 必须一致；认证服务仍读取 YAML Redis 配置 |
| `AUTH_TRUSTED_PROXY_CIDRS` | 认证服务可选，逗号分隔可信网关／反向代理 CIDR。默认不信任 X-Forwarded-For；从右向左剥离可信跳点。不要填 0.0.0.0/0 |

未配置发信主机和有效 From 时，`emailEnabled=false`。配置存在不代表投递可达；真实服务商接入后须验证 TLS、授权发件人、垃圾邮件和失败反馈。当前只完成本地收信测试，没有配置真实服务商或发送真实邮件。

## 本地运行（在 backend 根目录）

本目录是专用测试栈，端口均绑定 127.0.0.1。数据库、JWT 密钥、密码都是公开的本地测试值，不能用于生产。需要 Docker、Go 1.23+、Python 3；首次启动会拉取 MySQL、Redis、Mailpit 镜像。

    docker compose -p askxuan-email-auth-test -f scripts/dev/email-auth/compose.yaml up -d

确认 MySQL 就绪后，在两个终端启动：

    AUTH_SMTP_HOST=127.0.0.1 AUTH_SMTP_PORT=51025 AUTH_SMTP_FROM=no-reply@local.askxuan.test AUTH_SMTP_LOCAL_TEST=true go run ./services/platform/auth-service -f scripts/dev/email-auth/auth.yaml

    AUTH_REDIS_HOST=127.0.0.1:56379 go run ./services/platform/gateway-service -f scripts/dev/email-auth/gateway.yaml

H5／管理台开发服务器使用 `VITE_DEV_PROXY_TARGET=http://127.0.0.1:58081`。Mailpit 在 `http://127.0.0.1:58025`。测试栈只含认证接口，首页／订单等业务模块显示未加载属于隔离环境边界，不是全业务验收。

    python3 scripts/dev/email-auth/verify.py
    IDENTITY_TEST_REDIS=127.0.0.1:56379 IDENTITY_TEST_MYSQL='root:local-identity-fixture-only@tcp(127.0.0.1:53306)/askxuan_auth?charset=utf8mb4&parseTime=true' go test ./common/identity/...
    make test-ci

HTTP 脚本固定使用本地测试栈，向专用 Redis 写入 CAPTCHA 测试凭据，真实走 HTTP 处理、SMTP 收信和网关撤销；不在服务中设置测试后门。浏览器的图片识别和表单交互另行验证。脚本创建的 `httpqa_*` 测试账户保留在专用数据库供检查；模型集成测试会清理自身数据。

结束后可运行 `docker compose -p askxuan-email-auth-test -f scripts/dev/email-auth/compose.yaml down`，不影响其他项目。

## 现有账号迁移与上线顺序

1. 备份数据库和当前各端构建，先执行 `scripts/db/20260915_email_identity.sql`。保留原用户 ID、资料、订单、管理员角色及寺院／师傅绑定。多个无手机邮箱用户使用 NULL mobile，旧手机值保留。认证服务数据库账号需具备 `askxuan_user.user` 与 `user_profile` 的 SELECT/INSERT/UPDATE，以及 `askxuan_auth.auth_identity` 的 SELECT/INSERT/UPDATE 权限；不要只保留原先的只读用户权限。
2. 配好 SMTP、TLS、发信身份及共用 Redis。网关缺失 Redis 配置或 Redis 不可用时不应放行会话。认证服务仅在内网向网关开放，运维入口不提供公网 HTTP 接口。
3. 对现有账号先核实业务 ID 与本人身份，再使用运维工具发送绑定邮件。该工具只允许首次 INSERT，不能覆盖已验证身份；不会把未验证手机号或邮箱自动关联到老账户。domain=user 使用 user.id；domain=admin 使用 admin_account.id（包含师傅）。

       go run ./services/platform/auth-service/cmd/enroll -f <服务端配置路径> -domain admin -id <既有ID> -email <本人邮箱> -username <登录用户名> -send

   再执行相同命令但去掉 `-send`，从受限标准输入提供两行：收到的邮箱验证码、新密码。不要把密码放在命令参数或聊天中；可使用受限临时文件，完成后删除。发送进程需要同一 SMTP 环境，绑定进程必须使用同一认证配置与 Redis。角色不会被工具更改。
4. 必须先完成关键管理员和现有账户迁移，再协调发布认证／用户服务、网关及各客户端。旧 `1234`、明文默认密码、无 sid 的旧令牌和老手机注册接口被拒绝；iOS 老版本须升级。此版本不能单独替换某一端并宣称兼容旧演示登录。
5. 当前表单内协议说明是本地联调版本。开放真实注册前，应接入运营确认的完整协议／隐私政策并更新服务端 AgreementVersion 与各端展示内容，重新验证勾选和版本记录。
6. 上线后实际验证注册、用户名／邮箱登录、密码找回、权限和订单关联，再开放入口。

回滚应整体回滚兼容的服务与客户端，并保留新增身份表及 nullable mobile。不要恢复 `1234` 绕过，不要删除新注册身份；迁移后直接切回旧明文验证逻辑会造成登录故障。真实 SMTP、现有账户迁移、生产发布和真机验收尚不在本地通过结果之内。

## 独立大师认证与寺院入驻

工作身份自助注册、平台审核和寺院纳管账号分配见 [ONBOARDING.md](ONBOARDING.md)。此扩展需额外执行双轨入驻迁移后开放工作账号注册。

## 运维分配的演示账号

不可收信的测试占位地址使用 `.invalid` 保留域，`auth_identity.verified_at` 为 NULL，需要执行 `20260915_unverified_test_identity.sql`。这类账号仅接受用户名和密码登录，不接受占位邮箱登录；公共发码／注册／找回拒绝 `.invalid`，不尝试向其投递邮件。测试账号不通过公共接口创建，也不提升原有禁用或待审身份的业务权限。

绑定真实邮箱时，运维 enrollment 工具在验证邮件后可以升级同一 ID 下尚未验证的 `.invalid` 占位身份，保留业务关联。已验证身份仍不能被工具覆盖；新增普通注册与独立认证流程不变。
