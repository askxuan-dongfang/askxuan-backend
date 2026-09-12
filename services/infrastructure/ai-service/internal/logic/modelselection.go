package logic

import (
	"context"
	"encoding/json"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
)

func selectChatModel(ctx context.Context, s *svc.ServiceContext, sessionID int64, id string, images bool) (string, error) {
	if sessionID > 0 {
		history, err := s.ConversationModel.ListAllMessages(ctx, sessionID)
		if err != nil {
			return "", common.ErrSystem
		}
		completed := make([]*model.AIMessage, 0, len(history))
		for _, m := range history {
			if m.Status == model.MessageStatusCompleted {
				completed = append(completed, m)
			}
		}
		if max := s.AIConfig.MaxHistoryMessages; max > 0 && len(completed) > max {
			completed = completed[len(completed)-max:]
		}
		for _, m := range completed {
			if m.Role != model.RoleUser {
				continue
			}
			var attachments []types.AIImageAttachment
			if json.Unmarshal([]byte(m.AttachmentsJSON), &attachments) == nil && len(attachments) > 0 {
				images = true
			}
		}
	}
	catalog := s.Models
	if catalog == nil {
		catalog = provider.NewCatalog(s.Provider)
	}
	selected, err := catalog.Select(ctx, id, images)
	if err != nil {
		return "", common.NewBizError(40001, err.Error())
	}
	return selected, nil
}
