package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

func (m *model) copySelectedMedia() {
	if m.editing && m.selectedLine != nil && m.selectedLine.kind == "sticker" && m.selectedLine.stickerID > 0 {
		m.copiedSticker, m.copiedImage = m.selectedLine.stickerID, m.selectedLine.imageURL
		m.status = "Đã chép sticker. Mở chat đích rồi bấm Ctrl+G để gửi."
		return
	}
	if !m.editing || m.selectedLine == nil || m.selectedLine.kind != "image" {
		m.status = "Dùng ↑↓ chọn ảnh hoặc sticker rồi bấm F9."
		return
	}
	if !strings.HasPrefix(m.selectedLine.imageURL, "archive:") {
		m.status = "Ảnh chưa lưu máy. Bấm F9 khi ảnh đã lưu xong."
		return
	}
	m.copiedImage = m.selectedLine.imageURL
	m.copiedSticker = 0
	m.status = "Đã sao chép ảnh. Mở chat đích rồi bấm Ctrl+G để gửi."
}

func (m *model) sendCopiedMedia() tea.Cmd {
	if m.copiedImage == "" && m.copiedSticker == 0 {
		m.status = "Chọn ảnh hoặc sticker rồi bấm F9 để sao chép."
		return nil
	}
	return m.sendMedia(m.copiedImage, m.copiedSticker)
}

func (m *model) sendMedia(image string, stickerID int64) tea.Cmd {
	r, ok := m.recipient()
	if !m.editing || !ok || !m.connected || m.pending != nil {
		m.status = "Mở chat đích đã kết nối và đợi lượt gửi hiện tại kết thúc."
		return nil
	}
	client := m.client
	line := &chatLine{sender: "Bạn", self: true, kind: "image", imageURL: image,
		mediaState: "ready", state: "Đang gửi...", timestamp: time.Now().UnixMilli()}
	if stickerID > 0 {
		line.kind, line.stickerID, line.mediaState = "sticker", stickerID, ""
		return m.beginSend(line, func() (string, error) {
			return client.SendSticker(r.ThreadID, r.ThreadType, stickerID)
		})
	}
	return m.beginSend(line, func() (string, error) {
		return client.SendImage(r.ThreadID, r.ThreadType, image)
	})
}

func (m *model) beginSend(line *chatLine, request func() (string, error)) tea.Cmd {
	m.sendAttempt++
	attempt := m.sendAttempt
	m.pending = &outgoing{thread: m.active, line: line, attempt: attempt, early: make(map[string]ipc.Event)}
	m.appendLine(m.active, line)
	m.refreshChat(true)
	m.status = "Đang chờ Zalo xác nhận..."
	return tea.Batch(func() tea.Msg {
		id, err := request()
		return requestStarted{kind: "send", id: id, err: err, attempt: attempt}
	}, expires("send", attempt))
}
