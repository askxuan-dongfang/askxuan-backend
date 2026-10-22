# 管理操作日志接口

此接口记录总平台管理写请求的执行结果，与原有内容审核 `audit/queue` 相互独立。日志不保存请求正文、查询串、IP、邮箱、口令、令牌或文件内容。

部署前显式执行 `scripts/db/20261022_admin_operation_log.sql`。该迁移只新增 `askxuan_audit.operation_log`。现有 audit 挂载配置增加 `Auth.AccessSecret`，值必须映射现有 gateway/auth 使用的同一签名密钥；不能使用源码占位值或轮换其他服务密钥。默认请求正文日志已关闭。

## 内部写入

仅接受签名的 `access` 服务令牌：`UserType=service`、`userId=0`、`clientId=issuer=gateway-service`，且含 `platform_service` 角色。用户、刷新令牌、其他服务及身份请求头均不可写入。

`POST /api/v1/internal/admin-operations/start`

```json
{"requestId":"0123456789abcdef0123456789abcdef","actorId":7,"actorRoles":["platform_super"],"action":"platform.products.write","method":"PUT","routeTemplate":"/api/v1/admin/products/:id","targetType":"products","targetId":"17"}
```

`requestId` 为 32 位小写十六进制随机值；`actorId` 为正整数；角色非空、无重复，最多 32 个。角色和 `targetType` 使用 1–64 位安全编码；`action` 只能是权限目录编码或固定的 `admin.write|ai.admin.write|growth.admin.write|onboarding.admin.write`。`routeTemplate` 必须是 `common/adminpolicy` 中与写入方法对应的精确路由模板，或固定的 `/api/v1/admin/*`、`/api/v1/ai/admin/*`、`/api/v1/payments/growth/admin/*`、`/api/v1/auth/onboarding/*`。不接受包含实际参数的路径。`targetId` 可为空，最多 128 位，仅字母、数字、下划线、点、冒号和短横线；网关仅从安全的路由参数提取标识。

首次记录为 `pending`。相同标识与全部相同开始字段重试返回原记录状态；任何开始字段冲突均拒绝，已完成记录不会重置。

`POST /api/v1/internal/admin-operations/finish`

```json
{"requestId":"0123456789abcdef0123456789abcdef","result":"success","httpStatus":200,"businessCode":0,"durationMs":17}
```

`result` 为 `success|denied|failed|unknown`；`httpStatus` 为真实的 100–599 状态；`durationMs` 为 0–86400000；业务码通常非负，只有 `failed|unknown` 可用 `-1` 表示无法读取业务码。非标准响应不得据 HTTP 200 推断成功。

只允许 `pending` 转成终态；全部相同的终态字段可幂等重试，不允许覆盖结果、状态码或耗时。无开始记录的完成请求拒绝。开始及完成均限制 8 KiB JSON 正文，拒绝额外字段、尾随 JSON 及查询串。

成功返回 `{"code":0,"message":"success","data":{"requestId":"…","result":"pending"}}`（或实际终态）。参数错误为 `40001`，认证/权限错误为 `40101|40301`，幂等字段或状态冲突为 `40903`，存储不可用为 `50001`。沿用统一响应协议，以 `code` 判断是否成功。

## 总平台查询

`GET /api/v1/admin/audit/operations`

只接受 `UserType=admin`、正数用户 ID 的 access 令牌。`platform_super` 可读；其他角色须 `clientId=platform-admin`、`authzScope=platform` 且拥有 `platform.operations.read`。服务令牌、寺院/大师/商城角色不能读取。在线角色版本检查由 gateway/auth 负责，audit 不循环依赖 auth。

可选精确筛选：`actorId`、`action`、`method`、`targetType`、`result`（含 `pending`）、`startTime`、`endTime`。时间须 RFC3339；结束时间不得早于开始时间。`page` 默认 1（1–1000000），`size` 默认 20（1–100）。未知或重复查询字段拒绝。

返回 `data={list,total,page,size}`，按 `startedAt DESC, requestId DESC` 排序。每项包含开始字段，以及 `result,httpStatus,businessCode,durationMs,startedAt,finishedAt`；时间为 UTC RFC3339。`pending` 的 `finishedAt` 为空字符串，不能当作成功。空页 `list=[]`，总数保持实际筛选总数。无批量导出或删除日志接口。

## 验证

模块 `go test -race ./...` 覆盖令牌身份、请求头伪造、字段泄露、无效输入不访问存储、过滤及 UTC 参数。真实 MySQL 测试使用 `ADMIN_OPERATION_TEST_DSN`，仅允许 `askxuan_admin_test_*` 独立库，应用该表 DDL，并只清理本测试随机标识的行；覆盖并发开始去重、不可覆盖终态、冲突拒绝、未知结果和真实分页。
