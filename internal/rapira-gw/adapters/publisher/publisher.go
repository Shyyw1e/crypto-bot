package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"github.com/Shyyw1e/crypto-bot/internal/shared/redis"
)

type rawOrderbook struct {
	Bids      []domain.Order `json:"bids"`
	Asks      []domain.Order `json:"asks"`
	UpdatedAt string         `json:"updated_at"`
}

var ErrEmptyOrderbook = errors.New("empty orderbook")

func buildRawOrderbook(ob *usecase.Orderbook) (*rawOrderbook, error) {
	if ob == nil {
		return nil, ErrEmptyOrderbook
	}
	return &rawOrderbook{
		Bids:      ob.Bids,
		Asks:      ob.Asks,
		UpdatedAt: ob.UpdatedAt.Format(time.RFC3339),
	}, nil
}

type redisOrderbookPublisher struct {
	rdb *redis.Client
	log logger.Logger
	ttl time.Duration
}

func NewRedisOrderbookPublisher(
	rdb *redis.Client,
	log logger.Logger,
	ttl time.Duration,
) usecase.OrderbookPublisher {
	return &redisOrderbookPublisher{rdb: rdb, log: log, ttl: ttl}
}

func (p *redisOrderbookPublisher) Publish(
	ctx context.Context,
	src domain.Source,
	pair domain.Pair,
	ob *usecase.Orderbook,
) error {
	if ob == nil {
		p.log.Error("redis_publish_empty_ob", "source", src, "pair", pair)
		return ErrEmptyOrderbook
	}

	key := fmt.Sprintf("orderbook:%s:%s", src, pair)

	rawOB, err := buildRawOrderbook(ob)
	if err != nil {
		p.log.Error("redis_publish_build_raw_ob_failed", "source", src, "pair", pair, "err", err)
		return fmt.Errorf("build raw orderbook: %w", err)
	}

	data, err := json.Marshal(rawOB)
	if err != nil {
		p.log.Error("redis_orderbook_marshal_failed", "source", src, "pair", pair, "err", err)
		return fmt.Errorf("marshal orderbook: %w", err)
	}

	if err := p.rdb.Set(ctx, key, data, p.ttl).Err(); err != nil {
		p.log.Error("redis_orderbook_set_failed", "key", key, "err", err)
		return fmt.Errorf("set orderbook in redis: %w", err)
	}

	p.log.Debug(
		"redis_orderbook_published",
		"key", key,
		"source", src,
		"pair", pair,
		"bids", len(ob.Bids),
		"asks", len(ob.Asks),
	)

	return nil
}
