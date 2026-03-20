package worker

import (
	"context"
	"testing"
	"time"

	"github.com/clarive/cla-worker-go/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		ID:    "test-worker",
		URL:   "http://localhost:8080",
		Token: "test-token",
	}
	w := New(cfg, nil)
	assert.NotNil(t, w)
}

func TestWorker_RunWithCancel(t *testing.T) {
	cfg := &config.Config{
		ID:    "test-worker",
		URL:   "http://127.0.0.1:1",
		Token: "test-token",
		Tags:  []string{},
	}
	w := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	code, err := w.Run(ctx)
	assert.Error(t, err)
	assert.NotEqual(t, 0, code)
}

func TestWorker_GeneratesID(t *testing.T) {
	cfg := &config.Config{
		URL:   "http://127.0.0.1:1",
		Token: "test-token",
		Tags:  []string{},
	}
	w := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	w.Run(ctx)
	assert.NotEmpty(t, cfg.ID)
	// claude: verify deterministic user@hostname format (no random xid suffix)
	assert.Contains(t, cfg.ID, "@")
	assert.NotContains(t, cfg.ID, "/", "ID should not contain random xid suffix")
}

func TestWorker_GeneratesOrigin(t *testing.T) {
	cfg := &config.Config{
		ID:    "w1",
		URL:   "http://127.0.0.1:1",
		Token: "tok",
		Tags:  []string{},
	}
	w := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	w.Run(ctx)
	assert.NotEmpty(t, cfg.Origin)
}
