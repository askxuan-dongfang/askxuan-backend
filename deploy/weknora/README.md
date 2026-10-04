# 问玄 WeKnora 知识服务

固定 WeKnora v0.8.2，源码 `3e8b0bfc80b845b2d4b2ed683994748741450a97`，MIT。config.yaml 源自同一版本，LICENSE.upstream 保留原许可。标准部署包含应用、ParadeDB PostgreSQL 17（pgvector/BM25）、Redis、DocReader、本地中文 FastEmbed。应用只绑定回环 18085，并接入 askxuan-net；数据库、队列、解析器不公开端口。

## 配置与启动

复制 `.env.example` 至 `.env`，生成私有随机 DB_PASSWORD、REDIS_PASSWORD、JWT_SECRET、32 字节 SYSTEM_AES_KEY、AI_EMBEDDING_KEY。配置只保存在服务端，不提交 Git。首次模型下载约 90MB，可预填 embedding 模型卷；长文嵌入分段加权。本地嵌入不会上传原文。

    docker compose -p askxuan-knowledge up -d --build

首次在仅回环访问条件下临时设置 DISABLE_REGISTRATION=false，执行 `python3 bootstrap.py`，脚本生成专用服务账号及独立工作空间、API 密钥，并生成 owner-readable 的 ai-service.env。随后重建 app 关闭注册；验证未认证 API 被拒绝。后续启动不得重复开放注册。

只将 ai-service.env 内变量添加至 AI 服务当前环境，保留已有模型配置、挂载、限额。AI_WEKNORA_URL/KEY/EMBEDDING_ID 缺失时管理台明确显示未配置。接口使用服务端 REST 适配，不开放任意代理、不安装外部 MCP。

## 数据与权限

MySQL 执行 `scripts/db/20260930_ai_weknora.sql`，同时依赖知识记忆迁移。生产 Harness 需开启 AI_AGENT_OPERATIONS_ENABLED=true；管理台完成库绑定、质量评测、发布和覆盖比例后才使用新策略。评测目前最多 100 条，按用例数分配时限，最多 30 分钟。

MySQL 保存库登记、文档审核和操作审计；WeKnora 保存文件、分块与索引。平台总管理员才可管理知识；普通用户仅通过已发布 Harness 间接只读检索。模型不能传入 owner、工作空间或库 ID。空知识库绑定范围为拒绝全部。

新库、新文档默认停用；解析完成后管理员预览原文并确认来源，再审核启用。重解析先撤销检索资格，完成后重新审核。删除先撤销，再删除引擎内容；若引擎失败，资料保持停用，用户可重试。历史报告保留当时的引用快照。

## 回滚与备份

先关闭智能体知识开关，回滚 AI 镜像与管理台静态资源。数据库迁移只新增表，回滚时保留。分别备份 MySQL 策略/审计、PostgreSQL、文件卷和私有配置；两边缺失不能只靠一份向量索引恢复。停用 Compose 不带 `-v`，不得删除持久卷。

## 范围

本次提供建库/改名说明/启停/删库、TXT/Markdown 文本、PDF/DOCX/XLSX/CSV 文件导入、处理状态、分页分块预览、出处编辑、重解析、审核与删除、混合检索测试和 Harness 引用。没有开放 URL 抓取、第三方数据源同步、Wiki 自动建设、独立 WeKnora Agent、沙箱、任意模型配置。扫描件和复杂版式仍需实样核对；不把解析成功等同文献内容正确。

## 2026-10 智能体工作台扩展

管理台现在提供三个工作区：知识与能力、智能体开发、调试与发布。只维护一个面向用户的问事智能体；不部署第二个 WeKnora 对话产品。Harness 的职责、技能/MCP 范围、知识库绑定、重排策略、上下文预算随版本冻结；调试、评测、发布、灰度、回滚和运行轨迹沿用现有统一机制。

- **模型与连接**：登记 Embedding / Rerank / KnowledgeQA、测试、编辑未使用模型、删除未使用模型。部署预置模型只读；已被草稿、历史版本或知识库绑定的模型不可改写，保留回滚依赖。端点或供应商变化请新建模型。密钥写入服务端，读取 DTO 不返回密钥；连接测试不返回上游请求预览。新增外部供应商需运维将其准确域名加入 WeKnora 出站许可。
- **Embedding**：当前预置 BAAI/bge-small-zh-v1.5、512 维，负责原文向量化。建库时选定，升级应建新库重建索引，然后在草稿中切换库并评测发布；不直接用新维度覆盖旧库。
- **Rerank**：预置 BAAI/bge-reranker-base 中文/英文 CPU 服务。混合检索取得候选后进行语义重排。草稿设置候选数 5–40、引用数 1–10、模型、严格失败/降级策略。运行检索结果记录 retrieval / rerankStatus / graphStatus。关闭重排恢复混合检索；图谱故障保留原文检索并标记降级。
- **知识库**：导入、解析、出处、审核、停用、重解析、删除、分块预览与检索测试。未审核资料不会进入 Harness；测试可选择重排及实体扩展。单个知识库实体页可绑定抽取模型并启停抽取，模型绑定通过 WeKnora 官方 initialization API 保留现有解析设置。
- **Wiki**：生成设置、任务状态、条目搜索、来源核对、条目引用图、新建编辑、归档、分页版本历史、版本预览恢复、结构检查、重建链接、可修复结构问题及内容问题状态管理。Wiki 内容不作为系统指令，Wiki 发布不自动放行原文。
- **Neo4j 实体图谱**：Docker 内网专用 Neo4j 2025.10.1 + APOC，独立持久卷、内存上限，不暴露公网端口。展示由文档抽取的实体/关系，可搜索、拖动、查看出处及原文分块；仅显示仍获审核的原文节点。Harness 根据实际引用分块查邻接实体，重查审核状态后返回。图谱说明模型抽取的文献关系，不证明传统观点为客观事实。

### 部署补充

`.env` 增加随机 `NEO4J_PASSWORD`，先按 `../ai-rerank/README.md` 填充离线模型卷。构建并启动 neo4j/rerank，健康后重建 app。AI 服务加入 `AI_NEO4J_HTTP=http://askxuan-weknora-neo4j:7474` 与同一私有 `AI_NEO4J_PASSWORD`。仅保存于受限运行环境，禁止输出或提交。

新文档在启用图谱后自动抽取；已有文档不会凭开关自动补图，需要逐项重解析并再次审核。请先验证代表样本，再分批处理存量。实体抽取与 Wiki 使用 KnowledgeQA 模型并消耗 token，按平台并发和预算限额运行。导入或解析成功不等于资料可用，需检查分块、Wiki 和图谱实际结果。

原“范围”段为首次集成记录；Wiki、模型管理、Neo4j 已纳入此扩展。仍未开放任意第三方 MCP 安装、WeKnora 自有 Agent/沙箱、外部数据源自动同步；问事由本平台 Harness 统一驱动。传统专业算法仍由已审核的技能/MCP 实现，知识图谱不能替代排盘或测量。
