package rapiraapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

// ErrUnexpectedStatus возвращается, если Rapira ответила не 200 OK.
var ErrUnexpectedStatus = errors.New("rapira: unexpected status code")

// Client — HTTP-клиент Rapira API.
type Client struct {
	baseURL string
	http    *http.Client
	log     logger.Logger
}

// NewClient создаёт Rapira-клиент.
// Ожидается, что httpClient уже обёрнут auth-middleware (Authorization: Bearer ...).
func NewClient(cfg config.RapiraConfig, httpClient *http.Client, log logger.Logger) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.rapira.net"
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		baseURL: base,
		http:    httpClient,
		log:     log,
	}
}

// GetMarketDepth /market/exchange-plate-mini — получить стакан по symbol.
func (c *Client) GetMarketDepth(ctx context.Context, symbol string) (*PlateResponse, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("rapira: empty symbol")
	}

	url := c.baseURL + "/market/exchange-plate-mini"

	// multipart/form-data body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("symbol", symbol); err != nil {
		c.log.Error("rapira_client_write_field_failed", "err", err)
		return nil, fmt.Errorf("rapira: write form field: %w", err)
	}
	if err := writer.Close(); err != nil {
		c.log.Error("rapira_client_writer_close_failed", "err", err)
		return nil, fmt.Errorf("rapira: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		c.log.Error("rapira_client_new_request_failed", "err", err)
		return nil, fmt.Errorf("rapira: new request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			c.log.Warn("rapira_client_request_context_error", "ctx_err", ctx.Err())
			return nil, ctx.Err()
		}
		c.log.Error("rapira_client_do_failed", "err", err)
		return nil, fmt.Errorf("rapira: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.log.Error("rapira_client_unexpected_status",
			"status", resp.StatusCode)
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var pr PlateResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		c.log.Error("rapira_client_decode_failed", "err", err)
		return nil, fmt.Errorf("rapira: decode response: %w", err)
	}

	if isEmptyPlate(pr) {
		c.log.Warn("rapira_client_empty_orderbook", "symbol", symbol)
		return nil, ErrEmptyBook
	}

	return &pr, nil
}

// isEmptyPlate проверяет кейс, когда Rapira вернула {}.
func isEmptyPlate(pr PlateResponse) bool {
	return pr.Ask.Symbol == "" &&
		len(pr.Ask.Items) == 0 &&
		pr.Bid.Symbol == "" &&
		len(pr.Bid.Items) == 0
}
