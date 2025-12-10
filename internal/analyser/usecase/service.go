package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/cache"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/repository"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)
type Service struct {
    log logger.Logger

    orderbookCache cache.OrderbookCache

    userRepo   repository.UserSettingsRepository
    notifRepo  repository.NotificationRepository
    dedupStore cache.NotificationDedup

    notifier Notifier

    Fees      domain.FeesConfig // опционально: комиссии per source/pair 
	// Fees - комиссии, на Rapira и Grinex отличаются, но нам они известны.

}

type Notifier interface {
    // Отправить сигнал в сторону tg-bot (через HTTP/NATS/gRPC — не важно).
    Send(ctx context.Context, n *domain.Notification) error
}

func NewService(
    log logger.Logger,
    obCache cache.OrderbookCache,
    userRepo repository.UserSettingsRepository,
    notifRepo repository.NotificationRepository,
    dedup cache.NotificationDedup,
    notifier Notifier,
) *Service {
	return &Service{
		log: log,
		orderbookCache: obCache,
		userRepo: userRepo,
		notifRepo: notifRepo,
		dedupStore: dedup,
		notifier: notifier,
	}
}


func (s *Service) HandleTick(ctx context.Context) error {
	obRapira, err := s.orderbookCache.Get(ctx, domain.SourceRapira, domain.USDTRUB)
	if err != nil {
		if err == cache.ErrNotFound {
			s.log.Warn("usecase_handletick", "err", cache.ErrNotFound)
			return nil
		}
		s.log.Error("usecase_handletick_failed", "err", err)
		return nil
	}
	if obRapira == nil {
		s.log.Warn("usecase_handletick_empty_orderbook")
		return nil
	}
	
	opps := make([]*domain.Opportunity, 0)

	// fact rapira USDT/RUB
	if opp := DetectFact(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 0.0, 0.0); opp != nil {
		opps = append(opps, opp)
	}
	// potential by asks rapira USDT/RUB 
	if opp := DetectPotentialByAsks(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 5, 0.0, 0.0); opp != nil {
		opps = append(opps, opp) 
	}
	// potential by bids rapira USDT/RUB
	if opp := DetectPotentialByBids(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 5, 0.0, 0.0); opp != nil {
		opps = append(opps, opp)
	}
	if len(opps) == 0 {
		s.log.Warn("usecase_handletick_empty_opportunities")
		return nil
	}

	users, err := s.userRepo.ListActive(ctx)
    if err != nil {
		s.log.Error("usecase_handletick_failed", "err", err)
        return fmt.Errorf("handletick: %w", err)
    }

	for _, u := range users {
		for _, opp := range opps {
			if !MatchUserSettings(u, opp) {
				continue
			}

			hash := opp.Hash(u.ChatID)

			seen, err := s.dedupStore.SeenOrMark(ctx, u.ChatID, hash, 10*time.Minute)
			if err != nil {
				s.log.Warn("usecase_handletick_dedup_failed", "err", err)
				continue
			}
			if seen {
				continue
			}

			notif := buildNotification(u, opp, hash)
			
			if err := s.notifRepo.Create(ctx, notif); err != nil {
				s.log.Error("usecase_handletick_notif_create_failed", "err", err)
				continue
			}
			 if err := s.notifier.Send(ctx, notif); err != nil {
                s.log.Error("usecase_handletick_notif_send_failed", "chat_id", u.ChatID, "err", err)
            }
		}
	}

	return nil
}

func MatchUserSettings(u *domain.UserSettings, opp *domain.Opportunity) bool {
    switch opp.Type {
    case domain.Fact:
        if !u.WatchFact {
            return false
        }
        return opp.ProfitDiff >= u.MinDiffFact
    case domain.Potential:
        if !u.WatchPotential {
            return false
        }
        if opp.ProfitDiff < u.MinDiffPotential {
            return false
        }
        if u.MaxNotional > 0 && opp.Notional > u.MaxNotional {
            return false
        }
        return true
    default:
        return false
    }
}


func buildNotification(u *domain.UserSettings, opp *domain.Opportunity, hash string) *domain.Notification {
    return &domain.Notification{
        ChatID:     u.ChatID,
        Type:       opp.Type,
        Pair:       opp.Pair,
        Direction:  buildDirection(opp), // e.g. "buy_rapira_sell_rapira"
        ProfitDiff: opp.ProfitDiff,
        Notional:   opp.Notional,
        OpHash:     hash,
        CreatedAt:  time.Now(),
    }
}

func buildDirection(opp *domain.Opportunity) string{
	return fmt.Sprintf("buy_%s_sell_%s", opp.BuyExchange, opp.SellExchange)
}