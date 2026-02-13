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

	Fees domain.FeesConfig // опционально: комиссии per source/pair
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
		log:            log,
		orderbookCache: obCache,
		userRepo:       userRepo,
		notifRepo:      notifRepo,
		dedupStore:     dedup,
		notifier:       notifier,
	}
}

func (s *Service) UpsertUserSettings(ctx context.Context, in *domain.UserSettings) error {
	if in == nil {
		return fmt.Errorf("upsert user settings: nil input")
	}
	if in.ChatID == 0 {
		return fmt.Errorf("upsert user settings: empty chat_id")
	}

	// Пробуем найти существующие настройки
	existing, err := s.userRepo.GetByChatID(ctx, in.ChatID)
	if err != nil && err != repository.ErrNotFound {
		s.log.Error("usecase_upsert_user_settings_get_failed",
			"chat_id", in.ChatID,
			"err", err,
		)
		return fmt.Errorf("get user settings: %w", err)
	}

	var toSave *domain.UserSettings

	if err == repository.ErrNotFound || existing == nil {
		// Создаём новый объект
		toSave = &domain.UserSettings{
			ChatID:           in.ChatID,
			WatchFact:        in.WatchFact,
			WatchPotential:   in.WatchPotential,
			MinDiffFact:      in.MinDiffFact,
			MinDiffPotential: in.MinDiffPotential,
			MaxNotional:      in.MaxNotional,
			// Если в домене есть Active/CreatedAt/UpdatedAt — можно выставить:
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	} else {
		// Обновляем существующие поля
		existing.WatchFact = in.WatchFact
		existing.WatchPotential = in.WatchPotential
		existing.MinDiffFact = in.MinDiffFact
		existing.MinDiffPotential = in.MinDiffPotential
		existing.MaxNotional = in.MaxNotional
		existing.UpdatedAt = time.Now()

		// existing.Active = true

		toSave = existing
	}

	if err := s.userRepo.Save(ctx, toSave); err != nil {
		s.log.Error("usecase_upsert_user_settings_save_failed",
			"chat_id", in.ChatID,
			"err", err,
		)
		return fmt.Errorf("save user settings: %w", err)
	}

	s.log.Info("usecase_upsert_user_settings_ok",
		"chat_id", toSave.ChatID,
		"watch_fact", toSave.WatchFact,
		"watch_potential", toSave.WatchPotential,
		"min_diff_fact", toSave.MinDiffFact,
		"min_diff_potential", toSave.MinDiffPotential,
		"max_notional", toSave.MaxNotional,
	)

	return nil
}

// SetUserActive — включить/выключить пользователя.
func (s *Service) SetUserActive(ctx context.Context, chatID int64, active bool) error {
	if chatID == 0 {
		return fmt.Errorf("set user active: empty chat_id")
	}

	if err := s.userRepo.SetActive(ctx, chatID, active); err != nil {
		s.log.Error("usecase_set_user_active_failed",
			"chat_id", chatID,
			"active", active,
			"err", err,
		)
		return err
	}

	s.log.Info("usecase_set_user_active_ok",
		"chat_id", chatID,
		"active", active,
	)

	return nil
}

func (s *Service) HandleTick(ctx context.Context) error {
	s.log.Debug("usecase_handletick_start")

	obRapira, err := s.orderbookCache.Get(ctx, domain.SourceRapira, domain.USDTRUB)
	if err != nil {
		s.log.Debug("usecase_handletick_orderbook_get_err", "err", err)
		if err == cache.ErrNotFound {
			return nil
		}
		return err
	}
	if obRapira == nil {
		s.log.Debug("usecase_handletick_orderbook_nil")
		return nil
	}

	// покажем, что реально достали книгу
	bestAsk := 0.0
	bestBid := 0.0
	if len(obRapira.Asks) > 0 {
		bestAsk = obRapira.Asks[0].Price
	}
	if len(obRapira.Bids) > 0 {
		bestBid = obRapira.Bids[0].Price
	}

	s.log.Debug("usecase_handletick_orderbook_loaded",
		"asks", len(obRapira.Asks),
		"bids", len(obRapira.Bids),
		"updated_at", obRapira.UpdatedAt.Format(time.RFC3339Nano),
		"best_ask", bestAsk,
		"best_bid", bestBid,
		"spread", bestAsk-bestBid,
	)

	opps := make([]*domain.Opportunity, 0)

	oppFact := DetectFact(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 0.0, 0.0)
	s.log.Debug("detect_fact_done", "ok", oppFact != nil)
	if oppFact != nil {
		opps = append(opps, oppFact)
	}

	oppPA := DetectPotentialByAsks(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 5, 0.0, 0.0)
	s.log.Debug("detect_potential_by_asks_done", "ok", oppPA != nil)
	if oppPA != nil {
		opps = append(opps, oppPA)
	}

	oppPB := DetectPotentialByBids(obRapira.Asks, obRapira.Bids, domain.SourceRapira, domain.SourceRapira, domain.USDTRUB, 5, 0.0, 0.0)
	s.log.Debug("detect_potential_by_bids_done", "ok", oppPB != nil)
	if oppPB != nil {
		opps = append(opps, oppPB)
	}

	s.log.Debug("usecase_handletick_opps_count", "count", len(opps))
	if len(opps) == 0 {
		return nil
	}

	users, err := s.userRepo.ListActive(ctx)
	if err != nil {
		s.log.Debug("usecase_handletick_list_active_err", "err", err)
		return err
	}
	s.log.Debug("usecase_handletick_active_users", "count", len(users))

	for _, u := range users {
		for _, opp := range opps {
			ok := MatchUserSettings(u, opp)
			if !ok {
				s.log.Debug("usecase_match_failed",
					"chat_id", u.ChatID,
					"opp_type", opp.Type,
					"watch_fact", u.WatchFact,
					"watch_potential", u.WatchPotential,
					"min_diff_fact", u.MinDiffFact,
					"min_diff_potential", u.MinDiffPotential,
					"max_notional", u.MaxNotional,
					"opp_diff", opp.ProfitDiff,
					"opp_notional", opp.Notional,
				)
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

func buildDirection(opp *domain.Opportunity) string {
	return fmt.Sprintf("buy_%s_sell_%s", opp.BuyExchange, opp.SellExchange)
}
