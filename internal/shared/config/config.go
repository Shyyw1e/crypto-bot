package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	ServiceName string
	Env         string
	LogLevel    string
}

type HTTPConfig struct {
	Port              int
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
}

type PostgresConfig struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

type RedisConfig struct {
	Addr     string
	DB       int
	Password string
}

type NatsConfig struct {
	URL string
}

type ArbitrageConfig struct {
	MinDiffGlobal    float64
	MaxNotionalGlobal float64
	DedupTTL         time.Duration
	OrderbookDepth   int
}

type RapiraConfig struct {
    BaseURL          string        // RAPIRA_API_BASE_URL
    APIKeyKID        string        // RAPIRA_API_KEY_KID
    PrivateKeyBase64 string        // RAPIRA_JWT_PRIVATE_KEY

    PollIntervalMs   int           // RAPIRA_POLL_INTERVAL_MS
    PollInterval     time.Duration // производное поле

    SymbolsRaw       string        // RAPIRA_SYMBOLS
    Symbols          []string      // производное поле

    ClientJWTTTL     time.Duration // например, 1h (по умолчанию)
    RefreshMargin    time.Duration // например, 5-10 минут до exp
}

type GrinexConfig struct {
	BaseURL      string
	PollInterval time.Duration
	Symbols      []string
}


type TelegramConfig struct {
	BotToken            string
	AnalyserAddr        string		  // gRPC-адрес analyser ()
	Addr				string        // gRPC-адрес tg-bot (TGBOT_GRPC_ADDR)
	NotificationTimeout time.Duration // таймаут на отправку уведомления в tg-bot
}


type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Nats     NatsConfig
	Arb      ArbitrageConfig
	Rapira   RapiraConfig
	Grinex   GrinexConfig
	Telegram TelegramConfig
}

func MustLoad(serviceName string) *Config {
	cfg, err := Load(serviceName)
	if err != nil {
		panic(fmt.Errorf("config load failed: %w", err))
	}
	return cfg
}

func Load(serviceName string) (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	loadAppConfig(cfg)
	if err := loadHTTPConfig(cfg, serviceName); err != nil {
		return nil, err
	}
	loadPostgresConfig(cfg)
	if err := loadRedisConfig(cfg); err != nil {
		return nil, err
	}
	loadNatsConfig(cfg)
	if err := loadArbitrageConfig(cfg); err != nil {
		return nil, err
	}
	if err := loadRapiraConfig(cfg); err != nil {
		return nil, err
	}
	if err := loadGrinexConfig(cfg); err != nil {
		return nil, err
	}
	loadTelegramConfig(cfg)

	if err := validateConfig(cfg, serviceName); err != nil {
		return nil, err
	}

	cfg.App.ServiceName = serviceName

	return cfg, nil
}

func loadAppConfig(cfg *Config) {
	env := getEnv("APP_ENV", "local")
	logLevel := getEnv("LOG_LEVEL", "INFO")

	cfg.App = AppConfig{
		Env:      env,
		LogLevel: strings.ToUpper(logLevel),
	}
}

func loadHTTPConfig(cfg *Config, serviceName string) error {
	var portEnv string

	switch serviceName {
	case "analyser":
		portEnv = "ANALYSER_HTTP_PORT"
	case "rapira-gw":
		portEnv = "RAPIRA_GW_HTTP_PORT"
	case "grinex-gw":
		portEnv = "GRINEX_GW_HTTP_PORT"
	case "tg-bot":
		portEnv = "TGBOT_HTTP_PORT"
	default:
		return nil
	}

	port, err := getIntEnv(portEnv, 0)
	if err != nil {
		return fmt.Errorf("parse %s: %w", portEnv, err)
	}

	reqTimeoutMs, err := getIntEnv("REQUEST_TIMEOUT_MS", 300)
	if err != nil {
		return fmt.Errorf("parse REQUEST_TIMEOUT_MS: %w", err)
	}
	readHeaderMs, err := getIntEnv("READ_HEADER_TIMEOUT_MS", 100)
	if err != nil {
		return fmt.Errorf("parse READ_HEADER_TIMEOUT_MS: %w", err)
	}

	cfg.HTTP = HTTPConfig{
		Port:              port,
		RequestTimeout:    time.Duration(reqTimeoutMs) * time.Millisecond,
		ReadHeaderTimeout: time.Duration(readHeaderMs) * time.Millisecond,
	}
	return nil
}

