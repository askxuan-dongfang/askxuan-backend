package common

import (
	"errors"
	"strings"
)

const placeholderJWTSecret = "askxuan-access-secret-change-in-prod"

// ValidateJWTSecret rejects configuration values that make access and service tokens forgeable.
func ValidateJWTSecret(secret string) error {
	if len(secret) < 32 || strings.TrimSpace(secret) != secret || secret == placeholderJWTSecret {
		return errors.New("JWT signing secret must be a non-placeholder value of at least 32 bytes")
	}
	return nil
}
