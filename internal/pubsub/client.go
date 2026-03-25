package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/clarive/cla-worker-go/internal/retry"
	"github.com/clarive/cla-worker-go/internal/sse"
)

type Client struct {
	opts       Options
	sseClient  *sse.Client
	httpClient *http.Client
	logger     *slog.Logger
	connected  bool
	lastOID    string
	messages   chan Message
	done       chan struct{}
}

type RegisterResult struct {
	Token string `json:"token"`
	Error string `json:"-"`
	Msg   string `json:"msg"`
}

// UnmarshalJSON handles the server's polymorphic error field which can be
// a number (1), a string ("message"), or absent.
func (r *RegisterResult) UnmarshalJSON(data []byte) error {
	type Alias RegisterResult
	aux := &struct {
		*Alias
		Error json.RawMessage `json:"error"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Error != nil {
		s := string(aux.Error)
		// Remove quotes if it's a JSON string
		var str string
		if json.Unmarshal(aux.Error, &str) == nil {
			r.Error = str
		} else {
			// Numeric or boolean — use msg field as the error text
			if r.Msg != "" {
				r.Error = r.Msg
			} else {
				r.Error = s
			}
		}
	}
	return nil
}

func NewClient(opts ...Option) *Client {
	o := Options{
		Version:        "1.0.0",
		Logger:         slog.Default(),
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
		ReconnectDelay: 1 * time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Client{
		opts:       o,
		httpClient: o.HTTPClient,
		logger:     o.Logger,
		messages:   make(chan Message, 64),
		done:       make(chan struct{}),
	}
}

func (c *Client) address(path string) string {
	tagStr := strings.Join(c.opts.Tags, ",")
	params := url.Values{}
	params.Set("id", c.opts.ID)
	params.Set("token", c.opts.Token)
	params.Set("origin", c.opts.Origin)
	params.Set("oid", c.lastOID)
	params.Set("tags", tagStr)
	params.Set("version", c.opts.Version)
	return fmt.Sprintf("%s%s?%s", c.opts.BaseURL, path, params.Encode())
}

func (c *Client) Register(ctx context.Context, passkey string) (*RegisterResult, error) {
	params := url.Values{}
	params.Set("path", "register")
	params.Set("id", c.opts.ID)
	params.Set("token", c.opts.Token)
	params.Set("origin", c.opts.Origin)
	params.Set("tags", strings.Join(c.opts.Tags, ","))
	params.Set("version", c.opts.Version)
	params.Set("passkey", passkey)
	params.Set("server", c.opts.Server)
	params.Set("user", c.opts.User)
	if c.opts.ServerMID != "" {
		params.Set("server_mid", c.opts.ServerMID)
	}

	addr := fmt.Sprintf("%s/workeradmin/register_api?%s", c.opts.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("creating register request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("register failed: %d %s: %s",
			resp.StatusCode, resp.Status, string(body))
	}

	var result RegisterResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing register response: %w", err)
	}

	return &result, nil
}

func (c *Client) Unregister(ctx context.Context) error {
	params := url.Values{}
	params.Set("path", "unregister_by_token")
	params.Set("id", c.opts.ID)
	params.Set("token", c.opts.Token)

	addr := fmt.Sprintf("%s/workeradmin/register_api?%s", c.opts.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, nil)
	if err != nil {
		return fmt.Errorf("creating unregister request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unregister request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unregister failed: %d %s: %s",
			resp.StatusCode, resp.Status, string(body))
	}

	var result struct {
		Error json.RawMessage `json:"error"`
		Msg   string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Error != nil {
		if result.Msg != "" {
			return fmt.Errorf("unregister error: %s", result.Msg)
		}
		return fmt.Errorf("unregister error: %s", string(result.Error))
	}

	return nil
}

func (c *Client) Connect(ctx context.Context) (<-chan Message, error) {
	if c.connected {
		return c.messages, nil
	}

	// claude: use a dedicated HTTP client for SSE without the request timeout;
	// the pubsub httpClient has a 30s timeout which kills the long-lived stream
	sseHTTP := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   0,
	}
	sseURL := c.address("/pubsub/events")
	c.sseClient = sse.NewClient(sseURL,
		sse.WithHTTPClient(sseHTTP),
		sse.WithReconnectDelay(c.opts.ReconnectDelay),
		sse.WithLogger(c.logger),
	)

	events, err := c.sseClient.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("SSE connect: %w", err)
	}

	c.connected = true
	go c.processEvents(ctx, events)

	return c.messages, nil
}

func (c *Client) processEvents(ctx context.Context, events <-chan sse.Event) {
	defer close(c.messages)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}

			if ev.Event == "oid" {
				c.lastOID = ev.Data
				continue
			}

			var eventData EventData
			if err := json.Unmarshal([]byte(ev.Data), &eventData); err != nil {
				c.logger.Debug("failed to parse SSE data", "error", err, "data", ev.Data)
				continue
			}

			if eventData.OID != "" {
				c.lastOID = eventData.OID
			}

			for eventKey, items := range eventData.Events {
				for _, item := range items {
					msg := Message{
						OID:   c.lastOID,
						Event: eventKey,
						Data:  item,
					}
					select {
					case c.messages <- msg:
					case <-ctx.Done():
						return
					case <-c.done:
						return
					}
				}
			}
		}
	}
}

func (c *Client) Publish(ctx context.Context, event string, data map[string]interface{}) error {
	return retry.Do(ctx, retry.Config{
		MaxAttempts: 10,
		InitialWait: 1 * time.Second,
		MaxWait:     512 * time.Second,
		Multiplier:  2.0,
	}, func(ctx context.Context) error {
		return c.publishOnce(ctx, event, data)
	})
}

func (c *Client) publishOnce(ctx context.Context, event string, data map[string]interface{}) error {
	payload := make(map[string]interface{})
	for k, v := range data {
		payload[k] = v
	}
	payload["event"] = event

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling publish data: %w", err)
	}

	addr := c.address("/pubsub/publish")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating publish request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("publish request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("publish failed: status %d", resp.StatusCode)
	}

	c.logger.Debug("published event", "event", event)
	return nil
}

// claude: SetID updates the worker identity after a server-side rename,
// so that reconnects and publishes use the new name
func (c *Client) SetID(id string) {
	c.opts.ID = id
	if c.sseClient != nil {
		c.sseClient.SetURL(c.address("/pubsub/events"))
	}
}

func (c *Client) Push(ctx context.Context, key, filename string, r io.Reader) error {
	params := url.Values{}
	params.Set("id", c.opts.ID)
	params.Set("token", c.opts.Token)
	params.Set("key", key)
	params.Set("filename", filename)
	addr := fmt.Sprintf("%s/pubsub/push?%s", c.opts.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, r)
	if err != nil {
		return fmt.Errorf("creating push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push failed: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Pop(ctx context.Context, key string, w io.Writer) error {
	params := url.Values{}
	params.Set("id", c.opts.ID)
	params.Set("token", c.opts.Token)
	params.Set("key", key)
	addr := fmt.Sprintf("%s/pubsub/pop?%s", c.opts.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, nil)
	if err != nil {
		return fmt.Errorf("creating pop request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pop request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pop failed: status %d: %s", resp.StatusCode, string(body))
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("pop: reading body: %w", err)
	}

	return nil
}

func (c *Client) Close(ctx context.Context) error {
	if !c.connected {
		return nil
	}
	c.connected = false

	select {
	case <-c.done:
	default:
		close(c.done)
	}

	if c.sseClient != nil {
		c.sseClient.Close()
	}

	payload, _ := json.Marshal(map[string]string{
		"id":    c.opts.ID,
		"token": c.opts.Token,
	})

	addr := c.address("/pubsub/close")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Debug("pubsub close request failed", "error", err)
		return nil
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return nil
}
