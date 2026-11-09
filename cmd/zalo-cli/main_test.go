package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"zalo-cli/internal/ipc"
)

func TestSendWaitsForMatchingResult(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "failure"
		if success {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				var request ipc.SendRequest
				if err := conn.ReadJSON(&request); err != nil {
					return
				}
				// Một phản hồi của yêu cầu khác không được kết thúc lệnh hiện tại.
				conn.WriteJSON(ipc.Event{Event: "send_result", ID: "other", OK: !success, Error: "unrelated"})
				conn.WriteJSON(ipc.Event{Event: "send_result", ID: request.ID, OK: success, Error: "Zalo từ chối"})
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := run(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), "test-user", "user", "Chào Bảo")
			if success && err != nil {
				t.Fatal(err)
			}
			if !success && (err == nil || !strings.Contains(err.Error(), "Zalo từ chối")) {
				t.Fatalf("expected Zalo error, got %v", err)
			}
		})
	}
}
