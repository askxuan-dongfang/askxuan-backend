# DIY 定稿发布迁移与验收

本次增加 `diy_design_publication`，以及 `diy_design.publication_id`、`diy_design.source_publication_id`、`creator_reward.source_publication_id`。仅加表/列，无历史发布资格或财务数据回填。旧 public 作品通过读取过滤保留为本人可编辑稿。

## CI 与隔离数据库

默认分支工作流新增 `diy-publication-mysql`，部署的 `needs` 包含该 job，失败时禁止部署。该 job 执行：

```sh
bash scripts/test-diy-publication.sh
```

脚本新建仅绑定本机随机端口的 MySQL 8 容器，导入现有基础表、DIY 草稿版本、创作返现表和本次迁移，重复执行本次迁移验证可重入，再执行模型与 HTTP 集成回归。结束时只销毁该脚本创建的容器。测试支付回执为隔离库构造的数据，不调用真实支付网关，不是生产付款验收。

现有成长与返现回归脚本 `scripts/test-growth.sh` 同步加载本次迁移，避免新返现列在独立测试库缺失。

发布脚本的并发回归通过 MySQL `performance_schema.data_lock_waits` 确认等待关系：直接退款先锁支付时，发布或复制下单必须等退款提交后重新判断；下单先锁支付、等待库存时，退款必须等下单提交。测试复现的是退款事务的支付/退款临界区，不声称调用了真实支付网关。

## 生产迁移顺序

现有 ECS receiver **不执行发布压缩包中的脚本**，并以 `backend_contract` 核对数据库/运行配置基线。本次保留这一门槛，不能只上传新二进制或跳过 schema 审核。

1. 确认部署目标提交及工作流全部通过，保留当前二进制回滚包和现有 receiver/config。通过既有运维连接对 `askxuan_diy` 做备份；备份和连接凭据不得进入 Git 或公开验收记录。
2. 在既有发布锁保护下，审核目标提交的 `scripts/db/20261020_diy_publication_gate.sql`。通过既有数据库管理员通道执行这一文件；不要运行 `db/init.sql`，不要把隔离库测试数据导入生产。该 SQL 依赖已上线的 `creator_reward` 表。
3. 运行以下只读查询，确认三个新列均为 `bigint NOT NULL DEFAULT 0`、新表为 InnoDB，以及唯一键 `uk_design_revision(design_id,design_revision)`。记录目标提交、SQL SHA-256、执行退出码、脱敏 schema 结果和备份位置引用。

```sql
SELECT TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COLUMN_DEFAULT
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA='askxuan_diy'
  AND ((TABLE_NAME='diy_design' AND COLUMN_NAME IN ('publication_id','source_publication_id'))
    OR (TABLE_NAME='creator_reward' AND COLUMN_NAME='source_publication_id'))
ORDER BY TABLE_NAME,COLUMN_NAME;
SHOW CREATE TABLE askxuan_diy.diy_design_publication;
```

4. 从已提交的同一目标代码计算合约值，**不能从未提交工作树推断目标值**：

```sh
python3 scripts/ci/release.py contract
shasum -a 256 scripts/db/20261020_diy_publication_gate.sql
```

5. 仅在备份、迁移和 schema 核验完成后，通过现有运维流程更新 `/opt/askxuan/ci/config.json` 的 `backend_contract`，保留其他字段及原值备份。这个值必须与待部署包 `manifest.json.contract` 一致。
6. 使用既有受限 SSH receiver 发布同一提交的构建包。核对部署来源/包校验、DIY 健康、公开列表，以及本人草稿接口字段；目标环境验收记录应独立于 CI/MySQL 结果。

## 生产行为验收

- 原先 public 但无合格定稿记录：广场和公开详情隐藏，本人仍有草稿；只读验收不批改生产数据。
- 自己无订单、未付款、模拟付款、付款待审核、取消/退款/售后：`canPublish=false`，绕开客户端直接 PUT 也不能发布。
- 在批准的隔离验收数据上完成真实付款成功节点及现有审核节点后，发布固定样式/价格快照；不得把模拟回执写入生产或冒充真实支付验收。
- 保存新草稿不修改公开定稿；新版发布生成新的 publication ID，旧订单绑定不变。并发旧 revision 返回 `40914`。
- A→B→C 同款归属仍为 A；A 换新版不影响旧款的有效归属，明确下架或旧确认订单退款停止新归因。复制并改款后须自行付款确认才能成为新定稿。
- 合格定稿材料售罄仍可发布，但新下单继续库存拦截。
- 自购/模拟支付排除、净材料费分母、优惠后金额、等待期和退款回退按既有规则验收。

## 回滚边界

加表加列可留存，不需要为了二进制回滚删除数据。旧版本缺少本次发布门槛，不可在继续开放发布入口时把旧二进制回滚视为安全完成。优先修复或在维护期间阻断 DIY 写接口，并记录旧 contract 基线恢复的理由；不要删除定稿/订单/返现快照。回滚完成后重新确认对外门槛与已有订单读取。
