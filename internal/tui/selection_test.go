package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestArrowSelectsPhotoAndPreviewsInsideChat(t *testing.T) {
	m := fixture()
	m.composer.SetValue("Bản nháp còn nguyên")
	photo := &chatLine{kind: "image", imageURL: "https://example.org/a.png", sender: "An"}
	m.appendLine(m.active, photo)
	m.appendLine(m.active, &chatLine{text: "Tin mới nhất"})
	m.refreshChat(true)
	var calls int
	m.previewImage = func(ctx context.Context, url string, width, height int) (string, error) {
		calls++
		return "\x1b[38;2;255;0;0m▀▀\x1b[0m", nil
	}
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	result, command := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(model)
	if command == nil || m.selectedLine != photo || m.imageMenu {
		t.Fatal("photo was not selected directly in chat")
	}
	m = update(m, command())
	if !strings.Contains(m.chat.View(), "▀▀") || calls != 1 || m.imageOpening {
		t.Fatal("preview did not appear inside chat")
	}
	if m.composer.Value() != "Bản nháp còn nguyên" {
		t.Fatal("selection changed draft")
	}
	if m.ensurePreview(false) != nil {
		t.Fatal("cached preview loaded again")
	}
	for _, size := range [][2]int{{68, 18}, {100, 30}} {
		m.resize(size[0], size[1])
		view := m.View()
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatal("preview overflowed chat")
		}
	}
}

func TestOldPreviewCannotReplaceNewSelection(t *testing.T) {
	m := fixture()
	first := &chatLine{kind: "image", imageURL: "https://example.org/first.png"}
	second := &chatLine{kind: "image", imageURL: "https://example.org/second.png"}
	m.appendLine(m.active, first)
	m.appendLine(m.active, second)
	m.previewImage = func(context.Context, string, int, int) (string, error) { return "▀", nil }
	result, oldCommand := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(model)
	result, newCommand := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(model)
	m = update(m, oldCommand())
	if m.selectedLine != first || second.preview != "" {
		t.Fatal("stale preview was accepted")
	}
	m = update(m, newCommand())
	if first.preview != "▀" || first.previewLoading {
		t.Fatal("selected preview did not finish")
	}
}

func TestResizeCancelsPreviewAndCannotReuseWrongWidth(t *testing.T) {
	m := fixture()
	photo := &chatLine{kind: "image", imageURL: "https://example.org/a.png"}
	m.appendLine(m.active, photo)
	m.selectedLine = photo
	m.previewImage = func(context.Context, string, int, int) (string, error) { return "▀", nil }
	command := m.ensurePreview(false)
	m.resize(68, 18)
	newCommand := m.ensurePreview(false)
	m = update(m, command())
	if photo.preview != "" {
		t.Fatal("resize accepted old preview")
	}
	m = update(m, newCommand())
	if photo.preview != "▀" || photo.previewWidth > m.chat.Width {
		t.Fatal("new preview size was not used")
	}
}
