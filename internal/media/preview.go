package media

import (
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Preview dùng hai pixel trên mỗi ô ▀, với màu trước/nền 24-bit. Kết quả là
// nội dung viewport bình thường nên cuộn được cùng tin nhắn.
func Preview(ctx context.Context, address string, columns, rows int) (string, error) {
	path, err := cachedImage(ctx, address)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được ảnh đã tải")
	}
	defer file.Close()
	return renderPreview(ctx, file, columns, rows)
}

func renderPreview(ctx context.Context, reader io.ReadSeeker, columns, rows int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return "", fmt.Errorf("không giải mã được ảnh để xem trong chat")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40000000 {
		return "", fmt.Errorf("ảnh quá lớn để xem trong chat; Ctrl+O để mở ảnh gốc")
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return "", fmt.Errorf("dữ liệu ảnh bị hỏng hoặc định dạng chưa hỗ trợ")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	columns, rows = max(1, min(columns, 100)), max(1, min(rows, 30))
	scale := math.Min(float64(columns)/float64(config.Width), float64(rows*2)/float64(config.Height))
	scale = math.Min(1, scale)
	width := max(1, int(math.Floor(float64(config.Width)*scale)))
	height := max(1, int(math.Floor(float64(config.Height)*scale)))
	small := image.NewRGBA(image.Rect(0, 0, width, height))
	// Nền tối cho vùng trong suốt, không phụ thuộc màu nền terminal.
	draw.Draw(small, small.Bounds(), &image.Uniform{C: color.RGBA{R: 25, G: 28, B: 36, A: 255}}, image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(small, small.Bounds(), source, source.Bounds(), draw.Over, nil)
	var result strings.Builder
	for y := 0; y < height; y += 2 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if y > 0 {
			result.WriteByte('\n')
		}
		for x := 0; x < width; x++ {
			top := small.RGBAAt(x, y)
			bottom := small.RGBAAt(x, min(y+1, height-1))
			fmt.Fprintf(&result, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
		}
		result.WriteString("\x1b[0m")
	}
	return result.String(), nil
}
