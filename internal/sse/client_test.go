package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sseServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_SimpleMessage(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: hello world\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL, WithReconnectDelay(10*time.Millisecond))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	select {
	case ev := <-ch:
		assert.Equal(t, "message", ev.Event)
		assert.Equal(t, "hello world", ev.Data)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestClient_MultilineData(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: line1\ndata: line2\ndata: line3\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "line1\nline2\nline3", ev.Data)
}

func TestClient_EventType(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "event: oid\ndata: 12345\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "oid", ev.Event)
	assert.Equal(t, "12345", ev.Data)
}

func TestClient_IDField(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "id: evt-1\ndata: test\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "evt-1", ev.ID)
}

func TestClient_RetryField(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "retry: 5000\ndata: test\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, 5000, ev.Retry)
	assert.Equal(t, 5*time.Second, c.reconnectDelay)
}

func TestClient_BOMPrefix(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "\xEF\xBB\xBFdata: bom test\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "bom test", ev.Data)
}

func TestClient_CommentLines(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, ": this is a comment\ndata: after comment\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "after comment", ev.Data)
}

func TestClient_FieldNoValue(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "", ev.Data)
}

func TestClient_NoSpaceAfterColon(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data:no-space\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "no-space", ev.Data)
}

func TestClient_MultipleEvents(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "data: first\n\ndata: second\n\ndata: third\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	events := []string{}
	for i := 0; i < 3; i++ {
		select {
		case ev := <-ch:
			events = append(events, ev.Data)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	}
	assert.Equal(t, []string{"first", "second", "third"}, events)
}

func TestClient_LargePayload(t *testing.T) {
	largeData := strings.Repeat("x", 1024*1024)
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprintf(w, "data: %s\n\n", largeData)
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, len(largeData), len(ev.Data))
}

func TestClient_ServerError(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	c := NewClient(srv.URL)
	_, err := c.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Reconnect(t *testing.T) {
	var mu sync.Mutex
	connectCount := 0

	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectCount++
		n := connectCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		if n == 1 {
			fmt.Fprint(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			return
		}

		fmt.Fprint(w, "data: reconnected\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewClient(srv.URL, WithReconnectDelay(50*time.Millisecond))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev1 := <-ch
	assert.Equal(t, "first", ev1.Data)

	select {
	case ev2 := <-ch:
		assert.Equal(t, "reconnected", ev2.Data)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for reconnected event")
	}
}

func TestClient_LastEventIDOnReconnect(t *testing.T) {
	var mu sync.Mutex
	var lastEventIDs []string

	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastEventIDs = append(lastEventIDs, r.Header.Get("Last-Event-ID"))
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "id: evt-42\ndata: test\n\n")
		w.(http.Flusher).Flush()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL, WithReconnectDelay(50*time.Millisecond))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	<-ch
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(lastEventIDs) >= 2 {
		assert.Equal(t, "", lastEventIDs[0])
		assert.Equal(t, "evt-42", lastEventIDs[1])
	}
	mu.Unlock()
}

func TestClient_ContextCancel(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		time.Sleep(10 * time.Second)
	})

	ctx, cancel := context.WithCancel(context.Background())

	c := NewClient(srv.URL)
	ch, err := c.Connect(ctx)
	require.NoError(t, err)

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel should close after context cancel")
	}
}

func TestClient_InvalidRetryIgnored(t *testing.T) {
	srv := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, "retry: not-a-number\ndata: test\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(srv.URL, WithReconnectDelay(100*time.Millisecond))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close()

	ev := <-ch
	assert.Equal(t, "test", ev.Data)
	assert.Equal(t, 100*time.Millisecond, c.reconnectDelay)
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		line  string
		field string
		value string
	}{
		{"data: hello", "data", "hello"},
		{"data:hello", "data", "hello"},
		{"event: custom", "event", "custom"},
		{"id: 123", "id", "123"},
		{"retry: 5000", "retry", "5000"},
		{"data", "data", ""},
	}

	for _, tt := range tests {
		f, v := parseLine(tt.line)
		assert.Equal(t, tt.field, f, "field for %q", tt.line)
		assert.Equal(t, tt.value, v, "value for %q", tt.line)
	}
}

func TestStripBOM(t *testing.T) {
	assert.Equal(t, "hello", stripBOM("\xEF\xBB\xBFhello"))
	assert.Equal(t, "hello", stripBOM("hello"))
}
