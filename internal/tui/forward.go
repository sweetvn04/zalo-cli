package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

func (m *model) copySelectedImage() {
	if !m.editing || m.selectedLine == nil || m.selectedLine.kind != "image" {
		m.status = "Dùng ↑↓ chọn ảnh trong chat rồi bấm F9."
		return
	}
	if !strings.HasPrefix(m.selectedLine.imageURL, "archive:") {
		m.status = "Ảnh chưa lưu máy. Bấm F9 khi ảnh đã lưu xong."
		return
	}
	m.copiedImage = m.selectedLine.imageURL
	m.status = "Đã sao chép ảnh. Mở chat đích rồi bấm Ctrl+G để gửi."
}

func (m *model) sendCopiedImage() tea.Cmd {
	if m.copiedImage == "" {
		m.status = "Chọn ảnh rồi bấm F9 để sao chép."
		return nil
	}
	r, ok := m.recipient()
	if !m.editing || !ok || !m.connected || m.pending != nil {
		m.status = "Mở chat đích đã kết nối và đợi lượt gửi hiện tại kết thúc."
		return nil
	}
	client, image := m.client, m.copiedImage
	line := &chatLine{sender: "Bạn", self: true, kind: "image", imageURL: image,
		mediaState: "ready", state: "Đang gửi...", timestamp: time.Now().UnixMilli()}
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
