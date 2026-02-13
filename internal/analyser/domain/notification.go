package domain

import "time"

type ArbitrageType string
type Pair string

const (
	USDTRUB   Pair          = "USDT_RUB"
	USDTA7A5  Pair          = "USDT_A7A5"
	Fact      ArbitrageType = "fact"
	Potential ArbitrageType = "potential"
)

type Notification struct {
	ID         int64
	ChatID     int64
	Type       ArbitrageType
	Pair       Pair
	Direction  string // "buy_rapira_sell_grinex", "buy_rapira_sell_rapira" и тд.
	ProfitDiff float64
	Notional   float64
	OpHash     string // хэш арбитражной ситуации для dedup
	CreatedAt  time.Time
}
