package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zalo-cli/internal/ipc"
)

type stickerPicker struct {
	items                   []ipc.Message
	index, version, attempt int
	loading                 bool
	preview, status         string
	cancel                  context.CancelFunc
}

type stickerPreview struct {
	menu    *stickerPicker
	version int
	text    string
}

func (m *model) openStickerMenu() tea.Cmd {
	if _, ok := m.recipient(); !ok || !m.editing || !m.connected {
		m.status = "Mở cuộc trò chuyện muốn gửi sticker rồi bấm F4."
		return nil
	}
	m.stickerAttempt++
	attempt, client := m.stickerAttempt, m.client
	m.stickerMenu = &stickerPicker{loading: true, attempt: attempt, status: "Đang tải sticker đã lưu..."}
	return tea.Batch(func() tea.Msg {
		id, err := client.ListStickers()
		return requestStarted{kind: "stickers", id: id, err: err, attempt: attempt}
	}, expires("stickers", attempt))
}

func (m *model) receiveStickers(event ipc.Event) tea.Cmd {
	menu := m.stickerMenu
	if menu == nil || !menu.loading {
		return nil
	}
	menu.loading = false
	if !event.OK {
		menu.status = event.Error
		return nil
	}
	menu.items = event.Messages
	if len(menu.items) == 0 {
		menu.status = "Kho chat chưa có sticker."
		return nil
	}
	return m.previewSticker()
}

func (m *model) previewSticker() tea.Cmd {
	menu := m.stickerMenu
	if menu.cancel != nil {
		menu.cancel()
	}
	menu.version++
	menu.preview, menu.status = "", ""
	address := menu.items[menu.index].ImageSource()
	if address == "" {
		menu.status = "Chưa có ảnh xem trước."
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	menu.cancel = cancel
	version, render := menu.version, m.previewImage
	height := max(1, min(8, m.height-11))
	return func() tea.Msg {
		defer cancel()
		text, err := render(ctx, address, 20, height)
		if err != nil {
			text = "Không mở được ảnh xem trước."
		}
		return stickerPreview{menu: menu, version: version, text: text}
	}
}

func (m *model) updateStickerMenu(key string) tea.Cmd {
	menu := m.stickerMenu
	if key == "esc" || key == "f4" {
		if menu.cancel != nil {
			menu.cancel()
		}
		m.stickerMenu = nil
		return nil
	}
	if len(menu.items) == 0 {
		return nil
	}
	previous := menu.index
	switch key {
	case "up", "left":
		menu.index = max(0, menu.index-1)
	case "down", "right":
		menu.index = min(len(menu.items)-1, menu.index+1)
	case "enter":
		sticker := menu.items[menu.index]
		if cmd := m.sendMedia(sticker.ImageSource(), sticker.StickerID); cmd != nil {
			m.updateStickerMenu("esc")
			return cmd
		}
		menu.status = m.status
	}
	if menu.index != previous {
		return m.previewSticker()
	}
	return nil
}

func (m model) stickerView() string {
	menu := m.stickerMenu
	width := min(58, m.width-4)
	r, _ := m.recipient()
	lines := []string{bright.Render(clip("Sticker · "+oneLine(r.Name), width-4)),
		muted.Render("↑↓ chọn · Enter gửi · Esc đóng"), ""}
	if len(menu.items) > 0 {
		lines = append(lines, fmt.Sprintf("%d/%d", menu.index+1, len(menu.items)), menu.preview)
	}
	if menu.status != "" {
		lines = append(lines, muted.Render(clip(oneLine(menu.status), width-4)))
	}
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).
		Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}
