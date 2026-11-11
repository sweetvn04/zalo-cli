package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
)

type lineRange struct{ start, end int }

func (m *model) scrollMessages(delta int) tea.Cmd {
	previous := m.chat.YOffset
	m.chat.SetYOffset(previous + delta)
	lastVisible := min(m.chat.TotalLineCount()-1, m.chat.YOffset+m.chat.Height-1)
	target := lastVisible
	if bounds, ok := m.lineRanges[m.selectedLine]; ok {
		target = bounds.start + m.chat.YOffset - previous
	}
	target = min(lastVisible, max(m.chat.YOffset, target))
	for _, line := range m.history[m.active] {
		bounds := m.lineRanges[line]
		if bounds.end < target {
			continue
		}
		if m.selectedLine != line {
			m.stopPreview()
			m.selectedLine = line
			m.refreshChat(false)
		}
		break
	}
	preview := m.ensurePreview(false)
	if delta < 0 && m.chat.AtTop() {
		return tea.Batch(preview, m.loadOlder())
	}
	return preview
}

type previewReady struct {
	line                  *chatLine
	thread, address, text string
	attempt               int
	err                   error
}

func (m *model) stopPreview() {
	if m.previewCancel != nil {
		m.previewCancel()
	}
	if m.previewLine != nil {
		m.previewLine.previewLoading = false
		m.previewLine.previewAttempt++
	}
	m.previewCancel, m.previewLine = nil, nil
}

func (m *model) selectMessage(direction int) tea.Cmd {
	lines := m.history[m.active]
	if len(lines) == 0 {
		return nil
	}
	index := -1
	for i, line := range lines {
		if line == m.selectedLine {
			index = i
			break
		}
	}
	if index == 0 && direction < 0 {
		return m.loadOlder()
	}
	if index < 0 {
		index = len(lines) - 1
	} else {
		index = min(len(lines)-1, max(0, index+direction))
	}
	if m.selectedLine != lines[index] {
		m.stopPreview()
	}
	m.selectedLine = lines[index]
	m.refreshChat(false)
	m.ensureSelectedVisible()
	return m.ensurePreview(false)
}

func (m *model) ensureSelectedVisible() {
	bounds, ok := m.lineRanges[m.selectedLine]
	if !ok {
		return
	}
	if bounds.end-bounds.start+1 >= m.chat.Height || bounds.start < m.chat.YOffset {
		m.chat.SetYOffset(bounds.start)
	} else if bounds.end >= m.chat.YOffset+m.chat.Height {
		m.chat.SetYOffset(bounds.end - m.chat.Height + 1)
	}
}

func (m *model) ensurePreview(retry bool) tea.Cmd {
	line := m.selectedLine
	if line == nil || (line.kind != "image" && line.kind != "sticker") || m.width < 68 || m.height < 18 {
		return nil
	}
	if line.imageURL == "" {
		if line.kind == "sticker" {
			return nil
		}
		line.previewError = "Ảnh chưa có đường dẫn tải. F6 để cập nhật."
		m.refreshChat(false)
		m.ensureSelectedVisible()
		return nil
	}
	width, height := min(80, m.chat.Width), max(1, min(24, m.chat.Height-5))
	if line.kind == "sticker" {
		width, height = min(width, 20), min(height, 8)
	}
	if line.previewLoading && line.previewWidth == width && line.previewHeight == height {
		return nil
	}
	if line.preview != "" && line.previewWidth == width && line.previewHeight == height {
		return nil
	}
	if line.previewError != "" && !retry {
		return nil
	}
	m.stopPreview()
	line.preview = ""
	line.previewAttempt++
	line.previewWidth, line.previewHeight = width, height
	line.previewLoading, line.previewError = true, ""
	ctx, cancel := context.WithCancel(m.ctx)
	m.previewCancel, m.previewLine = cancel, line
	address, attempt, thread := line.imageURL, line.previewAttempt, m.active
	m.refreshChat(false)
	m.ensureSelectedVisible()
	return func() tea.Msg {
		text, err := m.previewImage(ctx, address, width, height)
		return previewReady{line: line, thread: thread, address: address, attempt: attempt, text: text, err: err}
	}
}

func (m *model) acceptPreview(msg previewReady) {
	line := msg.line
	if line.previewAttempt != msg.attempt || line.imageURL != msg.address {
		return
	}
	line.previewLoading = false
	if m.previewLine == line {
		if m.previewCancel != nil {
			m.previewCancel()
		}
		m.previewCancel, m.previewLine = nil, nil
	}
	line.previewError = ""
	if msg.err != nil {
		if !errors.Is(msg.err, context.Canceled) {
			line.previewError = msg.err.Error()
		}
	} else {
		line.preview = msg.text
	}
	if m.active == msg.thread && m.selectedLine == line {
		m.refreshChat(false)
		m.ensureSelectedVisible()
	}
}