func loadPostgresConfig(cfg *Config) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		host := getEnv("DB_HOST", "")
		port := getEnv("DB_PORT", "")
		user := getEnv("DB_USER", "")
		name := getEnv("DB_NAME", "")
		pass := getEnv("DB_PASSWORD", "")
		if host != "" && port != "" && user != "" && name != "" {
			dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
		}
	}

	cfg.Postgres = PostgresConfig{
		DSN:          dsn,
		MaxOpenConns: 20,
		MaxIdleConns: 10,
	}
}

func loadRedisConfig(cfg *Config) error {
	addr := getEnv("REDIS_ADDR", "")
	if addr == "" {
		return nil
	}

	db, err := getIntEnv("REDIS_DB", 0)
	if err != nil {
		return fmt.Errorf("parse REDIS_DB: %w", err)
	}
	password := os.Getenv("REDIS_PASSWORD")

	cfg.Redis = RedisConfig{
		Addr:     addr,
		DB:       db,
		Password: password,
	}
	return nil
}



func loadNatsConfig(cfg *Config) {
	cfg.Nats = NatsConfig{
		URL: os.Getenv("NATS_URL"),
	}
}

func loadArbitrageConfig(cfg *Config) error {
	minDiff, err := getFloatEnv("ARBITRAGE_MIN_DIFF_GLOBAL", 0.0)
	if err != nil {
		return fmt.Errorf("parse ARBITRAGE_MIN_DIFF_GLOBAL: %w", err)
	}
	maxNotional, err := getFloatEnv("ARBITRAGE_MAX_NOTIONAL_GLOBAL", 5000.0)
	if err != nil {
		return fmt.Errorf("parse ARBITRAGE_MAX_NOTIONAL_GLOBAL: %w", err)
	}
	dedupTTLSeconds, err := getIntEnv("ARBITRAGE_DEDUP_TTL_SECONDS", 1800)
	if err != nil {
		return fmt.Errorf("parse ARBITRAGE_DEDUP_TTL_SECONDS: %w", err)
	}
	orderbookDepth, err := getIntEnv("ORDERBOOK_DEPTH", 5)
	if err != nil {
		return fmt.Errorf("parse ORDERBOOK_DEPTH: %w", err)
	}

	cfg.Arb = ArbitrageConfig{
		MinDiffGlobal:    minDiff,
		MaxNotionalGlobal: maxNotional,
		DedupTTL:         time.Duration(dedupTTLSeconds) * time.Second,
		OrderbookDepth:   orderbookDepth,
	}
	return nil
}

