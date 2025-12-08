package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

type NotificationDedup interface {
	SeenOrMark(ctx context.Context, chatID int64, opHash string, ttl time.Duration) (seen bool, err error)
}

type redisNotificationDedup struct {
	rdb *redis.Client
	log logger.Logger
}

func NewNotificationDedup(rdb *redis.Client, log logger.Logger) NotificationDedup {
	return &redisNotificationDedup{
		rdb: rdb,
		log: log,
	}
}

func (d *redisNotificationDedup) SeenOrMark(
	ctx context.Context,
	chatID int64,
	opHash string,
	ttl time.Duration,
) (bool, error) {
	key := fmt.Sprintf("user:%d:seen_ops", chatID)

	isMember, err := d.rdb.SIsMember(ctx, key, opHash).Result()
	if err != nil {
		d.log.Error("redis_dedup_sismember_failed", "key", key, "err", err)
		return false, fmt.Errorf("check seen op in redis: %w", err)
	}
	if isMember {
		return true, nil
	}

	if err := d.rdb.SAdd(ctx, key, opHash).Err(); err != nil {
		d.log.Error("redis_dedup_sadd_failed", "key", key, "err", err)
		return false, fmt.Errorf("mark op as seen in redis: %w", err)
	}

	if ttl > 0 {
		if err := d.rdb.Expire(ctx, key, ttl).Err(); err != nil {
			d.log.Warn("redis_dedup_expire_failed", "key", key, "ttl", ttl, "err", err)
		}
	}

	return false, nil
}
