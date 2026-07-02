package common

import (
	"testing"
)

// ===== JWT 测试 =====

func TestGenAccessToken(t *testing.T) {
	secret := "test-secret-123"
	info := TokenInfo{
		UserId:   1001,
		Mobile:   "13800138000",
		UserType: "user",
		Roles:    []string{"customer"},
		ClientID: "customer",
	}

	token, err := GenAccessToken(secret, info, 7200) // 2h
	if err != nil {
		t.Fatalf("GenAccessToken 失败: %v", err)
	}
	if token == "" {
		t.Fatal("token 不应为空")
	}
}

func TestGenRefreshToken(t *testing.T) {
	secret := "test-secret-123"
	token, err := GenRefreshToken(secret, 1001, 604800) // 7d
	if err != nil {
		t.Fatalf("GenRefreshToken 失败: %v", err)
	}
	if token == "" {
		t.Fatal("token 不应为空")
	}
}

func TestParseToken_Success(t *testing.T) {
	secret := "test-secret-123"
	info := TokenInfo{
		UserId:   1001,
		Mobile:   "13800138000",
		UserType: "admin",
		Roles:    []string{"platform_super"},
		ClientID: "platform-admin",
	}

	token, _ := GenAccessToken(secret, info, 7200)
	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}
	if claims.UserId != 1001 {
		t.Errorf("UserId 期望 1001, 实际 %d", claims.UserId)
	}
	if claims.Mobile != "13800138000" {
		t.Errorf("Mobile 期望 13800138000, 实际 %s", claims.Mobile)
	}
	if claims.UserType != "admin" {
		t.Errorf("UserType 期望 admin, 实际 %s", claims.UserType)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "platform_super" {
		t.Errorf("Roles 期望 [platform_super], 实际 %v", claims.Roles)
	}
	if claims.Type != "access" {
		t.Errorf("Type 期望 access, 实际 %s", claims.Type)
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, _ := GenAccessToken("secret-a", TokenInfo{UserId: 1}, 7200)
	_, err := ParseToken("secret-b", token)
	if err == nil {
		t.Fatal("使用错误密钥应返回错误")
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	_, err := ParseToken("secret", "invalid.token.string")
	if err == nil {
		t.Fatal("无效 token 应返回错误")
	}
}

func TestIsRefreshToken(t *testing.T) {
	secret := "test-secret"
	rt, _ := GenRefreshToken(secret, 1, 3600)
	claims, _ := ParseToken(secret, rt)
	if !claims.IsRefreshToken() {
		t.Error("refresh token 的 IsRefreshToken 应为 true")
	}

	at, _ := GenAccessToken(secret, TokenInfo{UserId: 1}, 3600)
	claims2, _ := ParseToken(secret, at)
	if claims2.IsRefreshToken() {
		t.Error("access token 的 IsRefreshToken 应为 false")
	}
}

func TestHasRole(t *testing.T) {
	claims := &CustomClaims{Roles: []string{"temple_admin", "shop_admin"}}
	if !claims.HasRole("temple_admin") {
		t.Error("应拥有 temple_admin 角色")
	}
	if !claims.HasRole("shop_admin") {
		t.Error("应拥有 shop_admin 角色")
	}
	if claims.HasRole("customer") {
		t.Error("不应拥有 customer 角色")
	}
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name   string
		roles  []string
		expect bool
	}{
		{"平台超管", []string{"platform_super"}, true},
		{"平台运营", []string{"platform_service"}, true},
		{"寺院管理员", []string{"temple_admin"}, true},
		{"商城管理员", []string{"shop_admin"}, true},
		{"C端用户", []string{"customer"}, false},
		{"法师", []string{"master"}, false},
		{"无角色", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &CustomClaims{Roles: tt.roles}
			if claims.IsAdmin() != tt.expect {
				t.Errorf("IsAdmin() 期望 %v, 实际 %v (roles=%v)", tt.expect, !tt.expect, tt.roles)
			}
		})
	}
}

// ===== 错误码测试 =====

func TestBizError_Error(t *testing.T) {
	err := ErrParam
	expected := "[40001] 参数错误"
	if err.Error() != expected {
		t.Errorf("Error() 期望 %q, 实际 %q", expected, err.Error())
	}
}

func TestNewBizError(t *testing.T) {
	err := NewBizError(99999, "自定义错误")
	if err.Code != 99999 {
		t.Errorf("Code 期望 99999, 实际 %d", err.Code)
	}
	if err.Msg != "自定义错误" {
		t.Errorf("Msg 期望 '自定义错误', 实际 %q", err.Msg)
	}
}

func TestErrorCodeRanges(t *testing.T) {
	// 验证错误码区间约定
	tests := []struct {
		name string
		err  *BizError
		min  int
		max  int
	}{
		{"参数错误区间", ErrParam, 40001, 40099},
		{"认证错误区间", ErrUnauthorized, 40101, 40199},
		{"权限错误区间", ErrForbidden, 40301, 40399},
		{"资源不存在区间", ErrUserNotFound, 40401, 40499},
		{"业务冲突区间", ErrStatusInvalid, 40901, 40999},
		{"系统错误区间", ErrSystem, 50001, 50099},
		{"第三方服务区间", ErrPaymentService, 50201, 50299},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code < tt.min || tt.err.Code > tt.max {
				t.Errorf("错误码 %d 不在区间 [%d, %d]", tt.err.Code, tt.min, tt.max)
			}
		})
	}
}
