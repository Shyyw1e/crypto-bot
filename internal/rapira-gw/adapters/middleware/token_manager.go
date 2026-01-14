package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"

	jwt "github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
    cfg config.RapiraConfig
    log logger.Logger

    HTTPClient *http.Client
    PrivateKey *rsa.PrivateKey

    Token     string
    ExpiresAt time.Time

    mu sync.Mutex
}

func NewTokenManager(cfg config.RapiraConfig, log logger.Logger, httpClient *http.Client) (*TokenManager, error) {
    if cfg.BaseURL == "" {
        err := fmt.Errorf("RapiraConfig: empty BaseURL")
        log.Error("middleware_auth_newTM_not_valid_data", "err", err)
        return nil, err
    }
    if cfg.APIKeyKID == "" {
        err := fmt.Errorf("RapiraConfig: empty APIKeyKID")
        log.Error("middleware_auth_newTM_not_valid_data", "err", err)
        return nil, err
    }
    if cfg.PrivateKeyBase64 == "" {
        err := fmt.Errorf("RapiraConfig: empty PrivateKeyBase64")
        log.Error("middleware_auth_newTM_not_valid_data", "err", err)
        return nil, err
    }

    rawKey, err := base64.StdEncoding.DecodeString(cfg.PrivateKeyBase64)
    if err != nil {
        log.Error("middleware_auth_newTM_base64_decode", "err", err)
        return nil, fmt.Errorf("decode rapira private key base64: %w", err)
    }

    block, _ := pem.Decode(rawKey)
    if block == nil {
        err := fmt.Errorf("no PEM block found in private key")
        log.Error("middleware_auth_newTM_pem_decode", "err", err)
        return nil, err
    }

    var rsaKey *rsa.PrivateKey

    switch block.Type {
    case "RSA PRIVATE KEY":
        // PKCS#1
        k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
        if err != nil {
            log.Error("middleware_auth_newTM_parse_pkcs1", "err", err)
            return nil, fmt.Errorf("parse pkcs1 private key: %w", err)
        }
        rsaKey = k
    case "PRIVATE KEY":
        // PKCS#8
        k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
        if err != nil {
            log.Error("middleware_auth_newTM_parse_pkcs8", "err", err)
            return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
        }
        var ok bool
        rsaKey, ok = k.(*rsa.PrivateKey)
        if !ok {
            err := fmt.Errorf("parsed private key is not RSA")
            log.Error("middleware_auth_newTM_not_rsa_key", "err", err)
            return nil, err
        }
    default:
        err := fmt.Errorf("unsupported private key type: %s", block.Type)
        log.Error("middleware_auth_newTM_unsupported_key_type", "type", block.Type, "err", err)
        return nil, err
    }

    if httpClient == nil {
        httpClient = &http.Client{
            Timeout: 5 * time.Second,
        }
    }

    tm := &TokenManager{
        cfg:        cfg,
        log:        log,
        HTTPClient: httpClient,
        PrivateKey: rsaKey,
        Token:      "",
        ExpiresAt:  time.Time{},
    }

    return tm, nil
}

func (t *TokenManager) refreshToken(ctx context.Context) error {
    // мы вызываем refreshToken из EnsureToken под мьютексом,
    // поэтому здесь лок не нужен — просто работаем с полями

    // 1. exp для клиентского JWT (в секундах UNIX)
    exp := time.Now().Add(t.cfg.ClientJWTTTL).Unix()

    // 2. jti — случайная hex-строка (96–128 бит за глаза)
    randBytes := make([]byte, 12)
    if _, err := rand.Read(randBytes); err != nil {
        t.log.Error("middleware_auth_refresh_rand_failed", "err", err)
        return fmt.Errorf("generate jti: %w", err)
    }
    jti := strings.ToUpper(hex.EncodeToString(randBytes))

    // 3. Собираем claims для клиентского JWT
    claims := jwt.MapClaims{
        "exp": exp,
        "jti": jti,
    }

    // 4. Подписываем клиентский JWT приватным ключом (RS256)
    // Важно: jwt требует *rsa.PrivateKey, поэтому берём адрес
    clientJWT, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(&t.PrivateKey)
    if err != nil {
        t.log.Error("middleware_auth_refresh_sign_failed", "err", err)
        return fmt.Errorf("sign client jwt: %w", err)
    }

    // 5. Готовим body для /open/generate_jwt
    reqBody := struct {
        Kid      string `json:"kid"`
        JWToken  string `json:"jwt_token"`
    }{
        Kid:     t.cfg.APIKeyKID,
        JWToken: clientJWT,
    }

    bodyBytes, err := json.Marshal(reqBody)
    if err != nil {
        t.log.Error("middleware_auth_refresh_marshal_failed", "err", err)
        return fmt.Errorf("marshal generate_jwt body: %w", err)
    }

    // 6. Собираем URL
    url := strings.TrimRight(t.cfg.BaseURL, "/") + "/open/generate_jwt"

    // 7. HTTP-запрос
    client := t.HTTPClient
    if client == nil {
        client = http.DefaultClient
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
    if err != nil {
        t.log.Error("middleware_auth_refresh_new_request_failed", "err", err)
        return fmt.Errorf("build generate_jwt request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        t.log.Error("middleware_auth_refresh_http_failed", "err", err)
        return fmt.Errorf("call generate_jwt: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        // Для дебага имеет смысл залогировать тело
        b, _ := io.ReadAll(resp.Body)
        t.log.Error("middleware_auth_refresh_bad_status",
            "status", resp.StatusCode,
            "body", string(b),
        )
        return fmt.Errorf("generate_jwt returned status %d", resp.StatusCode)
    }

    // 8. Читаем ответ {"token": "..."}
    var res struct {
        Token string `json:"token"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
        t.log.Error("middleware_auth_refresh_decode_failed", "err", err)
        return fmt.Errorf("decode generate_jwt response: %w", err)
    }

    if res.Token == "" {
        err := fmt.Errorf("empty token in generate_jwt response")
        t.log.Error("middleware_auth_refresh_empty_token", "err", err)
        return err
    }

    // 9. Обновляем состояние менеджера
    now := time.Now()
    t.Token = res.Token

    // Если TTL токена Rapira заранее известен — задаём через cfg.ServerJWTTTL.
    if t.cfg.ClientJWTTTL > 0 {
        t.ExpiresAt = now.Add(t.cfg.ClientJWTTTL)
    } else {
        // fallback: почти сутки минус час, чтобы точно не протухнуть
        t.ExpiresAt = now.Add(23 * time.Hour)
    }

    t.log.Info("middleware_auth_refresh_success",
        "exp_at", t.ExpiresAt,
        "client_jwt_exp", exp,
    )

    return nil
}


func (t *TokenManager) EnsureToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	if t.Token == "" || !time.Now().Add(t.cfg.RefreshMargin).Before(t.ExpiresAt){
		if err := t.refreshToken(ctx); err != nil {
			t.mu.Unlock()
			t.log.Error("middleware_auth_ET_refresh", "err", err)
			return "", err
		}
	}
	token := t.Token
	t.mu.Unlock()

	return token, nil
}