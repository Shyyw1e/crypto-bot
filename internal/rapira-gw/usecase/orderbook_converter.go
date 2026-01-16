package usecase

import (
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/rapira-gw/adapters/rapiraapi"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

// локальный тип, который мы потом маршалим в JSON и кладём в Redis
// (поле names и формат должны совпасть с тем, что ждёт analyser/cache).
type Orderbook struct {
    Bids      []domain.Order	`json:"bids"`
    Asks      []domain.Order	`json:"asks"`
    UpdatedAt time.Time			`json:"updated_at"`
}

func Mapper(resp *rapiraapi.PlateResponse, log logger.Logger) (*Orderbook) {
	if resp == nil {
		err := fmt.Errorf("empty plateResponse")
		log.Error("rapira-gw_usecase_mapper", "err", err)
		return nil
	}

	asks := make([]domain.Order, 0, len(resp.Ask.Items))
	bids := make([]domain.Order, 0, len(resp.Bid.Items))

	var bidNotional, askNotional float64
	for i := 0; i < len(resp.Ask.Items); i++ {
		askNotional += resp.Ask.Items[i].Amount
		newAsk := domain.Order{
			Price: resp.Ask.Items[i].Price,				
			Amount: resp.Ask.Items[i].Amount,
			Notional: askNotional,
			Side: domain.SideAsk,
			Source: domain.SourceRapira,
			Pair: domain.USDTRUB,
		}
		asks = append(asks, newAsk)
	}
	for i := 0; i < len(resp.Bid.Items); i++ {
		bidNotional += resp.Bid.Items[i].Amount
		newBid := domain.Order{
			Price: resp.Bid.Items[i].Price,
			Amount: resp.Bid.Items[i].Amount,
			Notional: bidNotional,	// /market/exchange-plate-mini отдает price и amount, но не сумму, а сколько именно в этом order есть валюты
			Side: domain.SideBid,
			Source: domain.SourceRapira,
			Pair: domain.USDTRUB,
		}
		bids = append(bids, newBid)
	}

	if len(asks) == 0 || len(bids) == 0 {
		log.Warn("rapira-gw_usecase_mapper", "warn", fmt.Sprintf("len(asks): %v, len(bids): %v", len(asks), len(bids)))
		return nil
	}

	return &Orderbook{
		Asks: asks,
		Bids: bids,
		UpdatedAt: time.Now(),
	}

}
