package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/usecase"
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
	nc 	*nats.Conn
	log logger.Logger
	ttl time.Duration
}

func NewRedisOrderbookPublisher(
	rdb *redis.Client,
	log logger.Logger,
	ttl time.Duration,
	nc *nats.Conn,
) usecase.OrderbookPublisher {
	return &redisOrderbookPublisher{
		rdb: rdb,
		nc:  nc,
		log: log,
		ttl: ttl,
	}
}

func (p *redisOrderbookPublisher) Publish(
	ctx context.Context,
	src domain.Source,
	pair domain.Pair,
	ob *usecase.Orderbook,
) error {
	if ob == nil {
		return ErrEmptyOrderbook
	}

	key := fmt.Sprintf("orderbook:%s:%s", src, pair)

	raw, err := buildRawOrderbook(ob)
	if err != nil {
		return err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal orderbook: %w", err)
	}

	if err := p.rdb.Set(ctx, key, data, p.ttl).Err(); err != nil {
		return fmt.Errorf("set orderbook in redis: %w", err)
	}

	if p.nc != nil {
		subj := fmt.Sprintf("orderbook.updated.%s.%s", src, pair)
		if err := p.nc.Publish(subj, nil); err != nil {
			p.log.Warn("grinex_nats_publish_failed", "subject", subj, "err", err)
		}
	}

	p.log.Debug("grinex_orderbook_published",
		"key", key,
		"bids", len(ob.Bids),
		"asks", len(ob.Asks),
	)

	return nil
}