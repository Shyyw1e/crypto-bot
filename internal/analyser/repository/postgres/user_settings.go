package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/repository"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

// UserSettingsPostgres реализует UserSettingsRepository через таблицу user_pair_settings.
type UserSettingsPostgres struct {
	pool *pgxpool.Pool
	log  logger.Logger

	// пока что работаем только с одной парой (USDT/RUB)
	// можно вынести в конфиг при необходимости
	defaultPairCode string
}

func NewUserSettingsPostgres(pool *pgxpool.Pool, log logger.Logger) *UserSettingsPostgres {
	return &UserSettingsPostgres{
		pool:            pool,
		log:             log.With("repo", "user_settings"),
		defaultPairCode: "USDT/RUB",
	}
}

// helper: получить или создать user_id по chat_id
func (r *UserSettingsPostgres) resolveUserID(ctx context.Context, chatID int64) (int64, error) {
	const q = `
INSERT INTO users (chat_id, telegram_id)
VALUES ($1, $1)
ON CONFLICT (chat_id) DO UPDATE
SET chat_id = EXCLUDED.chat_id
RETURNING id;
`

	var userID int64
	if err := r.pool.QueryRow(ctx, q, chatID).Scan(&userID); err != nil {
		r.log.Error("user_settings_resolve_user_id_failed", "chat_id", chatID, "err", err)
		return 0, fmt.Errorf("resolve user id: %w", err)
	}

	return userID, nil
}

// helper: получить pair_id по коду пары.
func (r *UserSettingsPostgres) resolvePairID(ctx context.Context) (int64, error) {
	const q = `
SELECT id
FROM pairs
WHERE symbol = $1;
`
	// NOTE: если у тебя в таблице pairs колонка называется не code, а symbol/name — поменяй здесь.
	var pairID int64
	if err := r.pool.QueryRow(ctx, q, r.defaultPairCode).Scan(&pairID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("pair not found for code=%s", r.defaultPairCode)
		}
		r.log.Error("user_settings_resolve_pair_id_failed", "pair_code", r.defaultPairCode, "err", err)
		return 0, fmt.Errorf("resolve pair id: %w", err)
	}

	return pairID, nil
}

func (r *UserSettingsPostgres) GetByChatID(ctx context.Context, chatID int64) (*domain.UserSettings, error) {
	const q = `
SELECT
    ups.fact_enabled,
    ups.potential_enabled,
    ups.min_spread_abs,
    ups.min_spread_pct,
    COALESCE(ups.max_notional, 0),
    ups.enabled
FROM user_pair_settings ups
JOIN users u ON ups.user_id = u.id
JOIN pairs p ON ups.pair_id = p.id
WHERE u.chat_id = $1
  AND p.symbol = $2
LIMIT 1;
`

	row := r.pool.QueryRow(ctx, q, chatID, r.defaultPairCode)

	var (
		watchFact       bool
		watchPotential  bool
		minDiffFact     float64
		minDiffPotential float64
		maxNotional     float64
		enabled         bool
	)

	if err := row.Scan(
		&watchFact,
		&watchPotential,
		&minDiffFact,
		&minDiffPotential,
		&maxNotional,
		&enabled,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		r.log.Error("user_settings_get_failed", "chat_id", chatID, "err", err)
		return nil, fmt.Errorf("get user settings: %w", err)
	}

	s := &domain.UserSettings{
		ChatID:           chatID,
		WatchFact:        watchFact,
		WatchPotential:   watchPotential,
		MinDiffFact:      minDiffFact,
		MinDiffPotential: minDiffPotential,
		MaxNotional:      maxNotional,
		// если есть поле Active/IsActive — можно сюда проставить enabled
	}

	return s, nil
}

