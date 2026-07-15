package logic

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
)

func authenticatedUserID(ctx context.Context) (string, error) {
	uid := middleware.UserIDFromCtx(ctx)
	if uid <= 0 {
		return "", common.ErrUnauthorized
	}
	return strconv.FormatInt(uid, 10), nil
}
