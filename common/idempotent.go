package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// idempotentTTL 幂等标记存活时间（24 小时）
const idempotentTTL = 86400

// IdempotentKey 构建 MQ 消息幂等性 Redis key
// 格式：mq:processed:{exchange}:{messageId}
func IdempotentKey(exchange, messageId string) string {
	return fmt.Sprintf("mq:processed:%s:%s", exchange, messageId)
}

// ResolveMessageId 解析消息 ID：优先用 amqp.Delivery.MessageId，为空则用 body 的 SHA256 hash
func ResolveMessageId(messageId string, body []byte) string {
	if messageId != "" {
		return messageId
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// CheckMessageProcessed 检查并标记消息已处理（SETNX）。
//   - alreadyProcessed=true: 消息已处理过（重复消息），调用方应直接 ACK 跳过处理
//   - alreadyProcessed=false, err=nil: 消息首次处理，已标记，调用方应执行业务逻辑；若业务失败需调用 RollbackMessageProcessed
//   - err!=nil: Redis 不可用，调用方应 NACK requeue
func CheckMessageProcessed(rds *redis.Redis, exchange, messageId string) (alreadyProcessed bool, err error) {
	key := IdempotentKey(exchange, messageId)
	ok, err := rds.SetnxEx(key, "1", idempotentTTL)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// RollbackMessageProcessed 回滚消息处理标记（业务失败时调用，允许下次重试）
func RollbackMessageProcessed(rds *redis.Redis, exchange, messageId string) {
	key := IdempotentKey(exchange, messageId)
	_, _ = rds.Del(key)
}

// IsMessageProcessed 检查 MQ 消息是否已处理（SETNX 标记，TTL 24 小时）
// 已存在说明已处理，返回 true
func IsMessageProcessed(rds *redis.Redis, exchange, messageId string) bool {
	key := IdempotentKey(exchange, messageId)
	ok, _ := rds.SetnxEx(key, "1", idempotentTTL)
	return !ok
}

// MarkMessageProcessed 显式标记消息已处理（用于手动标记场景）
func MarkMessageProcessed(rds *redis.Redis, exchange, messageId string) error {
	key := IdempotentKey(exchange, messageId)
	return rds.Setex(key, "1", idempotentTTL)
}
