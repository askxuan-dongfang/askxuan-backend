package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// A deployment-owned adapter translates the authorized supplier API into this contract.
// Model/user input never controls URL or credentials. No result is not a zero count.
func nameSearch(ctx context.Context, in map[string]any) []EvidenceChart {
	mode := stringValue(in["nameSearch"])
	if mode != "duplicate" && mode != "trademark" {
		return nil
	}
	title := "重名检索"
	env := "AI_NAME_DUPLICATE"
	if mode == "trademark" {
		title = "商标检索"
		env = "AI_NAME_TRADEMARK"
	}
	out := EvidenceChart{Kind: "table", Title: title, Columns: []string{"名称", "状态", "来源与范围", "结果"}, Rows: [][]string{}}
	if strings.TrimSpace(stringValue(in["name"])) == "" {
		out.Rows = append(out.Rows, []string{"—", "待补充候选名称", "需要完整姓名或商标名称", "AI 建议产生后请再次提交查询"})
		return []EvidenceChart{out}
	}
	names, err := nameCandidates(in)
	if err != nil {
		return nil
	}
	endpoint := os.Getenv(env + "_URL")
	key := os.Getenv(env + "_KEY")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, name := range names {
		if endpoint == "" {
			out.Rows = append(out.Rows, []string{name, "未接通", "尚未配置授权数据源", "不能据此认定无重名或无商标"})
			continue
		}
		b, _ := json.Marshal(map[string]string{"name": name, "region": stringValue(in["searchRegion"]), "class": stringValue(in["trademarkClass"])})
		req, e := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
		if e != nil {
			out.Rows = append(out.Rows, []string{name, "配置无效", "—", "未获得结果"})
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		res, e := client.Do(req)
		if e != nil {
			out.Rows = append(out.Rows, []string{name, "查询失败", "—", "稍后重试"})
			continue
		}
		var data struct {
			Source string `json:"source"`
			Scope  string `json:"scope"`
			AsOf   string `json:"asOf"`
			Result string `json:"result"`
			Name   string `json:"name"`
		}
		e = json.NewDecoder(io.LimitReader(res.Body, 32768)).Decode(&data)
		res.Body.Close()
		if e != nil || res.StatusCode != 200 || data.Name != name || strings.TrimSpace(data.Source) == "" || data.Scope == "" || data.AsOf == "" || data.Result == "" || len(data.Result) > 4000 {
			out.Rows = append(out.Rows, []string{name, "结果未通过核验", "—", "需要来源、覆盖范围、更新时间及匹配的名称"})
			continue
		}
		out.Rows = append(out.Rows, []string{name, "已查询", data.Source + " · " + data.Scope + " · " + data.AsOf, data.Result})
	}
	out.Note = "外部检索仅对数据源覆盖范围负责；商标结果不是注册成功保证。"
	return []EvidenceChart{out}
}
