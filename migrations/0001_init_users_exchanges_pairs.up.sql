-- 001_init_users_exchanges_pairs.up.sql

CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    chat_id      BIGINT NOT NULL UNIQUE,
    telegram_id  BIGINT UNIQUE,              -- ⚠️ БОЛЬШЕ НЕ NOT NULL
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
CREATE UNIQUE INDEX users_chat_id_uq ON users(chat_id);

--------------------------------------------------------------------
-- ДАЛЬШЕ — СИДИНГ ОБЯЗАТЕЛЬНЫХ ДАННЫХ
--------------------------------------------------------------------

-- 1. Биржи
INSERT INTO exchanges (code, name)
VALUES
    ('RAPIRA', 'Rapira Exchange'),
    ('GRINEX', 'Grinex Exchange')
ON CONFLICT (code) DO NOTHING;

-- 2. Пара USDT/RUB для RAPIRA
WITH rapira AS (
    SELECT id FROM exchanges WHERE code = 'RAPIRA'
)
INSERT INTO pairs (
    exchange_id,
    symbol,
    base_currency,
    quote_currency,
    fee,
    min_volume,
    min_turnover,
    coin_scale,
    base_coin_scale,
    active
)
SELECT
    rapira.id,
    'USDT/RUB',   -- ⚠️ именно такой символ, как ждёт analyser
    'RUB',        -- base
    'USDT',       -- quote
    0.001,        -- 0.1% комиссия, при необходимости поправишь
    0.01,         -- минимальный объём USDT
    100,          -- минимальный оборот в RUB
    2,            -- scale USDT
    2,            -- scale RUB
    TRUE
FROM rapira
ON CONFLICT (exchange_id, symbol) DO NOTHING;
