package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

func TestHistoryMergesRealtimeWithoutCrossingConversations(t *testing.T) {
	m := fixture()
	m.handleEvent(ipc.Event{Event: "new_message", Data: ipc.Message{
		MsgID: "2", ThreadID: "an", ThreadType: "user", Timestamp: 2000, Content: "Realtime",
	}})
	m.cursor = 1
	m.openSelected()
	event := ipc.Event{Event: "history_result", ID: "history-an", OK: true, ThreadID: "an", ThreadType: "user",
		Messages: []ipc.Message{
			{MsgID: "2", ThreadID: "an", ThreadType: "user", Timestamp: 2000, Content: "Realtime"},
			{MsgID: "1", ThreadID: "an", ThreadType: "user", Timestamp: 1000, Content: "Cũ"},
			{MsgID: "other", ThreadID: "go", ThreadType: "group", Content: "Sai hội thoại"},
		}}
	// Kết quả đến trước khi tea.Cmd trả ID yêu cầu.
	m.handleEvent(event)
	m = update(m, requestStarted{kind: "history", thread: "user:an", id: "history-an", attempt: 1})
	lines := m.history["user:an"]
	if len(lines) != 2 || lines[0].text != "Cũ" || lines[1].text != "Realtime" {
		t.Fatalf("wrong merge: %+v", lines)
	}
	if len(m.history["group:go"]) != 0 || m.unread["user:an"] != 0 {
		t.Fatal("history changed another conversation or unread count")
	}
}

func TestHistoryOwnMessageDeduplicatesAfterSendAcknowledgement(t *testing.T) {
	m := fixture()
	m.composer.SetValue("Tin của tôi")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.mergeHistory("user:an", []ipc.Message{{MsgID: "sent-id", ThreadID: "an", ThreadType: "user",
		Content: "Tin của tôi", IsSelf: true, Timestamp: 1000}})
	m = update(m, requestStarted{kind: "send", id: "send-1", attempt: 1})
	m.handleEvent(ipc.Event{Event: "send_result", ID: "send-1", MsgID: "sent-id", OK: true})
	if len(m.history["user:an"]) != 1 || m.history["user:an"][0].state != "Đã gửi" {
		t.Fatal("local outgoing message duplicated history")
	}
}

func TestHistoryFailurePreservesRealtimeAndRetryIgnoresOldTimeout(t *testing.T) {
	m := fixture()
	m.appendLine("user:an", &chatLine{id: "live", text: "Giữ lại"})
	m = update(m, requestStarted{kind: "history", thread: "user:an", id: "history-1", attempt: 1})
	m.handleEvent(ipc.Event{Event: "history_result", ID: "history-1", OK: false, Error: "Mất mạng"})
	if len(m.history["user:an"]) != 1 || m.historyStates["user:an"].loaded {
		t.Fatal("failure removed live messages or marked history loaded")
	}
	m.loadHistory(true)
	m = update(m, requestExpired{kind: "history", thread: "user:an", attempt: 1})
	if !m.historyStates["user:an"].loading {
		t.Fatal("old timeout cancelled retry")
	}
}
