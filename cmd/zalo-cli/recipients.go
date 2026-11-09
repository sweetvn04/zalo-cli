package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"zalo-cli/internal/ipc"
)

// Chỉ mục được chọn từ chính danh sách vừa hiển thị trong lần chạy này.
func runList(ctx context.Context, url, text string, input io.Reader, output io.Writer) error {
	if text != "" && strings.TrimSpace(text) == "" {
		return fmt.Errorf("nội dung không được chỉ có khoảng trắng")
	}
	client, err := ipc.Dial(ctx, url)
	if err != nil {
		return err
	}
	defer client.Close()
	id, err := client.ListRecipients()
	if err != nil {
		return err
	}
	response, err := waitReply(ctx, client, id, "recipients_result")
	if err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("lấy danh sách thất bại: %s", response.Error)
	}
	recipients := response.Recipients
	if response.Notice != "" {
		fmt.Fprintln(output, response.Notice)
	}
	if len(recipients) == 0 {
		fmt.Fprintln(output, "Danh sách bạn bè và nhóm đang trống.")
		return nil
	}
	for i, recipient := range recipients {
		fmt.Fprintf(output, "%d. [%s] %s (%s)\n", i+1, recipient.ThreadType, recipient.Name, recipient.ThreadID)
	}
	if text == "" {
		return nil
	}
	fmt.Fprint(output, "Chọn số người nhận (Enter để hủy): ")
	// Đợi bàn phím mà vẫn cho phép Ctrl+C hủy chương trình.
	choice := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(input)
		if scanner.Scan() {
			choice <- scanner.Text()
			return
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		readErr <- err
	}()
	var line string
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-readErr:
		return fmt.Errorf("đọc lựa chọn: %w", err)
	case line = <-choice:
	}
	if strings.TrimSpace(line) == "" {
		fmt.Fprintln(output, "Đã hủy.")
		return nil
	}
	index, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || index < 1 || index > len(recipients) {
		return fmt.Errorf("chọn một số từ 1 đến %d", len(recipients))
	}
	recipient := recipients[index-1]
	id, err = client.SendMessage(recipient.ThreadID, recipient.ThreadType, text)
	if err != nil {
		return err
	}
	response, err = waitReply(ctx, client, id, "send_result")
	if err != nil {
		return fmt.Errorf("chưa xác định kết quả gửi, kiểm tra Zalo trước khi gửi lại: %w", err)
	}
	if !response.OK {
		return fmt.Errorf("gửi thất bại: %s", response.Error)
	}
	fmt.Fprintf(output, "Zalo đã chấp nhận gửi tin tới %s.\n", recipient.Name)
	if response.Notice != "" {
		fmt.Fprintln(output, response.Notice)
	}
	return nil
}

func waitReply(ctx context.Context, client *ipc.Client, id, event string) (ipc.Event, error) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ipc.Event{}, ctx.Err()
		case <-timer.C:
			return ipc.Event{}, fmt.Errorf("hết thời gian chờ daemon")
		case result, ok := <-client.Results():
			if !ok {
				return ipc.Event{}, fmt.Errorf("daemon đã đóng kết nối")
			}
			if result.Err != nil {
				return ipc.Event{}, result.Err
			}
			if result.Event.ID == id && result.Event.Event == event {
				return result.Event, nil
			}
		}
	}
}
