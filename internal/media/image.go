package media

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxImageSize = 25 * 1024 * 1024

// OpenImage tải ảnh thành file cục bộ rồi mở bằng ứng dụng mặc định.
// Không đưa URL hoặc nội dung tin nhắn vào shell.
func OpenImage(ctx context.Context, address string) error {
	path, err := cachedImage(ctx, address)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "xdg-open", path)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("không mở được trình xem ảnh mặc định (xdg-open): %w", err)
	}
	return nil
}

func cachedImage(ctx context.Context, address string) (string, error) {
	if strings.HasPrefix(address, "archive:") {
		return archivedImage(address)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("tìm thư mục cache: %w", err)
	}
	transport := &http.Transport{DialContext: dialPublic, ForceAttemptHTTP2: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport, Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("ảnh chuyển hướng quá nhiều lần")
			}
			return validateURL(req.URL.String())
		},
	}
	return downloadImage(ctx, address, filepath.Join(cache, "zalo-cli", "images"), client)
}

func validateURL(address string) error {
	u, err := url.Parse(address)
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil ||
		(u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("đường dẫn ảnh phải là HTTP hoặc HTTPS hợp lệ")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("đường dẫn ảnh không được trỏ tới máy cục bộ")
	}
	if ip := net.ParseIP(host); ip != nil && !publicIP(ip) {
		return fmt.Errorf("đường dẫn ảnh không được trỏ tới mạng nội bộ")
	}
	return nil
}

func publicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

// Kiểm tra DNS rồi kết nối chính IP đã kiểm tra, kể cả sau redirect.
func dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("không phân giải được máy chủ ảnh")
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("không tìm thấy máy chủ ảnh")
	}
	for _, address := range addresses {
		if !publicIP(address.IP) {
			return nil, fmt.Errorf("máy chủ ảnh trỏ tới mạng nội bộ")
		}
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	for _, address := range addresses {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("không kết nối được máy chủ ảnh")
}

func imageExtension(header []byte) string {
	switch http.DetectContentType(header) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	}
	return ""
}

func downloadImage(ctx context.Context, address, directory string, client *http.Client) (string, error) {
	if err := validateURL(address); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", fmt.Errorf("tạo cache ảnh: %w", err)
	}
	base := fmt.Sprintf("%x", sha256.Sum256([]byte(address)))
	for _, extension := range []string{".jpg", ".png", ".gif", ".webp", ".bmp"} {
		path := filepath.Join(directory, base+extension)
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= maxImageSize {
			return path, nil
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", fmt.Errorf("không tạo được yêu cầu tải ảnh")
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("không tải được ảnh; đường dẫn có thể đã hết hạn hoặc mất mạng")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("máy chủ ảnh trả HTTP %d; ảnh có thể đã hết hạn", response.StatusCode)
	}
	if response.ContentLength > maxImageSize {
		return "", fmt.Errorf("ảnh vượt giới hạn 25 MiB")
	}
	reader := bufio.NewReader(response.Body)
	header, err := reader.Peek(512)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("không đọc được dữ liệu ảnh")
	}
	extension := imageExtension(header)
	if extension == "" {
		return "", fmt.Errorf("dữ liệu tải về không phải ảnh JPEG, PNG, GIF, WebP hoặc BMP")
	}
	file, err := os.CreateTemp(directory, ".download-*")
	if err != nil {
		return "", fmt.Errorf("tạo file ảnh: %w", err)
	}
	defer os.Remove(file.Name())
	written, copyErr := io.Copy(file, io.LimitReader(reader, maxImageSize+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return "", fmt.Errorf("tải ảnh chưa hoàn tất")
	}
	if written > maxImageSize {
		return "", fmt.Errorf("ảnh vượt giới hạn 25 MiB")
	}
	path := filepath.Join(directory, base+extension)
	if err := os.Rename(file.Name(), path); err != nil {
		return "", fmt.Errorf("lưu cache ảnh: %w", err)
	}
	return path, nil
}
