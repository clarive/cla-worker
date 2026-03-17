package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockServer struct {
	srv            *httptest.Server
	sseEvents      chan string
	publishes      []map[string]interface{}
	mu             sync.Mutex
	registerResp   func(r *http.Request) (int, interface{})
	unregisterResp func(r *http.Request) (int, interface{})
	pushData       chan []byte
	popData        []byte
	closeCount     int
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	ms := &mockServer{
		sseEvents: make(chan string, 100),
		pushData:  make(chan []byte, 10),
	}

	ms.registerResp = func(r *http.Request) (int, interface{}) {
		return 200, map[string]interface{}{"token": "test-token", "projects": []string{"proj1"}}
	}
	ms.unregisterResp = func(r *http.Request) (int, interface{}) {
		return 200, map[string]interface{}{"ok": true}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/pubsub/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		flusher.Flush()

		for {
			select {
			case evt, ok := <-ms.sseEvents:
				if !ok {
					return
				}
				fmt.Fprint(w, evt)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	mux.HandleFunc("/pubsub/register", func(w http.ResponseWriter, r *http.Request) {
		code, resp := ms.registerResp(r)
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/pubsub/unregister", func(w http.ResponseWriter, r *http.Request) {
		code, resp := ms.unregisterResp(r)
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/pubsub/publish", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]interface{}
		json.NewDecoder(r.Body).Decode(&data)
		ms.mu.Lock()
		ms.publishes = append(ms.publishes, data)
		ms.mu.Unlock()
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("/pubsub/push", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		ms.pushData <- data
		w.WriteHeader(200)
	})

	mux.HandleFunc("/pubsub/pop", func(w http.ResponseWriter, r *http.Request) {
		if ms.popData != nil {
			w.WriteHeader(200)
			w.Write(ms.popData)
		} else {
			w.WriteHeader(404)
		}
	})

	mux.HandleFunc("/pubsub/close", func(w http.ResponseWriter, r *http.Request) {
		ms.mu.Lock()
		ms.closeCount++
		ms.mu.Unlock()
		w.WriteHeader(200)
	})

	ms.srv = httptest.NewServer(mux)
	t.Cleanup(ms.srv.Close)
	return ms
}

func (ms *mockServer) getPublishes() []map[string]interface{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	result := make([]map[string]interface{}, len(ms.publishes))
	copy(result, ms.publishes)
	return result
}

func TestClient_Register_Success(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"))

	result, err := c.Register(context.Background(), "valid-key")
	require.NoError(t, err)
	assert.Equal(t, "test-token", result.Token)
	assert.Contains(t, result.Projects, "proj1")
}

func TestClient_Register_RejectedPasskey(t *testing.T) {
	ms := newMockServer(t)
	ms.registerResp = func(r *http.Request) (int, interface{}) {
		return 403, map[string]interface{}{"error": "invalid passkey"}
	}
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"))

	_, err := c.Register(context.Background(), "bad-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestClient_Register_InvalidPasskey(t *testing.T) {
	ms := newMockServer(t)
	ms.registerResp = func(r *http.Request) (int, interface{}) {
		return 400, map[string]interface{}{"error": "missing passkey"}
	}
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"))

	_, err := c.Register(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestClient_Register_ServerError(t *testing.T) {
	ms := newMockServer(t)
	ms.registerResp = func(r *http.Request) (int, interface{}) {
		return 500, "internal error"
	}
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"))

	_, err := c.Register(context.Background(), "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Register_NetworkError(t *testing.T) {
	c := NewClient(WithBaseURL("http://127.0.0.1:1"), WithID("w1"))
	_, err := c.Register(context.Background(), "key")
	require.Error(t, err)
}

func TestClient_Unregister_Success(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	err := c.Unregister(context.Background())
	require.NoError(t, err)
}

func TestClient_Unregister_Error(t *testing.T) {
	ms := newMockServer(t)
	ms.unregisterResp = func(r *http.Request) (int, interface{}) {
		return 500, "error"
	}
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	err := c.Unregister(context.Background())
	require.Error(t, err)
}

func TestClient_Connect_Success(t *testing.T) {
	ms := newMockServer(t)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ms.sseEvents <- "data: {\"oid\":\"1\",\"events\":{\"worker.ready\":[{\"msg\":\"hello\"}]}}\n\n"
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close(ctx)

	select {
	case msg := <-ch:
		assert.Equal(t, "worker.ready", msg.Event)
		assert.Equal(t, "1", msg.OID)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestClient_Connect_AlreadyConnected(t *testing.T) {
	ms := newMockServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))
	ch1, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close(ctx)

	ch2, err := c.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, ch1, ch2)
}

func TestClient_Publish_Success(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	err := c.Publish(context.Background(), "test.event", map[string]interface{}{
		"key": "value",
	})
	require.NoError(t, err)

	pubs := ms.getPublishes()
	require.Len(t, pubs, 1)
	assert.Equal(t, "test.event", pubs[0]["event"])
	assert.Equal(t, "value", pubs[0]["key"])
}

func TestClient_Publish_URLParams(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(
		WithBaseURL(ms.srv.URL),
		WithID("worker-1"),
		WithToken("tok-123"),
		WithOrigin("user@host/123"),
		WithTags([]string{"linux", "docker"}),
	)

	err := c.Publish(context.Background(), "test", nil)
	require.NoError(t, err)
}

func TestClient_Push_Success(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	data := "file content here"
	err := c.Push(context.Background(), "fk-1", "test.txt", strings.NewReader(data))
	require.NoError(t, err)

	received := <-ms.pushData
	assert.Equal(t, data, string(received))
}

func TestClient_Pop_Success(t *testing.T) {
	ms := newMockServer(t)
	ms.popData = []byte("downloaded content")
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	var buf bytes.Buffer
	err := c.Pop(context.Background(), "fk-1", &buf)
	require.NoError(t, err)
	assert.Equal(t, "downloaded content", buf.String())
}

func TestClient_Pop_ServerError(t *testing.T) {
	ms := newMockServer(t)
	ms.popData = nil
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	var buf bytes.Buffer
	err := c.Pop(context.Background(), "fk-1", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestClient_Close_Success(t *testing.T) {
	ms := newMockServer(t)
	ctx := context.Background()
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	_ = ch

	err = c.Close(ctx)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	ms.mu.Lock()
	assert.Equal(t, 1, ms.closeCount)
	ms.mu.Unlock()
}

func TestClient_Close_NotConnected(t *testing.T) {
	c := NewClient(WithBaseURL("http://localhost"), WithID("w1"))
	err := c.Close(context.Background())
	require.NoError(t, err)
}

func TestClient_OIDTracking(t *testing.T) {
	ms := newMockServer(t)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ms.sseEvents <- "event: oid\ndata: oid-42\n\n"
		time.Sleep(50 * time.Millisecond)
		ms.sseEvents <- "data: {\"events\":{\"worker.ready\":[{\"msg\":\"hi\"}]}}\n\n"
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close(ctx)

	select {
	case msg := <-ch:
		assert.Equal(t, "worker.ready", msg.Event)
		assert.Equal(t, "oid-42", msg.OID)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestClient_MultipleEvents(t *testing.T) {
	ms := newMockServer(t)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ms.sseEvents <- "data: {\"oid\":\"1\",\"events\":{\"worker.exec\":[{\"cmd\":\"ls\"}],\"worker.ready\":[{\"msg\":\"hi\"}]}}\n\n"
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close(ctx)

	received := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			received[msg.Event] = true
		case <-time.After(3 * time.Second):
			t.Fatal("timeout")
		}
	}
	assert.True(t, received["worker.exec"])
	assert.True(t, received["worker.ready"])
}

func TestClient_InvalidJSON(t *testing.T) {
	ms := newMockServer(t)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ms.sseEvents <- "data: {invalid json}\n\n"
		time.Sleep(50 * time.Millisecond)
		ms.sseEvents <- "data: {\"oid\":\"2\",\"events\":{\"worker.ready\":[{\"ok\":true}]}}\n\n"
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))
	ch, err := c.Connect(ctx)
	require.NoError(t, err)
	defer c.Close(ctx)

	select {
	case msg := <-ch:
		assert.Equal(t, "worker.ready", msg.Event)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout - invalid JSON should not block")
	}
}

func TestClient_LargeFile_Push(t *testing.T) {
	ms := newMockServer(t)
	c := NewClient(WithBaseURL(ms.srv.URL), WithID("w1"), WithToken("tok"))

	data := strings.Repeat("A", 10*1024*1024)
	err := c.Push(context.Background(), "large-key", "large.bin", strings.NewReader(data))
	require.NoError(t, err)

	received := <-ms.pushData
	assert.Equal(t, len(data), len(received))
}

func TestClient_Address_Construction(t *testing.T) {
	c := NewClient(
		WithBaseURL("http://example.com:8080"),
		WithID("w1"),
		WithToken("tok-123"),
		WithOrigin("user@host/1"),
		WithTags([]string{"linux", "docker"}),
		WithVersion("2.0.0"),
	)

	addr := c.address("/pubsub/events")
	assert.Contains(t, addr, "http://example.com:8080/pubsub/events")
	assert.Contains(t, addr, "id=w1")
	assert.Contains(t, addr, "token=tok-123")
	assert.Contains(t, addr, "origin=")
	assert.Contains(t, addr, "version=2.0.0")
}
