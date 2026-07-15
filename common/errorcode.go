package common

import "fmt"

// 错误码区间约定（见 api-conventions.md §2.4）：
//   0           成功
//   40001-40099 参数校验错误
//   40101-40199 认证错误（未登录/Token过期）
//   40301-40399 权限错误（无权操作）
//   40401-40499 资源不存在
//   40901-40999 业务冲突（状态不允许/重复操作）
//   50001-50099 系统内部错误
//   50201-50299 第三方服务错误（支付/物流）

// 预定义业务错误
var (
	// 系统错误
	ErrSystem         = &BizError{Code: 50001, Msg: "服务器内部错误"}
	ErrNotImplemented = &BizError{Code: 50002, Msg: "接口未实现"}

	// 参数校验错误 40001-40099
	ErrParam        = &BizError{Code: 40001, Msg: "参数错误"}
	ErrParamMissing = &BizError{Code: 40002, Msg: "缺少必填参数"}
	ErrParamInvalid = &BizError{Code: 40003, Msg: "参数格式不正确"}

	// 认证错误 40101-40199
	ErrUnauthorized   = &BizError{Code: 40101, Msg: "未登录或登录已过期"}
	ErrTokenInvalid   = &BizError{Code: 40102, Msg: "token 无效"}
	ErrTokenExpired   = &BizError{Code: 40103, Msg: "token 已过期"}
	ErrPwdWrong       = &BizError{Code: 40104, Msg: "手机号或密码错误"}
	ErrRefreshExpired = &BizError{Code: 40105, Msg: "refresh token 已过期，请重新登录"}

	// 权限错误 40301-40399
	ErrForbidden       = &BizError{Code: 40301, Msg: "无权限访问"}
	ErrTooManyRequest  = &BizError{Code: 40302, Msg: "请求过于频繁"}
	ErrUserDisabled    = &BizError{Code: 40303, Msg: "用户已被禁用"}
	ErrRoleForbidden   = &BizError{Code: 40304, Msg: "角色权限不足"}
	ErrTempleIsolation = &BizError{Code: 40305, Msg: "无权操作其他寺院数据"}
	ErrMasterIsolation = &BizError{Code: 40306, Msg: "无权操作其他法师数据"}

	// 资源不存在 40401-40499
	ErrUserNotFound          = &BizError{Code: 40401, Msg: "用户不存在"}
	ErrTempleNotFound        = &BizError{Code: 40402, Msg: "寺院不存在"}
	ErrMasterNotFound        = &BizError{Code: 40403, Msg: "法师不存在"}
	ErrBookingNotFound       = &BizError{Code: 40404, Msg: "预约不存在"}
	ErrProductNotFound       = &BizError{Code: 40405, Msg: "商品不存在"}
	ErrDiyOrderNotFound      = &BizError{Code: 40406, Msg: "DIY订单不存在"}
	ErrOrderNotFound         = &BizError{Code: 40407, Msg: "商城订单不存在"}
	ErrPaymentNotFound       = &BizError{Code: 40408, Msg: "支付单不存在"}
	ErrReviewNotFound        = &BizError{Code: 40409, Msg: "评价不存在"}
	ErrCouponNotFound        = &BizError{Code: 40410, Msg: "优惠券不存在"}
	ErrActivityNotFound      = &BizError{Code: 40411, Msg: "活动不存在"}
	ErrSessionNotFound       = &BizError{Code: 40412, Msg: "AI会话不存在"}
	ErrBlessingNotFound      = &BizError{Code: 40413, Msg: "加持任务不存在"}
	ErrTempleServiceNotFound = &BizError{Code: 40414, Msg: "寺院未提供该服务"}

	// 业务冲突 40901-40999
	ErrUserAlreadyExists    = &BizError{Code: 40901, Msg: "用户已存在"}
	ErrBookingStatusInvalid = &BizError{Code: 40902, Msg: "预约状态流转非法"}
	ErrStatusInvalid        = &BizError{Code: 40903, Msg: "状态流转非法"}
	ErrDuplicateOperation   = &BizError{Code: 40904, Msg: "重复操作"}
	ErrStockInsufficient    = &BizError{Code: 40905, Msg: "库存不足"}
	ErrOrderStatusConflict  = &BizError{Code: 40906, Msg: "订单状态不允许此操作"}

	// 第三方服务错误 50201-50299
	ErrPaymentService   = &BizError{Code: 50201, Msg: "支付服务异常"}
	ErrLogisticsService = &BizError{Code: 50202, Msg: "物流服务异常"}
	ErrOssService       = &BizError{Code: 50203, Msg: "对象存储服务异常"}
	ErrAiService        = &BizError{Code: 50204, Msg: "AI服务异常"}
)

// BizError 业务错误类型，实现 error 接口
type BizError struct {
	Code int
	Msg  string
}

func (e *BizError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// NewBizError 构造自定义业务错误
func NewBizError(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg}
}
