package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// AES-GCM encrypts bank numbers at rest. The versioned key uses the persistent
// service secret; rotate it only with an explicit re-encryption migration.
func payoutCipher(secret string) (cipher.AEAD, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("payout encryption key unavailable")
	}
	key := sha256.Sum256([]byte("askxuan-creator-payout-v1:" + secret))
	b, e := aes.NewCipher(key[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func SealPayout(secret, user, value string) (string, error) {
	c, e := payoutCipher(secret)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, c.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return "v1:" + base64.RawStdEncoding.EncodeToString(c.Seal(nonce, nonce, []byte(value), []byte(user))), nil
}
func OpenPayout(secret, user, value string) (string, error) {
	if !strings.HasPrefix(value, "v1:") {
		return "", fmt.Errorf("unrecognized encrypted payout account")
	}
	c, e := payoutCipher(secret)
	if e != nil {
		return "", e
	}
	raw, e := base64.RawStdEncoding.DecodeString(value[3:])
	if e != nil || len(raw) < c.NonceSize() {
		return "", fmt.Errorf("invalid payout account")
	}
	out, e := c.Open(nil, raw[:c.NonceSize()], raw[c.NonceSize():], []byte(user))
	return string(out), e
}
