package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPreviewRendersActualPixelColors(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		photo.Set(x, 0, color.RGBA{R: 255, A: 255})
		photo.Set(x, 1, color.RGBA{B: 255, A: 255})
	}
	var data bytes.Buffer
	if err := png.Encode(&data, photo); err != nil {
		t.Fatal(err)
	}
	preview, err := renderPreview(context.Background(), bytes.NewReader(data.Bytes()), 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview, "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀") {
		t.Fatalf("pixel colors missing: %q", preview)
	}
	if ansi.StringWidth(preview) != 2 {
		t.Fatal("unexpected preview width")
	}
}

func TestPreviewFitsChatAndPreservesAspect(t *testing.T) {
	for _, size := range [][2]int{{400, 200}, {10, 100}, {100, 10}} {
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, size[0], size[1]))); err != nil {
			t.Fatal(err)
		}
		preview, err := renderPreview(context.Background(), bytes.NewReader(data.Bytes()), 40, 10)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(preview, "\n")
		if len(lines) > 10 {
			t.Fatal("preview too tall")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > 40 {
				t.Fatal("preview too wide")
			}
		}
		if size[0] == 400 && (len(lines) != 10 || ansi.StringWidth(lines[0]) != 40) {
			t.Fatal("aspect ratio changed")
		}
	}
}

func TestPreviewRejectsHugeDimensionsBeforeAllocating(t *testing.T) {
	var data bytes.Buffer
	data.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&data, binary.BigEndian, uint32(13))
	header := make([]byte, 17)
	copy(header, "IHDR")
	binary.BigEndian.PutUint32(header[4:], 100000)
	binary.BigEndian.PutUint32(header[8:], 100000)
	header[12], header[13] = 8, 6
	data.Write(header)
	_ = binary.Write(&data, binary.BigEndian, crc32.ChecksumIEEE(header))
	_, err := renderPreview(context.Background(), bytes.NewReader(data.Bytes()), 40, 10)
	if err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Fatalf("expected dimension limit, got %v", err)
	}
}
