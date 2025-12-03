CREATE TABLE arbitrage_signals (
    id               BIGSERIAL PRIMARY KEY,

    user_id          BIGINT   NOT NULL REFERENCES users(id),
    pair_id          BIGINT   NOT NULL REFERENCES pairs(id),
    buy_exchange_id  SMALLINT NOT NULL REFERENCES exchanges(id),
    sell_exchange_id SMALLINT NOT NULL REFERENCES exchanges(id),

    kind             SMALLINT NOT NULL,          -- 0=fact,1=potential,2=reverse_potential
    buy_price        NUMERIC(30,10) NOT NULL,
    sell_price       NUMERIC(30,10) NOT NULL,
    amount           NUMERIC(30,10) NOT NULL,    -- объём USDT
    profit_abs       NUMERIC(30,10) NOT NULL,    -- профит в RUB
    profit_pct       NUMERIC(18,8)  NOT NULL,

    status           SMALLINT    NOT NULL DEFAULT 0,  -- 0=new,1=sent,2=skipped,3=failed
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at          TIMESTAMPTZ
);

CREATE INDEX idx_signals_user_created
    ON arbitrage_signals (user_id, created_at DESC);
CREATE INDEX idx_signals_status_created
    ON arbitrage_signals (status, created_at);
