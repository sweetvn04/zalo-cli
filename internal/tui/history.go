package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

type historyState struct {
	archiveCursor            string
	cursor                   string
	more, older              bool
	id                       string
	attempt                  int
	loading, loaded, partial bool
	error                    string
	early                    map[string]ipc.Event
}

func (m *model) loadHistory(force bool) tea.Cmd {
	return m.loadHistoryPage(force, false)
}

func (m *model) loadOlder() tea.Cmd {
	state := m.historyStates[m.active]
	if state == nil || !state.loaded || state.loading {
		return nil
	}
	if !state.more || (state.cursor == "" && state.archiveCursor == "") {
		m.status = "Đã tới đầu các tin đã tải. F6 để cập nhật từ kho."
		return nil
	}
	return m.loadHistoryPage(true, true)
}

func (m *model) loadHistoryPage(force, older bool) tea.Cmd {
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
	state.older = older
	state.attempt++
	state.early = make(map[string]ipc.Event)
	attempt := state.attempt
	m.status = "Đang tải tin nhắn cũ..."
	beforeID := ""
	archiveBefore := ""
	if older {
		beforeID = state.cursor
		archiveBefore = state.archiveCursor
		m.status = "Đang tải thêm tin cũ..."
	}
	m.refreshChat(false)
	request := func() tea.Msg {
		id, err := m.client.History(r.ThreadID, r.ThreadType, beforeID, archiveBefore)
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
		initial := !state.loaded
		state.loaded, state.partial, state.error = true, event.Partial, ""
		state.cursor, state.more = event.Cursor, event.HasMore
		state.archiveCursor = event.ArchiveCursor
		oldHeight, oldOffset := m.chat.TotalLineCount(), m.chat.YOffset
		atBottom := m.chat.AtBottom()
		m.mergeHistory(k, event.Messages)
		if m.active == k {
			m.status = "Đã tải tin cũ. Cuộn chuột hoặc PgUp/PgDn để đọc."
			if state.more {
				m.status = "Cuộn lên đầu hoặc F7 để tải thêm tin cũ."
			} else if event.Partial {
				m.status = "Zalo chưa cung cấp thêm tin cũ. F6 để thử tải lại."
			}
			if event.Notice != "" {
				m.status = event.Notice
			}
			m.refreshChat(initial || (!state.older && atBottom))
			if state.older {
				// Tin được thêm phía trên: giữ nguyên đoạn người dùng đang đọc.
				m.chat.SetYOffset(oldOffset + max(0, m.chat.TotalLineCount()-oldHeight))
			}
		}
		return
	}
}

func (m *model) mergeHistory(k string, messages []ipc.Message) {
	seen := make(map[string]*chatLine)
	for _, line := range m.history[k] {
		if line.id != "" {
			seen[line.id] = line
		}
	}
	for _, message := range messages {
		if message.ThreadType+":"+message.ThreadID != k {
			continue
		}
		if existing := seen[message.MsgID]; message.MsgID != "" && existing != nil {
			if message.Kind == "image" && message.ImageSource() != "" {
				if existing.imageURL != message.ImageSource() {
					if m.previewLine == existing {
						m.stopPreview()
					}
					existing.preview, existing.previewError = "", ""
					existing.previewAttempt++
				}
				existing.kind, existing.imageURL = message.Kind, message.ImageSource()
				existing.mediaState = message.ImageState
			}
			continue
		}
		line := &chatLine{
			id: message.MsgID, sender: message.SenderName, text: message.Content,
			self: message.IsSelf, timestamp: message.Timestamp,
			kind: message.Kind, imageURL: message.ImageSource(), mediaState: message.ImageState,
		}
		m.history[k] = append(m.history[k], line)
		if message.MsgID != "" {
			seen[message.MsgID] = line
		}
	}
	sort.SliceStable(m.history[k], func(i, j int) bool {
		return m.history[k][i].timestamp < m.history[k][j].timestamp
	})
}