func loadRapiraConfig(cfg *Config) error {
	baseURL := os.Getenv("RAPIRA_API_BASE_URL")
	apiKeyKID := os.Getenv("RAPIRA_API_KEY_KID")
	privateKey := os.Getenv("RAPIRA_JWT_PRIVATE_KEY")

	pollIntervalMs, err := getIntEnv("RAPIRA_POLL_INTERVAL_MS", 500)
	if err != nil {
		return fmt.Errorf("parse RAPIRA_POLL_INTERVAL_MS: %w", err)
	}

	symbolsRaw := os.Getenv("RAPIRA_SYMBOLS")
	var symbols []string
	if symbolsRaw != "" {
		for _, s := range strings.Split(symbolsRaw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}
	refreshMargin, err := getIntEnv("RAPIRA_REFRESH_MARGIN_MIN", 5)
	if err != nil {
		return fmt.Errorf("parse RAPIRA_REFRESH_MARGIN_MIN: %w", err)
	}

	cfg.Rapira = RapiraConfig{
		BaseURL:        	baseURL,
		APIKeyKID:      	apiKeyKID,
		PrivateKeyBase64: 	privateKey,
		PollInterval:   	time.Duration(pollIntervalMs) * time.Millisecond,
		Symbols:        	symbols,
		RefreshMargin: 		time.Duration(refreshMargin) * time.Minute,
		ClientJWTTTL: 		time.Hour,
	}	
	return nil
}

func loadGrinexConfig(cfg *Config) error {
	baseURL := strings.TrimRight(getEnv("GRINEX_API_BASE_URL", "https://grinex.io/api/v1"), "/")

	pollIntervalMs, err := getIntEnv("GRINEX_POLL_INTERVAL_MS", 500)
	if err != nil {
		return fmt.Errorf("parse GRINEX_POLL_INTERVAL_MS: %w", err)
	}

	symbolsRaw := os.Getenv("GRINEX_SYMBOLS")
	var symbols []string
	if symbolsRaw != "" {
		for _, s := range strings.Split(symbolsRaw, ",") {
			s = strings.TrimSpace(strings.ToLower(s))
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}

	cfg.Grinex = GrinexConfig{
		BaseURL:      baseURL,
		PollInterval: time.Duration(pollIntervalMs) * time.Millisecond,
		Symbols:      symbols,
	}
	return nil
}


func loadTelegramConfig(cfg *Config) {
	cfg.Telegram = TelegramConfig{
		BotToken:        os.Getenv("TG_BOT_TOKEN"),
		Addr: os.Getenv("TGBOT_GRPC_ADDR"),
		AnalyserAddr: os.Getenv("ANALYSER_GRPC_ADDR"),
		NotificationTimeout: time.Duration(2000) * time.Millisecond,
	}
}

func validateConfig(cfg *Config, serviceName string) error {
	if cfg.Redis.Addr == "" {
		return errors.New("REDIS_ADDR is required")
	}

	switch serviceName {
	case "analyser":
		if cfg.HTTP.Port == 0 {
			return errors.New("ANALYSER_HTTP_PORT is required")
		}
		if cfg.Postgres.DSN == "" {
			return errors.New("POSTGRES_DSN or DB_* env vars are required for analyser")
		}
		if cfg.Arb.OrderbookDepth <= 0 {
			return errors.New("ORDERBOOK_DEPTH must be > 0")
		}
		if cfg.Telegram.Addr == "" {
			return errors.New("TGBOT_GRPC_ADDR is required for analyser")
		}

	case "rapira-gw":
		if cfg.HTTP.Port == 0 {
			return errors.New("RAPIRA_GW_HTTP_PORT is required")
		}
		if cfg.Rapira.BaseURL == "" {
			return errors.New("RAPIRA_API_BASE_URL is required for rapira-gw")
		}
		if cfg.Rapira.APIKeyKID == "" {
			return errors.New("RAPIRA_API_KEY_KID is required for rapira-gw")
		}
		if cfg.Rapira.PrivateKeyBase64 == "" {
			return errors.New("RAPIRA_JWT_PRIVATE_KEY is required for rapira-gw")
		}
		if len(cfg.Rapira.Symbols) == 0 {
			return errors.New("RAPIRA_SYMBOLS must contain at least one symbol for rapira-gw")
		}

	case "tg-bot":
		if cfg.HTTP.Port == 0 {
			return errors.New("TGBOT_HTTP_PORT is required")
		}
		if cfg.Telegram.BotToken == "" {
			return errors.New("TG_BOT_TOKEN is required for tg-bot")
		}
		if cfg.Telegram.AnalyserAddr == "" {
			return errors.New("ANALYSER_GRPC_ADDR is required for tg-bot")
		}

	case "grinex-gw":
	if cfg.HTTP.Port == 0 {
		return errors.New("GRINEX_GW_HTTP_PORT is required")
	}
	if cfg.Grinex.BaseURL == "" {
		return errors.New("GRINEX_API_BASE_URL is required for grinex-gw")
	}
	if len(cfg.Grinex.Symbols) == 0 {
		return errors.New("GRINEX_SYMBOLS must contain at least one symbol for grinex-gw")
	}

	// Временно поддерживаем только usdta7a5, пока usdt_rub недоступен на бирже.
	for _, s := range cfg.Grinex.Symbols {
		if s != "usdta7a5" {
			return fmt.Errorf("unsupported grinex symbol for now: %s (expected only usdta7a5)", s)
		}
	}

	default:
	}

	return nil
}



func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func getFloatEnv(key string, def float64) (float64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}
