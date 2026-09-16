# 预约履约与回执

支付成功后的状态依次为 `pending`（待确认）、`confirmed`（待执行）、`in_progress`（执行中）、`pending_receipt`（待信众确认回执）、`completed`（已完成）。确认接单不代表执行完成。信众要求补充回执时回到 `in_progress`，再次提交形成新版本，旧回执保留。

全院执行订单没有大师接收方，不进入私聊列表和私聊未读统计。订单详情保留服务进度；历史空接收方会话通过查询过滤，不删除订单或聊天数据。指定大师的有效会话继续保留。

## 接口

以下接口均要求 JWT，并按订单所有人、所属寺院或指定大师再次验证权限。

| 方法 | 路径（前缀 /api/v1） | 用途 |
| --- | --- | --- |
| GET | /bookings/:id/fulfillment | 状态、真实状态日志、各版本回执 |
| PUT | /admin/bookings/:id/start | 寺院管理员开始执行，沿用 action 请求格式 |
| POST | /bookings/:id/receipt-files | 执行方上传 multipart `file`，返回私有文件元数据 |
| GET | /bookings/:id/receipt-files/:file | 有权限者读取文件，校验 SHA-256 后返回媒体 |
| POST | /bookings/:id/receipts | 执行方提交 `{summary,fileIds}` |
| POST | /bookings/:id/accept-receipt | 订单信众提交 `{}`，确认完成 |
| POST | /bookings/:id/request-revision | 订单信众提交 `{remark}`，要求补充 |

执行说明 5–2000 字，补充理由 5–200 字。每份回执 1–8 个文件，每个最多 20 MB；支持 JPEG、PNG、WebP、MP4、WebM，服务端检查文件内容类型。每单累计上传上限 200 MB（含未提交草稿）。未提交文件只有上传者可读；提交后该订单信众可读。未提供自动删除历史回执或超时自动完成。

状态、日志和通知 outbox 同一数据库事务提交，订单行锁保证并发操作不能跳步或重复完成。取消同时释放预约容量。现有直接完成接口不能绕过提交回执和信众确认。

## 上线顺序

1. 备份数据库；执行增量 `scripts/db/20260916_booking_fulfillment.sql`。新安装已包含在 `db/init.sql`。
2. 核实 booking-service 的 `CHAT_FILES_DIR` 指向持久化、备份的私有目录；回执存于其 `booking-receipts` 子目录，禁止通过 Nginx 静态暴露。多副本必须共享该存储。
3. 核实反向代理和 API 网关允许至少 21 MB 请求体、上传超时适当；不得仅放宽前端限制。
4. 部署 booking-service、message-service，再部署共享状态包、寺院管理端和 H5（信众与大师端）。同时检查 outbox 消费正常。
5. 以真实已授权账户核验全院执行订单列表、进度和完整回执闭环。不得为历史订单伪造执行记录或回执。旧完成订单显示“未留存履约回执”。

回滚时先停止新流程写入；数据库新增表和媒体应保留。已有 `pending_receipt` 订单需要兼容代码处理，不能简单删除表或把所有订单改为完成。原生 iOS 尚未适配回执交互，不能把 H5 验收当作原生端验收。

SHA-256 用于校验文件完整性，不是区块链存证，也不能证明线下服务事实。接入第三方存证需另行确定服务商、费用和个人信息处理范围。

## 验证

在 booking-service 模块运行 `go test ./...`。真实数据库集成测试需设置 `FULFILLMENT_TEST_DSN`，数据库名必须以 `askxuan_fulfillment_test_` 开头；测试会创建并清理该测试库中的相关表，必须使用专用空库。

`internal/logic/fulfillment_integration_test.go` 验证空会话过滤、角色和租户隔离、并发防重复、状态机、私有媒体、补充回执版本、用户确认、文件篡改和事务回滚。`cmd/fulfillment-fixture` 是仅监听回环地址的本地浏览器验收程序，使用合成身份，不接生产认证或 RPC。真实数据库连接信息通过环境变量注入，不写入代码。
