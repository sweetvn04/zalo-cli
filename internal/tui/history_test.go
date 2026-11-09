package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

func TestScrollKeysAndMouseMoveChat(t *testing.T) {
	m := fixture()
	for i := 0; i < 80; i++ {
		m.appendLine("user:an", &chatLine{text: fmt.Sprintf("Dòng %d", i)})
	}
	m.refreshChat(true)
	bottom := m.chat.YOffset
	m = update(m, tea.MouseMsg{X: 60, Y: 10, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.chat.YOffset >= bottom {
		t.Fatal("mouse wheel did not scroll chat")
	}
	offset := m.chat.YOffset
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.chat.YOffset >= offset {
		t.Fatal("up arrow did not scroll while composing")
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.chat.YOffset >= offset {
		t.Fatal("PageUp did not scroll")
	}
}

func TestOlderHistoryKeepsViewportAnchorAndDoesNotDropLoadedMessages(t *testing.T) {
	m := fixture()
	state := m.historyStates["user:an"]
	state.loading, state.loaded, state.more, state.cursor = false, true, true, "1000"
	for i := 1000; i < 1600; i++ {
		m.appendLine("user:an", &chatLine{id: fmt.Sprint(i), text: fmt.Sprintf("Tin %d", i), timestamp: int64(i)})
	}
	m.refreshChat(false)
	m.chat.SetYOffset(9)
	anchor := m.chat.View()
	if m.loadOlder() == nil || !state.older {
		t.Fatal("expected older history request")
	}
	m = update(m, requestStarted{kind: "history", thread: "user:an", id: "older-1", attempt: state.attempt})
	m.handleEvent(ipc.Event{Event: "history_result", ID: "older-1", OK: true, ThreadID: "an", ThreadType: "user",
		Cursor: "999", HasMore: true, Messages: []ipc.Message{{MsgID: "999", ThreadID: "an", ThreadType: "user", Timestamp: 999, Content: "Cũ hơn"}}})
	if len(m.history["user:an"]) != 601 {
		t.Fatal("loaded messages were truncated")
	}
	if m.chat.View() != anchor {
		t.Fatal("loading older history moved the reading position")
	}
	if state.cursor != "999" || !state.more {
		t.Fatal("cursor did not advance")
	}
}

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
