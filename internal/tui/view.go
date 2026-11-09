package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	accent = lipgloss.Color("111")
	green  = lipgloss.Color("79")
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	bright = lipgloss.NewStyle().Foreground(accent).Bold(true)
)

// Tin nhắn và tên đến từ mạng không được chứa lệnh điều khiển terminal.
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(text))
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(clean(text)), " ")
}

func clip(text string, width int) string {
	return ansi.Truncate(text, max(1, width), "…")
}

func (m model) sidebarWidth() int { return min(34, max(24, m.width/3)) }

func (m *model) resize(width, height int) {
	m.stopPreview()
	m.width, m.height = width, height
	right := max(10, width-m.sidebarWidth()-4)
	m.chat.Width = right
	m.chat.Height = max(1, height-11)
	m.composer.Width = max(1, right-3)
	m.filter.Width = max(1, m.sidebarWidth()-7)
	m.refreshChat(false)
}

func (m *model) refreshChat(bottom bool) {
	var lines []string
	m.lineRanges = make(map[*chatLine]lineRange)
	row := 0
	imageNumber := 0
	for _, line := range m.history[m.active] {
		var parts []string
		name := oneLine(line.sender)
		if name == "" {
			name = "Người gửi"
		}
		style := lipgloss.NewStyle().Foreground(accent).Bold(true)
		if line.self {
			name = "Bạn"
			style = style.Foreground(green)
		}
		if line == m.selectedLine {
			name = "› " + name
		} else {
			name = "  " + name
		}
		parts = append(parts, style.Render(clip(name, m.chat.Width)))
		if line.kind == "image" {
			imageNumber++
			parts = append(parts, bright.Render(clip(fmt.Sprintf("[Ảnh %d] · ↑↓ chọn để xem", imageNumber), m.chat.Width)))
			if line == m.selectedLine {
				switch {
				case line.previewLoading:
					parts = append(parts, muted.Render("Đang tải ảnh..."))
				case line.previewError != "":
					parts = append(parts, muted.Render(ansi.Hardwrap(clean(line.previewError)+" · Enter thử lại", m.chat.Width, false)))
				case line.preview != "" && line.previewWidth <= m.chat.Width:
					parts = append(parts, line.preview)
				}
			}
		}
		if line.text != "" {
			parts = append(parts, ansi.Hardwrap(clean(line.text), m.chat.Width, false))
		}
		if line.state != "" {
			parts = append(parts, muted.Render(ansi.Hardwrap(clean(line.state), m.chat.Width, false)))
		}
		parts = append(parts, "")
		content := strings.Join(parts, "\n")
		height := strings.Count(content, "\n") + 1
		m.lineRanges[line] = lineRange{start: row, end: row + height - 1}
		row += height
		lines = append(lines, content)
	}
	if len(lines) == 0 {
		text := "Chưa có tin nhắn."
		if state := m.historyStates[m.active]; state != nil {
			if state.loading {
				text = "Đang tải tin nhắn cũ..."
			} else if state.error != "" {
				text = state.error
			} else if state.loaded {
				text = "Không có tin nhắn trong dữ liệu Zalo trả về."
			}
		}
		lines = []string{muted.Render(ansi.Hardwrap(text, m.chat.Width, false)), "", muted.Render("Tin mới sẽ hiện ở đây.")}
	}
	m.chat.SetContent(strings.Join(lines, "\n"))
	if bottom {
		m.chat.GotoBottom()
	}
}

func (m model) View() string {
	if m.width < 68 || m.height < 18 {
		return clip("Cửa sổ cần ít nhất 68 × 18. Phóng to terminal hoặc Esc để thoát.", m.width)
	}
	if m.imageMenu {
		return m.imageView()
	}
	connection := lipgloss.NewStyle().Foreground(green).Render("● Đã kết nối")
	if !m.connected {
		connection = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("● Mất kết nối")
	}
	header := bright.Render(" ZALO") + "  " + muted.Render("TERMINAL") + "    " + connection
	header += "\n" + muted.Render(" Tin nhắn & ảnh · Bạn bè & nhóm")
	bodyHeight := m.height - 4
	leftWidth := m.sidebarWidth() - 4
	innerHeight := bodyHeight - 2
	items := m.visible()
	rows := innerHeight - 4
	start := max(0, m.cursor-rows+1)
	var sidebar []string
	sidebar = append(sidebar, bright.Render("NGƯỜI NHẬN"), clip(m.filter.View(), leftWidth), "")
	for i := start; i < min(len(items), start+rows); i++ {
		r := items[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		label := oneLine(r.Name)
		if label == "" {
			label = r.ThreadID
		}
		if r.ThreadType == "group" {
			label = "# " + label
		}
		if m.unread[key(r)] > 0 {
			label += fmt.Sprintf(" (%d)", m.unread[key(r)])
		}
		line := clip(prefix+label, leftWidth)
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(accent).Bold(true).Render(line)
		}
		sidebar = append(sidebar, line)
	}
	if len(items) == 0 {
		empty := "Không có kết quả."
		if m.loading {
			empty = "Đang tải..."
		}
		sidebar = append(sidebar, muted.Render(empty))
	}
	// Đặt số lượng ở đáy cột, không làm chiều cao thay đổi theo danh sách.
	for len(sidebar) < innerHeight-1 {
		sidebar = append(sidebar, "")
	}
	sidebar = append(sidebar, muted.Render(clip(fmt.Sprintf("%d người nhận · F5 tải lại", len(items)), leftWidth)))
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	leftColor, rightColor := lipgloss.Color("240"), accent
	if !m.editing {
		leftColor, rightColor = accent, lipgloss.Color("240")
	}
	left := panel.BorderForeground(leftColor).Width(leftWidth + 2).Height(innerHeight).
		Render(strings.Join(sidebar, "\n"))
	rightWidth := m.width - m.sidebarWidth() - 4
	title, subtitle := "Chọn một người nhận", "↑ ↓ chọn tên · Enter mở cuộc chat"
	if r, ok := m.recipient(); ok {
		title = oneLine(r.Name)
		subtitle = "Cá nhân"
		if r.ThreadType == "group" {
			subtitle = "Nhóm"
		}
		subtitle += fmt.Sprintf(" · %d tin · %.0f%% · F7 thêm", len(m.history[m.active]), m.chat.ScrollPercent()*100)
		if state := m.historyStates[m.active]; state != nil {
			if state.loading {
				subtitle = "Đang tải tin nhắn cũ..."
			} else if state.partial {
				subtitle += " · Một phần lịch sử"
			}
		}
	}
	input := m.composer.View()
	if !m.connected {
		input = muted.Render("Kết nối đã đóng. Esc để thoát.")
	} else if m.active == "" {
		input = muted.Render("Chọn người nhận để bắt đầu nhắn tin.")
	}
	chat := strings.Join([]string{
		bright.Render(clip(title, rightWidth)), muted.Render(clip(subtitle, rightWidth)), "",
		m.chat.View(), muted.Render(strings.Repeat("─", rightWidth)), clip(input, rightWidth),
	}, "\n")
	right := panel.BorderForeground(rightColor).Width(rightWidth + 2).Height(innerHeight).Render(chat)
	footer := clip(" "+oneLine(m.status), m.width)
	help := muted.Render(clip(" ↑↓ chọn tin/ảnh · Chuột/PgUp cuộn · Ctrl+O ảnh gốc · Tab đổi cột · Enter gửi · F7 tin cũ · Esc thoát", m.width))
	return header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, right) + "\n" + footer + "\n" + help
}
