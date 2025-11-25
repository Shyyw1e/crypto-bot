
CREATE TABLE user_pair_settings (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pair_id            BIGINT  NOT NULL REFERENCES pairs(id),

    fact_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    potential_enabled  BOOLEAN NOT NULL DEFAULT FALSE,

    min_spread_abs     NUMERIC(18,8) NOT NULL,        -- RUB
    min_spread_pct     NUMERIC(18,8) NOT NULL DEFAULT 0,
    max_notional       NUMERIC(30,10),                -- USDT, только для potential

    enabled            BOOLEAN NOT NULL DEFAULT FALSE, -- анализ запущен/остановлен

    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, pair_id)
);

CREATE INDEX idx_user_pair_settings_user_enabled
    ON user_pair_settings (user_id, enabled);
CREATE INDEX idx_user_pair_settings_pair_enabled
    ON user_pair_settings (pair_id, enabled);

