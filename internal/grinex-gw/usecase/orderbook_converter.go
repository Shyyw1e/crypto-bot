package usecase

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/adapters/grinexapi"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type Orderbook struct {
	Asks 	[]domain.Order 	`json:"asks"`
	Bids	[]domain.Order 	`json:"bids"`
	UpdatedAt time.Time		`json:"updated_at"`
}

func Mapper(resp *grinexapi.DepthResponse, log logger.Logger) *Orderbook {
	if resp == nil {
		log.Error("grinex_mapper_nil_response")
		return nil
	}

	asks := make([]domain.Order, 0, len(resp.Asks))
	bids := make([]domain.Order, 0, len(resp.Bids))

	var askCum, bidCum float64

	for _, lvl := range resp.Asks {
		price, err := strconv.ParseFloat(lvl.Price, 64)
		if err != nil {
			log.Error("grinex_mapper_parse_ask_price_failed", "value", lvl.Price, "err", err)
			return nil
		}
		volume, err := strconv.ParseFloat(lvl.Volume, 64)
		if err != nil {
			log.Error("grinex_mapper_parse_ask_volume_failed", "value", lvl.Volume, "err", err)
			return nil
		}

		askCum += volume
		asks = append(asks, domain.Order{
			Price: price,
			Amount: volume,
			Notional: askCum,
			Side: domain.SideAsk,
			Source: domain.SourceGrinexUSDTA7A5,
			Pair: domain.USDTA7A5,
		})
	}

	for _, lvl := range resp.Bids {
		price, err := strconv.ParseFloat(lvl.Price, 64)
		if err != nil {
			log.Error("grinex_mapper_parse_bid_price_failed", "value", lvl.Price, "err", err)
			return nil
		}
		volume, err := strconv.ParseFloat(lvl.Volume, 64)
		if err != nil {
			log.Error("grinex_mapper_parse_bid_volume_failed", "value", lvl.Volume, "err", err)
			return nil
		}

		bidCum += volume
		bids = append(bids, domain.Order{
			Price:    price,
			Amount:   volume,
			Notional: bidCum,
			Side:     domain.SideBid,
			Source:   domain.SourceGrinexUSDTA7A5,
			Pair:     domain.USDTA7A5,
		})
	}

	if len(asks) == 0 || len(bids) == 0 {
		log.Warn("grinex_mapper_empty_book", "asks", len(asks), "bids", len(bids))
		return nil
	}

	return &Orderbook{
		Asks: asks,
		Bids: bids,
		UpdatedAt: time.Now(),
	}
}

func debugTop(ob *Orderbook) string {
	if ob == nil || len(ob.Asks) == 0 || len(ob.Bids) == 0 {
		return "empty"
	}
	return fmt.Sprintf("ask=%.4f bid=%.4f", ob.Asks[0].Price, ob.Bids[0].Price)
}

