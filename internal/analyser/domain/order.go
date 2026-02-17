package domain

import (
	"fmt"
	"time"
)

type OrderSide string
type Source string

const (
	SideBid OrderSide = "bid"
	SideAsk OrderSide = "ask"

	SourceRapira         Source = "rapira"
	SourceGrinexUSDTA7A5 Source = "grinex_usdt_a7a5"
)

type Order struct {
	Price    float64   	// цена
	Amount   float64   	// количество базовой валюты (USDT)
	Notional float64   	// объём в USDT (Sum)
	Side     OrderSide 	// bid или ask
	Source   Source		// rapira
	Pair     Pair		// const "USDT/RUB"
}


type Opportunity struct {
	Type ArbitrageType
	Pair Pair

	BuyExchange  Source
	SellExchange Source

	BuyPrice  float64
	SellPrice float64

	BuyAmount float64
	Notional  float64

	ProfitDiff float64
	// ProfitPct float64 // по желанию

	SuggestedBid float64
	CreatedAt    time.Time
}

func (o *Opportunity) Hash(chatID int64) string {
	return fmt.Sprintf(
		"%d-%s-%s-%s-%.2f-%.2f-%.4f",
		chatID,
		o.Pair,
		o.BuyExchange,
		o.SellExchange,
		o.BuyPrice,
		o.SellPrice,
		o.BuyAmount,
	)
}


type FeesConfig struct {
	Source	Source		// rapira/grinex
	Value 	float64		// в процентах, например 0.0 будет означать 0.0% комиссии
}