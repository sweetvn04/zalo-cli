// Package daemon starts the local bridge when the CLI cannot find a running one.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

func bridgeDirectory(explicit string) (string, error) {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else {
		cwd, _ := os.Getwd()
		executable, _ := os.Executable()
		for _, start := range []string{cwd, filepath.Dir(executable)} {
			for dir := start; ; dir = filepath.Dir(dir) {
				candidates = append(candidates, filepath.Join(dir, "daemon"))
				if filepath.Dir(dir) == dir {
					break
				}
			}
		}
	}
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, "index.mjs")); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(dir)
		}
	}
	return "", fmt.Errorf("không tìm thấy daemon/index.mjs; dùng -daemon-dir /đường/dẫn/daemon")
}

// probe also accepts an older bridge without daemon_status support.
func probe(ctx context.Context, address string) (bool, error) {
	dialCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	c, _, err := websocket.DefaultDialer.DialContext(dialCtx, address, nil)
	if err != nil {
		return false, err
	}
	defer c.Close()
	c.SetWriteDeadline(time.Now().Add(time.Second))
	if err := c.WriteJSON(map[string]string{"id": "startup", "action": "daemon_status"}); err != nil {
		return false, err
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return false, err
		}
		var status struct {
			ID, Event string
			Ready     bool `json:"ready"`
		}
		if err := json.Unmarshal(raw, &status); err != nil {
			return false, err
		}
		if status.ID != "startup" {
			continue
		}
		if status.Event != "daemon_status" {
			return true, nil
		}
		return status.Ready, nil
	}
}

// Ensure leaves the bridge alive after the CLI exits, so messages keep archiving.
// It never replaces a listening process or restarts a bridge after sending a message.
func Ensure(ctx context.Context, address, directory string, output io.Writer) error {
	ready, err := probe(ctx, address)
	if ready {
		return nil
	}
	if err != nil && !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("không kiểm tra được daemon: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	root := filepath.Join(cache, "zalo-cli")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, "daemon-start.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ready, err = probe(ctx, address)
	if ready {
		return nil
	}
	var exited <-chan error
	if err != nil {
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return err
		}
		dir, err := bridgeDirectory(directory)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, "node_modules", "zca-js")); err != nil {
			return fmt.Errorf("daemon chưa có dependencies; chạy npm --prefix %q ci", dir)
		}
		logPath := filepath.Join(root, "daemon.log")
		log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		command := exec.Command("node", "index.mjs", "--managed")
		command.Dir, command.Stdout, command.Stderr = dir, log, log
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		err = command.Start()
		log.Close()
		if err != nil {
			return fmt.Errorf("không bật được daemon: %w", err)
		}
		// Reap while this CLI lives; after exit the detached process is adopted by init.
		finished := make(chan error, 1)
		exited = finished
		go func() { finished <- command.Wait() }()
		fmt.Fprintf(output, "Đang bật daemon. Nhật ký: %s\nNếu cần đăng nhập, ảnh QR sẽ mở bằng ứng dụng xem ảnh.\n", logPath)
	}
	timeout := time.NewTimer(2 * time.Minute)
	defer timeout.Stop()
	for {
		ready, _ = probe(ctx, address)
		if ready {
			return nil
		}
		select {
		case err := <-exited:
			return fmt.Errorf("daemon đã dừng (%v); xem %s", err, filepath.Join(root, "daemon.log"))
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("daemon chưa sẵn sàng; xem %s", filepath.Join(root, "daemon.log"))
		case <-time.After(250 * time.Millisecond):
		}
	}
}
