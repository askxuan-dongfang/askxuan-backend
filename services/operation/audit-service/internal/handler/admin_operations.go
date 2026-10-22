package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/zeromicro/go-zero/rest"
)

const operationListPath = "/api/v1/admin/audit/operations"

func registerAdminOperations(server *rest.Server, ctx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/internal/admin-operations/start", Handler: operationStartHandler(ctx)},
		{Method: http.MethodPost, Path: "/api/v1/internal/admin-operations/finish", Handler: operationFinishHandler(ctx)},
		{Method: http.MethodGet, Path: operationListPath, Handler: operationListHandler(ctx)},
	})
}

func operationClaims(r *http.Request, secret string) (*common.CustomClaims, error) {
	if common.ValidateJWTSecret(secret) != nil {
		return nil, common.ErrUnauthorized
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, common.ErrUnauthorized
	}
	claims, err := common.ParseToken(secret, parts[1])
	if err != nil || claims.Type != "access" || claims.ExpiresAt == nil {
		return nil, common.ErrUnauthorized
	}
	return claims, nil
}

func operationInternalAuth(r *http.Request, secret string) error {
	claims, err := operationClaims(r, secret)
	if err != nil {
		return err
	}
	if claims.UserType != "service" || claims.UserId != 0 || claims.ClientID != "gateway-service" || claims.Issuer != "gateway-service" || !claims.HasRole("platform_service") {
		return common.ErrForbidden
	}
	return nil
}

func operationReadAuth(r *http.Request, secret string) error {
	claims, err := operationClaims(r, secret)
	if err != nil {
		return err
	}
	if claims.UserType != "admin" || claims.UserId <= 0 || claims.HasRole("platform_service") {
		return common.ErrForbidden
	}
	if claims.HasRole("platform_super") || (claims.ClientID == "platform-admin" && claims.AuthzScope == "platform" && adminpolicy.Allowed(http.MethodGet, operationListPath, claims.Permissions)) {
		return nil
	}
	return common.ErrForbidden
}

func decodeOperation(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" || r.URL.RawQuery != "" {
		return common.ErrParam
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return common.ErrParam
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return common.ErrParam
	}
	return nil
}

func operationStartHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := operationInternalAuth(r, ctx.Config.Auth.AccessSecret); err != nil {
			common.JsonError(w, err)
			return
		}
		var req model.OperationStart
		if err := decodeOperation(w, r, &req); err != nil {
			common.JsonError(w, err)
			return
		}
		if err := req.Validate(); err != nil {
			common.JsonError(w, err)
			return
		}
		if ctx.Operations == nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		receipt, err := ctx.Operations.Start(r.Context(), req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, receipt)
	}
}

func operationFinishHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := operationInternalAuth(r, ctx.Config.Auth.AccessSecret); err != nil {
			common.JsonError(w, err)
			return
		}
		var req model.OperationFinish
		if err := decodeOperation(w, r, &req); err != nil {
			common.JsonError(w, err)
			return
		}
		if err := req.Validate(); err != nil {
			common.JsonError(w, err)
			return
		}
		if ctx.Operations == nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		receipt, err := ctx.Operations.Finish(r.Context(), req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, receipt)
	}
}

func operationListFilter(r *http.Request) (model.OperationFilter, error) {
	f := model.OperationFilter{Page: 1, Size: 20}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return f, common.ErrParam
	}
	allowed := map[string]bool{"actorId": true, "action": true, "method": true, "targetType": true, "result": true, "startTime": true, "endTime": true, "page": true, "size": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return f, common.ErrParam
		}
	}
	if v := query.Get("actorId"); v != "" {
		f.ActorID, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return f, common.ErrParam
		}
	}
	if v := query.Get("page"); v != "" {
		f.Page, err = strconv.Atoi(v)
		if err != nil {
			return f, common.ErrParam
		}
	}
	if v := query.Get("size"); v != "" {
		f.Size, err = strconv.Atoi(v)
		if err != nil {
			return f, common.ErrParam
		}
	}
	if v := query.Get("startTime"); v != "" {
		f.StartTime, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return f, common.ErrParam
		}
	}
	if v := query.Get("endTime"); v != "" {
		f.EndTime, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return f, common.ErrParam
		}
	}
	f.Action, f.Method, f.TargetType, f.Result = query.Get("action"), query.Get("method"), query.Get("targetType"), query.Get("result")
	return f, f.Validate()
}

func operationListHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := operationReadAuth(r, ctx.Config.Auth.AccessSecret); err != nil {
			common.JsonError(w, err)
			return
		}
		filter, err := operationListFilter(r)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		if ctx.Operations == nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		list, err := ctx.Operations.List(r.Context(), filter)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, list)
	}
}
