## 1. Идея проекта

Telegram-бот помогает находить **арбитражные возможности между криптобиржами** (на старте — Rapira и Grinex) и почти мгновенно уведомляет пользователя, когда в стаканах появляется профитная ситуация.

Основные требования:

- Минимальная задержка между изменением стакана и уведомлением в Telegram.
- Поддержка разных типов ситуаций:
    - **Фактическая ситуация** — top-1 против top-1 (моментальный арбитраж).
    - **Потенциальная ситуация** — top-1 против более глубоких уровней (2–5) с учётом объёма.
    - **Обратная потенциальная ситуация** — зеркальная комбинация (покупка/продажа меняются местами).
- Для пользователя «Потенциальная» и «Обратная потенциальная» объединены в один тип.
- Персональные настройки порога профита и максимальной суммы сделки.
- Возможность масштабировать по сервисам и биржам.

---

## 2. Технологический стек

- **Язык:** Go
- **Монорепа:** один `go.mod` на корне.
- **Сервисы:**
    - `analyser` — поиск арбитража и бизнес-логика.
    - `rapira-gw` — шлюз к Rapira API.
    - `grinex-gw` — шлюз к Grinex API.
    - `tg-bot` — Telegram-бот, фронт для пользователя.
- **Хранилище:** PostgreSQL.
- **Кэш / быстрые данные:** Redis.
- **Взаимодействие сервисов:**
    - `rapira-gw` / `grinex-gw` → `analyser`: Redis-streaming.
    - `analyser` → `tg-bot`: GRPc (push сигнала в бот).
- **Контейнеризация:** Docker + docker-compose.

---

## 3. Структура репозитория

```
arbitrage-bot/
  go.mod
  go.sum

  cmd/
    analyser/
      main.go
    rapira-gw/
      main.go
    grinex-gw/
      main.go
    tg-bot/
      main.go

  internal/
    analyser/
      app/        # wire, запуск сервиса
      domain/     # сущности анализатора
      usecase/    # детект арбитража и бизнес-правила
      adapters/   # postgres, redis, http/grpc-хендлеры и клиенты
    rapira/
      app/
      adapters/   # Rapira HTTP client, Redis cache, push в analyser
    grinex/
      app/
      adapters/
    tgbot/
      app/
      adapters/   # Telegram API, internal HTTP server, клиент к analyser
    shared/
      domain/     # общие типы: Order, OrderBook, Opportunity, Pair, Source, User, Settings
      logger/     # slog-логгер
      config/     # загрузка конфигов
      postgres/   # базовый pgx-пул
      redis/      # базовый redis-клиент
      grpc/       # общие gRPC-обёртки / сгенерированный код

  proto/
    exchange.proto   # контракт gw <-> analyser
    analyser.proto   # (опционально) контракт analyser <-> другие сервисы

  migrations/
    analyser/        # SQL-схема БД анализатора

  deploy/
    docker-compose.yml
    Dockerfile.analyser
    Dockerfile.rapira-gw
    Dockerfile.grinex-gw
    Dockerfile.tg-bot

  .env
  README.md
  ARCHITECTURE.md

```

---

## 4. Высокоуровневая архитектура и поток данных

### 4.1. Сервисы и роли

**Rapira GW**

- Авторизуется в Rapira API.
- Регулярно опрашивает `/market/exchange-plate-mini` по нужным символам.
- Парсит стаканы (цена, объём, накопительный объём **Всего USDT**).
- Кэширует свежие стаканы в Redis.
- При изменении стакана пушит `OrderBookUpdate` в `analyser`
(через gRPC-stream или HTTP POST).

**Grinex GW**

- Аналогичная роль для Grinex (после появления API).

**Analyser**

- Принимает `OrderBookUpdate` от gw.
- Хранит последние стаканы в памяти.
- На каждый апдейт:
    - собирает snapshot по всем нужным парам/биржам;
    - считает **RawOpportunity** (арбитражные ситуации без привязки к пользователям):
        - fact (top-1 vs top-1),
        - potential (top-1 vs уровни 2–5, с учётом объёма и накопительного «Всего USDT»),
        - reverse-potential.
- Для каждой `RawOpportunity`:
    - находит пользователей с включённым анализом по этой паре;
    - фильтрует по их настройкам (min spread, max notional, тип ситуаций);
    - проверяет антиспам в Redis;
    - создаёт записи `arbitrage_signals` в PostgreSQL;
    - пушит сигнал в `tg-bot` через внутренний HTTP-эндпоинт.

**Telegram Bot**

- Принимает команды пользователя (`/start`, «Изменить параметры», «Начать анализ», «Остановить анализ»).
- Через HTTP API `analyser` регистрирует пользователя и обновляет его настройки.
- Принимает внутренний POST `/internal/send-signal` от `analyser` и отправляет сообщения в Telegram.

