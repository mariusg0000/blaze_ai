// embed_assets.go — embeds third-party vendor libraries (marked, highlight.js,
// DOMPurify, Monokai highlight themes) into the desktop binary so the desktop
// window renders Markdown with no network access at runtime.
// Layer: transport assets. Dependencies: none (read-only embedded FS).
package desktop

import "embed"

//go:embed assets/vendor/*
var vendorAssets embed.FS

// vendorFile returns the bytes of one vendor asset by name.
//
// WHAT:  Reads one embedded vendor file from assets/vendor/.
// WHY:   Startup injects vendor JS/CSS into the desktop HTML page string.
// PARAMS: name — filename inside assets/vendor/.
// RETURNS: file bytes; error if the named asset is missing.
func vendorFile(name string) ([]byte, error) {
	return vendorAssets.ReadFile("assets/vendor/" + name)
}