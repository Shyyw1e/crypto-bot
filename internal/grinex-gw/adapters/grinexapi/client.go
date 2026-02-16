package grinexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type Client struct {
	baseURL string
	http *http.Client
	log logger.Logger
}

func NewClient(cfg config.GrinexConfig, httpClient *http.Client, log logger.Logger) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://grinex.io/api/v1"
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{
		baseURL: base,
		http: httpClient,
		log: log,
	}
}

func (c *Client) GetDepth(ctx context.Context, symbol string) (*DepthResponse, error) {
	if strings.TrimSpace(symbol) == "" {
		return nil, fmt.Errorf("grinex: empty symbol")
	}

	u := fmt.Sprintf("%s/spot/depth?symbol=%s", c.baseURL, url.QueryEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("grinex: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("grinex: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("grinex: unexpected status: %d", resp.StatusCode)
	}

	var out DepthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("grinex: decode response: %w", err)
	}

	if len(out.Asks) == 0 || len(out.Bids) == 0 {
		return nil, ErrEmptyBook
	}

	return &out, nil
}