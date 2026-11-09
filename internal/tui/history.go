package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

type historyState struct {
	id                       string
	attempt                  int
	loading, loaded, partial bool
	error                    string
	early                    map[string]ipc.Event
}

func (m *model) loadHistory(force bool) tea.Cmd {
	r, ok := m.recipient()
	if !ok || !m.connected {
		return nil
	}
	k := key(r)
	state := m.historyStates[k]
	if state == nil {
		state = &historyState{}
		m.historyStates[k] = state
	}
	if state.loading || (state.loaded && !force) {
		return nil
	}
	state.loading, state.id, state.error = true, "", ""
	state.attempt++
	state.early = make(map[string]ipc.Event)
	attempt := state.attempt
	m.status = "Đang tải tin nhắn cũ..."
	m.refreshChat(false)
	request := func() tea.Msg {
		id, err := m.client.History(r.ThreadID, r.ThreadType)
		return requestStarted{kind: "history", thread: k, id: id, err: err, attempt: attempt}
	}
	return tea.Batch(request, tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return requestExpired{kind: "history", thread: k, attempt: attempt}
	}))
}

func (m *model) historyStarted(msg requestStarted) {
	state := m.historyStates[msg.thread]
	if state == nil || !state.loading || state.attempt != msg.attempt {
		return
	}
	if msg.err != nil {
		state.loading = false
		state.error = "Không tải được tin cũ: " + msg.err.Error()
		if m.active == msg.thread {
			m.status = state.error
		}
		m.refreshChat(false)
		return
	}
	state.id = msg.id
	if event, ok := state.early[msg.id]; ok {
		m.handleHistory(event)
	}
	state.early = nil
}

func (m *model) handleHistory(event ipc.Event) {
	for k, state := range m.historyStates {
		if !state.loading {
			continue
		}
		if state.id == "" {
			state.early[event.ID] = event
			continue
		}
		if state.id != event.ID {
			continue
		}
		state.loading = false
		if !event.OK {
			state.error = "Không tải được tin cũ: " + event.Error
			if m.active == k {
				m.status = state.error
			}
			m.refreshChat(false)
			return
		}
		if event.ThreadType+":"+event.ThreadID != k {
			state.error = "Daemon trả lịch sử sai hội thoại."
			if m.active == k {
				m.status = state.error
			}
			m.refreshChat(false)
			return
		}
		state.loaded, state.partial, state.error = true, event.Partial, ""
		m.mergeHistory(k, event.Messages)
		if m.active == k {
			m.status = "Đã tải tin văn bản gần đây. F6 để tải lại."
			if event.Partial {
				m.status = "Đã tải một phần tin gần đây Zalo cung cấp. F6 để tải lại."
			}
			m.refreshChat(true)
		}
		return
	}
}

func (m *model) mergeHistory(k string, messages []ipc.Message) {
	seen := make(map[string]bool)
	for _, line := range m.history[k] {
		if line.id != "" {
			seen[line.id] = true
		}
	}
	for _, message := range messages {
		if message.ThreadType+":"+message.ThreadID != k {
			continue
		}
		if message.MsgID != "" && seen[message.MsgID] {
			continue
		}
		if message.MsgID != "" {
			seen[message.MsgID] = true
		}
		m.history[k] = append(m.history[k], &chatLine{
			id: message.MsgID, sender: message.SenderName, text: message.Content,
			self: message.IsSelf, timestamp: message.Timestamp,
		})
	}
	sort.SliceStable(m.history[k], func(i, j int) bool {
		return m.history[k][i].timestamp < m.history[k][j].timestamp
	})
	if len(m.history[k]) > 500 {
		m.history[k] = m.history[k][len(m.history[k])-500:]
	}
}
