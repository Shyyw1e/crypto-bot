package rapiraapi

import "errors"

// используем, если вернулся {} от Rapira
var ErrEmptyBook = errors.New("empty orderbook from rapira") 

//"BUY" or "SELL"
type PlateSide struct { 
	Symbol 			string		`json:"symbol"`
	HighestPrice	float64		`json:"highestPrice"`
	LowestPrice		float64 	`json:"lowestPrice"`
	MinAmount		float64 	`json:"minAmount"`
	MaxAmount		float64 	`json:"maxAmount"`
	Items			[]PlateItem	`json:"items"`
	Direction		string		`json:"direction"`
}

// Одна позиция стакана. Позиция в стакане будет определяться индексом в слайсе Items
type PlateItem struct {	 
	Price	float64 `json:"price"`		
	Amount 	float64 `json:"amount"`
}

//Response RapiraAPI на POST запрос по эндпоинту /market/exchange-plate-mini
type PlateResponse struct {		
	Ask		PlateSide	`json:"ask"`
	Bid		PlateSide	`json:"bid"`
}