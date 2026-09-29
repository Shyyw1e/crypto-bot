package usecase

import (
	"testing"

	"github.com/Shyyw1e/crypto-bot/internal/abcex-gw/adapters/abcexws"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any) {}
func (testLogger) Info(string, ...any)  {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Error(string, ...any) {}
func (testLogger) With(...any) logger.Logger {
	return testLogger{}
}

func TestMapper(t *testing.T) {
	msg := &abcexws.OrderbookMessage{
		Stream: "USDTRUB@orderbook",
		Data: abcexws.OrderbookData{
			Asks: [][]string{{"81.02", "1000"}, {"81.03", "500"}},
			Bids: [][]string{{"81.00", "100"}, {"80.99", "20"}},
		},
	}

	ob := Mapper(msg, testLogger{})
	if ob == nil {
		t.Fatal("Mapper() returned nil")
	}
	if got := len(ob.Asks); got != 2 {
		t.Fatalf("asks len = %d", got)
	}
	if got := len(ob.Bids); got != 2 {
		t.Fatalf("bids len = %d", got)
	}
	if ob.Asks[1].Notional != 1500 {
		t.Fatalf("ask cumulative notional = %v", ob.Asks[1].Notional)
	}
	if ob.Bids[1].Notional != 120 {
		t.Fatalf("bid cumulative notional = %v", ob.Bids[1].Notional)
	}
	if ob.Asks[0].Source != domain.SourceABCEX || ob.Asks[0].Pair != domain.USDTRUB {
		t.Fatalf("unexpected order identity: source=%s pair=%s", ob.Asks[0].Source, ob.Asks[0].Pair)
	}
}
