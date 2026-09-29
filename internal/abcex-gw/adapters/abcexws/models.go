package abcexws

import "errors"

var ErrEmptyBook = errors.New("empty orderbook from abcex")

type OrderbookData struct {
	Asks     [][]string `json:"asks"`
	Bids     [][]string `json:"bids"`
	MarketID string     `json:"marketId"`
	Seq      int64      `json:"seq"`
	Ts       int64      `json:"ts"`
	Type     string     `json:"type"`
}

type OrderbookMessage struct {
	Stream string
	Data   OrderbookData
}
