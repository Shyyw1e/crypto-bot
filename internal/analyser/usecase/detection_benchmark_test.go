package usecase

import (
	"fmt"
	"testing"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
)

func makeAsks(levels int, startPrice float64, step float64, amount float64) []domain.Order {
	asks := make([]domain.Order, levels)
	for i := 0; i < levels; i++ {
		price := startPrice + float64(i)*step
		asks[i] = domain.Order{
			Price:    price,
			Amount:   amount,
			Notional: amount,
			Side:     domain.SideAsk,
			Source:   domain.SourceRapira,
			Pair:     domain.USDTRUB,
		}
	}
	return asks
}

func makeBids(levels int, startPrice float64, step float64, amount float64) []domain.Order {
	bids := make([]domain.Order, levels)
	for i := 0; i < levels; i++ {
		price := startPrice - float64(i)*step
		bids[i] = domain.Order{
			Price:    price,
			Amount:   amount,
			Notional: amount,
			Side:     domain.SideBid,
			Source:   domain.SourceGrinexUSDTA7A5,
			Pair:     domain.USDTRUB,
		}
	}
	return bids
}

func BenchmarkDetectFact(b *testing.B) {
	asks := makeAsks(1, 92.00, 0.01, 100)
	bids := makeBids(1, 90.00, 0.01, 100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectFact(
			asks,
			bids,
			domain.SourceRapira,
			domain.SourceGrinexUSDTA7A5,
			domain.USDTRUB,
			0.0,
			0.0005,
		)
	}
}

func BenchmarkDetectPotentialByAsks(b *testing.B) {
	depths := []int{10, 100, 1000}

	for _, depth := range depths {
		depth := depth
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			asks := makeAsks(depth, 92.00, 0.01, 100)
			bids := makeBids(depth, 90.00, 0.01, 100)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = DetectPotentialByAsks(
					asks,
					bids,
					domain.SourceRapira,
					domain.SourceGrinexUSDTA7A5,
					domain.USDTRUB,
					depth,
					0.0,
					0.0005,
				)
			}
		})
	}
}

func BenchmarkDetectPotentialByBids(b *testing.B) {
	depths := []int{10, 100, 1000}

	for _, depth := range depths {
		depth := depth
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			asks := makeAsks(depth, 92.00, 0.01, 100)
			bids := makeBids(depth, 90.00, 0.01, 100)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = DetectPotentialByBids(
					asks,
					bids,
					domain.SourceRapira,
					domain.SourceGrinexUSDTA7A5,
					domain.USDTRUB,
					depth,
					0.0,
					0.0005,
				)
			}
		})
	}
}
