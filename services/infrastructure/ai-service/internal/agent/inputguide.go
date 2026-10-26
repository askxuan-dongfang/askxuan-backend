package agent

import (
	"encoding/json"
	"strings"
)

// GuidedInputSchema decorates the configured catalog, keeping its fields and
// option values authoritative. Both clients consume this presentation metadata.
func GuidedInputSchema(code, raw string) string {
	var schema map[string]interface{}
	if json.Unmarshal([]byte(raw), &schema) != nil {
		return raw
	}
	fields, _ := schema["fields"].([]interface{})
	for _, field := range solarInputFields(code) {
		encoded, _ := json.Marshal(field)
		var value map[string]interface{}
		_ = json.Unmarshal(encoded, &value)
		replaced := false
		for i, existing := range fields {
			if f, ok := existing.(map[string]interface{}); ok && f["key"] == field.Key {
				fields[i], replaced = value, true
				break
			}
		}
		if !replaced {
			fields = append(fields, value)
		}
	}
	schema["fields"] = fields
	for _, value := range fields {
		f, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := f["key"].(string)
		help, initial := "", ""
		descriptions := map[string]string{}
		switch key {
		case "birthDate":
			help = "填写 1900–2100 年的出生年月日。农历按 YYYY-MM-DD 输入，例如 1990-02-30；不确定时请先核对出生记录。"
			if code == "marriage" {
				help = "选填公历生日；不知道可以留空，解读会以您描述的关系背景为主。"
			}
		case "partnerBirthDate":
			help = "选填对方的公历生日；只填写您已知的信息。"
			if f["helpText"] == nil { // Legacy personal/matching forms only.
				f["visibleWhen"] = map[string]string{"key": "mode", "value": "matching"}
			}
		case "birthTime":
			help = "按北京时间（UTC+8）填写原始时、分，不要自行减经度时差；已知历史夏令钟表记录应先确认是否还原为标准时间。"
		case "calendarType":
			initial = "solar"
			help = "先确认公历或农历。农历闰月暂请换算为公历后填写。"
		case "gender":
			help = "用于传统排盘规则，请按出生资料选择。"
		case "birthplace":
			help = "例如：浙江省杭州市西湖区；填写城市和区县即可，无需详细地址。"
		case "mode":
			if code != "marriage" {
				break
			}
			initial = "personal"
			descriptions = map[string]string{"personal": "梳理自己的关系期待与相处模式", "matching": "结合双方背景，讨论相处与沟通"}
			help = "关系背景是本专题的主要依据，生日为补充资料。"
		case "spread":
			initial = "single"
			descriptions = map[string]string{"single": "聚焦一个问题，获得一条提醒", "three": "从多个角度梳理事情的线索", "love": "关注感受、关系与相处", "decision": "比较选择，思考各自的得失"}
			help = "按这次问题选择一种牌阵；提交后完成抽牌。"
		case "scene":
			initial = "home"
			descriptions = map[string]string{"home": "起居、采光与生活动线", "office": "工位、协作与工作环境", "shop": "门面、陈列与顾客动线"}
			help = "下一步请描述布局、使用方式和困扰，已知信息越具体越好。"
		case "orientation":
			help = "例如：入户门朝南、书桌朝东；不清楚可以留空，无需猜测角度。"
		case "location":
			help = "填写所在城市，例如：上海市。无需定位或详细住址。"
		case "method":
			if code == "liuyao" {
				initial = "auto"
				help = "选一种即可，系统只收集该方式需要的资料。"
				descriptions = map[string]string{"auto": "系统起卦，无需填写数字或时间", "time": "用您指定的问事时间起卦", "number": "用您想到的 2–3 个整数起卦"}
			}
		case "numbers":
			if code == "liuyao" {
				help = "输入 2–3 个 0–999999 的整数，用空格或逗号隔开，例如：12 34 56。"
				f["placeholder"] = "例如：12 34 56"
				f["visibleWhen"] = map[string]string{"key": "method", "value": "number"}
				f["requiredWhen"] = map[string]string{"key": "method", "value": "number"}
				f["validation"] = "divination-numbers"
			}
		case "yongShenTarget":
			if code == "liuyao" {
				f["label"] = "这次主要关心什么"
				help = "选择最贴近问题的一项；具体人物、背景和期待在下一步说明。"
				descriptions = map[string]string{"官鬼": "工作、求职、职责与规则", "妻财": "收入、支出与资源安排", "子孙": "成果、身心状态与生活感受", "父母": "学习、考试、合同与长辈", "兄弟": "伙伴关系、竞争与协作"}
			}
		case "eventTime":
			help = "按北京时间（UTC+8）填写；若问的是当下，可以使用当前时间。"
			if code == "liuyao" {
				f["visibleWhen"] = map[string]string{"key": "method", "value": "time"}
				f["requiredWhen"] = map[string]string{"key": "method", "value": "time"}
			}
		}
		if help != "" && f["helpText"] == nil {
			f["helpText"] = help
		}
		options, _ := f["options"].([]interface{})
		for _, value := range options {
			if o, ok := value.(map[string]interface{}); ok {
				v, _ := o["value"].(string)
				if d := descriptions[v]; d != "" {
					o["description"] = d
				}
				if initial == v {
					f["defaultValue"] = initial
				}
			}
		}
	}
	// Calendar comes first because it changes how the birthday is entered.
	if code == "bazi" || code == "ziwei" {
		for i, value := range fields {
			if f, ok := value.(map[string]interface{}); ok && f["key"] == "calendarType" {
				ordered := append([]interface{}{value}, fields[:i]...)
				schema["fields"] = append(ordered, fields[i+1:]...)
				break
			}
		}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return string(encoded)
}

// Both H5/native forms and the tool guard consume the same time/location fields.
func solarInputFields(code string) []Field {
	birth := oneOf(code, "bazi", "bazi_dayun", "ziwei", "ziwei_horoscope", "ziwei_flying_star", "marriage", "fortune")
	if !birth && !oneOf(code, "qimen", "liuyao", "meihua", "daliuren", "taiyi") {
		return nil
	}
	lo, hi := -180.0, 180.0
	place, label := "location", "起局地点"
	if birth {
		place, label = "birthplace", "出生地"
	}
	fields := []Field{
		{Key: "timeBasis", Label: "排盘时间口径", Type: "select", DefaultValue: "true_solar", Options: []Option{{Value: "true_solar", Label: "真太阳时（按地点校正）"}, {Value: "beijing", Label: "北京时间（不校正）"}}, HelpText: "时间仍填写北京时间。真太阳时由地点经度和当日均时差计算，跨时辰或跨日会同步更新排盘。"},
		{Key: place, Label: label, Type: "text", HelpText: "填写城市及区县；用于解析经度。地点不明确时请核对，不能猜测。"},
		{Key: "longitude", Label: "经度（选填）", Type: "number", Min: &lo, Max: &hi, HelpText: "已确认经度可直接填写；东经为正、西经为负。优先使用此值，勿填写纬度。"},
	}
	if code == "fortune" {
		for i := range fields {
			fields[i].VisibleWhen = &FieldCondition{Key: "fortuneMode", Value: "personal"}
		}
	}
	if code == "marriage" {
		for _, field := range append([]Field(nil), fields...) {
			field.Key = "partner" + strings.ToUpper(field.Key[:1]) + field.Key[1:]
			field.Label = "对方的" + field.Label
			fields = append(fields, field)
		}
	}
	if code == "ziwei" {
		fields = append(fields, Field{Key: "targetDate", Label: "运限参考日期（选填）", Type: "date", HelpText: "留空以生成当天为起点，同时核对一年后的运限。"})
	}
	return fields
}
