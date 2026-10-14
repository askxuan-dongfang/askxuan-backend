package settings

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/askxuan/ai-service/internal/provider"
)

// A fixed 64x64 white PNG checks the configured image route without user data.
const probeImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAAXklEQVR4nO3PMQ0AMAzAsPInvYLYYVWKESTzjhsd8KsBrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BrQGtAa0BbQHKU9LC7/CP1AAAAABJRU5ErkJggg=="

// Endpoint IDs cannot be authenticated by listing public models. Probe every
// configured ID before publishing, with bounded concurrency and total latency.
func probeVolcengineModels(ctx context.Context, p provider.Provider, models []provider.ModelOption, plan bool) error {
	budget, timeout := 64, 15*time.Second
	if plan {
		// Reserve space for models with mandatory reasoning before their answer.
		budget, timeout = 512, 25*time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
				_, err := p.Complete(ctx, provider.Request{Model: m.ID, Messages: []provider.Message{message}, MaxTokens: budget})
				if err != nil {
					// Never return upstream response bodies or credential-bearing errors.
					errs[i] = fmt.Errorf("火山方舟模型 %s 连接测试失败：%s", m.ID, volcengineProbeFailure(err))
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

func volcengineProbeFailure(err error) string {
	var status *provider.HTTPError
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
		return "请求超时，请稍后重试或检查服务连通性"
	case errors.Is(err, context.Canceled):
		return "请求已取消，请重新测试"
	case errors.Is(err, provider.ErrEmptyContent):
		return "模型未返回正文，可能已耗尽测试输出预算；请核对模型能力后重试"
	case errors.As(err, &status):
		switch status.StatusCode {
		case 401, 403:
			return "鉴权或权限不足，请核对 API Key、套餐与接口地址是否匹配，以及模型是否已开通"
		case 404:
			return "模型或接口不存在，请核对套餐 Model Name、模型服务 Model ID / Endpoint ID 和地域"
		case 400, 422:
			return "模型不支持请求参数或输入类型，请核对模型能力；Agent Plan 使用模型默认思考设置"
		case 429:
			return "请求受限或额度不足，请在火山方舟检查套餐额度和并发限制"
		default:
			if status.StatusCode >= 500 {
				return "火山方舟服务暂时异常，请稍后重试"
			}
		}
	}
	return "无法完成请求，请检查网络、接口地址和模型能力"
}
