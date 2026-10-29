package logic

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var reportLiuyaoObserved95Rules = []string{
	"兄弟旺不证明团队已具实力",
	"父母日破不证明实际旧流程存在漏洞",
	"世爻空亡不证明当前兜底未就绪",
	"空亡、日破、动变等只描述传统卦象倾向，不能证明任何对象的现实实施或完成状态",
	"项目、方案或流程是否已启动、落地、验收，须从用户确认或实际记录核实",
	"先列已确认现实条件，再把象意单列为传统解释，不用提示或说明当前把二者连接为现状结论",
	"不得从爻辞推断真实合作关系、他人动机或猜疑成因",
	"摘要和正文不能把这些象意写成用户已确认的现状，应改为条件或待核对检查项",
	"总免责声明不能抵消具体断言",
	"灰度、回滚可作为独立建议，不声称实际现状",
	"起卦方式只写时间、数字、手选或自动等中文名称，不在正文附带time、number、select、auto等方法枚举值",
	"卦辞、爻辞原文由平台图表展示；正文只写明确标为现代中文释义的解释，不复引原句，也不以省略号拼接原句",
	"必要文献释义逐项核对本轮片段编号，分别保留对应主体与适用条件",
	"正文末尾不再另列整组来源或复述取用规则",
	"未知团队与回退条件须在对应表格单元格用若或建议验证写清，不用表前总声明代替",
	"古籍个案保留主体与组合条件，不能仅凭共同特征把个案泛化成该特征的吉凶通则",
	"原文肯定一项不等于否定另一项",
	"灰度、交接等应用须另句明确为本报告建议，不用[资料N]为新增结论背书",
	"缺少适用通则片段时保留盘面事实和一般建议",
	"回滚与风险控制效果也须写清待验证前提，不无条件声称不顺已受控制",
}

func TestReportLiuyaoRealWorldGroundingRulesStayTopicScoped(t *testing.T) {
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		t.Run(topic, func(t *testing.T) {
			writer := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
			critic, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192}, 32768, reportCalibrationInput{Topic: topic, Draft: reportBody{Summary: "只核对本次已确认的现实事实。"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range reportLiuyaoObserved95Rules {
				for _, prompt := range []string{writer, critic.SystemPrompt} {
					if strings.Contains(prompt, rule) != (topic == "liuyao") {
						t.Errorf("liuyao-only writer/critic rule missing or leaked to %s: %q", topic, rule)
					}
				}
			}
		})
	}
}

