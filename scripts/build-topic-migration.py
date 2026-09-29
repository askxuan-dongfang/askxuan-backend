"""Regenerate the reviewed topic configuration migration from the embedded catalog."""
import json
from pathlib import Path
root = Path(__file__).resolve().parents[1]
rows = json.loads((root / "services/infrastructure/ai-service/internal/agent/topic_catalog.json").read_text())
def sql(v):return "CONVERT(0x"+v.encode().hex()+" USING utf8mb4)"
s=['SET NAMES utf8mb4;','-- Apply with the matching AI service. Keeps prices, availability and existing reports unchanged.','START TRANSACTION;']
for r in rows:
 prompt='依据工具已返回的数据生成报告。明确区分用户原述、计算结果、编辑规则和模型解释；引用证据中的规则编号或数据来源。'+r['description']
 cfg=json.dumps(dict(enabled=True,server='builtin',tool=r['code']))
 s.append("UPDATE askxuan_ai.ai_skill SET version='3.1.0',source_type='reviewed_skill',source_ref='askxuan/topic-evidence@20260929',description="+sql(r['description'])+",input_schema="+sql(json.dumps(r['inputSchema'],ensure_ascii=False))+",prompt_template="+sql(prompt)+",tool_config="+sql(cfg)+" WHERE code='"+r['code']+"';")
 s.append("UPDATE askxuan_ai.ai_report_product SET version='3.0' WHERE code='"+r['code']+"';")
s.append('COMMIT;');(root/'scripts/db/20260929_ai_topic_evidence.sql').write_text('\n'.join(s)+'\n')
