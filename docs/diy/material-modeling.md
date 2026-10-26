# 材料建模与发布

`renderAssets.studio` 的 v1 合同由前端仓库 `packages/diy-materials/studio-contract.md` 维护，H5、管理端和 iOS 共同消费。旧记录没有 studio 时保持兼容；不批量改写现有材料或订单快照。

- `part=bead` 仅允许 `motion=fixed`，`connectorMm=0`；`part=hanging` 允许 rigid/flexible，挂点 Z 必须小于 0.5、连接绳 1–30 mm。
- 普通模板和草稿不依赖 AI 配置。studio 新建默认为 off_shelf；改变 studio 后重新变为 off_shelf。上架要求有效尺寸、预览确认；generated 还必须绑定就绪任务及服务器持久化的两端模型地址。草稿允许 generated 且 model 为空。
- `source=template` 仅使用已注册模板；generated 的 templateId 固定为 custom。保存不能把已有 studio 清空以绕过发布条件。

## 管理 API

所有路径以前缀 `/api/v1/admin/diy/modeling` 开始，使用真实管理端访问令牌和网关权限；不能用 X-User-* 冒充。

- `GET /config` → `{provider:"meshy",model,configured,hasApiKey,canConfigure,revision}`。不返回密钥。
- `PUT /config` → `{apiKey?,clearApiKey?,model?}`，仅人类 platform_super；空密钥保留已有值，clearApiKey 显式清除。支持 meshy-7.1、meshy-6、latest，默认 meshy-7.1。
- `POST /tasks` → `{requestId,photoUrls}`。requestId 为 16–64 位字母/数字/下划线/连字符；照片 1–4 张。相同操作者与 requestId、相同输入返回同一任务；同 key 不同输入返回冲突。
- `GET /tasks/:id` → `{id,status,progress,photoUrls,model?,errorMessage?}`。任务仅创建者或 platform_super 可见。model 就绪时含 `{jobId,glbUrl,usdzUrl}`。
- 状态：queued → submitting → processing → succeeded；失败为 failed。提交前需要已配置服务。未配置或失败绝不回退成伪造的 AI 模型。

状态持久化于 MySQL；进程重启继续读取 provider_id 轮询。提交响应丢失的任务不会自动再次调用收费生成；5 分钟后标记失败并提示运营到供应商核对。轮询与入库可重试，生成请求不可隐式重试。任务超过两小时失败。运营显式重试需新 requestId。

## 文件与凭据

Meshy 接口为固定 `https://api.meshy.ai/openapi/v1/multi-image-to-3d`，返回 GLB、USDZ。worker 从 file-service 读取本桶已上传的 JPG/PNG/WebP（每张最多 10MB），通过 data URI 提交，避免要求 Meshy 访问本站自签名证书。它不会读取任意 URL。

下载只接受 `assets.meshy.ai` 的 HTTPS 地址，拒绝私有/保留 IP，解析后固定连接该 IP，不跟随重定向。每份模型最多 32MB。file-service 新增仅 diy-service 服务 JWT 可访问的内部端点：

- `GET /internal/diy-model-photos?url=...`：严格限定本桶 temp/<32hex>.<image>。
- `PUT /internal/diy-models/:id/:format`：校验 GLB 头/块/自包含资源，无骨骼或内嵌动画；限制 10 万位置顶点（包括 node 实例）、128 个 node、64 个 mesh、512 个 accessor，accessor 总分量最多 200 万；拒绝稀疏/压缩几何/实例扩展与循环层级。纹理仅 PNG/JPEG，每张最大 4096×4096、总计 1600 万像素，最多 16 张；USDZ 校验 ZIP 路径、文件种类、压缩/展开体积、重复名及符号链接。存储键包含任务 ID 和文件 SHA256，重试不会覆盖别的任务版本。

沿用 file-service 的 MinIO 与公网 URL，生成完成后不依赖供应商过期地址。配置密钥使用现有持久 AuthSecret 和独立 `askxuan-diy-modeling-v1` 上下文 AES-GCM 加密入库。更换 AuthSecret 前必须进行密钥重加密迁移；不要直接旋转后丢弃旧密钥。任务/日志/API 不包含明文 API key。DIY config 路径使用不记录请求体、Header 或查询串的日志中间件，避免框架在 5xx/verbose 时转储凭据；其余路径保留原日志行为。网关操作审计只保存权限/路由/目标元数据。

## 部署与验证

1. 先备份并执行 `scripts/db/20261026_diy_material_modeling.sql`，只有两张 CREATE TABLE IF NOT EXISTS；重复执行安全。
2. 当前 CI receiver **不自动执行 SQL**。核对迁移后，按现有运维流程更新 backend_contract 基线，才能发布新的 backend CI 包。
3. 发布 diy-service 与 file-service；默认 `ModelingFileURL=http://file-service:8097` 使用既有 Docker 网络。非标准部署可配置 ModelingFileURL；不得从客户端指定这个地址。
4. 管理端超管配置真实 Meshy 密钥后才可进行收费生成。没有真实凭据时仅证明本地 fixture 联调，不声称已完成供应商实测。

运行 `bash scripts/test-diy-modeling.sh` 会创建并删除一次性 MySQL，重复迁移，验证凭据加密、生成幂等、重建 manager 后恢复、模型地址绑定、跨操作者隔离、未知提交不重复收费以及草稿/发布/修改回草稿。文件格式与内部服务鉴权单测位于 file-service；普通 DIY 回归保留。

### 固定供应商与验收边界

2026-10-26 已核对 [Meshy 官方 Multi-Image to 3D API](https://docs.meshy.ai/en/api/multi-image-to-3d)：`meshy-7.1`、1–4 张 URL/data URI、`/openapi/v1/multi-image-to-3d`、`target_formats` 含 GLB/USDZ。模型/API 可随供应商变化，发布前仍需使用真实账户做一次付费验收。

USDZ 的 ZIP 路径与展开预算并不等同于对二进制 USDC 引用语义的完整解析。这里只接收固定 Meshy API/CDN 产生、经内部服务 JWT 入库的资源；iOS 使用平台原生沙箱加载。未验证任意第三方上传 USDC 的安全性，不开放用户直传模型端点。

`diy-modeling-mysql` 已进入 `.github/workflows/ci.yml` 的独立 job，并作为 deploy 与 recover-package 的前置条件。`scripts/ci/release.py:contract()` 将 `db/` 和 `scripts/db/` 等路径纳入契约散列；`scripts/ci/ecs_receiver.py` 比对部署配置 `/opt/askxuan/ci/config.json` 的 `backend_contract`，不会执行 SQL。迁移核对和基线更新必须先于新包接收。
