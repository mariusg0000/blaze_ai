// server.go — stdio request server for the Electron desktop backend.
// Owns request decoding, method dispatch, and JSON response encoding over stdin/stdout.
// Layer: desktop backend protocol server. Dependencies: desktop service, standard library.
package desktopbackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/platform"
)

// ErrQuitRequested stops the backend loop after a successful quit request.
var ErrQuitRequested = errors.New("desktop backend quit requested")

// Run starts the desktop backend server over stdio.
//
// WHAT:  Boots the desktop backend service and serves protocol requests.
// WHY:   Electron spawns this binary and talks to it through stdin/stdout.
// PARAMS: ctx — process lifetime context; cfg — loaded runtime config; osType — detected OS;
//
//	promptsFS — embedded prompt filesystem; opts — startup options; in/out — stdio transport.
//
// RETURNS: error if startup, decode, dispatch, or encode fails.
func Run(ctx context.Context, cfg *config.Config, osType platform.OS, promptsFS fs.FS, opts BackendOptions, in io.Reader, out io.Writer) error {
	service, err := NewService(cfg, osType, promptsFS, opts)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(in)
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("cannot decode desktop backend request: %w", err)
		}
		resp, quit, err := service.handleRequest(req)
		if err != nil {
			resp = Response{ID: req.ID, OK: false, Error: err.Error()}
		}
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("cannot encode desktop backend response: %w", err)
		}
		if quit {
			return ErrQuitRequested
		}
	}
}

func decodeParams(raw json.RawMessage, target interface{}) error {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}
