package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type imageOpened struct{ err error }

func (m model) images() []*chatLine {
	var images []*chatLine
	for _, line := range m.history[m.active] {
		if line.kind == "image" {
			images = append(images, line)
		}
	}
	return images
}

func (m model) imageSelection() ([]*chatLine, int) {
	images := m.images()
	for i, line := range images {
		if line == m.selectedImage {
			return images, i
		}
	}
	return images, -1
}

func (m model) updateImageMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	images, index := m.imageSelection()
	switch msg.String() {
	case "esc", "f8", "ctrl+o":
		m.imageMenu = false
	case "up":
		if index > 0 {
			m.selectedImage = images[index-1]
		}
	case "down":
		if index >= 0 && index < len(images)-1 {
			m.selectedImage = images[index+1]
		}
	case "enter":
		if index < 0 {
			return m, nil
		}
		return m, m.openExternalImage(m.selectedImage)
	}
	return m, nil
}

func (m *model) openExternalImage(line *chatLine) tea.Cmd {
	if m.imageOpening {
		m.status = "Đang mở ảnh trước đó..."
		return nil
	}
	if line.imageURL == "" {
		m.status = "Ảnh này chưa có đường dẫn tải do Zalo cung cấp."
		return nil
	}
	address := line.imageURL
	m.imageMenu, m.imageOpening = false, true
	m.status = "Đang tải và mở ảnh..."
	return func() tea.Msg { return imageOpened{err: m.openImage(m.ctx, address)} }
}

func (m model) imageView() string {
	images, index := m.imageSelection()
	width := min(84, m.width-4)
	rows := max(1, m.height-9)
	start := max(0, index-rows+1)
	title := "ẢNH TRONG CUỘC CHAT"
	if r, ok := m.recipient(); ok {
		title += " · " + oneLine(r.Name)
	}
	lines := []string{bright.Render(clip(title, width-4)), muted.Render("↑↓ chọn · Enter mở · Esc quay lại chat"), ""}
	for i := start; i < min(len(images), start+rows); i++ {
		line := images[i]
		name := oneLine(line.sender)
		if line.self {
			name = "Bạn"
		}
		label := fmt.Sprintf("  %d. %s", i+1, name)
		if line.text != "" {
			label += " · " + oneLine(line.text)
		}
		if line.imageURL == "" {
			label += " · Không có URL"
		}
		if i == index {
			label = "›" + label[1:]
		}
		label = clip(label, width-4)
		if i == index {
			label = bright.Render(label)
		}
		lines = append(lines, label)
	}
	lines = append(lines, "", muted.Render(clip(oneLine(m.status), width-4)))
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).
		Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}
