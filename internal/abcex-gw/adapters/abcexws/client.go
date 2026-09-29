package abcexws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"

	"github.com/Shyyw1e/crypto-bot/internal/shared/config"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
)

type Client struct {
	wsURL        string
	pingInterval time.Duration
	log          logger.Logger
	nextID       atomic.Int64
	writeMu      sync.Mutex
}

type wsEnvelope struct {
	Method string          `json:"method,omitempty"`
	ID     int64           `json:"id,omitempty"`
	Stream string          `json:"stream,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type wsRequest struct {
	Method string   `json:"method"`
	ID     int64    `json:"id,omitempty"`
	Data   []string `json:"data,omitempty"`
}

func NewClient(cfg config.ABCEXConfig, log logger.Logger) *Client {
	pingInterval := cfg.PingInterval
	if pingInterval <= 0 {
		pingInterval = 25 * time.Second
	}

	return &Client{
		wsURL:        cfg.WSURL,
		pingInterval: pingInterval,
		log:          log.With("component", "abcex_ws_client"),
	}
}

func (c *Client) StreamOrderbook(
	ctx context.Context,
	symbol string,
	handle func(context.Context, *OrderbookMessage) error,
) error {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return fmt.Errorf("abcex: empty symbol")
	}
	if c.wsURL == "" {
		return fmt.Errorf("abcex: empty ws url")
	}

	wsCfg, err := websocket.NewConfig(c.wsURL, "https://abcex.io/")
	if err != nil {
		return fmt.Errorf("abcex: build websocket config: %w", err)
	}

	conn, err := websocket.DialConfig(wsCfg)
	if err != nil {
		return fmt.Errorf("abcex: dial websocket: %w", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	channel := symbol + "@orderbook"
	if err := c.writeJSON(ctx, conn, wsRequest{
		Method: "subscribe",
		ID:     c.nextRequestID(),
		Data:   []string{channel},
	}); err != nil {
		return err
	}

	c.log.Info("abcex_ws_subscribed", "symbol", symbol, "channel", channel)

	errCh := make(chan error, 1)
	go c.pingLoop(ctx, conn, errCh)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		default:
		}

		var raw string
		err := websocket.Message.Receive(conn, &raw)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("abcex: read websocket message: %w", err)
		}

		msg, ok, err := decodeOrderbookMessage([]byte(raw))
		if err != nil {
			c.log.Warn("abcex_ws_decode_failed", "err", err)
			continue
		}
		if !ok {
			continue
		}
		if err := handle(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn, errCh chan<- error) {
	t := time.NewTicker(c.pingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.writeJSON(ctx, conn, wsRequest{Method: "ping", ID: c.nextRequestID()}); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}
}

func (c *Client) writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("abcex: set websocket deadline: %w", err)
	}
	if err := websocket.JSON.Send(conn, v); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("abcex: write websocket json: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
	return nil
}

func (c *Client) nextRequestID() int64 {
	return c.nextID.Add(1)
}

func decodeOrderbookMessage(data []byte) (*OrderbookMessage, bool, error) {
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false, err
	}
	if env.Stream == "" || !strings.HasSuffix(env.Stream, "@orderbook") {
		return nil, false, nil
	}

	var ob OrderbookData
	if err := json.Unmarshal(env.Data, &ob); err != nil {
		return nil, false, err
	}
	if len(ob.Asks) == 0 || len(ob.Bids) == 0 {
		return nil, false, ErrEmptyBook
	}

	return &OrderbookMessage{Stream: env.Stream, Data: ob}, true, nil
}
