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
	"正文优先采用明确标为现代中文释义的解释；必要短引文须忠实保留本轮来源的原字形和适用条件，不整段复述",
	"必要文献释义逐项核对本轮片段编号，分别保留对应主体与适用条件",
	"正文末尾不再另列整组来源或复述取用规则",
	"未知团队与回退条件须在对应表格单元格用若或建议验证写清，不用表前总声明代替",
	"古籍个案保留主体与组合条件，不能仅凭共同特征把个案泛化成该特征的吉凶通则",
	"原文肯定一项不等于否定另一项",
	"灰度、交接等应用须另句明确为本报告建议，不用[资料N]为新增结论背书",
	"缺少适用通则片段时保留盘面事实和一般建议",
	"回滚与风险控制效果也须写清待验证前提，不无条件声称不顺已受控制",
}

var reportNativeSourceRules = []string{
	"对应本次计算对象与字段的逐字复述沿用工具字形，不强改为检索版本",
	"[资料N]或书籍引文仍按该片段原字形，不能借工具另版放行",
	"混合来源分句标清，不任选来源",
	"工具不背书未输出的关键词、象意或解释",
	"拼接、编造、错对象和未计算事实仍属冲突",
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
			"起卦方式：时间起卦（time）。\n" +
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
		{Quote: observed97Attribution, Basis: "资料7只有兄弟等用神对象说明，没有生旺、月破旬空条件；实际资料6的占弟兄姐妹否泰段才有这些条件。不能把资料6的条件归入资料7，也不应为补末尾来源说明新增无必要的取用规则。"},
	}
	// Model only the two relevant source distinctions, preserving numbers 6/7.
	sourceFixture := &reportAttributionSearch{}
	for i, text := range []string{"合成未采用来源一。", "合成未采用来源二。", "合成未采用来源三。", "合成未采用来源四。", "合成未采用来源五。", "如占弟兄姐妹之否泰者，宜其生旺不宜臨月破旬空。", "兄弟爻：占兄弟、姐妹、族中兄弟、姑姨、妹夫及結拜兄弟都以兄弟爻爲用神。"} {
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("liuyao-fixture-%d", i+1), Text: text, Source: "synthetic", Revision: 1, Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))})
	}
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture, nil)
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
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture, nil)
}

// Only the short claims in this scenario are from report104's safe audit.
// The three short source distinctions are synthetic, not an export of its failed body/tools.
func TestReportLiuyaoObserved104LiteralAndSemanticShareRepair(t *testing.T) {
	const literalOne = "“小规模、小事可行，大事需谨慎”[资料2]"
	const literalTwo = "“贸然全面前进不利、宜预知弊端”[资料1]"
	const changedGlyph = "如占弟兄姐妹之否泰者，宜其生旺，不宜临月破旬空[资料3]"
	sources := []string{"歸妹：征凶，无攸利。象曰：澤上有雷，歸妹；君子以永終知敝。", "睽：小事吉。彖曰：是以小事吉。象曰：君子以同而異。", "如占弟兄姐妹之否泰者，宜其生旺不宜臨月破旬空。"}
	sourceFixture := &reportAttributionSearch{}
	for i, text := range sources {
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("report104-synthetic-source-%d", i+1), Text: text, Source: "synthetic-short-audit-distinction", Revision: 1, Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(text)))})
	}
	bad := reportBody{Summary: "保留本次计算和已确认输入，比较小范围试用与一次替换，现实条件仍需核对。", Content: "## 分析依据\n" + literalOne + "。\n" + literalTwo + "。\n" + changedGlyph + "。\n" + strings.Repeat("本地合成回归只验证双重反馈和既有修订上限，不代表真实模型质量。", 10) + "\n## 方案比较\n|方案|待核对|\n|---|---|\n|试用|验证回滚条件|\n## 行动建议\n先核实再决定范围。"}
	good := bad
	good.Content = "## 分析依据\n本次计算已完成，盘面关键事实如下：\n睽卦经文卦辞作“小事吉”[资料2]，彖辞谓“是以小事吉”[资料2]，象辞谓“君子以同而異”[资料2]。\n“如占弟兄姐妹之否泰者，宜其生旺不宜臨月破旬空”[资料3]。这是所引片段对该对象的传统观点，不是对项目实际团队或风险的证明。\n" + strings.Repeat("本地合成回归只验证双重反馈和既有修订上限，不代表真实模型质量。", 10) + "\n## 方案比较\n|条件|本报告建议|\n|---|---|\n|若回滚可行性未知|先验证隔离与回退演练|\n## 行动建议\n灰度、交接等为本报告自己的建议，先核实再决定范围。"
	for _, semantic := range []bool{true, false} {
		t.Run(fmt.Sprint("semantic_", semantic), func(t *testing.T) {
			issues := []reportCalibrationIssue{}
			if semantic {
				issues = []reportCalibrationIssue{{Quote: changedGlyph, Basis: "资料3原文为臨而非临；该直接转述未标转写，必须忠实字形或明确改作释义。"}}
			}
			fixture := &reportAttributionSearch{hits: sourceFixture.hits}
			required := []string{"直接引文小规模、小事可行，大事需谨慎", "直接引文贸然全面前进不利、宜预知弊端"}
			if semantic {
				required = append(required, "摘要或正文与已确认事实不一致", changedGlyph)
			}
			runReportLiuyaoGroundingFixture(t, bad, good, issues, fixture, nil, required...)
		})
	}
}

