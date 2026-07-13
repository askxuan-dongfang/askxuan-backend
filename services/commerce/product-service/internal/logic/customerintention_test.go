package logic

import (
	"context"
	"testing"

	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"
)

type fakeIntentionModel struct {
	code string
	page int
	size int
}

func (f *fakeIntentionModel) FindTags(context.Context) ([]*model.IntentTag, error) {
	return []*model.IntentTag{{Code: "peace", Name: "求平安", Sort: 10}}, nil
}
func (f *fakeIntentionModel) FindResources(_ context.Context, code string, page, size int) ([]*model.IntentionResource, int64, error) {
	f.code, f.page, f.size = code, page, size
	return []*model.IntentionResource{{ResourceType: "service", SourceId: "1", Title: "灵隐寺 · 祈福", Price: 200, OrderTarget: "service:T001:S001", TempleCode: "T001", ServiceCode: "S001"}}, 1, nil
}
func (*fakeIntentionModel) FindProductTags(context.Context, int64) ([]string, error)  { return nil, nil }
func (*fakeIntentionModel) ReplaceProductTags(context.Context, int64, []string) error { return nil }

func TestCustomerIntentionList(t *testing.T) {
	fake := &fakeIntentionModel{}
	logic := NewCustomerIntentionLogic(context.Background(), &svc.ServiceContext{IntentionModel: fake})
	resp, err := logic.List(&types.CustomerIntentionReq{Code: "peace", Page: 0, Size: 999})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if fake.code != "peace" || fake.page != 1 || fake.size != 20 {
		t.Fatalf("unexpected normalized request: %#v", fake)
	}
	if resp.Total != 1 || len(resp.List) != 1 || resp.List[0].OrderTarget != "service:T001:S001" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestCustomerIntentionRejectsUnknownCode(t *testing.T) {
	logic := NewCustomerIntentionLogic(context.Background(), &svc.ServiceContext{IntentionModel: &fakeIntentionModel{}})
	if _, err := logic.List(&types.CustomerIntentionReq{Code: "unknown", Page: 1, Size: 20}); err == nil {
		t.Fatal("unknown code accepted")
	}
}
