package settings

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/askxuan/ai-service/internal/provider"
)

// A fixed 64x64 white PNG checks the configured image route without user data.
const probeImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAAXklEQVR4nO3PMQ0AMAzAsPInvYLYYVWKESTzjhsd8KsBrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BbQHKU9LC7/CP1AAAAABJRU5ErkJggg=="

// Endpoint IDs cannot be authenticated by listing public models. Probe every
// configured ID before publishing, with bounded concurrency and total latency.
func probeVolcengineModels(ctx context.Context, p provider.Provider, models []provider.ModelOption) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	jobs := make(chan int, len(models))
	errs := make([]error, len(models))
	for i := range models {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for worker := 0; worker < min(4, len(models)); worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				m := models[i]
				message := provider.Message{Role: "user", Content: "Reply with OK only."}
				if m.SupportsVision {
					message.ImageDataURLs = []string{probeImage}
				}
				_, err := p.Complete(ctx, provider.Request{Model: m.ID, Messages: []provider.Message{message}, MaxTokens: 64})
				if err != nil {
					// Never return upstream response bodies or credential-bearing errors.
					errs[i] = fmt.Errorf("火山方舟模型 %s 连接测试失败，请检查 API Key、Model ID / Endpoint ID、地域和模型能力", m.ID)
				}
			}
		}()
	}
	workers.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
