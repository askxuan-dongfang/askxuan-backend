package logic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
)

type chatHistory struct {
	model.ConversationModel
}

func (chatHistory) ListAllMessages(context.Context, int64) ([]*model.AIMessage, error) {
	return []*model.AIMessage{{Role: model.RoleUser, Status: model.MessageStatusCompleted, AttachmentsJSON: `[{}]`}}, nil
}

func TestChatModelCustomerPoolAndPinnedSelection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-flash"},{"id":"deepseek-v4-pro"},{"id":"second-vision"}]}`))
	}))
	defer upstream.Close()
	p := provider.NewOpenAICompatible(upstream.URL, "fixture-key", "deepseek-flash", "second-vision")
	s := &svc.ServiceContext{Provider: p, Models: provider.NewCatalog(p), ConversationModel: chatHistory{}}
	for _, test := range []struct {
		name, chosen, pinned, want string
		ids                        []string
		session                    int64
		images                     bool
	}{
		{name: "explicit customer choice", chosen: "deepseek-v4-pro", ids: []string{"deepseek-v4-pro"}, want: "deepseek-v4-pro"},
		{name: "explicit hidden model", chosen: "deepseek-flash", ids: []string{"deepseek-v4-pro"}},
		{name: "omitted customer choice", ids: []string{"deepseek-v4-pro"}, want: "deepseek-v4-pro"},
		{name: "customer image default", images: true, ids: []string{"deepseek-flash", "second-vision"}, want: "second-vision"},
		{name: "open alternate image model", images: true, ids: []string{"deepseek-flash"}, want: "deepseek-flash"},
		{name: "images in completed history", session: 1, chosen: "deepseek-v4-pro", ids: []string{"deepseek-v4-pro"}},
		{name: "pinned image retry", session: 1, chosen: "deepseek-flash", want: "deepseek-flash"},
		{name: "pinned internal default", pinned: "deepseek-flash", ids: []string{"deepseek-v4-pro"}, want: "deepseek-flash"},
		{name: "all configured models unavailable", ids: []string{"unknown"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s.AgentDefaultModel = test.pinned
			got, err := selectChatModel(context.Background(), s, test.session, test.chosen, test.images, test.ids)
			if got != test.want || (err != nil) != (test.want == "") {
				t.Fatal(got, err)
			}
		})
	}
}