// The short claims are from synthetic reports 95 and 97. MCP and model are
// fixtures: this verifies policy transport and repair bounds, not model quality.
func TestReportLiuyaoObserved95ClaimsReachWriterCriticAndSingleRepair(t *testing.T) {
	const observed97Attribution = "兄弟爻主同辈、协作，占兄弟姐妹宜生旺、不宜月破旬空的取用方法见于古籍[资料7]"
	bad := reportBody{
		Summary: "用神兄弟丑土旺相，团队有实力可用；世爻子孙空亡、应爻父母日破并存，提示协作摩擦与旧流程破口。",
		Content: "## 影响因素\n用神兄弟丑土旺相、静、不空，说明“同类之人”（团队成员、协作方）本身是有实力、可依靠的，具备推进试点的条件。猜疑多半是自我放大的，真实关系仍是协作，可通过沟通化解。\n" +
			"应爻初九父母巳火“日破”，指向对方环境或旧流程一侧有“破口”，一次性触碰全部旧流程，风险会叠加在这处破口上。世爻子孙酉金为“制鬼、解忧”之位，但当前值空亡，说明“化解压力、兜底保障”的能力此刻尚未到位。\n" +
			"## 方案比较\n| 选择 | 核对项目 |\n| --- | --- |\n| 小范围试用 | 核对回退步骤 |\n| 全量替换 | 核对覆盖范围 |\n" +
			"当前保障机制（世爻子孙空亡）与旧流程破口（应爻父母日破）尚未处理到位前，不宜一步到位。\n" +
			"世持子孙而空亡，提示**新方案/新模块本身目前处于“未落实、未填实”的状态**。\n" +
			"起卦方式：时间起卦（time）。上九爻辞“睽孤……往遇雨则吉”。\n" +
			"## 行动建议\n先记录模块范围与验证指标。卦象不是确定性的成败预言；模块规模、团队人数、验收指标、时间窗口等现实参数未包含在本次资料中，最终决策仍需结合实际测试结果与专业意见。\n传统依据说明：" + observed97Attribution + "。",
	}
	good := reportBody{
		Summary: "团队能力、旧流程漏洞及兜底保障状态均未在用户资料中确认。可把它们列为待核对项，灰度与回滚作为独立行动选项。",
		Content: "## 依据与边界\n" + strings.Repeat("本地合成回归只验证指令传递，不代表真实模型质量。", 8) +
			"\n起卦方式：时间起卦。现代中文释义：遇到疑虑时可先核对信息并沟通；这是传统象意的转述，不证明实际团队已有猜疑或关系变化。\n团队是否已有推进实力、旧流程是否存在漏洞、当前兜底是否就绪，都应以用户确认或实际测试记录为准。兄弟旺、父母日破和世爻空亡本身不能证明这些现实状态。新方案是否已经启动或落地，须从实际实施记录核实；空亡仅作传统卦象倾向解释。\n" +
			"## 条件比较\n| 条件 | 核对行动 |\n| --- | --- |\n| 若现有记录不能说明团队能力 | 先核对负责人和验收经验 |\n| 若尚无流程缺陷证据 | 先做差异测试 |\n| 若保障状态未知 | 核对监控与回退演练记录 |\n| 若试点涉及多个团队 | 建议分别验证负责人和协作接口 |\n| 若回退条件未知 | 建议验证旧流程是否保留及回退是否可用 |\n" +
			"## 独立行动建议\n可选择小范围验证并准备回滚，具体规模与时点待确认。这些是独立的执行建议，不意味着已有漏洞或当前保障不足；没有证据时不排除其他可行选择。",
	}
	issues := []reportCalibrationIssue{
		{Quote: "用神兄弟丑土旺相，团队有实力可用", Basis: "用户没有确认团队已有实力；兄弟旺不证明现实能力。"},
		{Quote: "旧流程一侧有“破口”", Basis: "用户没有确认旧流程存在漏洞；父母日破不能证明实际缺陷。"},
		{Quote: "“化解压力、兜底保障”的能力此刻尚未到位", Basis: "用户没有确认当前兜底未就绪；世爻空亡不能证明现实状态，总免责声明不能抵消该断言。"},
		{Quote: "猜疑多半是自我放大的，真实关系仍是协作", Basis: "用户没有确认真实合作关系、他人动机或猜疑成因，不能从爻辞推断这些现实情况。"},
		{Quote: "世持子孙而空亡，提示**新方案/新模块本身目前处于“未落实、未填实”的状态**", Basis: "用户未确认模块实施状态；空亡不能证明项目、方案或流程是否实际落地。"},
		{Quote: "起卦方式：时间起卦（time）", Basis: "本地合成输入method仍为time，但正文只写中文起卦名称，不附原始方法枚举；计算输入不因此改动。"},
		{Quote: "上九爻辞“睽孤……往遇雨则吉”", Basis: "本地回归仅检验正文释义边界：原文在图表，正文应明确标为现代中文释义，不复引或省略拼接。这不判断09失败全文的引文归属或字形责任。"},
		{Quote: observed97Attribution, Basis: "资料7只有兄弟等用神对象说明，没有生旺、月破旬空条件；实际资料6的占弟兄姐妹否泰段才有这些条件。不能把资料6的条件归入资料7，也不应为补末尾来源说明新增无必要的取用规则。"},
	}
	// Model only the two relevant source distinctions, preserving numbers 6/7.
	sourceFixture := &reportAttributionSearch{}
	for i, text := range []string{"合成未采用来源一。", "合成未采用来源二。", "合成未采用来源三。", "合成未采用来源四。", "合成未采用来源五。", "如占弟兄姐妹之否泰者，宜其生旺不宜臨月破旬空。", "兄弟爻：占兄弟、姐妹、族中兄弟、姑姨、妹夫及結拜兄弟都以兄弟爻爲用神。"} {
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("liuyao-fixture-%d", i+1), Text: text, Source: "synthetic", Revision: 1, Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))})
	}
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture)
}

