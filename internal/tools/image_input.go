// image_input.go — shared local image detection and preprocessing for vision tools.
// Detects supported image files, decodes them, rescales them for efficient vision use,
// and returns a JPEG data URL payload for one-shot multimodal requests.
// Layer: tool execution. Dependencies: standard image codecs, golang.org/x/image/draw.
package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	analyzeImageMaxLongSide = 1200
	analyzeImageJPEGQuality = 90
)

// preparedVisionImage holds the normalized local image plus metadata for the vision call.
//
// WHAT:  One decoded, resized, JPEG-encoded local image ready for multimodal transport.
// WHY:   Vision models should receive a compact image payload instead of raw local files.
// PARAMS: InputFile — original path; SourceMediaType — detected source MIME type; SourceSizeBytes —
// source file size; SourceWidth/SourceHeight — decoded source dimensions; OutputWidth/OutputHeight —
// resized dimensions; DataURL — JPEG image_url payload for the provider request.
type preparedVisionImage struct {
	InputFile       string
	SourceMediaType string
	SourceSizeBytes int64
	SourceWidth     int
	SourceHeight    int
	OutputWidth     int
	OutputHeight    int
	DataURL         string
}

// detectInputMediaType sniffs the file header to identify the real content type.
func detectInputMediaType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open input_file %s: %w", path, err)
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("cannot read input_file header %s: %w", path, err)
	}
	return http.DetectContentType(header[:n]), nil
}

// isImageMediaType reports whether the MIME type is any image/* value.
func isImageMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "image/")
}

// isSupportedVisionImageMediaType reports whether the image format is supported by the decoder path.
func isSupportedVisionImageMediaType(mediaType string) bool {
	switch mediaType {
	case "image/gif", "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}

// prepareVisionImage decodes, rescales, and JPEG-encodes one local image for the vision role.
func prepareVisionImage(ctx context.Context, inputFile string) (preparedVisionImage, error) {
	if ctx != nil && ctx.Err() != nil {
		return preparedVisionImage{}, ctx.Err()
	}
	stat, err := os.Stat(inputFile)
	if err != nil {
		return preparedVisionImage{}, fmt.Errorf("cannot stat input_file %s: %w", inputFile, err)
	}
	if !stat.Mode().IsRegular() {
		return preparedVisionImage{}, fmt.Errorf("input_file is not a regular file: %s", inputFile)
	}
	mediaType, err := detectInputMediaType(inputFile)
	if err != nil {
		return preparedVisionImage{}, err
	}
	if !isImageMediaType(mediaType) {
		return preparedVisionImage{}, fmt.Errorf("input_file is not an image: %s", inputFile)
	}
	if !isSupportedVisionImageMediaType(mediaType) {
		return preparedVisionImage{}, fmt.Errorf("unsupported image format for vision input: %s", mediaType)
	}
	if ctx != nil && ctx.Err() != nil {
		return preparedVisionImage{}, ctx.Err()
	}

	file, err := os.Open(inputFile)
	if err != nil {
		return preparedVisionImage{}, fmt.Errorf("cannot open input_file %s: %w", inputFile, err)
	}
	defer file.Close()

	decoded, _, err := image.Decode(file)
	if err != nil {
		return preparedVisionImage{}, fmt.Errorf("cannot decode image input_file %s: %w", inputFile, err)
	}
	if ctx != nil && ctx.Err() != nil {
		return preparedVisionImage{}, ctx.Err()
	}

	sourceBounds := decoded.Bounds()
	flattened := flattenImageToWhite(decoded)
	resized := resizeImageToLongSide(flattened, analyzeImageMaxLongSide)
	if ctx != nil && ctx.Err() != nil {
		return preparedVisionImage{}, ctx.Err()
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: analyzeImageJPEGQuality}); err != nil {
		return preparedVisionImage{}, fmt.Errorf("cannot encode vision image as jpeg: %w", err)
	}
	outputBounds := resized.Bounds()

	return preparedVisionImage{
		InputFile:       inputFile,
		SourceMediaType: mediaType,
		SourceSizeBytes: stat.Size(),
		SourceWidth:     sourceBounds.Dx(),
		SourceHeight:    sourceBounds.Dy(),
		OutputWidth:     outputBounds.Dx(),
		OutputHeight:    outputBounds.Dy(),
		DataURL:         "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// flattenImageToWhite removes alpha before JPEG encoding so transparent screenshots stay readable.
func flattenImageToWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	canvas := image.NewRGBA(bounds)
	stddraw.Draw(canvas, bounds, &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	stddraw.Draw(canvas, bounds, src, bounds.Min, stddraw.Over)
	return canvas
}

// resizeImageToLongSide rescales an image only when its longest edge exceeds the vision target.
func resizeImageToLongSide(src image.Image, maxLongSide int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	longSide := width
	if height > longSide {
		longSide = height
	}
	if longSide <= maxLongSide {
		return src
	}
	targetWidth, targetHeight := scaledDimensions(width, height, maxLongSide)
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}

// scaledDimensions computes proportional dimensions for the given max long side.
func scaledDimensions(width, height, maxLongSide int) (int, int) {
	if width >= height {
		targetHeight := maxLongSide * height / width
		if targetHeight < 1 {
			targetHeight = 1
		}
		return maxLongSide, targetHeight
	}
	targetWidth := maxLongSide * width / height
	if targetWidth < 1 {
		targetWidth = 1
	}
	return targetWidth, maxLongSide
}
