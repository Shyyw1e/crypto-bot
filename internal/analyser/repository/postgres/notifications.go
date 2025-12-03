package postgres

import (
	"context"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/repository"
	"github.com/Shyyw1e/crypto-bot/internal/shared/db"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type NotificationPostgres struct {
	q   db.Querier
	log logger.Logger
}

func NewNotificationPostgres(q db.Querier, log logger.Logger) repository.NotificationRepository {
	return &NotificationPostgres{
		q:   q,
		log: log.With("repo", "notifications"),
	}
}

func (r *NotificationPostgres) Create(ctx context.Context, n *domain.Notification) error {
	const query = `
INSERT INTO notifications (
    chat_id,
    type,
    pair,
    direction,
    profit_diff,
    notional,
    op_hash,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, NOW()
);
`

	_, err := r.q.Exec(
		ctx,
		query,
		n.ChatID,
		string(n.Type),
		string(n.Pair),
		n.Direction,
		n.ProfitDiff,
		n.Notional,
		n.OpHash,
	)
	if err != nil {
		r.log.Error("notification_create_failed", "chat_id", n.ChatID, "err", err)
		return err
	}

	return nil
}

func (r *NotificationPostgres) ListByChatID(
	ctx context.Context,
	chatID int64,
	limit int,
) ([]*domain.Notification, error) {
	const query = `
SELECT
    id,
    chat_id,
    type,
    pair,
    direction,
    profit_diff,
    notional,
    op_hash,
    created_at
FROM notifications
WHERE chat_id = $1
ORDER BY created_at DESC
LIMIT $2;
`

	rows, err := r.q.Query(ctx, query, chatID, limit)
	if err != nil {
		r.log.Error("notification_list_query_failed", "chat_id", chatID, "err", err)
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Notification

	for rows.Next() {
		var n domain.Notification
		var t string
		var p string

		if err := rows.Scan(
			&n.ID,
			&n.ChatID,
			&t,
			&p,
			&n.Direction,
			&n.ProfitDiff,
			&n.Notional,
			&n.OpHash,
			&n.CreatedAt,
		); err != nil {
			r.log.Error("notification_list_scan_failed", "chat_id", chatID, "err", err)
			return nil, err
		}

		n.Type = domain.ArbitrageType(t)
		n.Pair = domain.Pair(p)

		result = append(result, &n)
	}

	if err := rows.Err(); err != nil {
		r.log.Error("notification_list_rows_err", "chat_id", chatID, "err", err)
		return nil, err
	}

	return result, nil
}
