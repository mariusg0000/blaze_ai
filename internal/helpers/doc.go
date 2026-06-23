// helpers.go — host helper utility detection and prompt section builder.
// Defines the catalog of cross-platform helper utilities, detects which are installed
// via LookPath, filters contextual helpers by project ecosystem files, and builds
// a compact prompt section describing available and optionally missing helpers.
// Layer: host capability detection. Dependencies: internal/config (HelperSetup preferences),
// internal/platform (app home for Python venv path).
package helpers
