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
	"zalo-cli/internal/media"
)

// transport giữ UI độc lập với socket để có thể thử luồng giao diện cục bộ.
type transport interface {
	Results() <-chan ipc.Result
	ListRecipients() (string, error)
	SendMessage(string, string, string, ...string) (string, error)
	SendImage(string, string, string) (string, error)
	SendSticker(string, string, int64) (string, error)
	ListStickers() (string, error)
	SyncNow(string, string) (string, error)
	History(string, string, ...string) (string, error)
}

type packet struct {
	result ipc.Result
	open   bool
}

type requestStarted struct {
	thread  string
	kind    string
	id      string
	err     error
	attempt int
}

type requestExpired struct {
	thread  string
	kind    string
	attempt int
}

type chatLine struct {
	replyPreview                                string
	stickerID                                   int64
	mediaState                                  string
	preview, previewError                       string
	previewLoading                              bool
	previewWidth, previewHeight, previewAttempt int
	kind, imageURL                              string
	timestamp                                   int64
	id, sender, text, state                     string
	self                                        bool
}

type outgoing struct {
	thread  string
	id      string
	line    *chatLine
	attempt int
	early   map[string]ipc.Event
}

type model struct {
	zaloOnline              bool
	replies                 map[string]*chatLine
	copiedImage             string
	copiedSticker           int64
	stickerMenu             *stickerPicker
	stickerAttempt          int
	previewImage            func(context.Context, string, int, int) (string, error)
	previewCancel           context.CancelFunc
	previewLine             *chatLine
	selectedLine            *chatLine
	lineRanges              map[*chatLine]lineRange
	ctx                     context.Context
	openImage               func(context.Context, string) error
	imageMenu, imageOpening bool
	selectedImage           *chatLine
	client                  transport
	recipients              []ipc.Recipient
	cursor                  int
	active                  string
	history                 map[string][]*chatLine
	historyStates           map[string]*historyState
	historyDirty            map[string]bool
	listDirty               bool
	drafts                  map[string]string
	unread                  map[string]int
	filter                  textinput.Model
	composer                textinput.Model
	chat                    viewport.Model
	editing                 bool
	connected               bool
	loading                 bool
	listID                  string
	listNextLoad            time.Time
	listAttempt             int
	earlyList               map[string]ipc.Event
	pending                 *outgoing
	sendAttempt             int
	status                  string
	width, height           int
}

func Run(ctx context.Context, url string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client, err := ipc.Dial(ctx, url)
	if err != nil {
		return err
	}
	defer client.Close()
	m := newModel(client)
	m.ctx = ctx
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx)).Run()
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
		ctx: context.Background(), openImage: media.OpenImage, previewImage: media.Preview,
		lineRanges: make(map[*chatLine]lineRange),
		client:     client, filter: filter, composer: composer,
		chat: viewport.New(40, 10), connected: true,
		history: make(map[string][]*chatLine), drafts: make(map[string]string),
		replies:       make(map[string]*chatLine),
		historyStates: make(map[string]*historyState),
		historyDirty:  make(map[string]bool),
		unread:        make(map[string]int), status: "Đang tải bạn bè và nhóm...",
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
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg { return requestExpired{kind: kind, attempt: attempt} })
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
	if m.active != key(items[m.cursor]) {
		m.stopPreview()
		m.selectedLine = nil
	}
	m.active = key(items[m.cursor])
	m.unread[m.active] = 0
	m.composer.SetValue(m.drafts[m.active])
	m.editing = true
	m.filter.Blur()
	m.refreshChat(true)
	return tea.Batch(m.composer.Focus(), m.loadHistory(false))
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
	case "zalo_connection":
		m.zaloOnline = event.Data.Online
	case "archive_status":
		m.status = event.Data.Notice
	case "media_updated":
		msg := event.Data
		k := msg.ThreadType + ":" + msg.ThreadID
		m.mergeHistory(k, []ipc.Message{msg})
		if m.active == k {
			m.refreshChat(false)
		}
	case "history_result":
		m.handleHistory(event)
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
		if event.RetryAfter > 0 {
			m.listNextLoad = time.Now().Add(time.Duration(event.RetryAfter) * time.Second)
		}
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
		if event.Notice != "" {
			m.status = event.Notice
		}
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
		m.appendLine(k, &chatLine{id: msg.MsgID, sender: msg.SenderName, text: msg.Content, self: msg.IsSelf, timestamp: msg.Timestamp, kind: msg.Kind, imageURL: msg.ImageSource(), mediaState: msg.ImageState, stickerID: msg.StickerID, replyPreview: msg.ReplyPreview})
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
			if event.OK && event.MsgID != "" {
				m.pending.line.id = event.MsgID
				// Lịch sử có thể chứa tin này trước khi phản hồi gửi về tới UI.
				var lines []*chatLine
				for _, line := range m.history[m.pending.thread] {
					if line == m.pending.line || line.id != event.MsgID {
						lines = append(lines, line)
					}
				}
				m.history[m.pending.thread] = lines
			}
			message := "Gửi thất bại: " + event.Error
			m.finishSend(event.OK, message)
			if event.Notice != "" {
				m.status = event.Notice
			}
		}
	}
}

