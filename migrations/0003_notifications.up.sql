CREATE TABLE notifications (
    id           BIGSERIAL PRIMARY KEY,
    chat_id      BIGINT       NOT NULL,
    type         TEXT         NOT NULL,  -- fact / potential
    pair         TEXT         NOT NULL,  -- USDT/RUB, USDT/A7A5
    direction    TEXT         NOT NULL,  -- buy_rapira_sell_grinex и т.п.
    profit_diff  NUMERIC(18, 8),
    notional     NUMERIC(18, 8),
    op_hash      TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_chat_created
    ON notifications (chat_id, created_at DESC);

CREATE INDEX idx_notifications_chat_op_hash
    ON notifications (chat_id, op_hash);