---

### 4.2. Поток одного «тика»

1. `rapira-gw` опрашивает стакан `USDT/RUB` у Rapira.
2. Если стакан изменился:
    - сохраняет его в Redis;
    - отправляет `OrderBookUpdate` в `analyser`.
3. `analyser` обновляет in-memory кэш стаканов.
4. Используя новый snapshot:
    - считает все возможные `RawOpportunity` для комбинаций Rapira↔Rapira, Grinex↔Grinex, Rapira↔Grinex.
5. Для каждой `RawOpportunity`:
    - берёт настройки всех пользователей по этой паре;
    - фильтрует по:
        - типу ситуаций (fact / potential / both),
        - `min_spread_abs`,
        - `max_notional` (только для potential),
        - общему флагу `enabled`.
    - для прошедших пользователей:
        - проверяет Redis-ключ `arb:seen:<userID>:<hash>`;
        - если новый — пишет сигнал в PG и пушит в `tg-bot`.
6. `tg-bot` отправляет сообщение пользователю.

---

## 5. Логика арбитража: fact vs potential

### 5.1. Стакан Rapira (пример)

У Rapira в стакане есть три важных колонки:

- **Цена RUB** — цена сделки.
- **Объём USDT** — объём на данном уровне.
- **Всего USDT** — накопительный объём в USDT (сумма объёмов от top-1 до текущего уровня).

Для потенциальных ситуаций используем именно колонку **«Всего USDT»**, чтобы понимать, сколько USDT нужно, чтобы «пройти» все уровни до текущего.

### 5.2. Фактическая ситуация (Fact)

- Берём `top-1 ask` и `top-1 bid` между нужными биржами/парами.
- Считаем эффективные цены (с учётом комиссий).
- Вычисляем профит на единицу.
- Детектор решает, что есть факт арбитража (по тех.порогу, например `profit >= 0`).
- Для пользователя факт-ситуация считается подходящей, если:
    - `fact_enabled = true` в настройках,
    - `profit >= user.MinSpreadAbs`.

**`max_notional` пользователя в fact-сценариях не используется.**

### 5.3. Потенциальная ситуация (Potential + Reverse Potential)

- Фиксируем `top-1 ask` и идём по bid-уровням (или наоборот для reverse).
- На каждом уровне:
    - берём `Всего USDT = cumulative_usdt` (накопительный объём),
    - считаем профит.
- На уровне с индексом `i` ситуация считается потенциальной, если:

```
profit >= minDiff_global               // тех.порог для детектора
cumulative_usdt <= maxNotional_global  // глобальный потолок глубины поиска

```

- Для каждого пользователя с `potential_enabled = true` дополнительно:

```
cumulative_usdt <= user.MaxNotional    // условие, что его лимит хватает на эти уровни
profit >= user.MinSpreadAbs

```

Если `cumulative_usdt > user.MaxNotional`, для этого пользователя ситуация перестаёт быть потенциальной — он не готов заходить на такой объём.

Итого:

- **Глобальный `maxNotional_global`** ограничивает глубину детектора.
- **Пользовательский `MaxNotional`** фильтрует уже найденные потенциальные ситуации по его рисковому лимиту.

---

## 6. PostgreSQL: схема БД

### 6.1. `users`

```sql
users (
  id           bigserial primary key,
  telegram_id  bigint not null unique,
  username     text,
  created_at   timestamptz not null default now()
);

```

### 6.2. `exchanges`

```sql
exchanges (
  id    smallserial primary key,
  code  text not null unique,  -- 'RAPIRA', 'GRINEX'
  name  text not null
);

```

### 6.3. `pairs`

```sql
pairs (
  id              bigserial primary key,
  exchange_id     smallint not null references exchanges(id),
  symbol          text not null,        -- 'USDT/RUB'
  base_currency   text not null,        -- 'RUB'
  quote_currency  text not null,        -- 'USDT'
  fee             numeric(10,6) not null,
  min_volume      numeric(30,10) not null,
  min_turnover    numeric(30,10) not null,
  coin_scale      smallint not null,
  base_coin_scale smallint not null,
  active          boolean not null default true,

  unique(exchange_id, symbol)
);

```

### 6.4. `user_pair_settings`

```sql
user_pair_settings (
  id                 bigserial primary key,
  user_id            bigint not null references users(id) on delete cascade,
  pair_id            bigint not null references pairs(id),

  fact_enabled       boolean not null default false,
  potential_enabled  boolean not null default false, -- включает и обратный потенциал

  min_spread_abs     numeric(18,8) not null,         -- минимальный профит в валюте (RUB)
  min_spread_pct     numeric(18,8) not null default 0,
  max_notional       numeric(30,10),                 -- max сумма сделки в USDT (только potential)

  enabled            boolean not null default false, -- анализ запущен / остановлен

  created_at         timestamptz not null default now(),
  updated_at         timestamptz not null default now(),

  unique(user_id, pair_id)
);

```

