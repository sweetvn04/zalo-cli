package ipc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

type Client struct {
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