func (r *UserSettingsPostgres) Save(ctx context.Context, s *domain.UserSettings) error {
	if s == nil {
		return fmt.Errorf("nil settings")
	}

	userID, err := r.resolveUserID(ctx, s.ChatID)
	if err != nil {
		return err
	}

	pairID, err := r.resolvePairID(ctx)
	if err != nil {
		return err
	}

	const q = `
INSERT INTO user_pair_settings (
    user_id,
    pair_id,
    fact_enabled,
    potential_enabled,
    min_spread_abs,
    min_spread_pct,
    max_notional,
    enabled,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
ON CONFLICT (user_id, pair_id) DO UPDATE SET
    fact_enabled      = EXCLUDED.fact_enabled,
    potential_enabled = EXCLUDED.potential_enabled,
    min_spread_abs    = EXCLUDED.min_spread_abs,
    min_spread_pct    = EXCLUDED.min_spread_pct,
    max_notional      = EXCLUDED.max_notional,
    enabled           = EXCLUDED.enabled,
    updated_at        = NOW();
`

	enabled := s.WatchFact || s.WatchPotential // логично: если что-то смотрим — анализ включён

	_, err = r.pool.Exec(ctx, q,
		userID,
		pairID,
		s.WatchFact,
		s.WatchPotential,
		s.MinDiffFact,
		s.MinDiffPotential,
		s.MaxNotional,
		enabled,
	)
	if err != nil {
		r.log.Error("user_settings_save_failed", "chat_id", s.ChatID, "err", err)
		return fmt.Errorf("save user settings: %w", err)
	}

	return nil
}

func (r *UserSettingsPostgres) SetActive(ctx context.Context, chatID int64, active bool) error {
	userID, err := r.resolveUserID(ctx, chatID)
	if err != nil {
		return err
	}

	pairID, err := r.resolvePairID(ctx)
	if err != nil {
		return err
	}

	const q = `
UPDATE user_pair_settings
SET enabled = $3,
    updated_at = NOW()
WHERE user_id = $1
  AND pair_id = $2;
`

	ct, err := r.pool.Exec(ctx, q, userID, pairID, active)
	if err != nil {
		r.log.Error("user_settings_set_active_failed", "chat_id", chatID, "err", err)
		return fmt.Errorf("set active: %w", err)
	}

	if ct.RowsAffected() == 0 {
		// Можно вернуть ErrNotFound, а можно тихо игнорировать.
		return repository.ErrNotFound
	}

	return nil
}

func (r *UserSettingsPostgres) ListActive(ctx context.Context) ([]*domain.UserSettings, error) {
	const q = `
SELECT
    u.chat_id,
    ups.fact_enabled,
    ups.potential_enabled,
    ups.min_spread_abs,
    ups.min_spread_pct,
    COALESCE(ups.max_notional, 0)
FROM user_pair_settings ups
JOIN users u ON ups.user_id = u.id
JOIN pairs p ON ups.pair_id = p.id
WHERE ups.enabled = TRUE
  AND p.symbol = $1;
`

	rows, err := r.pool.Query(ctx, q, r.defaultPairCode)
	if err != nil {
		r.log.Error("user_settings_list_active_query_failed", "err", err)
		return nil, fmt.Errorf("list active: %w", err)
	}
	defer rows.Close()

	var res []*domain.UserSettings

	for rows.Next() {
		var (
			chatID           int64
			watchFact        bool
			watchPotential   bool
			minDiffFact      float64
			minDiffPotential float64
			maxNotional      float64
		)

		if err := rows.Scan(
			&chatID,
			&watchFact,
			&watchPotential,
			&minDiffFact,
			&minDiffPotential,
			&maxNotional,
		); err != nil {
			r.log.Error("user_settings_list_active_scan_failed", "err", err)
			return nil, fmt.Errorf("list active scan: %w", err)
		}

		res = append(res, &domain.UserSettings{
			ChatID:           chatID,
			WatchFact:        watchFact,
			WatchPotential:   watchPotential,
			MinDiffFact:      minDiffFact,
			MinDiffPotential: minDiffPotential,
			MaxNotional:      maxNotional,
		})
	}

	if err := rows.Err(); err != nil {
		r.log.Error("user_settings_list_active_rows_err", "err", err)
		return nil, fmt.Errorf("list active rows: %w", err)
	}

	return res, nil
}
