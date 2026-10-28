package reportdoc

import "testing"

func TestLiuyaoSelectionKeepsPrimaryAndCandidateSeparate(t *testing.T) {
	d := New()
	d.Add("liuyao", `{"用神取用":[{"目标六亲":"兄弟","取用状态":"resolved","已选用神":{"爻位":"六三爻","六亲":"兄弟","纳甲":"丑","来源":"本卦","依据":["工具既有取用"]},"候选用神":[{"爻位":"六五爻","六亲":"兄弟","纳甲":"未","来源":"本卦","依据":["候选比较"]}]}]}`)
	if len(d.Blocks) != 1 || d.Blocks[0].Title != "用神主次与取用依据" || len(d.Blocks[0].Rows) != 2 {
		t.Fatal(d.Blocks)
	}
	rows := d.Blocks[0].Rows
	if rows[0][0] != "主用神" || rows[0][2] != "六三爻" || rows[0][4] != "丑" || rows[1][0] != "备选用神" || rows[1][2] != "六五爻" || rows[1][4] != "未" {
		t.Fatal("selection lost or reversed", rows)
	}
	unknown := New()
	unknown.Add("liuyao", `{"用神取用":[{"目标六亲":"兄弟","取用状态":"unresolved","取用说明":"未取定","候选用神":[]}]}`)
	if len(unknown.Blocks) != 1 || unknown.Blocks[0].Rows[0][0] != "未取定" || unknown.Blocks[0].Rows[0][2] != "—" {
		t.Fatal("manufactured a primary selection", unknown.Blocks)
	}
}
