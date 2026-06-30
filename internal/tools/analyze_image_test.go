// analyze_image_test.go — tests for the analyze_image tool and shared image preprocessing.
package tools

import (
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAnalyzeImageExecuteSuccess verifies image preprocessing and delegated vision payload creation.
func TestAnalyzeImageExecuteSuccess(t *testing.T) {
	path := writePNGImage(t, 2000, 1000)
	tool := NewAnalyzeImageTool(func(ctx context.Context, req AnalyzeImageRequest) (string, error) {
		if req.SourceMediaType != "image/png" {
			t.Fatalf("SourceMediaType = %q, want image/png", req.SourceMediaType)
		}
		if req.SourceWidth != 2000 || req.SourceHeight != 1000 {
			t.Fatalf("source size = %dx%d, want 2000x1000", req.SourceWidth, req.SourceHeight)
		}
		if req.OutputWidth != 1200 || req.OutputHeight != 600 {
			t.Fatalf("output size = %dx%d, want 1200x600", req.OutputWidth, req.OutputHeight)
		}
		if !strings.HasPrefix(req.ImageDataURL, "data:image/jpeg;base64,") {
			t.Fatalf("ImageDataURL missing jpeg data URL prefix: %q", req.ImageDataURL)
		}
		payload := strings.TrimPrefix(req.ImageDataURL, "data:image/jpeg;base64,")
		if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
			t.Fatalf("ImageDataURL is not valid base64: %v", err)
		}
		if req.Question != "Describe the screenshot" {
			t.Fatalf("Question = %q", req.Question)
		}
		return "Image findings", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"input_file":"`+path+`","question":"Describe the screenshot"}`))
	if result != "Image findings" {
		t.Fatalf("Execute() = %q, want %q", result, "Image findings")
	}
}

// TestAnalyzeImageExecuteRejectsNonImage verifies hard failure for non-image files.
func TestAnalyzeImageExecuteRejectsNonImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	tool := NewAnalyzeImageTool(func(ctx context.Context, req AnalyzeImageRequest) (string, error) {
		return "", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"input_file":"`+path+`","question":"Describe the file"}`))
	if result != "error: input_file is not an image: "+path {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAnalyzeImageFormatArgs verifies path and question display.
func TestAnalyzeImageFormatArgs(t *testing.T) {
	tool := NewAnalyzeImageTool(nil)
	result := tool.FormatArgs([]byte(`{"input_file":"/tmp/screen.png","question":"Describe all visible text and next steps"}`))
	if !strings.Contains(result, "screen.png") {
		t.Fatalf("FormatArgs() = %q, want filename", result)
	}
	if !strings.Contains(result, "Describe all visible text") {
		t.Fatalf("FormatArgs() = %q, want question excerpt", result)
	}
}

// writePNGImage writes a simple PNG test image to disk.
func writePNGImage(t *testing.T, width, height int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode() error: %v", err)
	}
	return path
}
