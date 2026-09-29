// Exports normalized chart fixtures for the public example reader. Synthetic
// inputs only; never use production user reports or competitor report content.
package main

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"os"
)

func main() {
	raw, e := os.ReadFile("internal/reportdoc/testdata/synthetic.json")
	if e != nil {
		panic(e)
	}
	var results map[string]json.RawMessage
	if e = json.Unmarshal(raw, &results); e != nil {
		panic(e)
	}
	out := map[string]reportdoc.Document{}
	for code, r := range results {
		d := reportdoc.New()
		d.Runtime = "example"
		d.Add(code, string(r))
		if code == "bazi" {
			d.Add("bazi_dayun", string(results["bazi_dayun"]))
		}
		d.Evidence = nil
		out[code] = d
	}
	e = json.NewEncoder(os.Stdout).Encode(out)
	if e != nil {
		panic(e)
	}
}
