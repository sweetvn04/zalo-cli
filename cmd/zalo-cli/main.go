package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"zalo-cli/internal/ipc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverURL := "ws://127.0.0.1:8989"
	log.Printf("Connecting to daemon at %s", serverURL)
	client, err := ipc.Dial(ctx, serverURL)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	defer client.Close()
	log.Printf("Kết nối tới daemon thành công tại %s!", serverURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("👋 Đang đóng Go CLI...")
			return
		case result, ok := <-client.Results():
			if !ok {
				log.Println("Kết nối đã đóng, CLI kết thúc.")
				return
			}
			if result.Err != nil {
				log.Print(result.Err)
				continue
			}
			if result.Event.Event == "new_message" {
				msg := result.Event.Data
				log.Printf("📩 [Zalo Go-CLI] %s: %s", msg.SenderName, msg.Content)
			}
		}
	}
}