Маппинг выбора:

- «Фактическая» → `fact_enabled = true`, `potential_enabled = false`.
- «Потенциальная» → `fact_enabled = false`, `potential_enabled = true`.
- «Оба варианта» → `fact_enabled = true`, `potential_enabled = true`.

### 6.5. `arbitrage_signals`

```sql
arbitrage_signals (
  id               bigserial primary key,

  user_id          bigint not null references users(id),
  pair_id          bigint not null references pairs(id),
  buy_exchange_id  smallint not null references exchanges(id),
  sell_exchange_id smallint not null references exchanges(id),

  kind             smallint not null,     -- 0=fact, 1=potential, 2=reverse_potential
  buy_price        numeric(30,10) not null,
  sell_price       numeric(30,10) not null,
  amount           numeric(30,10) not null,
  profit_abs       numeric(30,10) not null,
  profit_pct       numeric(18,8) not null,

  status           smallint not null default 0,  -- 0=new,1=sent,2=skipped,3=failed
  created_at       timestamptz not null default now(),
  sent_at          timestamptz
);

create index idx_signals_user_created on arbitrage_signals(user_id, created_at desc);
create index idx_signals_status       on arbitrage_signals(status, created_at);

```

---

## 7. Redis

### 7.1. Кэш стаканов

- Ключ: `orderbook:<exchange>:<symbol>`
- Значение: сериализованный `OrderBook` (включая `Sum = Всего USDT`).
- TTL: 1–3 секунды.

### 7.2. Дедуп сигналов

- Ключ: `arb:seen:<userID>:<hash>`
- Значение: `"1"`
- TTL: 10–60 минут.

### 7.3. Rate-limit (опционально)

- `rl:rapira:exchange-plate-mini:sec`, `rl:rapira:exchange-plate-mini:min` и т.п.

---

## 8. Domain-структуры (Go, кратко)

```go
type Source int
const (
    SourceUnknown Source = iota
    SourceRapira
    SourceGrinex
)

type Order struct {
    Price  float64
    Amount float64
    Sum    float64 // cumulative (Всего USDT)
}

type OrderBook struct {
    Source Source
    Pair   Pair
    Bids   []*Order
    Asks   []*Order
    TS     time.Time
}

type OpportunityKind int
const (
    OpportunityFact OpportunityKind = iota
    OpportunityPotential
    OpportunityReversePotential
)

type RawOpportunity struct {
    Kind         OpportunityKind
    Pair         Pair
    BuyExchange  Source
    SellExchange Source
    BuyPrice     float64
    SellPrice    float64
    Amount       float64
    ProfitAbs    float64
    ProfitPct    float64
    CreatedAt    time.Time
}

type UserPairSettings struct {
    ID               int64
    UserID           int64
    PairID           int64
    FactEnabled      bool
    PotentialEnabled bool
    MinSpreadAbs     float64
    MinSpreadPct     float64
    MaxNotional      float64
    Enabled          bool
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

```

---

## 9. Интерфейс Telegram-бота (UX)

- `/start` или «Изменить параметры»:
    - бот показывает текущие параметры (опционально),
    - спрашивает тип ситуаций,
    - просит ввести пороги (для fact — одно число, для potential/оба — два),
    - после сохранения настроек показывает кнопку «Начать анализ».
- «Начать анализ»:
    - включает `enabled = true` в `user_pair_settings`,
    - сигнализирует пользователю о запуске,
    - показывает кнопки «Остановить анализ» и «Изменить параметры».
- «Остановить анализ»:
    - выключает `enabled = false`,
    - сообщает об остановке,
    - возвращает кнопки «Начать анализ» / «Изменить параметры».

---

## 10. Резюме

- Архитектура event-driven: биржевые gw пушат стаканы → `analyser` считает рыночные ситуации → фильтрует под пользователей → пушит в `tg-bot`.
- Fact-сценарии используют только top-1 и `min_spread_abs`.
- Potential-сценарии учитывают накопительный объём «Всего USDT» и лимиты `max_notional`.
- Redis нужен для горячих данных и антиспама, PostgreSQL — для пользователей, настроек и истории сигналов.
---

## 11. ����������� �����

���������� ����������� ������ ����������� � ������ �� ��� ��������� �:

- `docs/LOAD_TESTING.md`

������� ������ �������� 2026-02-18. ���� ���� ����� ��������� ����� ������ �������� ����������� � `internal/analyser/usecase`.
