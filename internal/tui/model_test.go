package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zalo-cli/internal/ipc"
)

type fakeTransport struct {
	results chan ipc.Result
	sent    chan ipc.SendPayload
}

func (f *fakeTransport) Results() <-chan ipc.Result { return f.results }
func (f *fakeTransport) History(id, kind string) (string, error) {
	f.results <- ipc.Result{Event: ipc.Event{Event: "history_result", ID: "history-1", OK: true, ThreadID: id, ThreadType: kind}}
	return "history-1", nil
}
func (f *fakeTransport) ListRecipients() (string, error) {
	f.results <- ipc.Result{Event: ipc.Event{Event: "recipients_result", ID: "list-1", OK: true,
		Recipients: []ipc.Recipient{{ThreadID: "an", ThreadType: "user", Name: "An"}}}}
	return "list-1", nil
}
func (f *fakeTransport) SendMessage(id, kind, text string) (string, error) {
	f.sent <- ipc.SendPayload{ThreadID: id, ThreadType: kind, Text: text}
	f.results <- ipc.Result{Event: ipc.Event{Event: "send_result", ID: "send-1", OK: true}}
	return "send-1", nil
}

func fixture() model {
	f := &fakeTransport{results: make(chan ipc.Result, 10), sent: make(chan ipc.SendPayload, 10)}
	m := newModel(f)
	m.loading = false
	m.recipients = []ipc.Recipient{
		{ThreadID: "an", ThreadType: "user", Name: "An"},
		{ThreadID: "go", ThreadType: "group", Name: "Nhóm Go"},
	}
	m.openSelected()
	return m
}

func update(m model, msg tea.Msg) model {
	result, _ := m.Update(msg)
	return result.(model)
}

func TestSendAcknowledgementBeforeCommandAndConversationSwitch(t *testing.T) {
	m := fixture()
	m.composer.SetValue("Chào An")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.pending == nil || m.history["user:an"][0].state != "Đang gửi..." {
		t.Fatal("expected pending message")
	}
	m.composer.SetValue("Bản nháp An")
	m.cursor = 1
	m.openSelected()
	m.composer.SetValue("Bản nháp nhóm")
	m.handleEvent(ipc.Event{Event: "send_result", ID: "old-request", OK: false})
	m.handleEvent(ipc.Event{Event: "send_result", ID: "send-1", OK: true})
	m = update(m, requestStarted{kind: "send", id: "send-1", attempt: 1})
	if m.pending != nil || m.history["user:an"][0].state != "Đã gửi" {
		t.Fatal("acknowledgement did not update original conversation")
	}
	if len(m.history["group:go"]) != 0 {
		t.Fatal("message leaked into the selected group")
	}
	m.cursor = 0
	m.openSelected()
	if m.composer.Value() != "Bản nháp An" || m.drafts["group:go"] != "Bản nháp nhóm" {
		t.Fatal("drafts crossed conversations")
	}
}

func TestIncomingRoutingDedupAndDisconnect(t *testing.T) {
	m := fixture()
	event := ipc.Event{Event: "new_message", Data: ipc.Message{
		MsgID: "message-1", ThreadID: "go", ThreadType: "group", SenderName: "Bảo", Content: "Xin chào",
	}}
	m.handleEvent(event)
	m.handleEvent(event)
	if len(m.history["group:go"]) != 1 || m.unread["group:go"] != 1 {
		t.Fatal("duplicate or incorrect unread count")
	}
	m.composer.SetValue("Tin đang gửi")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(m, packet{open: false})
	if m.connected || m.pending != nil || !strings.Contains(m.history["user:an"][0].state, "Chưa rõ") {
		t.Fatal("disconnect did not mark pending message as unknown")
	}
	m.composer.SetValue("Không được gửi")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.pending != nil {
		t.Fatal("sending must be disabled after disconnect")
	}
}

func TestExpiredAttemptCannotResolveNextSend(t *testing.T) {
	m := fixture()
	m.composer.SetValue("Một")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(m, requestExpired{kind: "send", attempt: 1})
	m.composer.SetValue("Hai")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(m, requestStarted{kind: "send", id: "stale", attempt: 1})
	m = update(m, requestExpired{kind: "send", attempt: 1})
	if m.pending == nil || m.pending.attempt != 2 || m.pending.id != "" {
		t.Fatal("stale command modified new request")
	}
}

func TestViewFitsResizedTerminalAndUnicode(t *testing.T) {
	m := fixture()
	m.appendLine("user:an", &chatLine{sender: "Quốc Bảo", text: strings.Repeat("Tiếng Việt 🌸 ", 100)})
	for _, size := range [][2]int{{68, 18}, {100, 30}, {160, 40}, {40, 10}} {
		m.resize(size[0], size[1])
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("view %dx%d exceeds terminal %dx%d\n%s", lipgloss.Width(view), lipgloss.Height(view), size[0], size[1], view)
		}
	}
	if clean("\x1b[31mChào\x1b[0m\x07") != "Chào" {
		t.Fatal("terminal control codes were not removed")
	}
}

func TestProgramReceivesListAndSendsThroughTransport(t *testing.T) {
	f := &fakeTransport{results: make(chan ipc.Result, 10), sent: make(chan ipc.SendPayload, 10)}
	var output bytes.Buffer
	program := tea.NewProgram(newModel(f), tea.WithInput(nil), tea.WithOutput(&output), tea.WithoutRenderer())
	done := make(chan error, 1)
	go func() { _, err := program.Run(); done <- err }()
	defer program.Kill()
	// Retry opening until asynchronous list loading has completed.
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			program.Send(tea.KeyMsg{Type: tea.KeyEnter})
			program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Chào")})
			program.Send(tea.KeyMsg{Type: tea.KeyEnter})
		case payload := <-f.sent:
			if payload.ThreadID != "an" || payload.ThreadType != "user" || !strings.Contains(payload.Text, "Chào") {
				t.Fatalf("wrong send: %+v", payload)
			}
			program.Send(tea.KeyMsg{Type: tea.KeyEsc})
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-deadline:
				t.Fatal("program did not quit")
			}
			return
		case <-deadline:
			t.Fatal("UI did not send through transport")
		}
	}
}
