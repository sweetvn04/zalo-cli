package media

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var archiveKey = regexp.MustCompile(`^archive:([0-9]{1,32})/([a-f0-9]{64}\.(jpg|png|gif|webp|bmp|mp4|webm))$`)

// Chỉ đọc media trong kho của ứng dụng; không chấp nhận file:// tùy ý.
func archivedImage(key string) (string, error) {
	parts := archiveKey.FindStringSubmatch(key)
	if parts == nil {
		return "", fmt.Errorf("đường dẫn ảnh lưu trên máy không hợp lệ")
	}
	root := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".local", "share")
	}
	base := filepath.Join(root, "zalo-cli", "accounts")
	path := filepath.Join(base, parts[1], "media", parts[2])
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("ảnh chưa có trong kho local; F6 để tải lại")
	}
	limit := int64(100 * 1024 * 1024)
	if strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".webm") {
		limit = 1024 * 1024 * 1024
	}
	if info.Size() <= 0 || info.Size() > limit {
		return "", fmt.Errorf("kích thước ảnh local không hợp lệ")
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(base, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ảnh nằm ngoài kho local")
	}
	return path, nil
}
