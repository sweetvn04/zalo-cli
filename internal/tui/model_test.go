package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zalo-cli/internal/ipc"
)

func TestSelectedVideoOpensLocalWithoutImagePreview(t *testing.T) {
	m := fixture()
	m.resize(100, 30)
	m.editing = true
	line := &chatLine{kind: "video", imageURL: "archive:1001/" + strings.Repeat("a", 64) + ".mp4", mediaState: "ready"}
	m.history[m.active] = []*chatLine{line}
	m.selectedLine = line
	if cmd := m.ensurePreview(false); cmd != nil {
		t.Fatal("video must not enter image decoder")
	}
	opened := ""
	m.openImage = func(_ context.Context, address string) error { opened = address; return nil }
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd == nil {
		t.Fatal("Enter did not open selected video")
	}
	cmd()
	if opened != line.imageURL {
		t.Fatal("video did not use local archive")
	}
	m.imageOpening = false
	line.imageURL = "https://example.org/video.mp4"
	if cmd := m.openExternalImage(line); cmd != nil {
		t.Fatal("must wait for archived video before opening")
	}
}

type fakeTransport struct {
	results chan ipc.Result
	sent    chan ipc.SendPayload
}

func (f *fakeTransport) Results() <-chan ipc.Result { return f.results }
func (f *fakeTransport) History(id, kind string, beforeID ...string) (string, error) {
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

func (f *fakeTransport) SendImage(id, kind, image string) (string, error) {
	f.sent <- ipc.SendPayload{ThreadID: id, ThreadType: kind, ImageLocal: image}
	return "image-1", nil
}

func (f *fakeTransport) SendSticker(id, kind string, stickerID int64) (string, error) {
	f.sent <- ipc.SendPayload{ThreadID: id, ThreadType: kind, StickerID: stickerID}
	return "sticker-1", nil
}

func (f *fakeTransport) ListStickers() (string, error) { return "stickers-1", nil }

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

func TestWheelMovesMessageSelectionWithViewport(t *testing.T) {
	m := fixture()
	for i := 0; i < 30; i++ {
		m.appendLine(m.active, &chatLine{sender: "An", text: "Tin nhắn"})
	}
	m.selectedLine = m.history[m.active][10]
	m.refreshChat(false)
	m.chat.SetYOffset(m.lineRanges[m.selectedLine].start - 3)
	before := m.chat.YOffset
	m = update(m, tea.MouseMsg{X: m.sidebarWidth() + 1, Button: tea.MouseButtonWheelUp})
	if m.chat.YOffset != before-3 || m.selectedLine != m.history[m.active][9] {
		t.Fatal("mouse wheel must move both viewport and selected message")
	}
}

func TestCopyImageThenSendToAnotherConversation(t *testing.T) {
	m := fixture()
	image := "archive:1001/" + strings.Repeat("a", 64) + ".png"
	m.selectedLine = &chatLine{kind: "image", imageURL: image}
	m = update(m, tea.KeyMsg{Type: tea.KeyF9})
	if m.copiedImage != image || m.pending != nil {
		t.Fatal("F9 must copy without sending")
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyTab})
	m.cursor = 1
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.composer.SetValue("Tin nháp")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(model)
	if cmd == nil || m.pending == nil || m.pending.thread != "group:go" || m.composer.Value() != "Tin nháp" {
		t.Fatal("Ctrl+G must target open group and preserve text draft")
	}
	batch := cmd().(tea.BatchMsg)
	m = update(m, batch[0]())
	payload := <-m.client.(*fakeTransport).sent
	if payload.ThreadID != "go" || payload.ThreadType != "group" || payload.ImageLocal != image {
		t.Fatalf("wrong send payload: %+v", payload)
	}
	m.handleEvent(ipc.Event{Event: "send_result", ID: "image-1", OK: true, MsgID: "789"})
	if m.pending != nil || m.history["group:go"][0].id != "789" {
		t.Fatal("send acknowledgement was not applied")
	}
}

