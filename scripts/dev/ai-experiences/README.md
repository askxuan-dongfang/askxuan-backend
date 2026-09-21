# AI 互动技能：接口与验收

版本 2026-09-21.1。新增能力是**确定性规则技能**，不是模型虚构的排盘或来源。姓名当前 16 字/12 组/3 种风格；两难比较按用户权重计算。原通用聊天和专题仍走管理平台配置的模型。

## 接口（网关要求真实 user 身份）

所有路径以 `/api/v1/ai` 开头；服务仅信任内网网关注入的 `X-User-Id` 和 `X-User-Type=user`，不能直接对公网暴露 AI 服务。

- `POST /experiences/run`：输入 `{skill:"naming",naming:{purpose:"name"|"pen",style:"gentle"|"nature"|"clear",surname:"",avoid:""}}` 或 `{skill:"decision",decision:{a,b,constraintA,constraintB,factors:[{label,weight,a,b}]}}`。
- `POST /notes`：`{requestKey,input,favorites:[],note:""}`。不接受客户端 result；服务端重算；返回 `{id,createdAt,data:{input,result,favorites,note}}`。
- `GET /notes?page=1`：每页 20 条，最多 10000 页，全部限定当前用户。
- `GET /notes/:id`：私密完整快照，含规则版本。
- `DELETE /notes/:id`：只删除当前用户所有的该条手记，重复删除幂等。

姓名姓氏 0–2 汉字（笔名不带姓），避用字最多16汉字；候选排除避用字与姓氏重复字。字义采用汉典基本释义的简述，不引用未经核验的典籍句。组合意象为编辑联想，不是古籍来源。真实命名还需自行检查谐音、重名、登记要求。

因素 2–6 项且不重名，重要性/满足度为 0–5 整数；权重不能全为零。分数 = sum(权重×满足度)/sum(权重)/5×100，保留一位小数。硬约束有文本即表示该方案不可用；不推断约束是否成立。分数不是概率。

保存键 8–64 字节，同用户同键重试返回同一条，同键不同输入返回 40901。仅可收藏本次实际候选，不能提交伪造候选。请求限 16KB，正文/备注限制在服务端执行。私人资料不写日志、不默认分享；浏览器草稿仅存当前会话、按账号隔离。

## 原有技能能力审核

- 原专题生成有结构化输入校验、MCP工具白名单、模型输出审核；它们原本仍共用长文报告。
- 修复必须计算的工具被关闭却返回空成功的问题；现在必需工具不可用/失败/空证据均失败，不再继续生成貌似计算后的内容。
- 未启用计算工具的专题只提供文化参考，系统提示明确禁止伪造排盘、抽牌或工具调用。
- 报告追问用户可见文案移除内部指令说明；服务端系统提示仍保留历史/报告不可信边界。
- 此审核不等于验证线上每个计算工具的算法正确性。上线前需分别验证真实工具与真实模型结果；不承诺预测准确。

## 本地验证

1. 新建隔离 MySQL，创建 askxuan_ai 后执行 `scripts/db/20260921_ai_experiences.sql`。
2. 在 ai-service 目录运行 `GOCACHE=/private/tmp/askxuan-ai-go-cache go test -race ./...` 与 `go vet ./...`。
3. 只对隔离库设置 `AI_EXPERIENCE_TEST_DSN`，运行 `go test ./internal/logic -run TestExperienceMySQLPrivateHistory -count=1`。测试创建临时 user，覆盖并发重试、异账号读删、快照版本、伪造候选、冲突和删除，自动清理测试手记。
4. H5 `npm run build`；浏览器测试有历史会话仍到发现、问事深链、草稿保留、两种工具、候选比较、保存回看、失败重试、删除确认、320px/宽屏/深浅主题/reduced-motion。

## 发布

数据库增量迁移 → AI 服务 → H5。原 iOS 不变，不宣称原生已同步。回滚只回代码，保留手记表。接口未知字段拒绝，结果带规则版本，后续版本添加字段要先处理客户端兼容。
