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
