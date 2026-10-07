# Harness 联网搜索

## 使用
1. 管理台 → 智能体工作台 →「模型与联网设置」：选择 Tavily 或 Brave Search，填写该搜索服务的 API Key（与 DeepSeek 独立），测试搜索连接后保存。测试提交固定公共查询，可能消耗搜索额度。密钥加密持久化、不回显；留空保留，同服务可轮换，换服务必须重新填写。可以停用服务或清除密钥。
2. 智能体配置打开「允许用户按需联网搜索」，保存草稿、调试和评测通过后发布。配置随版本冻结；不自动修改现有草稿或已发布版本。未配置搜索服务的联网版本禁止调试和发布。
3. H5 问 AI 的输入区点击「联网」后提问。默认关闭；每条请求携带 webSearch，旧客户端默认关闭。切换会话时重置。当前版本未开放或搜索服务未配置时显示「联网未开放」。
4. Harness 按需调用 search_web，与知识库、排盘、记忆等工具共享每轮最多 4 次工具调用和任务时限。处理过程显示「联网搜索」；回答要求引用搜索返回的来源链接。

## 边界
- 本功能是公网搜索摘要，不是任意网页抓取或自动入库；搜索资料不会自动写入 WeKnora，也不直接修改 Wiki/图谱。知识库资料仍需解析、审核和发布。
- 只有管理员发布许可 + 搜索服务可用 + 用户本轮开启三者同时满足，才注入工具。选择与该轮消息一起保存，重试只使用原轮选择；输入字段不能伪造联网许可。
- 工具强制要求 1–400 字查询，无自动上传完整问题/报告。系统指令要求模型只提炼公开关键词，不提交姓名、出生资料、地址、记忆或报告正文；这是模型行为约束，不是对所有自然语言个人信息的自动识别保证。
- 搜索使用固定 HTTPS 服务地址，禁止重定向；15 秒超时、1 MiB 返回体上限、最多 5 个去重来源、每条最多 1200 字摘要。只允许普通 HTTP(S) 来源链接，不抓取返回链接。
- 网页内容按不可信数据处理；有来源不等于结论正确，摘要不等于全文核验。故障、余额不足或没有命中均不应声称已查证。
- 默认不配置、不联网。AI_WEB_SEARCH_PROVIDER / AI_WEB_SEARCH_API_KEY 可用于首次环境初始化；有加密配置文件后以已保存配置为准。已有运行保留接收请求时的配置快照；停用影响新请求。
- 本次新增的客户端入口为 H5；原生客户端未增加开关，仍默认关闭。

## 接口
- GET /api/v1/ai/capabilities：认证用户、按其发布灰度版本返回 webSearchAvailable，不包含密钥。
- POST /api/v1/ai/sessions 与 /sessions/:id/messages：可选 webSearch boolean。
- GET/PUT /api/v1/ai/admin/provider：新增 webSearchProvider、hasWebSearchKey（只读）、webSearchApiKey（只写）、clearWebSearchKey。保留现有 platform_super 权限、乐观锁与加密审计。
- POST /api/v1/ai/admin/provider/web-search/test：使用未保存的设置测试一次固定查询，不持久化、不改变发布许可。

## 验证
单元测试使用假服务，不消耗真实搜索额度：提供方协议、错误脱敏、结果大小和 URL 过滤、加密配置恢复/清除、旧客户端兼容、单轮开关与重试隔离、实际 Eino 搜索循环及未提供工具的拒绝。真实提供方验收需配置有效搜索密钥后执行。

官方接口协议：
- https://docs.tavily.com/documentation/api-reference/endpoint/search
- https://api-dashboard.search.brave.com/documentation/guides/authentication
- https://api-dashboard.search.brave.com/app/documentation/web-search/responses
