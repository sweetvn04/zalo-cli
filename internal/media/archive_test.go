package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchivedImagePreviewDoesNotNeedRemoteURL(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	directory := filepath.Join(root, "zalo-cli", "accounts", "1001", "media")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("a", 64) + ".png"
	if err := os.WriteFile(filepath.Join(directory, name), pngBytes(t), 0600); err != nil {
		t.Fatal(err)
	}
	preview, err := Preview(context.Background(), "archive:1001/"+name, 40, 10)
	if err != nil || !strings.Contains(preview, "▀") {
		t.Fatalf("local preview failed: %v", err)
	}
	for _, key := range []string{"archive:../a.png", "archive:1001/../../etc/passwd", "file:///etc/passwd", "archive:1001/not-a-hash.png"} {
		if _, err := archivedImage(key); err == nil {
			t.Fatalf("accepted invalid key %s", key)
		}
	}
}
