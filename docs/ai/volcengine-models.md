# 火山引擎模型接入

平台超级管理员进入「问事智能体开发中心 → 模型与连接 → 大模型连接」，选择「火山引擎 · 火山方舟」。

1. 填写方舟模型服务的 API Key。默认地址为 `https://ark.cn-beijing.volces.com/api/v3`；使用其他地域时填写对应官方模型服务地址，地域须与接入点一致。
2. 默认模型填写账号已开通的 **Model ID** 或 **Endpoint ID（ep-...）**。图片模型可选，须支持图片输入。
3. 如需开放多个模型，在「向用户开放的模型」中逐个填写 ID 并按回车添加，同时包含默认模型和图片模型。留空仅开放前两项已登记模型。
4. 点击「测试已填模型连接」。每个 ID 会收到一次固定测试请求（最多输出 64 Token，关闭思考）；图片模型还会收到固定测试图片。测试可能消耗少量额度，不包含用户问事内容，也不保存配置。
5. 点击「保存并生效」，查看确认提示后保存。保存会重新验证全部 ID；任一失败均保留当前生效配置。成功后新请求使用新快照，进行中的请求继续使用原快照。

已登记的模型进入现有模型目录，可在智能体默认模型及用户端模型选择中使用。平台沿用单个当前大模型连接；切换供应商会替换该连接。已发布智能体若固定了旧供应商模型，应修改草稿、评测并发布后再使用。

## 调用与配置约束

- 使用 Bearer API Key 与 `/chat/completions`，原样发送 Model ID / Endpoint ID；不依赖 `/models` 枚举账户接入点，也不需要 AK/SK 管控面权限。
- 支持普通生成、SSE 流式生成和 Eino/Harness 工具调用。`thinking.type` 显式传递 enabled/disabled；开启思考时传递所选 `reasoning_effort`，模型须支持该参数。
- 开启思考时以 `max_completion_tokens` 限制思考与回答总量；关闭思考时使用 `max_tokens`。不会同时发送两个预算字段。
- 手动登记不推断模型版本或上下文能力。未提供上游容量元数据时按现有 32K 保守额度处理，并将单次输出限制在窗口的一半以内。
- 测试通过表示所填 ID 完成了固定请求，不代表业务回答质量或所有工具能力均已验收。特定模型的能力仍需在智能体调试与质量评测中验证。
- 继续使用平台超级管理员权限、HTTPS 公网地址校验、禁止重定向、AES-GCM 加密保存、密钥不回显、配置修订冲突检查和审计；火山配置不套用 DeepSeek 的计费价格。
- 该入口面向对话和图片理解模型。方舟多模态向量接口采用独立协议，知识模型、图像生成、视频生成与 Plan 订阅端点不在本次范围。

服务启动配置也可设置 `AI_PROVIDER=volcengine`、`AI_API_KEY`、`AI_MODEL`、可选 `AI_VISION_MODEL` 和 `AI_BASE_URL`。未填写 Base URL 时使用北京地址；平台保存配置后仍按现有规则优先加载持久化记录。

## 本地验证

AI 服务目录运行 `go test -race ./...` 与 `go vet ./...`。测试使用模拟上游，覆盖错误密钥、不存在的接入点、空响应、取消请求、图片探测、保存/重启/密钥隐藏、模型白名单、流式协议和 Harness 工具循环。

可选 Ego Lite 验证使用 `ASKXUAN_VOLCENGINE_BROWSER_TEST=1 go test -run '^TestVolcengineBrowserFixture$' -v -timeout 25m ./internal/settings` 启动本机 `127.0.0.1:18546` 配置服务，并将管理台开发代理指向该地址。该服务使用真实 settings.Manager 和临时加密存储，供应商响应与管理员登录为测试替身；只接受测试密钥 `ark-browser-fixture`，含 `missing` 的模型 ID 返回失败。结束时 `POST /_fixture/stop`。不可将此测试服务部署到公网。

## 官方依据

- [兼容 OpenAI SDK](https://docs.volcengine.com/docs/ark/compatible-with-openai-sdk?lang=zh)
- [对话 Chat API](https://docs.volcengine.com/docs/ark/chat-api?lang=zh)
- [Base URL 及鉴权](https://docs.volcengine.com/docs/ark/base-url-and-authentication?lang=zh)

核对日期：2026-10-14。本地测试不替代实际账号调用、部署及线上验收。
