package grinexapi

import "errors"

var ErrEmptyBook = errors.New("empty orderbook from grinex")

type DepthLevel struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
	Amount string `json:"amount"` // price * volume (не используем в analyser)
}

type DepthResponse struct {
	Timestamp int64        `json:"timestamp"`
	Asks      []DepthLevel `json:"asks"`
	Bids      []DepthLevel `json:"bids"`
}
