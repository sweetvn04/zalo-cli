package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
)

// 1. Gom nhom goi tin tu daemon.
type MessageEvent struct {
	Event string `json:"event"`
	Data  struct {
		MsgID      string `json:"msg_id"`
		ThreadID   string `json:"thread_id"`
		ThreadType string `json:"thread_type"`
		SenderID   string `json:"sender_id"`
		SenderName string `json:"sender_name"`
		Content    string `json:"content"`
		Timestamp  int64  `json:"timestamp"`
		IsSelf     bool   `json:"is_self"`
	}
}

func main() {
	serverURL := "ws://127.0.0.1:8989"
	log.Printf("Connecting to daemon at %s", serverURL)

	// 2. Cu phap kiem tra loi IF ERR != NIL: Kiem tra xem ket noi co thanh cong khong.
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatal("Can not connect to daemon!")
	}
	// 3. Khi nao ham main bi tat do loi thi tu dong tat websocket
	defer conn.Close()

	log.Printf("Ket noi toi daemon thanh cong tai %s!", serverURL)

	// 4. Cu pha Goroutine Go Func(): Tao mot luong lien tuc chay ngam de doc tin nhan.
	go func() {
		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				log.Println("Mất kết nối với Daemon:", err)
				return
			}
			// Giải mã chuỗi JSON thành Struct của Go
			var event MessageEvent
			if err := json.Unmarshal(messageBytes, &event); err != nil {
				log.Println("Lỗi giải mã JSON:", err)
				continue
			}

			// In tin nhắn Zalo ra màn hình của Go!
			if event.Event == "new_message" {
				log.Printf("📩 [Zalo Go-CLI] %s: %s", event.Data.SenderName, event.Data.Content)
			}
		}
	}()

	// 5. Cú pháp CHANNEL & SIGNAL: Đợi người dùng bấm Ctrl+C để thoát êm ái (Graceful Shutdown)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt // Đứng đợi ở đây, khi nào có Ctrl+C mới đi tiếp

	log.Println("👋 Đang đóng Go CLI...")
}
