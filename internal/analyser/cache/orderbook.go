package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"

	rds "github.com/redis/go-redis/v9"
)

type Orderbook struct {
	Bids      []domain.Order
	Asks      []domain.Order
	UpdatedAt time.Time
}

type redisOrderbookCache struct {
	rdb *redis.Client
	log logger.Logger
}

type rawOrderbook struct {
	Bids      []domain.Order `json:"bids"`
	Asks      []domain.Order `json:"asks"`
	UpdatedAt string         `json:"updated_at"`
}

type OrderbookCache interface {
	Get(ctx context.Context, source domain.Source, pair domain.Pair) (*Orderbook, error)
}

var ErrNotFound = errors.New("orderbook not found")

func NewOrderbookCache(rdb *redis.Client, log logger.Logger) OrderbookCache {
	return &redisOrderbookCache{
		rdb: rdb,
		log: log,
	}
}

func (c *redisOrderbookCache) Get(
	ctx context.Context,
	source domain.Source,
	pair domain.Pair,
) (*Orderbook, error) {
	key := fmt.Sprintf("orderbook:%s:%s", source, pair)

	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == rds.Nil {
			return nil, ErrNotFound
		}
		c.log.Error("redis_orderbook_cache_get_failed", "key", key, "err", err)
		return nil, fmt.Errorf("get orderbook from redis: %w", err)
	}

	var raw rawOrderbook
	if err := json.Unmarshal([]byte(val), &raw); err != nil {
		c.log.Error("redis_orderbook_cache_unmarshal_failed", "key", key, "err", err)
		return nil, fmt.Errorf("unmarshal orderbook: %w", err)
	}

	updAt, err := time.Parse(time.RFC3339, raw.UpdatedAt)
	if err != nil {
		c.log.Error("redis_orderbook_cache_parse_time_failed", "key", key, "value", raw.UpdatedAt, "err", err)
		return nil, fmt.Errorf("parse orderbook updated_at: %w", err)
	}

	return &Orderbook{
		Bids:      raw.Bids,
		Asks:      raw.Asks,
		UpdatedAt: updAt,
	}, nil
}
