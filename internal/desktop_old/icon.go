// icon.go — generated desktop tray and window icon.
// Builds one small in-memory PNG so the desktop transport always has a stable
// app icon without relying on external asset files.
// Layer: transport assets. Dependencies: bytes, image, image/color, image/png.
package desktop

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
)

var (
	desktopIconOnce  sync.Once
	desktopIconBytes []byte
	desktopIconErr   error
)

// desktopIconPNG returns the generated BlazeAI desktop icon as PNG bytes.
//
// WHAT:  Lazily renders one transport-local icon.
// WHY:   Tray and native window code need the same stable image.
// RETURNS: []byte — PNG-encoded icon; error if encoding fails.
func desktopIconPNG() ([]byte, error) {
	desktopIconOnce.Do(func() {
		img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		fillRect(img, image.Rect(0, 0, 64, 64), color.NRGBA{R: 16, G: 20, B: 29, A: 255})
		fillRect(img, image.Rect(4, 4, 60, 60), color.NRGBA{R: 29, G: 39, B: 61, A: 255})
		fillRect(img, image.Rect(12, 10, 52, 54), color.NRGBA{R: 89, G: 163, B: 255, A: 255})
		fillRect(img, image.Rect(18, 16, 46, 48), color.NRGBA{R: 17, G: 20, B: 29, A: 255})
		fillRect(img, image.Rect(24, 18, 37, 48), color.NRGBA{R: 123, G: 201, B: 111, A: 255})
		fillRect(img, image.Rect(31, 18, 41, 28), color.NRGBA{R: 123, G: 201, B: 111, A: 255})
		fillRect(img, image.Rect(27, 34, 40, 44), color.NRGBA{R: 255, G: 214, B: 102, A: 255})
		var buf bytes.Buffer
		desktopIconErr = png.Encode(&buf, img)
		desktopIconBytes = buf.Bytes()
	})
	if desktopIconErr != nil {
		return nil, fmt.Errorf("cannot encode desktop icon: %w", desktopIconErr)
	}
	return desktopIconBytes, nil
}

func fillRect(img *image.NRGBA, rect image.Rectangle, c color.NRGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}
