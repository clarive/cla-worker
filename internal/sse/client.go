package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	url            string
	httpClient     *http.Client
	headers        http.Header
	lastEventID    string
	reconnectDelay time.Duration
	events         chan Event
	logger         *slog.Logger
	closed         chan struct{}
	closeOnce      sync.Once
	eventsOnce     sync.Once
	mu             sync.Mutex
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(s *Client) { s.httpClient = c }
}

func WithHeaders(h http.Header) Option {
	return func(s *Client) { s.headers = h }
}

func WithReconnectDelay(d time.Duration) Option {
	return func(s *Client) { s.reconnectDelay = d }
}

func WithLogger(l *slog.Logger) Option {
	return func(s *Client) { s.logger = l }
}

func NewClient(url string, opts ...Option) *Client {
	c := &Client{
		url:            url,
		httpClient:     &http.Client{Timeout: 0},
		headers:        http.Header{},
		reconnectDelay: 1 * time.Second,
		events:         make(chan Event, 64),
		logger:         slog.Default(),
		closed:         make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Connect(ctx context.Context) (<-chan Event, error) {
	err := c.doConnect(ctx)
	if err != nil {
		return nil, err
	}
	return c.events, nil
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
}

func (c *Client) shutdownEvents() {
	c.eventsOnce.Do(func() {
		close(c.events)
	})
}

func (c *Client) doConnect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("creating SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	c.mu.Lock()
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}
	c.mu.Unlock()
	for k, vals := range c.headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connect: status %d", resp.StatusCode)
	}

	go c.readStream(ctx, resp.Body)
	return nil
}

func (c *Client) readStream(ctx context.Context, body io.ReadCloser) {
	defer body.Close()

	reader := bufio.NewReader(body)
	first := true
	var event Event

	for {
		if c.isDone(ctx) {
			c.shutdownEvents()
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if c.isDone(ctx) {
				c.shutdownEvents()
				return
			}
			// claude: WARN on connection loss so operators see it
			if err == io.EOF {
				c.logger.Warn("SSE connection lost (server closed)")
			} else {
				c.logger.Warn("SSE connection lost", "error", err)
			}
			c.handleDisconnect(ctx)
			return
		}

		line = strings.TrimRight(line, "\r\n")

		if first {
			line = stripBOM(line)
			first = false
		}

		if line == "" {
			if event.Data != "" {
				event.Data = strings.TrimSuffix(event.Data, "\n")
				if event.Event == "" {
					event.Event = "message"
				}
				if event.ID != "" {
					c.mu.Lock()
					c.lastEventID = event.ID
					c.mu.Unlock()
				}

				select {
				case c.events <- event:
				case <-ctx.Done():
					c.shutdownEvents()
					return
				case <-c.closed:
					c.shutdownEvents()
					return
				}
			}
			event = Event{}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value := parseLine(line)

		switch field {
		case "data":
			event.Data += value + "\n"
		case "event":
			event.Event = value
		case "id":
			event.ID = value
		case "retry":
			if ms, err := strconv.Atoi(value); err == nil {
				event.Retry = ms
				c.mu.Lock()
				c.reconnectDelay = time.Duration(ms) * time.Millisecond
				c.mu.Unlock()
			}
		}
	}
}

func (c *Client) isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *Client) handleDisconnect(ctx context.Context) {
	c.mu.Lock()
	delay := c.reconnectDelay
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		c.shutdownEvents()
		return
	case <-c.closed:
		c.shutdownEvents()
		return
	case <-time.After(delay):
	}

	c.logger.Info("SSE reconnecting...", "delay", delay)
	if err := c.doConnect(ctx); err != nil {
		c.logger.Warn("SSE reconnect failed, will retry", "error", err)
		c.handleDisconnect(ctx)
	} else {
		c.logger.Info("SSE reconnected")
	}
}

func parseLine(line string) (field, value string) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return line, ""
	}
	field = line[:idx]
	value = line[idx+1:]
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return field, value
}

func stripBOM(s string) string {
	bom := "\xEF\xBB\xBF"
	return strings.TrimPrefix(s, bom)
}
