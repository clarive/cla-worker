package pubsub

import (
	"log/slog"
	"net/http"
	"time"
)

type Options struct {
	ID             string
	Token          string
	BaseURL        string
	Origin         string
	Tags           []string
	Version        string
	Server         string
	ServerMID      string
	User           string
	Logger         *slog.Logger
	HTTPClient     *http.Client
	ReconnectDelay time.Duration
}

type Option func(*Options)

func WithID(id string) Option {
	return func(o *Options) { o.ID = id }
}

func WithToken(token string) Option {
	return func(o *Options) { o.Token = token }
}

func WithBaseURL(url string) Option {
	return func(o *Options) { o.BaseURL = url }
}

func WithOrigin(origin string) Option {
	return func(o *Options) { o.Origin = origin }
}

func WithTags(tags []string) Option {
	return func(o *Options) { o.Tags = tags }
}

func WithVersion(v string) Option {
	return func(o *Options) { o.Version = v }
}

func WithPubSubLogger(l *slog.Logger) Option {
	return func(o *Options) { o.Logger = l }
}

func WithPubSubHTTPClient(c *http.Client) Option {
	return func(o *Options) { o.HTTPClient = c }
}

func WithPubSubReconnectDelay(d time.Duration) Option {
	return func(o *Options) { o.ReconnectDelay = d }
}

func WithServer(s string) Option {
	return func(o *Options) { o.Server = s }
}

func WithServerMID(s string) Option {
	return func(o *Options) { o.ServerMID = s }
}

func WithUser(u string) Option {
	return func(o *Options) { o.User = u }
}
