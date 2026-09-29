package abcexws

import "testing"

func TestDecodeOrderbookMessage(t *testing.T) {
	raw := []byte(`{
		"stream": "USDTRUB@orderbook",
		"data": {
			"asks": [["81.02", "1000"]],
			"bids": [["81.00", "123.4567"]],
			"marketId": "USDTRUB",
			"seq": 131,
			"ts": 1775638037510910700,
			"type": "snapshot"
		}
	}`)

	msg, ok, err := decodeOrderbookMessage(raw)
	if err != nil {
		t.Fatalf("decodeOrderbookMessage() error = %v", err)
	}
	if !ok {
		t.Fatal("decodeOrderbookMessage() ignored orderbook message")
	}
	if msg.Stream != "USDTRUB@orderbook" {
		t.Fatalf("stream = %q", msg.Stream)
	}
	if len(msg.Data.Asks) != 1 || len(msg.Data.Bids) != 1 {
		t.Fatalf("unexpected book depth: asks=%d bids=%d", len(msg.Data.Asks), len(msg.Data.Bids))
	}
}

func TestDecodeOrderbookMessageIgnoresServiceMessages(t *testing.T) {
	_, ok, err := decodeOrderbookMessage([]byte(`{"method":"pong","id":1}`))
	if err != nil {
		t.Fatalf("decodeOrderbookMessage() error = %v", err)
	}
	if ok {
		t.Fatal("decodeOrderbookMessage() should ignore non-stream messages")
	}
}