// Both complete sources and the complete calculation are from normally unlocked
// report102, not report104's failed body or its unavailable source attribution.
func TestReportNativeFieldsAndBookQuotesKeepTheirOwnSource(t *testing.T) {
	var fixture struct {
		Canonical       json.RawMessage
		CanonicalSHA256 string
		Sources         []struct {
			Citation            int
			Text, Title, SHA256 string
		}
	}
	raw, err := os.ReadFile("testdata/liuyao_report102_round13_dual_sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	var canonical any
	if err := json.Unmarshal(fixture.Canonical, &canonical); err != nil {
		t.Fatal(err)
	}
	fixture.Canonical, _ = json.Marshal(canonical)
	if fmt.Sprintf("%x", sha256.Sum256(fixture.Canonical)) != fixture.CanonicalSHA256 || fixture.CanonicalSHA256 != "cf20c6c8c7ffd4ca5d8baee6a68599367a0de7e835efd28dcd4af0d78c91c7d8" || len(fixture.Sources) != 2 {
		t.Fatal("normally unlocked complete canonical/source fixture changed")
	}
	sourceFixture := &reportAttributionSearch{}
	doc := reportdoc.New()
	for i, passage := range fixture.Sources {
		if passage.Citation != i+1 || fmt.Sprintf("%x", sha256.Sum256([]byte(passage.Text))) != passage.SHA256 {
			t.Fatal("normally unlocked numbered book passage changed")
		}
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("report102-book-%d", i+1), Text: passage.Text, Source: passage.Title, Revision: 2, Hash: passage.SHA256})
		doc.Blocks = append(doc.Blocks, reportdoc.Block{Kind: "citations", Items: []reportdoc.Item{{Label: fmt.Sprintf("资料%d", i+1), Value: passage.Text}}})
	}
	const nativeHexagram = "归妹：征凶，无攸利。"
	const nativeLine = "上九：睽孤，见豕负涂，载鬼一车，先张之弧，后说之弧，匪寇婚媾，往遇雨则吉。"
	for _, value := range []string{nativeHexagram, nativeLine} {
		if !strings.Contains(string(fixture.Canonical), value) {
			t.Fatal("native display text missing from actual canonical")
		}
	}
	const explicitBook = "“归妹：征凶，无攸利。”[资料1]"
	if err := validateReportCitations(reportBody{Content: explicitBook}, doc); err == nil {
		t.Fatal("the same native text must still fail when explicitly attributed to the different book version")
	}
	const wrongObject = "本次计算本卦卦辞为“归妹：征凶，无攸利。”"
	const stitched = "本次计算上九爻辞为“先张之弧，群疑亡也。”"
	const invented = "本次计算确认团队现已消除猜疑。"
	bad := reportBody{Summary: "对照本次计算与各自来源，比较小范围试用与一次替换，实际协作和回退条件待核实。", Content: "## 依据\n文献原句：" + explicitBook + "\n" + wrongObject + "\n" + stitched + "\n" + invented + "\n" + strings.Repeat("本地合成回归只验证来源归属传递与拒绝边界，不代表真实模型质量。", 8) + "\n## 条件比较\n|方案|待核对|\n|---|---|\n|试用|验证回退条件|\n## 本报告建议\n先核实再决定范围。"}
	good := bad
	good.Content = "## 本次计算字段\n|对应字段|本次计算原文|\n|---|---|\n|变卦卦辞|“" + nativeHexagram + "”|\n|返回的动爻爻辞，上九|“" + nativeLine + "”|\n## 文献原句\n书籍卦辞：“歸妹：征凶，无攸利。”[资料1]。书籍上九爻辞节选：“見豕負涂，載鬼一車”[资料2]。以上分别核对计算字段和所引书籍，不混为同一版本。\n" + strings.Repeat("本地合成回归只验证来源归属传递与拒绝边界，不代表真实模型质量。", 8) + "\n## 条件比较\n|条件|本报告建议|\n|---|---|\n|若回退条件尚未确认|先验证隔离和回退可行性|\n## 本报告建议\n实际团队状态待核实；工程验证是独立建议，不归为计算字段输出或文献结论。"
	if err := validateReportCitations(good, doc); err != nil {
		t.Fatalf("correctly separated native fields and faithful book quotes failed: %v", err)
	}
	issues := []reportCalibrationIssue{
		{Quote: wrongObject, Basis: "本次计算本卦是火泽睽，所引字段实际属于变卦雷泽归妹；字串存在也不能错配对象。"},
		{Quote: stitched, Basis: "本次计算上九爻辞没有这个连续字串；不能把不同段落拼接后声称为计算字段。"},
		{Quote: invented, Basis: "本次计算没有输出真实团队已消除猜疑；字段不能为未输出的现实解释背书。"},
	}
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture, fixture.Canonical, "直接引文归妹：征凶，无攸利。", wrongObject, stitched, invented)
}

