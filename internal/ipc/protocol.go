package ipc

// Event là sự kiện JSON mà daemon phát qua WebSocket.
type Event struct {
	Event string  `json:"event"`
	Data  Message `json:"data"`
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
