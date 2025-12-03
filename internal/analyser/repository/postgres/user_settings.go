package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/repository"
	"github.com/Shyyw1e/crypto-bot/internal/shared/db"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type UserSettingsPostgres struct {
	q   db.Querier
	log logger.Logger
}

func NewUserSettingsPostgres(q db.Querier, log logger.Logger) repository.UserSettingsRepository {
	return &UserSettingsPostgres{
		q:   q,
		log: log.With("repo", "user_settings"),
	}
}

func (r *UserSettingsPostgres) GetByChatID(ctx context.Context, chatID int64) (*domain.UserSettings, error) {
	const query = `
SELECT
    chat_id,
    watch_fact,
    watch_potential,
    min_diff_fact,
    min_diff_potential,
    max_notional,
    is_active,
    created_at,
    updated_at
FROM user_settings
WHERE chat_id = $1;
`

	row := r.q.QueryRow(ctx, query, chatID)

	var s domain.UserSettings
	if err := row.Scan(
		&s.ChatID,
		&s.WatchFact,
		&s.WatchPotential,
		&s.MinDiffFact,
		&s.MinDiffPotential,
		&s.MaxNotional,
		&s.IsActive,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		r.log.Error("user_settings_get_failed", "chat_id", chatID, "err", err)
		return nil, err
	}

	return &s, nil
}

func (r *UserSettingsPostgres) Save(ctx context.Context, s *domain.UserSettings) error {
	const query = `
INSERT INTO user_settings (
    chat_id,
    watch_fact,
    watch_potential,
    min_diff_fact,
    min_diff_potential,
    max_notional,
    is_active,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, NOW(), NOW()
)
ON CONFLICT (chat_id) DO UPDATE SET
    watch_fact         = EXCLUDED.watch_fact,
    watch_potential    = EXCLUDED.watch_potential,
    min_diff_fact      = EXCLUDED.min_diff_fact,
    min_diff_potential = EXCLUDED.min_diff_potential,
    max_notional       = EXCLUDED.max_notional,
    is_active          = EXCLUDED.is_active,
    updated_at         = NOW();
`

	_, err := r.q.Exec(
		ctx,
		query,
		s.ChatID,
		s.WatchFact,
		s.WatchPotential,
		s.MinDiffFact,
		s.MinDiffPotential,
		s.MaxNotional,
		s.IsActive,
	)
	if err != nil {
		r.log.Error("user_settings_save_failed", "chat_id", s.ChatID, "err", err)
		return err
	}

	return nil
}

func (r *UserSettingsPostgres) SetActive(ctx context.Context, chatID int64, active bool) error {
	const query = `
UPDATE user_settings
SET
    is_active  = $2,
    updated_at = NOW()
WHERE chat_id = $1;
`

	_, err := r.q.Exec(ctx, query, chatID, active)
	if err != nil {
		r.log.Error("user_settings_set_active_failed", "chat_id", chatID, "active", active, "err", err)
		return err
	}

	return nil
}

func (r *UserSettingsPostgres) ListActive(ctx context.Context) ([]*domain.UserSettings, error) {
	const query = `
SELECT
    chat_id,
    watch_fact,
    watch_potential,
    min_diff_fact,
    min_diff_potential,
    max_notional,
    is_active,
    created_at,
    updated_at
FROM user_settings
WHERE is_active = TRUE;
`

	rows, err := r.q.Query(ctx, query)
	if err != nil {
		r.log.Error("user_settings_list_active_query_failed", "err", err)
		return nil, err
	}
	defer rows.Close()

	var result []*domain.UserSettings

	for rows.Next() {
		var s domain.UserSettings
		if err := rows.Scan(
			&s.ChatID,
			&s.WatchFact,
			&s.WatchPotential,
			&s.MinDiffFact,
			&s.MinDiffPotential,
			&s.MaxNotional,
			&s.IsActive,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			r.log.Error("user_settings_list_active_scan_failed", "err", err)
			return nil, err
		}
		result = append(result, &s)
	}

	if err := rows.Err(); err != nil {
		r.log.Error("user_settings_list_active_rows_err", "err", err)
		return nil, err
	}

	return result, nil
}