func TestStickerPickerSendsSelectedSticker(t *testing.T) {
	m := fixture()
	m = update(m, tea.KeyMsg{Type: tea.KeyF4})
	m = update(m, packet{open: true, result: ipc.Result{Event: ipc.Event{
		Event: "stickers_result", OK: true, Messages: []ipc.Message{{StickerID: 123}, {StickerID: 456}},
	}}})
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m.composer.SetValue("Bản nháp")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd == nil || m.stickerMenu != nil || m.composer.Value() != "Bản nháp" {
		t.Fatal("sticker selection must send and preserve the draft")
	}
	cmd().(tea.BatchMsg)[0]()
	payload := <-m.client.(*fakeTransport).sent
	if payload.ThreadID != "an" || payload.StickerID != 456 {
		t.Fatalf("wrong sticker payload: %+v", payload)
	}
}

func TestCopiedStickerCanBeSentWithoutLocalPreview(t *testing.T) {
	m := fixture()
	m.selectedLine = &chatLine{kind: "sticker", stickerID: 123}
	m = update(m, tea.KeyMsg{Type: tea.KeyF9})
	m.cursor = 1
	m.openSelected()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil {
		t.Fatal("copied sticker did not produce a send command")
	}
	cmd().(tea.BatchMsg)[0]()
	payload := <-m.client.(*fakeTransport).sent
	if payload.ThreadID != "go" || payload.ThreadType != "group" || payload.StickerID != 123 {
		t.Fatalf("wrong forwarded sticker: %+v", payload)
	}
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

func TestF5RateLimitKeepsRecipientsAndHonorsCooldown(t *testing.T) {
	m := fixture()
	m.loading = true
	m.listID = "refresh-1"
	m.handleEvent(ipc.Event{Event: "recipients_result", ID: "refresh-1", OK: false,
		Error: "Zalo đang giới hạn yêu cầu", RetryAfter: 60})
	if len(m.recipients) != 2 || m.active != "user:an" {
		t.Fatal("refresh failure changed current recipients or conversation")
	}
	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m = updated.(model)
	if command != nil || m.loading || !strings.Contains(m.status, "giây") {
		t.Fatal("F5 must wait during cooldown")
	}
	m.listNextLoad = time.Now().Add(-time.Second)
	updated, command = m.Update(tea.KeyMsg{Type: tea.KeyF5})
	if command == nil || !updated.(model).loading {
		t.Fatal("refresh must work after cooldown")
	}
}

func TestArchiveNotificationDuringRequestTriggersAnotherRead(t *testing.T) {
	m := fixture()
	state := m.historyStates[m.active]
	state.id = "first"
	state.loaded = true
	attempt := state.attempt
	m = update(m, packet{open: true, result: ipc.Result{Event: ipc.Event{Event: "archive_updated", Data: ipc.Message{ThreadType: "user"}}}})
	if !m.historyDirty[m.active] || state.attempt != attempt {
		t.Fatal("notification lost while history request pending")
	}
	if !state.loaded {
		t.Fatal("background sync must not mark active chat as first load and jump to bottom")
	}
	m = update(m, packet{open: true, result: ipc.Result{Event: ipc.Event{Event: "history_result", ID: "first", OK: true, ThreadID: "an", ThreadType: "user"}}})
	if m.historyDirty[m.active] || !state.loading || state.attempt != attempt+1 {
		t.Fatal("latest local snapshot was not requested after reply")
	}

	m.loading, m.listID = true, "contacts-first"
	listAttempt := m.listAttempt
	m = update(m, packet{open: true, result: ipc.Result{Event: ipc.Event{Event: "recipients_updated"}}})
	if !m.listDirty {
		t.Fatal("contact notification lost")
	}
	m = update(m, packet{open: true, result: ipc.Result{Event: ipc.Event{Event: "recipients_result", ID: "contacts-first", OK: true, Recipients: m.recipients}}})
	if m.listDirty || !m.loading || m.listAttempt != listAttempt+1 {
		t.Fatal("contacts were not refreshed after outstanding reply")
	}
}
