package usecase

import (
	"math"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
)

func DetectFact(
    asks []domain.Order,
    bids []domain.Order,
    sourceAsk domain.Source,
    sourceBid domain.Source,
    pair domain.Pair,
    feeAsk float64, //0.0 для Rapira, 0.05 для Grinex USDT/A7A5, 0.1 для Grinex USDT/RUB
    feeBid float64, //0.0 для Rapira, 0.05 для Grinex USDT/A7A5, 0.1 для Grinex USDT/RUB
) *domain.Opportunity {
	if len(asks) == 0 || len(bids) == 0 {
		return nil
	}

	ask := asks[0]
	bid := bids[0]

	effectiveAsk := ask.Price / (1 + feeAsk)
	effectiveBid := bid.Price * (1 + feeBid)
	profit := round2(effectiveAsk - effectiveBid)
	if profit <= 0.01 {	// По умолчанию ищем ситуации не меньше 0.03 diff
		return nil
	}
	
	// Объем, который реально можно исполнить на top-1 в обеих книгах
	tradeAmount := min(ask.Amount, bid.Amount)
	if tradeAmount <= 0 {
		return nil
	}

	opp := &domain.Opportunity{
		Type:         domain.Fact,
		Pair:         pair,
		BuyExchange:  sourceBid,
		SellExchange: sourceAsk,

		BuyPrice:  round2(bid.Price),
		SellPrice: round2(ask.Price),

		BuyAmount: tradeAmount,
		Notional:  tradeAmount,

		ProfitDiff:   round2(profit),
		SuggestedBid: round2(bid.Price + 0.01),
		CreatedAt:    time.Now(),
	}


	return opp
}

func round2(a float64) float64 {
	return math.Round((a)*100) / 100
}

func DetectPotentialByBids(
	asks []domain.Order,
	bids []domain.Order,
	sourceAsk domain.Source,
	sourceBid domain.Source,
	pair domain.Pair,
	depth int,
	feeAsk float64,
	feeBid float64,
) *domain.Opportunity {
	if len(asks) == 0 || len(bids) == 0 {
		return nil
	}
	if depth <= 0 || depth > len(bids) {
		depth = len(bids)
	}
	ask := asks[0]

	var best *domain.Opportunity

	for i := 1; i < depth; i++ {
		bid := bids[i]

		effectiveAsk := ask.Price / (1 + feeAsk)
		effectiveBid := bid.Price * (1 + feeBid) 
		profit := round2(effectiveAsk - effectiveBid) 

		if profit <= 0.03 {
			continue
		}

		buyAmount := min(ask.Notional, bid.Notional)

		candidate := &domain.Opportunity{
			Type:         domain.Potential,
			Pair:         pair,
			BuyExchange:  sourceBid,
			SellExchange: sourceAsk,

			BuyPrice:  round2(bid.Price),
			SellPrice: round2(ask.Price),

			BuyAmount: buyAmount,
			Notional:  buyAmount,

			ProfitDiff:   round2(profit),
			SuggestedBid: round2(bid.Price + 0.01),
			CreatedAt:    time.Now(),
		}

		if best == nil || candidate.ProfitDiff > best.ProfitDiff {
			best = candidate
		}
	}

	return best
}

func DetectPotentialByAsks(
	asks []domain.Order,
	bids []domain.Order,
	sourceAsk domain.Source,
	sourceBid domain.Source,
	pair domain.Pair,
	depth int,
	feeAsk float64,
	feeBid float64,
) *domain.Opportunity {
	if len(asks) == 0 || len(bids) == 0 {
		return nil
	}
	if depth <= 0 || depth > len(asks) {
		depth = len(asks) // fixed
	}
	bid := bids[0]

	var best *domain.Opportunity

	for i := 1; i < depth; i++ {
		ask := asks[i]

		effectiveAsk := ask.Price / (1 + feeAsk)
		effectiveBid := bid.Price * (1 + feeBid)
		profit := round2(effectiveAsk - effectiveBid)

		if profit <= 0.03 {
			continue
		}

		buyAmount := min(ask.Notional, bid.Notional)

		candidate := &domain.Opportunity{
			Type:         domain.Potential,
			Pair:         pair,
			BuyExchange:  sourceBid,
			SellExchange: sourceAsk,

			BuyPrice:  round2(bid.Price),
			SellPrice: round2(ask.Price),

			BuyAmount: buyAmount,
			Notional:  buyAmount,

			ProfitDiff:   round2(profit),
			SuggestedBid: round2(bid.Price + 0.01),
			CreatedAt:    time.Now(),
		}

		if best == nil || candidate.ProfitDiff > best.ProfitDiff {
			best = candidate
		}
	}

	return best
}