// The claims and complete numbered passages are from normally unlocked report102.
func TestReportLiuyaoObserved102SourceScopeReachesSingleRepair(t *testing.T) {
	const caseClaim = "关于“化墓”，古籍中多将动爻化墓视为一种不吉或受困的象意，需谨慎对待[资料6]"
	const extensionClaim = "睽卦辞与彖传的传统释义是：火泽相离、意见不同，但若能喜悦而附丽于光明、以柔顺上行、得中而应刚，则利于**小事、渐进、分而治之**，不利于以大事强行推进[资料2]。"
	bad := reportBody{Summary: "本次保留盘面与既定用神，比较小范围试用与整体替换；现实条件仍需核实。", Content: "## 分析依据\n" + caseClaim + "。\n" + extensionClaim + "\n" + strings.Repeat("本地合成回归只验证规则传递，不代表真实模型质量。", 8) + "\n## 方案比较\n|方案|待核对|\n|---|---|\n|试用|验证回滚条件|\n## 行动建议\n保留本次计算，核对交接安排。"}
	good := reportBody{Summary: bad.Summary, Content: "## 文献释义\n现代中文释义：睽的片段说小事吉，并说明相异之中仍可相通[资料2]。这不等于所有大事不利。\n现代中文释义：资料所述戌月占南行例中，官鬼巳火动而生世，却月墓又化墓，并兼六冲，作者对该组合疑为不吉[资料6]。不能只取共同的化墓特征，把这个案概括成本盘化墓的通则。\n" + strings.Repeat("本地合成回归只验证规则传递，不代表真实模型质量。", 8) + "\n## 条件比较\n|条件|核对行动|\n|---|---|\n|若试用要限定影响面并可回滚|建议先验证隔离与回退演练，效果未知|\n## 本报告建议\n可以把灰度、交接和回滚列为本报告的独立工程建议，先验证再决定范围；这些不是所引片段本身的结论。没有适用本盘的通则原文时，保留原盘面与一般建议，不新增吉凶规则。"}
	issues := []reportCalibrationIssue{
		{Quote: caseClaim, Basis: "资料6是官鬼巳火在戌月入墓又化墓且兼六冲的特定案例，作者疑该组合不吉；不能只因化墓共同特征，泛化成所有动爻化墓不吉或受困的通则。"},
		{Quote: extensionClaim, Basis: "资料2说小事吉，不代表大事不利；分而治之是报告新增的工程应用，应另句明确为本报告建议，不能由[资料2]为额外结论背书。"},
	}
	var passages []struct {
		Citation    int
		Text, Title string
		SHA256      string
	}
	raw, err := os.ReadFile("testdata/liuyao_report102_round13_sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &passages); err != nil || len(passages) != 2 {
		t.Fatal("actual numbered source fixture missing", err)
	}
	sourceFixture := &reportAttributionSearch{}
	for number := 1; number <= 6; number++ {
		text, title := fmt.Sprintf("合成未采用来源%d。", number), "synthetic"
		for _, passage := range passages {
			if passage.Citation == number {
				if fmt.Sprintf("%x", sha256.Sum256([]byte(passage.Text))) != passage.SHA256 {
					t.Fatal("actual public source text changed")
				}
				text, title = passage.Text, passage.Title
			}
		}
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("report102-source-%d", number), Text: text, Source: title, Revision: 2, Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))})
	}
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture)
}

