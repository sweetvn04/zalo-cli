package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

// transport giữ UI độc lập với socket để có thể thử luồng giao diện cục bộ.
type transport interface {
	Results() <-chan ipc.Result
	ListRecipients() (string, error)
	SendMessage(string, string, string) (string, error)
}

type packet struct {
	result ipc.Result
	open   bool
}

type requestStarted struct {
	kind    string
	id      string
	err     error
	attempt int
}

type requestExpired struct {
	kind    string
	attempt int
}

type chatLine struct {
	id, sender, text, state string
	self                    bool
}

type outgoing struct {
	id      string
	line    *chatLine
	attempt int
	early   map[string]ipc.Event
}

type model struct {
	client        transport
	recipients    []ipc.Recipient
	cursor        int
	active        string
	history       map[string][]*chatLine
	drafts        map[string]string
	unread        map[string]int
	filter        textinput.Model
	composer      textinput.Model
	chat          viewport.Model
	editing       bool
	connected     bool
	loading       bool
	listID        string
	listAttempt   int
	earlyList     map[string]ipc.Event
	pending       *outgoing
	sendAttempt   int
	status        string
	width, height int
}

func Run(ctx context.Context, url string) error {
	client, err := ipc.Dial(ctx, url)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = tea.NewProgram(newModel(client), tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func newModel(client transport) model {
	filter := textinput.New()
	filter.Placeholder = "Tìm tên bạn bè, nhóm..."
	filter.Prompt = "⌕ "
	filter.CharLimit = 100
	filter.Focus()
	composer := textinput.New()
	composer.Placeholder = "Nhập tin nhắn..."
	composer.Prompt = "> "
	composer.CharLimit = 2000
	m := model{
		client: client, filter: filter, composer: composer,
		chat: viewport.New(40, 10), connected: true,
		history: make(map[string][]*chatLine), drafts: make(map[string]string),
		unread: make(map[string]int), status: "Đang tải bạn bè và nhóm...",
		loading: true, listAttempt: 1, earlyList: make(map[string]ipc.Event),
	}
	m.resize(100, 30)
	return m
}

func key(r ipc.Recipient) string { return r.ThreadType + ":" + r.ThreadID }

func (m model) Init() tea.Cmd {
	return tea.Batch(m.listen(), m.loadList(), expires("list", m.listAttempt), textinput.Blink)
}

func (m model) listen() tea.Cmd {
	return func() tea.Msg {
		result, ok := <-m.client.Results()
		return packet{result, ok}
	}
}

func (m model) loadList() tea.Cmd {
	return func() tea.Msg {
		id, err := m.client.ListRecipients()
		return requestStarted{kind: "list", id: id, err: err, attempt: m.listAttempt}
	}
}

func expires(kind string, attempt int) tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg { return requestExpired{kind, attempt} })
}

func (m model) visible() []ipc.Recipient {
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var result []ipc.Recipient
	for _, r := range m.recipients {
		if strings.Contains(strings.ToLower(r.Name), query) {
			result = append(result, r)
		}
	}
	return result
}

func (m model) recipient() (ipc.Recipient, bool) {
	for _, r := range m.recipients {
		if key(r) == m.active {
			return r, true
		}
	}
	return ipc.Recipient{}, false
}

func (m *model) openSelected() tea.Cmd {
	items := m.visible()
	if m.cursor >= len(items) {
		return nil
	}
	if m.active != "" {
		m.drafts[m.active] = m.composer.Value()
	}
	m.active = key(items[m.cursor])
	m.unread[m.active] = 0
	m.composer.SetValue(m.drafts[m.active])
	m.editing = true
	m.filter.Blur()
	m.refreshChat(true)
	return m.composer.Focus()
}

func (m *model) finishSend(ok bool, message string) {
	if m.pending == nil {
		return
	}
	if ok {
		m.pending.line.state = "Đã gửi"
		m.status = "Zalo đã chấp nhận gửi tin."
	} else {
		m.pending.line.state = message
		m.status = message
	}
	m.pending = nil
	m.refreshChat(false)
}

func (m *model) handleEvent(event ipc.Event) {
	switch event.Event {
	case "recipients_result":
		if !m.loading {
			return
		}
		if m.listID == "" {
			// Phản hồi có thể tới trước requestStarted trong event loop.
			m.earlyList[event.ID] = event
			return
		}
		if event.ID != m.listID {
			return
		}
		m.loading = false
		if !event.OK {
			m.status = "Không tải được danh sách: " + event.Error
			return
		}
		// Giữ người lạ vừa nhắn tới dù họ không nằm trong danh sách bạn bè.
		known := make(map[string]bool)
		for _, r := range event.Recipients {
			known[key(r)] = true
		}
		for _, r := range m.recipients {
			if !known[key(r)] {
				event.Recipients = append(event.Recipients, r)
			}
		}
		m.recipients = event.Recipients
		m.cursor = 0
		m.status = fmt.Sprintf("Đã tải %d người nhận. Chọn tên rồi Enter.", len(m.recipients))
	case "new_message":
		msg := event.Data
		r := ipc.Recipient{ThreadID: msg.ThreadID, ThreadType: msg.ThreadType, Name: msg.SenderName}
		if r.ThreadID == "" {
			return
		}
		if r.ThreadType == "group" {
			r.Name = "Nhóm " + r.ThreadID
		}
		k := key(r)
		for _, line := range m.history[k] {
			if msg.MsgID != "" && line.id == msg.MsgID {
				return
			}
		}
		found := false
		for _, existing := range m.recipients {
			if key(existing) == k {
				found = true
				break
			}
		}
		if !found {
			m.recipients = append(m.recipients, r)
		}
		m.appendLine(k, &chatLine{id: msg.MsgID, sender: msg.SenderName, text: msg.Content, self: msg.IsSelf})
		if k != m.active {
			m.unread[k]++
		} else {
			m.refreshChat(m.chat.AtBottom())
		}
	case "send_result":
		if m.pending == nil {
			return
		}
		if m.pending.id == "" {
			m.pending.early[event.ID] = event
		} else if event.ID == m.pending.id {
			message := "Gửi thất bại: " + event.Error
			m.finishSend(event.OK, message)
		}
	}
}

