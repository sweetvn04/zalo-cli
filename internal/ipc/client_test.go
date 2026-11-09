package ipc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestReceiveAfterMalformedJSONAndDisconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.WriteMessage(websocket.TextMessage, []byte("{"))
		conn.WriteJSON(Event{Event: "new_message", Data: Message{Content: "Chào Bảo", Timestamp: 123}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var results []Result
	for result := range client.Results() {
		results = append(results, result)
	}
	if len(results) != 3 || results[0].Err == nil || results[1].Event.Data.Content != "Chào Bảo" || results[1].Event.Data.Timestamp != 123 || results[2].Err == nil {
		t.Fatalf("unexpected stream: %+v", results)
	}
}

func TestCancelUnblocksIdleConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.ReadMessage()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cancel()
	select {
	case <-client.done:
	case <-time.After(3 * time.Second):
		t.Fatal("reader did not stop after cancellation")
	}
}
