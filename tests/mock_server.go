//go:build integration

package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

type CapturedPublish struct {
	Event string
	Data  map[string]interface{}
}

type MockClariveServer struct {
	Server          *httptest.Server
	SSEEvents       chan string
	PublishedEvents []CapturedPublish
	RegisterHandler func(r *http.Request) (int, interface{})
	PushReceived    chan []byte
	PopData         []byte
	CloseCount      int
	mu              sync.Mutex
}

func NewMockClariveServer() *MockClariveServer {
	ms := &MockClariveServer{
		SSEEvents:    make(chan string, 100),
		PushReceived: make(chan []byte, 10),
	}

	ms.RegisterHandler = func(r *http.Request) (int, interface{}) {
		return 200, map[string]interface{}{
			"token": "test-token-abc",
		}
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
			case evt, ok := <-ms.SSEEvents:
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

	mux.HandleFunc("/workeradmin/register_api", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		switch path {
		case "register":
			code, resp := ms.RegisterHandler(r)
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(resp)
		case "unregister_by_token":
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{"deleted": 1})
		default:
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "unknown path"})
		}
	})

	mux.HandleFunc("/pubsub/publish", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]interface{}
		json.NewDecoder(r.Body).Decode(&data)
		ms.mu.Lock()
		event, _ := data["event"].(string)
		ms.PublishedEvents = append(ms.PublishedEvents, CapturedPublish{
			Event: event,
			Data:  data,
		})
		ms.mu.Unlock()
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("/pubsub/push", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		ms.PushReceived <- data
		w.WriteHeader(200)
	})

	mux.HandleFunc("/pubsub/pop", func(w http.ResponseWriter, r *http.Request) {
		ms.mu.Lock()
		popData := ms.PopData
		ms.mu.Unlock()
		if popData != nil {
			w.WriteHeader(200)
			w.Write(popData)
		} else {
			w.WriteHeader(404)
			w.Write([]byte("no data"))
		}
	})

	mux.HandleFunc("/pubsub/close", func(w http.ResponseWriter, r *http.Request) {
		ms.mu.Lock()
		ms.CloseCount++
		ms.mu.Unlock()
		w.WriteHeader(200)
	})

	ms.Server = httptest.NewServer(mux)
	return ms
}

func (ms *MockClariveServer) Close() {
	ms.Server.Close()
}

func (ms *MockClariveServer) GetPublished() []CapturedPublish {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	result := make([]CapturedPublish, len(ms.PublishedEvents))
	copy(result, ms.PublishedEvents)
	return result
}

func (ms *MockClariveServer) GetPublishedByEvent(event string) []CapturedPublish {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	var result []CapturedPublish
	for _, p := range ms.PublishedEvents {
		if p.Event == event {
			result = append(result, p)
		}
	}
	return result
}

func (ms *MockClariveServer) SendSSE(data string) {
	ms.SSEEvents <- fmt.Sprintf("data: %s\n\n", data)
}

func (ms *MockClariveServer) SendSSEEvent(eventType, data string) {
	ms.SSEEvents <- fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
}

func (ms *MockClariveServer) SendCommand(oid, command string, data map[string]interface{}) {
	events := map[string]interface{}{
		command: []interface{}{data},
	}
	payload := map[string]interface{}{
		"oid":    oid,
		"events": events,
	}
	jsonData, _ := json.Marshal(payload)
	ms.SendSSE(string(jsonData))
}
