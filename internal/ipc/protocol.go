package ipc

// Event là sự kiện JSON mà daemon phát qua WebSocket.
type Event struct {
	Notice     string      `json:"notice,omitempty"`
	RetryAfter int         `json:"retry_after,omitempty"`
	Cursor     string      `json:"cursor,omitempty"`
	HasMore    bool        `json:"has_more,omitempty"`
	Messages   []Message   `json:"messages,omitempty"`
	ThreadID   string      `json:"thread_id,omitempty"`
	ThreadType string      `json:"thread_type,omitempty"`
	Partial    bool        `json:"partial,omitempty"`
	MsgID      string      `json:"msg_id,omitempty"`
	Recipients []Recipient `json:"recipients,omitempty"`
	Event      string      `json:"event"`
	ID         string      `json:"id,omitempty"`
	OK         bool        `json:"ok,omitempty"`
	Error      string      `json:"error,omitempty"`
	Data       Message     `json:"data"`
}

type Message struct {
	MsgID      string `json:"msg_id"`
	ThreadID   string `json:"thread_id"`
	ThreadType string `json:"thread_type"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp"`
	IsSelf     bool   `json:"is_self"`
}

// Result chứa một sự kiện hoặc lỗi đọc/giải mã.
type Result struct {
	Event Event
	Err   error
}

// SendRequest là lệnh gửi tin tới một hội thoại Zalo.
type SendRequest struct {
	ID      string       `json:"id"`
	Action  string       `json:"action"`
	Payload *SendPayload `json:"payload,omitempty"`
}

type SendPayload struct {
	BeforeID   string `json:"before_id,omitempty"`
	ThreadID   string `json:"thread_id"`
	ThreadType string `json:"thread_type"`
	Text       string `json:"text"`
}

// Recipient là một bạn bè hoặc nhóm có thể chọn để gửi tin.
type Recipient struct {
	ThreadID   string `json:"thread_id"`
	ThreadType string `json:"thread_type"`
	Name       string `json:"name"`
}
