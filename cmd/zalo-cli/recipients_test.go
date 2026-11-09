package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"zalo-cli/internal/ipc"
)

func TestChooseRecipient(t *testing.T) {
	for _, choice := range []string{"2", "", "99"} {
		t.Run("choice_"+choice, func(t *testing.T) {
			sent := make(chan ipc.SendRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				var request ipc.SendRequest
				if conn.ReadJSON(&request) != nil {
					return
				}
				if request.Action != "list_recipients" {
					return
				}
				conn.WriteJSON(ipc.Event{Event: "recipients_result", ID: request.ID, OK: true,
					Recipients: []ipc.Recipient{
						{ThreadID: "user-1", ThreadType: "user", Name: "An"},
						{ThreadID: "group-2", ThreadType: "group", Name: "Bạn bè"},
					},
				})
				if conn.ReadJSON(&request) != nil {
					return
				}
				sent <- request
				conn.WriteJSON(ipc.Event{Event: "send_result", ID: request.ID, OK: true})
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var output bytes.Buffer
			err := runList(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), "Chào", strings.NewReader(choice+"\n"), &output)
			if choice == "99" {
				if err == nil {
					t.Fatal("expected invalid choice error")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "2. [group] Bạn bè") {
				t.Fatal(output.String())
			}
			if choice == "2" {
				select {
				case request := <-sent:
					if request.Payload == nil || request.Payload.ThreadID != "group-2" || request.Payload.ThreadType != "group" {
						t.Fatalf("wrong recipient: %+v", request)
					}
				default:
					t.Fatal("expected send command")
				}
			} else {
				select {
				case <-sent:
					t.Fatal("must not send after cancellation or invalid choice")
				default:
				}
			}
		})
	}
}
