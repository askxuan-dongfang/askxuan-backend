package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/model"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type authKey struct{}

func WithAuthorization(ctx context.Context, auth string) context.Context {
	return context.WithValue(ctx, authKey{}, auth)
}
func upstream(ctx context.Context, method, path string, body any, out any) error {
	base := os.Getenv("ASKXUAN_GATEWAY_URL")
	if base == "" {
		base = "http://gateway-service:8080"
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	auth, _ := ctx.Value(authKey{}).(string)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return common.ErrSystem
	}
	defer res.Body.Close()
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&envelope); err != nil {
		return common.ErrSystem
	}
	if res.StatusCode != 200 || envelope.Code != 0 {
		return common.ErrForbidden
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}
func masterCode(ctx context.Context) (string, error) {
	if middleware.MasterIDFromCtx(ctx) == 0 {
		return "", common.ErrForbidden
	}
	var profile struct {
		ID string `json:"id"`
	}
	if err := upstream(ctx, "GET", "/admin/masters/profile", nil, &profile); err != nil {
		return "", err
	}
	if profile.ID == "" {
		return "", common.ErrForbidden
	}
	return profile.ID, nil
}
func providerScope(ctx context.Context) (context.Context, string, error) {
	if middleware.MasterIDFromCtx(ctx) > 0 {
		code, err := masterCode(ctx)
		return ctx, code, err
	}
	code := middleware.TempleCodeFromCtx(ctx)
	if code == "" {
		return ctx, "", common.ErrForbidden
	}
	return model.WithTemple(ctx, code), "", nil
}
func ownsReview(ctx context.Context, r model.Review) error {
	scoped, master, err := providerScope(ctx)
	if err != nil {
		return err
	}
	rows, _, err := model.ListReviews(scoped, r.TargetType, r.TargetId, "", 0, "", master, 1, 1)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].Id != r.Id {
		return common.ErrForbidden
	}
	return nil
}
func callerID(ctx context.Context) string {
	return strconv.FormatInt(middleware.UserIDFromCtx(ctx), 10)
}
