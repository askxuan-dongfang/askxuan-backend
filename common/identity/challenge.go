package identity

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"strings"
)

type Challenges struct {
	Redis  *redis.Redis
	Secret string
}

func RandomID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digits(n int) string {
	b := make([]byte, n)
	for i := range b {
		x, e := rand.Int(rand.Reader, big.NewInt(10))
		if e != nil {
			panic(e)
		}
		b[i] = '0' + byte(x.Int64())
	}
	return string(b)
}
func (s *Challenges) digest(value string) string {
	m := hmac.New(sha256.New, []byte(s.Secret))
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}
func (s *Challenges) Key(kind, value string) string {
	return "identity:" + kind + ":" + s.digest(value)
}
func (s *Challenges) Limit(ctx context.Context, kind, value string, max, ttl int) error {
	v, e := s.Redis.EvalCtx(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],ARGV[1]) end; return n`, []string{s.Key("limit:"+kind, value)}, ttl)
	if e != nil {
		return fmt.Errorf("验证服务暂不可用")
	}
	if v.(int64) > int64(max) {
		return fmt.Errorf("操作频繁，请稍后重试")
	}
	return nil
}
func (s *Challenges) Save(ctx context.Context, kind, id, code string, ttl int) error {
	_, err := s.Redis.EvalCtx(ctx, `redis.call('SET',KEYS[1],ARGV[1],'EX',ARGV[2]);redis.call('DEL',KEYS[2]);return 1`, []string{s.Key(kind, id), s.Key(kind+":attempt", id)}, s.digest(id+":"+code), ttl)
	return err
}
func (s *Challenges) Consume(ctx context.Context, kind, id, code string, max int) error {
	if len(id) > 320 || len(code) > 16 {
		return fmt.Errorf("验证码无效或已过期")
	}
	v, e := s.Redis.EvalCtx(ctx, `local expected=redis.call('GET',KEYS[1]); if not expected then return 0 end; local n=redis.call('INCR',KEYS[2]);if n==1 then redis.call('EXPIRE',KEYS[2],600) end;if n>tonumber(ARGV[2]) then redis.call('DEL',KEYS[1]);return 0 end;if expected==ARGV[1] then redis.call('DEL',KEYS[1],KEYS[2]);return 1 end;return 0`, []string{s.Key(kind, id), s.Key(kind+":attempt", id)}, s.digest(id+":"+strings.TrimSpace(code)), max)
	if e != nil {
		return fmt.Errorf("验证服务暂不可用")
	}
	if v.(int64) != 1 {
		return fmt.Errorf("验证码无效或已过期")
	}
	return nil
}

type Captcha struct {
	ID        string `json:"id"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expiresIn"`
}

var glyphs = []string{"111101101101111", "010110010010111", "111001111100111", "111001111001111", "101101111001001", "111100111001111", "111100111101111", "111001010010010", "111101111101111", "111101111001111"}

func (s *Challenges) Captcha(ctx context.Context) (*Captcha, error) {
	id, code := RandomID(), digits(5)
	if e := s.Save(ctx, "captcha", id, code, 180); e != nil {
		return nil, e
	}
	img := image.NewRGBA(image.Rect(0, 0, 180, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 180; x++ {
			img.Set(x, y, color.RGBA{245, 243, 234, 255})
		}
	}
	for i, c := range code {
		g := glyphs[c-'0']
		for k, v := range g {
			if v != '1' {
				continue
			}
			for dy := 0; dy < 6; dy++ {
				for dx := 0; dx < 6; dx++ {
					img.Set(12+i*33+(k%3)*6+dx, 13+(k/3)*6+dy, color.RGBA{30, 77, 65, 255})
				}
			}
		}
	}
	for i := 0; i < 180; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(180*60))
		img.Set(int(n.Int64())%180, int(n.Int64())/180, color.RGBA{153, 126, 78, 180})
	}
	var buf bytes.Buffer
	if e := png.Encode(&buf, img); e != nil {
		return nil, e
	}
	return &Captcha{ID: id, Image: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), ExpiresIn: 180}, nil
}
