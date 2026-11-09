package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zalo-cli/internal/ipc"
)

func TestImageMenuSelectsStableMessageAndPreservesDraft(t *testing.T) {
	m := fixture()
	m.composer.SetValue("Bản nháp")
	m.handleEvent(ipc.Event{Event: "new_message", Data: ipc.Message{MsgID: "photo-1", ThreadID: "an", ThreadType: "user", Kind: "image", ImageURL: "https://example.org/first.png", Content: "Ảnh thứ nhất"}})
	m.handleEvent(ipc.Event{Event: "new_message", Data: ipc.Message{MsgID: "photo-2", ThreadID: "an", ThreadType: "user", Kind: "image", ImageURL: "https://example.org/second.png"}})
	m = update(m, tea.KeyMsg{Type: tea.KeyF8})
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	selected := m.selectedImage
	m.mergeHistory("user:an", []ipc.Message{{MsgID: "older", ThreadID: "an", ThreadType: "user", Kind: "image", ImageURL: "https://example.org/older.png", Timestamp: -1}})
	if m.selectedImage != selected {
		t.Fatal("prepending history changed selected image")
	}
	var opened string
	m.openImage = func(ctx context.Context, address string) error { opened = address; return nil }
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(model)
	if command == nil {
		t.Fatal("opening image did not produce a command")
	}
	m = update(m, command())
	if opened != "https://example.org/first.png" || m.composer.Value() != "Bản nháp" || m.imageMenu || m.imageOpening {
		t.Fatal("image opening changed draft or opened wrong image")
	}
	if !strings.Contains(m.chat.View(), "Ảnh") {
		t.Fatal("image placeholder missing")
	}
}

func TestImageMenuFitsAndEscapeReturnsToChat(t *testing.T) {
	m := fixture()
	for i := 0; i < 60; i++ {
		m.appendLine(m.active, &chatLine{kind: "image", sender: "Bảo", text: strings.Repeat("Ảnh ", 100)})
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyF8})
	for _, size := range [][2]int{{68, 18}, {100, 30}, {160, 40}} {
		m.resize(size[0], size[1])
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatal("image menu overflows terminal")
		}
	}
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result.(model).imageMenu || command != nil {
		t.Fatal("Escape must return from images to chat")
	}
}

func TestRefreshingPhotoUpdatesExpiredURLWithoutDuplicate(t *testing.T) {
	m := fixture()
	m.appendLine(m.active, &chatLine{id: "photo", kind: "image", imageURL: "https://example.org/expired"})
	m.mergeHistory(m.active, []ipc.Message{{MsgID: "photo", ThreadID: "an", ThreadType: "user", Kind: "image", ImageURL: "https://example.org/new"}})
	if len(m.history[m.active]) != 1 || m.history[m.active][0].imageURL != "https://example.org/new" {
		t.Fatal("photo refresh failed")
	}
}
