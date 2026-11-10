package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBridgeDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := bridgeDirectory(root); err == nil {
		t.Fatal("accepted missing bridge")
	}
	if err := os.WriteFile(filepath.Join(root, "index.mjs"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := bridgeDirectory(root); err != nil || got != root {
		t.Fatalf("%s: %v", got, err)
	}
}

// This helper stands in for Node so the launcher can be tested without login.
func TestDaemonProcess(t *testing.T) {
	if os.Getenv("ZALO_DAEMON_HELPER") != "1" {
		return
	}
	count := os.Getenv("ZALO_DAEMON_COUNT")
	f, err := os.OpenFile(count, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(2)
	}
	fmt.Fprintln(f, os.Getpid())
	f.Close()
	upgrade := websocket.Upgrader{}
	server := &http.Server{Addr: os.Getenv("ZALO_DAEMON_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrade.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			var request map[string]string
			if c.ReadJSON(&request) != nil {
				return
			}
			c.WriteJSON(map[string]any{"event": "daemon_status", "id": request["id"], "ready": true})
		}
	})}
	go server.ListenAndServe()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	server.Close()
	os.Exit(0)
}

func TestEnsureStartsOnceAndReusesBridge(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZALO_TEST_BINARY", binary)
	t.Setenv("ZALO_DAEMON_HELPER", "1")
	count := filepath.Join(root, "starts")
	t.Setenv("ZALO_DAEMON_COUNT", count)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	t.Setenv("ZALO_DAEMON_ADDRESS", address)
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0700)
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\nexec \"$ZALO_TEST_BINARY\" -test.run=^TestDaemonProcess$\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := filepath.Join(root, "daemon")
	os.MkdirAll(filepath.Join(dir, "node_modules", "zca-js"), 0700)
	os.WriteFile(filepath.Join(dir, "index.mjs"), nil, 0600)
	t.Cleanup(func() {
		data, _ := os.ReadFile(count)
		for _, value := range strings.Fields(string(data)) {
			pid, _ := strconv.Atoi(value)
			if pid > 0 {
				syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- Ensure(ctx, "ws://"+address, dir, io.Discard) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := Ensure(ctx, "ws://"+address, "/missing", io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(count)
	if err != nil || len(strings.Fields(string(data))) != 1 {
		t.Fatalf("started multiple bridges: %s %v", data, err)
	}
}
