package common

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims JWT 自定义声明，携带用户身份信息
type CustomClaims struct {
	UserId   int64    `json:"userId"`
	Mobile   string   `json:"mobile,omitempty"`
	UserType string   `json:"userType,omitempty"`          // user / master / admin
	Roles    []string `json:"roles,omitempty"`             // 角色列表：customer/temple_admin/master/shop_admin/platform_super/platform_service
	ClientID string   `json:"clientId,omitempty"`          // 端标识：customer/temple-admin/master/shop-admin/platform-admin
	TempleID int64    `json:"templeId,omitempty"`          // 寺院ID（temple_admin 专用）
	MasterID int64    `json:"masterId,omitempty"`          // 法师ID（master 专用）
	Type     string   `json:"type,omitempty"`              // access / refresh
	jwt.RegisteredClaims
}

// TokenInfo 签发 Access Token 所需的用户信息
type TokenInfo struct {
	UserId   int64
	Mobile   string
	UserType string
	Roles    []string
	ClientID string
	TempleID int64
	MasterID int64
}

// JWT 签发与校验工具
// - Access Token：2h，携带完整用户信息，用于接口鉴权
// - Refresh Token：7d，仅携带 userId，用于续期

// GenAccessToken 签发 Access Token
// secret: 签名密钥；expireSeconds: 有效期（秒）
func GenAccessToken(secret string, info TokenInfo, expireSeconds int64) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserId:   info.UserId,
		Mobile:   info.Mobile,
		UserType: info.UserType,
		Roles:    info.Roles,
		ClientID: info.ClientID,
		TempleID: info.TempleID,
		MasterID: info.MasterID,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireSeconds) * time.Second)),
			Subject:   "access",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenRefreshToken 签发 Refresh Token（仅含 userId）
func GenRefreshToken(secret string, userId int64, expireSeconds int64) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserId: userId,
		Type:   "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireSeconds) * time.Second)),
			Subject:   "refresh",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// SignServiceToken 签发内部服务调用 token
// secret: 签名密钥；serviceName: 调用方服务名（如 order-service）
// 该 token 为 access 类型，UserType=service，用于服务间调用绕过管理台角色校验
// 默认有效期 1 小时
func SignServiceToken(secret, serviceName string) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserId:   0,
		UserType: "service",
		Roles:    []string{"platform_service"},
		ClientID: serviceName,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			Subject:   "service",
			Issuer:    serviceName,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken 解析并校验 token，返回自定义声明
func ParseToken(secret, tokenStr string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token 无效")
	}
	return claims, nil
}

// IsRefreshToken 判断是否为 refresh token
func (c *CustomClaims) IsRefreshToken() bool {
	return c.Type == "refresh"
}

// HasRole 判断用户是否拥有指定角色
func (c *CustomClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin 判断用户是否为平台/寺院/商城管理角色，不包含法师工作台角色
func (c *CustomClaims) IsAdmin() bool {
	return c.HasRole("temple_admin") || c.HasRole("shop_admin") ||
		c.HasRole("platform_super") || c.HasRole("platform_service")
}
