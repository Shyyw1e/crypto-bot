package middleware

import (
	"context"
	"net/http"

	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type AuthTransport struct {
	base *http.Transport
	tm   *TokenManager
	log  logger.Logger
}

func NewAuthTransport(base *http.Transport, tm *TokenManager, log logger.Logger) *AuthTransport {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport)
	}
	return &AuthTransport{
		base: base,
		tm:   tm,
		log:  log,
	}
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	token, err := t.tm.EnsureToken(ctx)
	if err != nil {
		t.log.Error("rapira_auth_roundtrip_ensure_token_failed", "err", err)
		return nil, err
	}

	req2 := req.Clone(context.Background())
	req2 = req2.WithContext(ctx)

	req2.Header = req.Header.Clone()
	req2.Header.Set("Authorization", "Bearer "+token)

	return t.base.RoundTrip(req2)
}

// NewRapiraHTTPClient — http.Client, который автоматически
// подставляет Bearer-токен в каждый запрос к Rapira API.
func NewRapiraHTTPClient(tm *TokenManager, log logger.Logger) *http.Client {
	baseTransport := http.DefaultTransport.(*http.Transport)

	return &http.Client{
		Transport: NewAuthTransport(baseTransport, tm, log),
		Timeout: tm.HTTPClient.Timeout,
	}
}