func runReportLiuyaoGroundingFixture(t *testing.T, bad, good reportBody, issues []reportCalibrationIssue, sourceFixture *reportAttributionSearch) {
	t.Helper()
	const question = "现有模块先做小范围试用，还是一次替换全部旧流程？重点梳理团队协作、变更风险与可执行的验证步骤。"
	const inputs = `{"method":"time","eventTime":"2026-10-28T14:30","location":"中国新疆乌鲁木齐市沙依巴克区","timeBasis":"true_solar","yongShenTarget":"兄弟"}`
	fixture, err := os.ReadFile("../reportdoc/testdata/synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	var calculations map[string]json.RawMessage
	if err := json.Unmarshal(fixture, &calculations); err != nil {
		t.Fatal(err)
	}
	mcpCalls := 0
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": calculations["liuyao"]}})
	}))
	t.Cleanup(mcp.Close)
	writers, reviews, searchRequests := 0, 0, 0
	p := provider.NewOpenAICompatible("http://liuyao-grounding.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		messages := req["messages"].([]any)
		system := fmt.Sprint(messages[0].(map[string]any)["content"])
		for _, rule := range reportLiuyaoObserved95Rules {
			if !strings.Contains(system, rule) {
				t.Errorf("writer, repair or critic lost liuyao grounding: %q", rule)
			}
		}
		critic := isReportCalibrationFixtureRequest(req)
		wantEffort := "low"
		if critic {
			wantEffort = "high"
		}
		thinking, _ := req["thinking"].(map[string]any)
		if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != wantEffort || thinking["type"] != "enabled" {
			t.Error("grounding repair changed output budget or reasoning policy")
		}
		w := httptest.NewRecorder()
		if critic {
			reviews++
			var input reportCalibrationInput
			if err := json.Unmarshal([]byte(fmt.Sprint(messages[1].(map[string]any)["content"])), &input); err != nil {
				return nil, err
			}
			want := bad
			if reviews == 2 {
				want = good
			}
			if input.Topic != "liuyao" || input.Question != question || string(input.Inputs) != `{"eventTime":"2026-10-28T14:30","location":"中国新疆乌鲁木齐市沙依巴克区","method":"time","timeBasis":"true_solar","yongShenTarget":"兄弟"}` || input.Draft != want {
				t.Error("critic lost actual question, confirmed inputs or exact current draft")
			}
			var items []reportdoc.Item
			if len(input.Sources) != 1 || json.Unmarshal([]byte(input.Sources[0].Text), &items) != nil || len(items) != len(sourceFixture.hits) {
				t.Fatal("critic lost complete source passages and original numbers")
			}
			for i, item := range items {
				if item.Label != fmt.Sprintf("资料%d", i+1) || item.Value != sourceFixture.hits[i].Text {
					t.Error("critic changed actual source number or passage")
				}
			}
			result := `{"issues":[]}`
			if reviews == 1 {
				raw, _ := json.Marshal(map[string]any{"issues": issues})
				result = string(raw)
			}
			writeReportCalibrationFixture(w, result)
		} else {
			if searchRequests == 0 {
				searchRequests++
				event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "source-1", "type": "function", "function": map[string]any{"name": "search_knowledge", "arguments": `{"query":"合成用神依据"}`}}}}, "finish_reason": "tool_calls"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
				response := w.Result()
				response.Request = r
				return response, nil
			}
			writers++
			body := bad
			if writers == 2 {
				body = good
				last := fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"])
				for _, issue := range issues {
					if !strings.Contains(last, issue.Quote) {
						t.Errorf("single repair lost exact issue: %q", issue.Quote)
					}
				}
				if !strings.Contains(last, "不要重新计算") {
					t.Error("single repair lost cached calculation boundary")
				}
				_, sourceJSON, ok := strings.Cut(last, "本轮已检索片段（保留原资料编号）：")
				sourceJSON, _, end := strings.Cut(sourceJSON, "\n请重新读取已计算图表")
				var frames []reportdoc.Evidence
				var items []reportdoc.Item
				if !ok || !end || json.Unmarshal([]byte(sourceJSON), &frames) != nil || len(frames) != 1 || json.Unmarshal([]byte(frames[0].Text), &items) != nil || len(items) != len(sourceFixture.hits) {
					t.Fatal("repair lost complete numbered source frames")
				}
				for i, hit := range sourceFixture.hits {
					if items[i].Label != fmt.Sprintf("资料%d", i+1) || items[i].Value != hit.Text {
						t.Error("repair changed original numbered source passages")
					}
				}
			}
			raw, _ := json.Marshal(body)
			event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(raw)}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
		}
		response := w.Result()
		response.Request = r
		return response, nil
	})})
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stage := range []string{"checking", "calculating", "writing"} {
		mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	skill := &model.AISkill{Code: "liuyao", Status: model.SkillStatusEnabled, InputSchema: reportToolSchema("liuyao"), ToolConfig: `{"enabled":true,"server":"taibu","tool":"liuyao"}`}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil), Knowledge: &knowledge.Store{Remote: sourceFixture}, KnowledgeEnabled: true}
	s.AIConfig.ContextWindow = 1048576
	got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, UserID: "synthetic-owner", SkillCode: "liuyao", Question: question, InputsJSON: inputs, ChaptersJSON: `["依据与边界","条件比较","独立行动建议"]`})
	if err != nil || got != good || writers != 2 || reviews != 2 || doc.ModelCalls != 5 || mcpCalls != 1 || doc.ToolCalls != 2 || searchRequests != 1 || sourceFixture.calls != 1 || usage.PromptTokens != 31 || usage.CompletionTokens != 20 || usage.ReasoningTokens != 9 {
		t.Fatalf("changed shared repair/calculation/usage bounds: err=%v bodyEqual=%t writers=%d reviews=%d models=%d mcp=%d tools=%d usage=%+v", err, got == good, writers, reviews, doc.ModelCalls, mcpCalls, doc.ToolCalls, usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
