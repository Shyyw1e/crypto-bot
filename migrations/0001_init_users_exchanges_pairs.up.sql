CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    telegram_id  BIGINT      NOT NULL UNIQUE,
    username     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE exchanges (
    id    SMALLSERIAL PRIMARY KEY,
    code  TEXT NOT NULL UNIQUE,  -- 'RAPIRA', 'GRINEX'
    name  TEXT NOT NULL
);

CREATE TABLE pairs (
    id              BIGSERIAL PRIMARY KEY,
    exchange_id     SMALLINT    NOT NULL REFERENCES exchanges(id),
    symbol          TEXT        NOT NULL,  -- 'USDT/RUB'
    base_currency   TEXT        NOT NULL,  -- 'RUB'
    quote_currency  TEXT        NOT NULL,  -- 'USDT'
    fee             NUMERIC(10,6)  NOT NULL,
    min_volume      NUMERIC(30,10) NOT NULL,
    min_turnover    NUMERIC(30,10) NOT NULL,
    coin_scale      SMALLINT       NOT NULL,
    base_coin_scale SMALLINT       NOT NULL,
    active          BOOLEAN        NOT NULL DEFAULT TRUE,

    UNIQUE (exchange_id, symbol)
);

-- индексы под запросы по символам
CREATE INDEX idx_pairs_exchange_symbol ON pairs(exchange_id, symbol);

