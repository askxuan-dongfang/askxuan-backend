package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type HTTPEmbedder struct {
	URL, Key, Model string
	Client          *http.Client
}

func EmbeddingFromEnv() Embedder {
	u, m := os.Getenv("AI_EMBEDDING_URL"), os.Getenv("AI_EMBEDDING_MODEL")
	if u == "" || m == "" {
		return nil
	}
	return &HTTPEmbedder{u, os.Getenv("AI_EMBEDDING_KEY"), m, &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (e *HTTPEmbedder) Identity() string { return e.Model }
func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	u, err := url.Parse(e.URL)
	if err != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("embedding URL invalid")
	}
	b, _ := json.Marshal(map[string]any{"model": e.Model, "input": text})
	r, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(e.URL, "/")+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if e.Key != "" {
		r.Header.Set("Authorization", "Bearer "+e.Key)
	}
	res, err := e.Client.Do(r)
	if err != nil {
		return nil, errors.New("嵌入服务不可用")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("嵌入服务调用失败")
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&out) != nil || len(out.Data) != 1 || len(out.Data[0].Embedding) < 2 || len(out.Data[0].Embedding) > 8192 {
		return nil, errors.New("embedding result invalid")
	}
	v := out.Data[0].Embedding
	n := 0.
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, errors.New("embedding result nonfinite")
		}
		n += x * x
	}
	if n == 0 {
		return nil, errors.New("embedding result zero")
	}
	return v, nil
}
