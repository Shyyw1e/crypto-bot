package usecase

import (
	"strconv"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/adapters/abcexws"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type Orderbook struct {
	Asks      []domain.Order `json:"asks"`
	Bids      []domain.Order `json:"bids"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func Mapper(msg *abcexws.OrderbookMessage, log logger.Logger) *Orderbook {
	if msg == nil {
		log.Error("abcex_mapper_nil_message")
		return nil
	}

	const depth = 5

	asksDepth := len(msg.Data.Asks)
	if asksDepth > depth {
		asksDepth = depth
	}
	bidsDepth := len(msg.Data.Bids)
	if bidsDepth > depth {
		bidsDepth = depth
	}

	asks := make([]domain.Order, 0, asksDepth)
	bids := make([]domain.Order, 0, bidsDepth)

	var askCum, bidCum float64

	for i := 0; i < asksDepth; i++ {
		price, volume, ok := parseLevel(msg.Data.Asks[i])
		if !ok {
			log.Warn("abcex_mapper_skip_bad_ask", "level", i, "value", msg.Data.Asks[i])
			continue
		}
		askCum += volume
		asks = append(asks, domain.Order{
			Price:    price,
			Amount:   volume,
			Notional: askCum,
			Side:     domain.SideAsk,
			Source:   domain.SourceABCEX,
			Pair:     domain.USDTRUB,
		})
	}

	for i := 0; i < bidsDepth; i++ {
		price, volume, ok := parseLevel(msg.Data.Bids[i])
		if !ok {
			log.Warn("abcex_mapper_skip_bad_bid", "level", i, "value", msg.Data.Bids[i])
			continue
		}
		bidCum += volume
		bids = append(bids, domain.Order{
			Price:    price,
			Amount:   volume,
			Notional: bidCum,
			Side:     domain.SideBid,
			Source:   domain.SourceABCEX,
			Pair:     domain.USDTRUB,
		})
	}

	if len(asks) == 0 || len(bids) == 0 {
		log.Warn("abcex_mapper_empty_book", "asks", len(asks), "bids", len(bids))
		return nil
	}

	return &Orderbook{
		Asks:      asks,
		Bids:      bids,
		UpdatedAt: time.Now(),
	}
}

func parseLevel(level []string) (price float64, volume float64, ok bool) {
	if len(level) < 2 {
		return 0, 0, false
	}
	price, err := strconv.ParseFloat(level[0], 64)
	if err != nil {
		return 0, 0, false
	}
	volume, err = strconv.ParseFloat(level[1], 64)
	if err != nil {
		return 0, 0, false
	}
	return price, volume, price > 0 && volume > 0
}