// Short claims and complete evidence are from normally paid/unlocked report104.
// The mock review proves transport and bounded repair, not real-model judgement.
func TestReportObserved104LabelsAndTableEffectsReachCriticAndRepair(t *testing.T) {
	var fixture struct {
		Canonical       json.RawMessage
		CanonicalSHA256 string
		Sources         []struct {
			Citation            int
			Text, Title, SHA256 string
		}
	}
	raw, err := os.ReadFile("testdata/liuyao_report104_round17_sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	var canonical any
	if err := json.Unmarshal(fixture.Canonical, &canonical); err != nil {
		t.Fatal(err)
	}
	fixture.Canonical, _ = json.Marshal(canonical)
	if fmt.Sprintf("%x", sha256.Sum256(fixture.Canonical)) != fixture.CanonicalSHA256 || fixture.CanonicalSHA256 != "cf20c6c8c7ffd4ca5d8baee6a68599367a0de7e835efd28dcd4af0d78c91c7d8" || len(fixture.Sources) != 10 {
		t.Fatal("normally unlocked complete report104 canonical/sources changed")
	}
	sourceFixture := &reportAttributionSearch{}
	for i, source := range fixture.Sources {
		if source.Citation != i+1 || fmt.Sprintf("%x", sha256.Sum256([]byte(source.Text))) != source.SHA256 {
			t.Fatal("report104 numbered complete public source changed")
		}
		sourceFixture.hits = append(sourceFixture.hits, knowledge.Hit{ID: fmt.Sprintf("report104-source-%d", i+1), Text: source.Text, Source: source.Title, Revision: 2, Hash: source.SHA256})
	}
	const sourceClaim = "传统释义认为睽卦象征乖离、分歧，适合从小处着手，不宜大举推进。"
	const currentClaim = "九四子孙酉金空亡，传统上表示决策者当前思虑未定或准备未足。"
	const effectClaim = "容易回滚，只影响局部用户，恢复快"
	padding := strings.Repeat("本地合成回归只验证核对范围和修订边界，不代表真实模型或新报告内容质量。", 8)
	bad := reportBody{Summary: "保留本次计算与原始问题，比较小范围试用和一次替换，实际团队与技术条件仍待核对。", Content: "## 分析依据\n本卦卦辞为“睽：小事吉”[资料1]，" + sourceClaim + "\n## 影响因素\n" + currentClaim + "若现实中决策者已有清晰计划，可忽略此象。\n" + padding + "\n## 方案比较\n|维度|小范围试用|一次替换|\n|---|---|---|\n|回滚与恢复|" + effectClaim + "|整体回滚复杂，服务中断风险大|\n## 行动建议\n执行前核对实际条件。"}
	good := reportBody{Summary: bad.Summary, Content: "## 本次计算\n本卦火泽睽、变卦雷泽归妹，上九父母巳火明动化兄弟戌土且化墓，实际排盘时间为12:37。\n## 文献与报告建议\n本卦卦辞为“睽：小事吉”[资料1]。现代释义仅说明小事吉，不能由此反推大事不利。本报告另建议先验证小范围方案，这是工程建议，不归作该片段的原有结论。\n## 待核对问题\n决策者当前依据是否充足，应查实际需求与评估记录，空亡本身不指定其当前状态。\n" + padding + "\n## 条件比较\n|条件或方案要求|本报告建议|\n|---|---|\n|小范围方案的要求|先验证影响隔离、数据兼容和回退演练，这些条件尚未证明|\n|若上述前提已经验证|再比较局部恢复的影响与实测耗时|\n|若全量切换可由已验证的开关撤回|全量也可能有较低回退成本，仍据实际测量比较|\n## 行动建议\n先核实现实条件，再决定试用和替换范围；不以传统标签或免责声明替代所需证据。"}
	issues := []reportCalibrationIssue{
		{Quote: sourceClaim, Basis: "资料1只肯定小事吉，不提供大举不宜的否定通则；传统释义标签不能补足缺失结论，工程建议须另句归报告。"},
		{Quote: currentClaim, Basis: "用户没有确认决策者当前思虑或准备状态；传统上表示及后续可忽略不把具体当前状态改成条件，应写为待核对问题。"},
		{Quote: effectClaim, Basis: "输入没有确认影响隔离、数据兼容或回退演练；这是未确认的方案效果，不能因不是盘面字段而免审，须列明确前提。"},
	}
	runReportLiuyaoGroundingFixture(t, bad, good, issues, sourceFixture, fixture.Canonical, sourceClaim, currentClaim, effectClaim)
}

func runReportLiuyaoGroundingFixture(t *testing.T, bad, good reportBody, issues []reportCalibrationIssue, sourceFixture *reportAttributionSearch, calculation json.RawMessage, requiredRepairText ...string) {
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
	actualCalculation := calculation != nil
	if calculation == nil {
		calculation = calculations["liuyao"]
	}
	mcpCalls := 0
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": calculation}})
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
		for _, rule := range reportNativeSourceRules {
			if !strings.Contains(system, rule) {
				t.Errorf("writer, repair or critic lost native/book source attribution: %q", rule)
			}
		}
		allMessages := ""
		for _, message := range messages {
			content := fmt.Sprint(message.(map[string]any)["content"])
			allMessages += content
			if message.(map[string]any)["role"] == "tool" {
				var read struct{ Content string }
				_ = json.Unmarshal([]byte(content), &read)
				allMessages += read.Content
			}
		}
		critic := isReportCalibrationFixtureRequest(req)
		if critic {
			for _, rule := range reportCriticFactScopeRules {
				if !strings.Contains(system, rule) {
					t.Errorf("critic lost actual fact/condition/source scope: %q", rule)
				}
			}
		}
		if actualCalculation {
			if searchRequests > 0 && (critic || writers == 0) && !strings.Contains(allMessages, string(calculation)) {
				t.Error("initial writer or critic lost the complete unchanged calculation")
			}
			if !critic {
				tools, _ := json.Marshal(req["tools"])
				if !strings.Contains(string(tools), `"name":"read_report"`) {
					t.Error("writer or repair lost access to the original calculation")
				}
			}
		}
		if critic && !strings.Contains(system, "普通“本次计算已完成，盘面关键事实如下”等自然陈述不是机器状态标签") {
			t.Error("critic may invent a machine-status violation from an ordinary sentence")
		}
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
				calls := []any{map[string]any{"index": 0, "id": "source-1", "type": "function", "function": map[string]any{"name": "search_knowledge", "arguments": `{"query":"合成用神依据"}`}}}
				if actualCalculation {
					calls = append(calls, map[string]any{"index": 1, "id": "calculation-1", "type": "function", "function": map[string]any{"name": "read_report", "arguments": `{}`}})
				}
				event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": calls}, "finish_reason": "tool_calls"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
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
				for _, required := range requiredRepairText {
					if !strings.Contains(last, required) {
						t.Errorf("single repair lost literal/semantic feedback %q", required)
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
