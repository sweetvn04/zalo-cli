package media

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestDownloadCachesImageWithPrivatePermissions(t *testing.T) {
	data := pngBytes(t)
	calls := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}, nil
	})}
	dir := t.TempDir() + "/images"
	first, err := downloadImage(context.Background(), "https://images.example.org/a?token=test", dir, client)
	if err != nil {
		t.Fatal(err)
	}
	second, err := downloadImage(context.Background(), "https://images.example.org/a?token=test", dir, client)
	if err != nil || first != second || calls != 1 || !strings.HasSuffix(first, ".png") {
		t.Fatalf("cache failed: %s, %s, calls=%d, err=%v", first, second, calls, err)
	}
	info, _ := os.Stat(first)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("image permissions: %v", info.Mode())
	}
	info, _ = os.Stat(dir)
	if info.Mode().Perm() != 0700 {
		t.Fatalf("cache permissions: %v", info.Mode())
	}
	content, _ := os.ReadFile(first)
	if !bytes.Equal(content, data) {
		t.Fatal("download changed image bytes")
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestRejectsNonImagesOversizeAndExpiredURL(t *testing.T) {
	for _, name := range []string{"html", "oversize", "expired"} {
		t.Run(name, func(t *testing.T) {
			client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				response := &http.Response{StatusCode: 200, ContentLength: -1}
				switch name {
				case "html":
					response.Body = io.NopCloser(strings.NewReader("<html>not an image</html>"))
				case "oversize":
					response.Body = io.NopCloser(io.MultiReader(bytes.NewReader(pngBytes(t)), io.LimitReader(zeros{}, maxImageSize)))
				case "expired":
					response.StatusCode = 403
					response.Body = io.NopCloser(strings.NewReader("expired"))
				}
				return response, nil
			})}
			dir := t.TempDir()
			if _, err := downloadImage(context.Background(), "https://images.example.org/a", dir, client); err == nil {
				t.Fatal("invalid response accepted")
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 0 {
				t.Fatal("failed download left files behind")
			}
		})
	}
}

func TestRejectsLocalAndNonHTTPImageAddresses(t *testing.T) {
	for _, address := range []string{"file:///etc/passwd", "javascript:alert(1)", "http://127.0.0.1/image", "http://[::1]/image", "http://192.168.1.2/a", "http://localhost/a", "https://user:pass@example.org/a"} {
		if validateURL(address) == nil {
			t.Fatalf("accepted %s", address)
		}
	}
	for _, address := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "fc00::1"} {
		if publicIP(net.ParseIP(address)) {
			t.Fatalf("accepted internal IP %s", address)
		}
	}
	if err := validateURL("https://photo.talk.zdn.vn/example.jpg?token=test"); err != nil {
		t.Fatal(err)
	}
}
