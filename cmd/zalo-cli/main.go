package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zalo-cli/internal/ipc"
	"zalo-cli/internal/tui"
)

func main() {
	ui := flag.Bool("tui", false, "Mở giao diện chat tương tác")
	to := flag.String("to", "", "ID hội thoại Zalo cần gửi")
	kind := flag.String("type", "user", "Loại hội thoại: user hoặc group")
	list := flag.Bool("list", false, "Liệt kê bạn bè/nhóm; thêm -text để chọn người nhận và gửi")
	text := flag.String("text", "", "Nội dung tin nhắn")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if *ui {
		if *list || *to != "" || *text != "" {
			err = fmt.Errorf("dùng -tui riêng để mở giao diện chat")
		} else {
			err = tui.Run(ctx, "ws://127.0.0.1:8989")
		}
	} else if *list {
		if *to != "" {
			err = fmt.Errorf("dùng -list để chọn hoặc -to để chỉ định người nhận")
		} else {
			err = runList(ctx, "ws://127.0.0.1:8989", *text, os.Stdin, os.Stdout)
		}
	} else {
		err = run(ctx, "ws://127.0.0.1:8989", *to, *kind, *text)
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, serverURL, to, kind, text string) error {
	sending := to != "" || text != ""
	if sending && (to == "" || text == "") {
		return fmt.Errorf("cần cả -to và -text để gửi tin")
	}
	if kind != "user" && kind != "group" {
		return fmt.Errorf("-type phải là user hoặc group")
	}
	log.Printf("Connecting to daemon at %s", serverURL)
	client, err := ipc.Dial(ctx, serverURL)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Printf("Kết nối tới daemon thành công tại %s!", serverURL)

	var requestID string
	var timeout <-chan time.Time
	if sending {
		requestID, err = client.SendMessage(to, kind, text)
		if err != nil {
			return err
		}
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		timeout = timer.C
		log.Println("Đang chờ kết quả gửi từ daemon...")
	}
	for {
		select {
		case <-ctx.Done():
			if sending {
				return fmt.Errorf("đã dừng chờ; chưa xác định được kết quả gửi")
			}
			log.Println("👋 Đang đóng Go CLI...")
			return nil
		case <-timeout:
			return fmt.Errorf("hết thời gian chờ; chưa xác định được kết quả gửi, hãy kiểm tra Zalo trước khi gửi lại")
		case result, ok := <-client.Results():
			if !ok {
				if sending {
					return fmt.Errorf("kết nối đã đóng; chưa xác định được kết quả gửi")
				}
				return fmt.Errorf("kết nối đã đóng, CLI kết thúc")
			}
			if result.Err != nil {
				log.Print(result.Err)
				continue
			}
			event := result.Event
			switch event.Event {
			case "new_message":
				msg := event.Data
				log.Printf("📩 [%s:%s] %s: %s", msg.ThreadType, msg.ThreadID, msg.SenderName, msg.Content)
			case "send_result":
				if sending && event.ID == requestID {
					if !event.OK {
						return fmt.Errorf("gửi thất bại: %s", event.Error)
					}
					log.Printf("📤 Zalo đã chấp nhận gửi tin tới %s", to)
					return nil
				}
			}
		}
	}
}
