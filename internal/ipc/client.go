package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	writeMu sync.Mutex
	nextID  atomic.Uint64
	conn    *websocket.Conn
	cancel  context.CancelFunc
	results chan Result
	done    chan struct{}
}

func Dial(ctx context.Context, url string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("kết nối daemon: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	client := &Client{
		conn: conn, cancel: cancel,
		results: make(chan Result, 32), done: make(chan struct{}),
	}
	// Hủy context phải đánh thức cả ReadMessage đang chờ dữ liệu.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	go func() {
		defer close(client.done)
		defer close(client.results)
		defer conn.Close()
		defer stop()
		defer cancel()
		client.read(ctx)
	}()
	return client, nil
}

// Results chỉ cho phép bên gọi nhận dữ liệu, không gửi hoặc đóng channel.
func (c *Client) Results() <-chan Result { return c.results }

// Close chờ goroutine đọc dừng để không để lại công việc nền.
func (c *Client) Close() {
	c.cancel()
	c.conn.Close()
	<-c.done
}

func (c *Client) read(ctx context.Context) {
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.deliver(ctx, Result{Err: fmt.Errorf("mất kết nối daemon: %w", err)})
			return
		}
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			if !c.deliver(ctx, Result{Err: fmt.Errorf("giải mã JSON: %w", err)}) {
				return
			}
			continue
		}
		if !c.deliver(ctx, Result{Event: event}) {
			return
		}
	}
}

func (c *Client) deliver(ctx context.Context, result Result) bool {
	select {
	case c.results <- result:
		return true
	case <-ctx.Done():
		return false
	}
}

// SendImage gửi ảnh từ kho local và trả ID để ghép với send_result.
func (c *Client) SendImage(threadID, threadType, imageLocal string) (string, error) {
	return c.request("send_image", &SendPayload{ThreadID: threadID, ThreadType: threadType, ImageLocal: imageLocal})
}

func (c *Client) SendSticker(threadID, threadType string, stickerID int64) (string, error) {
	return c.request("send_sticker", &SendPayload{ThreadID: threadID, ThreadType: threadType, StickerID: stickerID})
}

func (c *Client) ListStickers() (string, error) {
	return c.request("list_stickers", nil)
}

// SendMessage gửi lệnh qua IPC. Bên gọi phải đợi send_result cùng ID để biết
// Zalo đã chấp nhận lệnh hay chưa. Không tự gửi lại khi mất phản hồi.
func (c *Client) SendMessage(threadID, threadType, text string, replyTo ...string) (string, error) {
	if strings.TrimSpace(threadID) == "" || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("người nhận và nội dung không được để trống")
	}
	if threadType != "user" && threadType != "group" {
		return "", fmt.Errorf("loại hội thoại phải là user hoặc group")
	}
	payload := &SendPayload{ThreadID: threadID, ThreadType: threadType, Text: text}
	if len(replyTo) > 0 {
		payload.ReplyTo = replyTo[0]
	}
	return c.request("send_message", payload)
}

// ListRecipients yêu cầu danh sách bạn bè và nhóm, kết quả về qua Results.
func (c *Client) ListRecipients() (string, error) {
	return c.request("list_recipients", nil)
}

func (c *Client) History(threadID, threadType string, beforeID ...string) (string, error) {
	if strings.TrimSpace(threadID) == "" || (threadType != "user" && threadType != "group") {
		return "", fmt.Errorf("hội thoại không hợp lệ")
	}
	payload := &SendPayload{ThreadID: threadID, ThreadType: threadType}
	if len(beforeID) > 0 {
		payload.BeforeID = beforeID[0]
	}
	if len(beforeID) > 1 {
		payload.ArchiveBefore = beforeID[1]
	}
	return c.request("get_history", payload)
}

func (c *Client) request(action string, payload *SendPayload) (string, error) {
	id := fmt.Sprintf("req-%d", c.nextID.Add(1))
	request := SendRequest{ID: id, Action: action, Payload: payload}
	// Gorilla WebSocket cho phép một reader và một writer đồng thời.
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return "", fmt.Errorf("đặt thời hạn ghi: %w", err)
	}
	if err := c.conn.WriteJSON(request); err != nil {
		return "", fmt.Errorf("gửi lệnh tới daemon: %w", err)
	}
	return id, nil
}
