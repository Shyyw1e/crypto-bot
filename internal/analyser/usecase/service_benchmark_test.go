package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/repository"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type benchLogger struct{}

func (benchLogger) Debug(string, ...any) {}
func (benchLogger) Info(string, ...any)  {}
func (benchLogger) Warn(string, ...any)  {}
func (benchLogger) Error(string, ...any) {}
func (benchLogger) With(...any) logger.Logger {
	return benchLogger{}
}

type benchOrderbookCache struct {
	books map[string]*cache.Orderbook
}

func (c *benchOrderbookCache) Get(_ context.Context, source domain.Source, pair domain.Pair) (*cache.Orderbook, error) {
	key := fmt.Sprintf("%s|%s", source, pair)
	ob, ok := c.books[key]
	if !ok {
		return nil, cache.ErrNotFound
	}
	return ob, nil
}

type benchUserRepo struct {
	users []*domain.UserSettings
}

func (r *benchUserRepo) GetByChatID(_ context.Context, _ int64) (*domain.UserSettings, error) {
	return nil, repository.ErrNotFound
}

func (r *benchUserRepo) Save(_ context.Context, _ *domain.UserSettings) error {
	return nil
}

func (r *benchUserRepo) SetActive(_ context.Context, _ int64, _ bool) error {
	return nil
}

func (r *benchUserRepo) ListActive(_ context.Context) ([]*domain.UserSettings, error) {
	return r.users, nil
}

type benchNotifRepo struct{}

func (r *benchNotifRepo) Create(_ context.Context, _ *domain.Notification) error {
	return nil
}

func (r *benchNotifRepo) ListByChatID(_ context.Context, _ int64, _ int) ([]*domain.Notification, error) {
	return nil, nil
}

type benchDedup struct{}

func (d *benchDedup) SeenOrMark(_ context.Context, _ int64, _ string, _ time.Duration) (bool, error) {
	return false, nil
}

type benchNotifier struct{}

func (n *benchNotifier) Send(_ context.Context, _ *domain.Notification) error {
	return nil
}

func makeUsers(n int) []*domain.UserSettings {
	users := make([]*domain.UserSettings, 0, n)
	for i := 0; i < n; i++ {
		users = append(users, &domain.UserSettings{
			ChatID:            int64(100000 + i),
			WatchFact:         true,
			WatchPotential:    true,
			MinDiffFact:       0.01,
			MinDiffPotential:  0.01,
			MaxNotional:       0,
			IsActive:          true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		})
	}
	return users
}

func benchOrderbook(levels int, askStart float64, bidStart float64) *cache.Orderbook {
	return &cache.Orderbook{
		Asks:      makeAsks(levels, askStart, 0.01, 100),
		Bids:      makeBids(levels, bidStart, 0.01, 100),
		UpdatedAt: time.Now(),
	}
}

func BenchmarkServiceHandleTick(b *testing.B) {
	cases := []struct {
		users  int
		depth  int
		name   string
	}{
		{users: 100, depth: 10, name: "users_100_depth_10"},
		{users: 1000, depth: 10, name: "users_1000_depth_10"},
		{users: 5000, depth: 50, name: "users_5000_depth_50"},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			obCache := &benchOrderbookCache{
				books: map[string]*cache.Orderbook{
					fmt.Sprintf("%s|%s", domain.SourceRapira, domain.USDTRUB): benchOrderbook(tc.depth, 92.00, 90.00),
					fmt.Sprintf("%s|%s", domain.SourceGrinexUSDTA7A5, domain.USDTA7A5): benchOrderbook(tc.depth, 92.05, 90.05),
				},
			}

			svc := NewService(
				benchLogger{},
				obCache,
				&benchUserRepo{users: makeUsers(tc.users)},
				&benchNotifRepo{},
				&benchDedup{},
				&benchNotifier{},
			)

			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := svc.HandleTick(ctx); err != nil {
					b.Fatalf("HandleTick failed: %v", err)
				}
			}
		})
	}
}
