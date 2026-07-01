// platform_other.go — non-Linux desktop platform integration stub.
// Keeps the package buildable outside Linux while failing explicitly if the
// Linux-native desktop extras are requested at runtime.
// Layer: transport platform integration.
//go:build !linux

package desktop

import (
	"fmt"

	webview "github.com/webview/webview_go"

	"blazeai/internal/platform"
)

type desktopPlatform interface {
	Shutdown()
}

type unsupportedDesktopPlatform struct{}

func startDesktopPlatform(view webview.WebView, ui *desktopUI, cfg *Config, osType platform.OS) (desktopPlatform, error) {
	return nil, fmt.Errorf("desktop tray, hotkey, and window persistence are supported only on Linux right now")
}

func (unsupportedDesktopPlatform) Shutdown() {}