func (m *model) appendLine(k string, line *chatLine) {
	m.history[k] = append(m.history[k], line)
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case previewReady:
		m.acceptPreview(msg)
		return m, nil
	case imageOpened:
		m.imageOpening = false
		if msg.err != nil {
			m.status = "Không mở được media: " + msg.err.Error()
		} else {
			m.status = "Đã mở media bằng ứng dụng mặc định."
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, m.ensurePreview(false)
	case tea.MouseMsg:
		if m.imageMenu || m.stickerMenu != nil {
			return m, nil
		}
		if m.width < 68 || m.height < 18 {
			return m, nil
		}
		if msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown {
			return m, nil
		}
		if msg.X < m.sidebarWidth() {
			if msg.Button == tea.MouseButtonWheelUp {
				m.cursor = max(0, m.cursor-1)
			} else {
				m.cursor = min(max(0, len(m.visible())-1), m.cursor+1)
			}
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp {
			cmd := m.scrollMessages(-3)
			return m, cmd
		}
		cmd := m.scrollMessages(3)
		return m, cmd
	case packet:
		if !msg.open {
			m.connected = false
			m.loading = false
			for _, state := range m.historyStates {
				state.loading = false
			}
			m.finishSend(false, "Chưa rõ kết quả gửi; kiểm tra Zalo trước khi gửi lại.")
			m.status = "Đã mất kết nối daemon. Esc để thoát, khởi động lại daemon rồi mở lại giao diện."
			return m, nil
		}
		if msg.result.Event.Event == "stickers_result" {
			return m, tea.Batch(m.listen(), m.receiveStickers(msg.result.Event))
		}
		if msg.result.Err != nil {
			m.status = msg.result.Err.Error()
		} else {
			m.handleEvent(msg.result.Event)
		}
		if msg.result.Event.Event == "archive_updated" {
			for k, state := range m.historyStates {
				if strings.HasPrefix(k, msg.result.Event.Data.ThreadType+":") {
					if k != m.active {
						state.loaded = false
					}
					m.historyDirty[k] = true
				}
			}
		}
		if msg.result.Event.Event == "recipients_updated" {
			m.listDirty = true
		}
		return m, tea.Batch(m.listen(), m.refreshFromArchive(), m.ensurePreview(false))
	case requestStarted:
		if msg.kind == "sync" && msg.err != nil {
			m.status = "Không yêu cầu được đồng bộ: " + msg.err.Error()
			return m, nil
		}
		if msg.kind == "stickers" && msg.err != nil && m.stickerMenu != nil && msg.attempt == m.stickerMenu.attempt {
			m.stickerMenu.loading = false
			m.stickerMenu.status = msg.err.Error()
			return m, nil
		}
		if msg.kind == "history" {
			m.historyStarted(msg)
			return m, tea.Batch(m.refreshFromArchive(), m.ensurePreview(false))
		}
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
		return m, m.refreshFromArchive()
	case requestExpired:
		if msg.kind == "stickers" && m.stickerMenu != nil && m.stickerMenu.loading && msg.attempt == m.stickerMenu.attempt {
			m.stickerMenu.loading = false
			m.stickerMenu.status = "Hết thời gian tải sticker. Esc rồi F4 để thử lại."
			return m, nil
		}
		if msg.kind == "history" {
			if state := m.historyStates[msg.thread]; state != nil && state.loading && state.attempt == msg.attempt {
				state.loading = false
				state.error = "Hết thời gian tải tin cũ. F6 để thử lại."
				if m.active == msg.thread {
					m.status = state.error
				}
				m.refreshChat(false)
			}
			return m, nil
		}
		if msg.kind == "send" && m.pending != nil && m.pending.attempt == msg.attempt {
			m.finishSend(false, "Chưa rõ kết quả gửi; kiểm tra Zalo trước khi gửi lại.")
		} else if msg.kind == "list" && m.loading && m.listAttempt == msg.attempt {
			m.loading = false
			m.status = "Hết thời gian tải danh sách. F5 để thử lại."
		}
		return m, nil
	case stickerPreview:
		if m.stickerMenu == msg.menu && msg.menu.version == msg.version {
			msg.menu.preview = msg.text
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.imageMenu {
			return m.updateImageMenu(msg)
		}
		if m.stickerMenu != nil {
			cmd := m.updateStickerMenu(msg.String())
			return m, cmd
		}
		if msg.String() == "esc" {
			if m.replies[m.active] != nil {
				delete(m.replies, m.active)
				m.status = "Đã bỏ trích dẫn."
				return m, nil
			}
			return m, tea.Quit
		}
		if m.width < 68 || m.height < 18 {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+r":
			if m.editing && m.selectedLine != nil && m.selectedLine.id != "" {
				m.replies[m.active] = m.selectedLine
				m.status = "Nhập nội dung trả lời · Enter gửi · Esc bỏ trích dẫn"
			} else {
				m.status = "Dùng ↑↓ chọn tin nhắn rồi bấm Ctrl+R."
			}
			return m, nil
		case "f4":
			cmd := m.openStickerMenu()
			return m, cmd
		case "f9":
			m.copySelectedMedia()
			return m, nil
		case "ctrl+g":
			cmd := m.sendCopiedMedia()
			return m, cmd
		case "f8", "ctrl+o":
			if msg.String() == "ctrl+o" && m.selectedLine != nil && (m.selectedLine.kind == "image" || m.selectedLine.kind == "video") {
				return m, m.openExternalImage(m.selectedLine)
			}
			images := m.images()
			if len(images) == 0 {
				m.status = "Cuộc chat này chưa có ảnh đã tải. F6/F7 để lấy tin cũ."
				return m, nil
			}
			m.imageMenu = true
			m.selectedImage = images[len(images)-1]
			return m, nil
		case "f7":
			return m, m.loadOlder()
		case "ctrl+home":
			m.chat.GotoTop()
			return m, m.loadOlder()
		case "ctrl+end":
			m.stopPreview()
			m.selectedLine = nil
			m.refreshChat(false)
			m.chat.GotoBottom()
			return m, nil
		case "f6":
			if r, ok := m.recipient(); ok && m.connected {
				m.chat.GotoBottom()
				local := m.loadHistory(true)
				m.status = "Đang yêu cầu tin mới từ Zalo..."
				client := m.client
				return m, tea.Batch(local, func() tea.Msg {
					_, err := client.SyncNow(r.ThreadID, r.ThreadType)
					return requestStarted{kind: "sync", err: err}
				})
			}
			return m, nil
		case "tab", "shift+tab":
			if !m.editing {
				return m, m.openSelected()
			}
			m.editing = false
			m.composer.Blur()
			return m, m.filter.Focus()
		case "f5":
			if remaining := time.Until(m.listNextLoad); remaining > 0 {
				m.status = fmt.Sprintf("Đang giữ danh sách hiện tại. Có thể tải lại sau %d giây.", int(remaining.Seconds())+1)
				return m, nil
			}
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
				if m.chat.AtTop() {
					return m, m.loadOlder()
				}
			} else {
				m.chat.HalfViewDown()
			}
			return m, nil
		case "up":
			if !m.editing {
				m.cursor = max(0, m.cursor-1)
				return m, nil
			}
			return m, m.selectMessage(-1)
		case "down":
			if !m.editing {
				m.cursor = min(max(0, len(m.visible())-1), m.cursor+1)
				return m, nil
			}
			return m, m.selectMessage(1)
		case "enter":
			if !m.editing {
				return m, m.openSelected()
			}
			r, ok := m.recipient()
			text := m.composer.Value()
			if strings.TrimSpace(text) == "" && m.selectedLine != nil && m.selectedLine.kind == "video" {
				return m, m.openExternalImage(m.selectedLine)
			}
			if strings.TrimSpace(text) == "" && m.selectedLine != nil && (m.selectedLine.kind == "image" || m.selectedLine.kind == "sticker") {
				return m, m.ensurePreview(true)
			}
			if !ok || !m.connected || m.pending != nil || strings.TrimSpace(text) == "" {
				return m, nil
			}
			line := &chatLine{sender: "Bạn", text: text, self: true, state: "Đang gửi...", timestamp: time.Now().UnixMilli()}
			var replyTo []string
			if reply := m.replies[m.active]; reply != nil {
				replyTo = []string{reply.id}
				line.replyPreview = replySummary(reply)
				delete(m.replies, m.active)
			}
			m.composer.Reset()
			m.drafts[m.active] = ""
			cmd := m.beginSend(line, func() (string, error) {
				return m.client.SendMessage(r.ThreadID, r.ThreadType, text, replyTo...)
			})
			return m, cmd
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