func (m *model) appendLine(k string, line *chatLine) {
	m.history[k] = append(m.history[k], line)
	if len(m.history[k]) > 500 {
		m.history[k] = m.history[k][len(m.history[k])-500:]
	}
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case packet:
		if !msg.open {
			m.connected = false
			m.loading = false
			m.finishSend(false, "Chưa rõ kết quả gửi; kiểm tra Zalo trước khi gửi lại.")
			m.status = "Đã mất kết nối daemon. Esc để thoát, khởi động lại daemon rồi mở lại giao diện."
			return m, nil
		}
		if msg.result.Err != nil {
			m.status = msg.result.Err.Error()
		} else {
			m.handleEvent(msg.result.Event)
		}
		return m, m.listen()
	case requestStarted:
		if msg.kind == "list" && m.loading && msg.attempt == m.listAttempt {
			if msg.err != nil {
				m.loading = false
				m.status = msg.err.Error()
			} else {
				m.listID = msg.id
				if event, ok := m.earlyList[msg.id]; ok {
					m.handleEvent(event)
				}
				m.earlyList = nil
			}
		} else if msg.kind == "send" && m.pending != nil && msg.attempt == m.pending.attempt {
			if msg.err != nil {
				m.finishSend(false, "Chưa xác nhận gửi: "+msg.err.Error())
			} else {
				m.pending.id = msg.id
				if event, ok := m.pending.early[msg.id]; ok {
					m.handleEvent(event)
				}
			}
		}
		return m, nil
	case requestExpired:
		if msg.kind == "send" && m.pending != nil && m.pending.attempt == msg.attempt {
			m.finishSend(false, "Chưa rõ kết quả gửi; kiểm tra Zalo trước khi gửi lại.")
		} else if msg.kind == "list" && m.loading && m.listAttempt == msg.attempt {
			m.loading = false
			m.status = "Hết thời gian tải danh sách. F5 để thử lại."
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
		if m.width < 68 || m.height < 18 {
			return m, nil
		}
		switch msg.String() {
		case "tab", "shift+tab":
			if !m.editing {
				return m, m.openSelected()
			}
			m.editing = false
			m.composer.Blur()
			return m, m.filter.Focus()
		case "f5":
			if m.connected && !m.loading {
				m.loading, m.listID = true, ""
				m.earlyList = make(map[string]ipc.Event)
				m.listAttempt++
				m.status = "Đang tải lại danh sách..."
				return m, tea.Batch(m.loadList(), expires("list", m.listAttempt))
			}
			return m, nil
		case "pgup", "pgdown", "ctrl+u", "ctrl+d":
			if msg.String() == "pgup" || msg.String() == "ctrl+u" {
				m.chat.HalfViewUp()
			} else {
				m.chat.HalfViewDown()
			}
			return m, nil
		case "up":
			if !m.editing {
				m.cursor = max(0, m.cursor-1)
				return m, nil
			}
		case "down":
			if !m.editing {
				m.cursor = min(max(0, len(m.visible())-1), m.cursor+1)
				return m, nil
			}
		case "enter":
			if !m.editing {
				return m, m.openSelected()
			}
			r, ok := m.recipient()
			text := m.composer.Value()
			if !ok || !m.connected || m.pending != nil || strings.TrimSpace(text) == "" {
				return m, nil
			}
			m.sendAttempt++
			line := &chatLine{sender: "Bạn", text: text, self: true, state: "Đang gửi..."}
			m.pending = &outgoing{line: line, attempt: m.sendAttempt, early: make(map[string]ipc.Event)}
			m.appendLine(m.active, line)
			m.composer.Reset()
			m.drafts[m.active] = ""
			m.refreshChat(true)
			m.status = "Đang chờ Zalo xác nhận..."
			send := func() tea.Msg {
				id, err := m.client.SendMessage(r.ThreadID, r.ThreadType, text)
				return requestStarted{kind: "send", id: id, err: err, attempt: m.sendAttempt}
			}
			return m, tea.Batch(send, expires("send", m.sendAttempt))
		}
	}
	var cmd tea.Cmd
	if m.editing {
		m.composer, cmd = m.composer.Update(message)
	} else {
		previous := m.filter.Value()
		m.filter, cmd = m.filter.Update(message)
		if previous != m.filter.Value() {
			m.cursor = 0
		}
	}
	return m, cmd
}
