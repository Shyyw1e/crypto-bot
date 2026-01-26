package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type TokenManager struct {
	log        logger.Logger
	privateKey *rsa.PrivateKey

	issuer   string
	audience string
	kid      string
	ttl      time.Duration

	HTTPClient *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// NewTokenManager:
//
// privateKeyBase64 — base64 от ПОЛНОГО PEM-блока с ключом (как в RAPIRA_JWT_PRIVATE_KEY).
// issuer / audience — что просит Rapira в iss/aud (если нет требований, можно одинаково).
// kid — RAPIRA_API_KEY_KID (кладём в header).
// ttl — сколько живёт токен.
func NewTokenManager(
	log logger.Logger,
	privateKeyBase64 string,
	issuer string,
	audience string,
	kid string,
	ttl time.Duration,
	baseHTTP *http.Client,
) (*TokenManager, error) {
	log = log.With("component", "token_manager")

	rawPEM, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		log.Error("middleware_auth_newTM_base64_decode", "err", err)
		return nil, fmt.Errorf("base64 decode private key: %w", err)
	}

	block, _ := pem.Decode(rawPEM)
	if block == nil {
		log.Error("middleware_auth_newTM_pem_decode_nil")
		return nil, fmt.Errorf("pem decode private key: no pem block found")
	}

	var rsaKey *rsa.PrivateKey

	switch block.Type {
	case "RSA PRIVATE KEY":
		// Сначала пробуем PKCS1.
		rsaKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			// Если вдруг это PKCS8 в RSA-блоке — попробуем и его.
			parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				log.Error("middleware_auth_newTM_parse_pkcs1_pkcs8_failed",
					"err_pkcs1", err, "err_pkcs8", err2)
				return nil, fmt.Errorf("parse rsa private key: pkcs1=%v; pkcs8=%v", err, err2)
			}
			var ok bool
			rsaKey, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("pkcs8 private key is not RSA: %T", parsed)
			}
		}
	case "PRIVATE KEY":
		// Классический PKCS8.
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			log.Error("middleware_auth_newTM_parse_pkcs8", "err", err)
			return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
		}
		rsaKeyTyped, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			log.Error("middleware_auth_newTM_pkcs8_not_rsa", "type", fmt.Sprintf("%T", parsed))
			return nil, fmt.Errorf("pkcs8 private key is not RSA: %T", parsed)
		}
		rsaKey = rsaKeyTyped
	default:
		log.Error("middleware_auth_newTM_unsupported_type", "type", block.Type)
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	log.Info("middleware_auth_newTM_key_loaded",
		"key_type", block.Type,
		"bits", rsaKey.N.BitLen(),
	)

	if baseHTTP == nil {
		baseHTTP = &http.Client{Timeout: 10 * time.Second}
	}

	return &TokenManager{
		log:        log,
		privateKey: rsaKey,
		issuer:     issuer,
		audience:   audience,
		kid:        kid,
		ttl:        ttl,
		HTTPClient: baseHTTP,
	}, nil
}

// GenerateToken генерирует новый JWT RS256 с cache-friendly exp.
func (tm *TokenManager) GenerateToken(ctx context.Context, subject string) (string, time.Time, error) {
	_ = ctx

	now := time.Now()
	exp := now.Add(tm.ttl)

	claims := jwt.RegisteredClaims{
		Issuer:    tm.issuer,
		Subject:   subject,
		Audience:  jwt.ClaimStrings{tm.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if tm.kid != "" {
		t.Header["kid"] = tm.kid
	}

	signed, err := t.SignedString(tm.privateKey)
	if err != nil {
		tm.log.Error("token_manager_generate_sign_failed", "err", err)
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}

	return signed, exp, nil
}

// EnsureToken — возвращает актуальный токен, обновляя его при необходимости.
func (tm *TokenManager) EnsureToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	// небольшой запас, чтобы не истекал прямо во время запроса
	skew := 10 * time.Second

	if tm.token != "" && now.Before(tm.expiresAt.Add(-skew)) {
		return tm.token, nil
	}

	token, exp, err := tm.GenerateToken(ctx, "rapira-api")
	if err != nil {
		return "", err
	}

	tm.token = token
	tm.expiresAt = exp

	tm.log.Debug("token_manager_ensure_token_refreshed", "expires_at", exp)
	return tm.token, nil
}
