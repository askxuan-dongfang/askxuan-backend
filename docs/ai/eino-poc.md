# AI 问事 Eino 兼容性验证

日期：2026-09-28。分支：`codex/eino-poc-20260928`。起点：`309d75d7eac0f134937435af9c9134d9fa034105`。

后续管理台集成及其交付边界见 [智能体运营管理](agent-operations.md)。本文保留独立 PoC 阶段的验证记录。

## 目的与结论边界

验证现有 Go AI 服务能否复用 Eino ADK，实现“调用计算工具 → 根据结果继续 → 缺资料暂停 → 补充后恢复”，为问事模块从固定流程演进为受控智能体提供依据。

这是独立命令行 PoC，未注册 HTTP 产品接口，未更改 H5/iOS，未切换生产问事流程。测试使用真实 Eino 运行时、OpenAI 兼容适配器和现有 MCP 客户端，模型响应与 MCP 结果由本地协议服务器提供。通过这些测试不能证明生产模型的工具调用质量、生产 MCP 可用性或用户端体验。

## 复用关系

1. `settings.Manager.Snapshot()` 固定一次任务的模型配置；澄清恢复后继续使用同一实例。
2. `provider.NewEinoModel` 复用当前 Provider 的密钥、模型和 HTTP 客户端，包括既有网络限制。继续沿用模型目录的选择规则。
3. `einopoc.SkillTool` 复用技能数据库中的输入规则、`Guard`、`BuildToolArguments` 和 `MCPClient`。
4. Eino ADK 负责多轮循环、工具调用事件、流式结果和中断恢复。
5. 本项目的 Harness 限制调用次数、执行时长、上下文/输出长度及可用工具。

固定依赖：`github.com/cloudwego/eino v0.9.21`、`github.com/cloudwego/eino-ext/components/model/openai v0.1.13`。AI 模块最低 Go 版本从 1.22 调整至 1.23.0，与仓库 workspace/CI 基线一致。增加的依赖也会进入常规 AI 服务的构建依赖图，正式接入前需要评估镜像增量。

参考：[Eino 官方仓库](https://github.com/cloudwego/eino)、[OpenAI 适配器](https://github.com/cloudwego/eino-ext/tree/main/components/model/openai)。

## 工具与资料规则

- 仅允许已启用的 `bazi`、`ziwei`、`qimen` 计算技能；每个运行实例只绑定服务端选择的工具。
- 模型调用参数必须是空对象。出生日期、时间等事实来自用户确认的结构化输入，不能由模型补造。
- 输入缺失/无效时产生中断；宿主取得用户补充的 JSON 后恢复。有效补充会保留，工具失败重试不会丢失资料。
- 工具失败只返回脱敏错误类型，不把上游错误正文、地址或凭据交给模型。
- 不注册支付、订单修改、退款、Shell 等能力。
- `SkillTool` 和 Harness 都应按任务创建，不跨用户复用；工具按顺序执行。

## 可执行验证

在 `services/infrastructure/ai-service` 下：

```sh
GOTOOLCHAIN=go1.23.0 go test -race ./...
GOTOOLCHAIN=go1.23.0 go vet ./...
GOTOOLCHAIN=go1.23.0 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/eino-probe ./cmd/eino-probe
```

新增协议测试覆盖：

| 场景 | 验证内容 |
| --- | --- |
| 多步计算与失败恢复 | 首次 MCP 失败、模型重试、成功依据传回模型、最终结果逐段输出 |
| 澄清与继续 | 无资料不调用 MCP；错误中断 ID 被拒绝；无效补充再次暂停；有效补充后重试保留资料；结束后不可重放 |
| 工具权限 | 未注册工具、模型擅自填写事实都无法触达 MCP |
| 预算 | 循环调用在模型/工具次数上限处停止 |
| 超时与输出检查 | 请求超时中止；跨流式分片的拦截词不能完整流出 |
| 配置兼容 | 保留模型、认证、流式参数、输出上限及既有 HTTP transport；管理员未开放的模型被拒绝 |

2026-09-28 本地验证结果：上述 `go test -race ./...` 全部通过，`go vet ./...` 通过，Linux amd64 独立 probe 编译通过。覆盖范围是 AI 服务模块，不代表整个后端所有服务的回归结果。

### 使用已有配置做真实渠道验证

独立程序支持读取已有配置、加密模型设置和数据库技能记录。应在能读取现有配置的受控环境运行，不复制密钥进命令行或仓库：

```sh
/tmp/eino-probe -config /path/to/ai.yaml -skill bazi -inputs /tmp/synthetic-facts.json
/tmp/eino-probe -config /path/to/ai.yaml -skill bazi -reply /tmp/synthetic-facts.json
```

`-inputs` 省略时先尝试缺资料流程；若模型调用工具并触发中断，`-reply` 会在同一进程内恢复一次。合成输入示例：

```json
{"birthDate":"1990-01-02","birthTime":"12:30","gender":"male"}
```

命令会产生少量模型费用，最多 4 次模型调用尝试、4 次工具执行尝试，每段执行 45 秒，整个命令 60 秒。暂停与恢复也消耗工具尝试预算。`modelAttempts` / `toolAttempts` 是尝试计数，包含被上限拒绝的尝试，并非实际账单或成功调用数。程序不写产品会话、业务订单、资金账本或模型设置。

本次尚未执行真实渠道验证。生产模型是否正确选择工具、支持对应流式协议，以及真实 MCP 的响应质量，仍需使用上述合成资料验证。

## 正式接入还需完成

1. 将任务与登录用户绑定，持久化检查点、状态和过期时间，支持重启后恢复并防止重复提交；当前仅有进程内检查点和 ID 匹配。
2. 增加产品 API/SSE 事件：正在理解、正在计算、待补充、已完成、失败；H5/iOS 复用统一协议。
3. 将执行轨迹、实际 token 用量、失败原因和成本纳入现有审计；目前只有次数/长度/时间上限，并非精确金额限额。
4. 对真实模型、真实 MCP、断网取消、并发请求和长期运行做验收；先灰度并保留原流程回退。
5. 当前输入/输出 Guard 只是已有规则检查，提示词也不能保证模型绝不产生错误结论；需要按实际场景补评估集与质量门槛。

第一阶段以单智能体搭配少量只读工具为宜，无须先引入多个智能体、长期记忆或新的运行语言。
