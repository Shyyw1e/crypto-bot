package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/grinex-gw/adapters/grinexapi"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type GrinexClient interface {
	GetDepth(ctx context.Context, symbol string) (*grinexapi.DepthResponse, error)
}

type Service struct {
	log 			logger.Logger
	client			GrinexClient
	publisher		OrderbookPublisher

	pollInterval 	time.Duration
	symbol		 	string
	source			domain.Source
	pair 			domain.Pair
}

func NewService(
	log logger.Logger,
	client GrinexClient,
	publisher OrderbookPublisher,
	pollInterval time.Duration,
	symbol string,
) *Service {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	if symbol == "" {
		symbol = "usdta7a5"
	}

	return &Service{
		log: log,
		client:       client,
		publisher:    publisher,
		pollInterval: pollInterval,
		symbol:       symbol,
		source:       domain.SourceGrinexUSDTA7A5,
		pair:         domain.USDTA7A5,
	}
}

func (s *Service) Run(ctx context.Context) error {
	s.log.Info("grinex_gw_service_started",
		"symbol", s.symbol,
		"interval", s.pollInterval.String(),
	)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	if err := s.tick(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		s.log.Warn("grinex_gw_first_tick_failed", "err", err)
	}

	for  {
		select {
		case <-ctx.Done():
			s.log.Info("grinex_gw_service_stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				s.log.Warn("grinex_gw_tick_failed", "err", err)
			}
		}
	}
}

func (s *Service) tick(ctx context.Context) error {
	ctxReq, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.client.GetDepth(ctxReq, s.symbol)
	if err != nil {
		if errors.Is(err, grinexapi.ErrEmptyBook) {
			s.log.Warn("grinex_gw_empty_book", "symbol", s.symbol)
			return nil
		}
		return err
	}

	ob := Mapper(resp, s.log)
	if ob == nil {
		return nil
	}

	return s.publisher.Publish(ctx, s.source, s.pair, ob)
}