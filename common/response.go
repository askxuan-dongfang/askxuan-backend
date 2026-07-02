// Package common 提供东方玄学后端各微服务共享的公共能力
// 包括：统一响应体、错误码、JWT 工具、CORS / 鉴权中间件
package common

import (
	"errors"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// Body 统一响应体，所有接口返回 {code,message,data} 结构
// code=0 表示成功，非 0 表示业务错误
type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Ok 成功响应，data 可为任意结构或 nil
func Ok(w http.ResponseWriter, data interface{}) {
	httpx.OkJson(w, &Body{Code: 0, Message: "success", Data: data})
}

// OkWithMsg 成功响应，自定义 message
func OkWithMsg(w http.ResponseWriter, data interface{}, msg string) {
	httpx.OkJson(w, &Body{Code: 0, Message: msg, Data: data})
}

// JsonError 统一错误响应
// 若 err 实现了 BizError 接口，则返回其业务错误码与消息；否则统一返回 5000
func JsonError(w http.ResponseWriter, err error) {
	var be *BizError
	if errors.As(err, &be) {
		httpx.WriteJson(w, http.StatusOK, &Body{Code: be.Code, Message: be.Msg})
		return
	}
	httpx.WriteJson(w, http.StatusOK, &Body{Code: ErrSystem.Code, Message: ErrSystem.Msg})
}
