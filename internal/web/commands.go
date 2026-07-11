// commands.go — local slash-command handling for the web transport.
// Recognized commands are processed without sending them to the LLM.
// Layer: transport commands. Dependencies: internal/runtime.
package web

import (
	"fmt"
	"strings"
)

// handleSlashCommand dispatches a recognized slash command locally.
// Unknown commands return handled=false so they preserve console behavior and reach the LLM.
func (s *Server) handleSlashCommand(input string) (handled bool, err error) {
	parts := strings.SplitN(strings.TrimSpace(input), " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) == 2 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/clear", "/new":
		if err := s.resetConversation(); err != nil {
			return true, fmt.Errorf("cannot reset session: %w", err)
		}
		s.broadcastClear()
		s.sendBlock("system", `<span class="orange">⚡ Session cleared.</span>`)
		return true, nil

	case "/cd":
		if arg == "" {
			return true, fmt.Errorf("usage: /cd <path>")
		}
		if err := s.agent.SetWorkDir(arg); err != nil {
			return true, err
		}
		s.sendBlock("system", `<span class="orange">⚡ Work folder: `+escapeHTML(arg)+`</span>`)
		s.broadcastConfig()
		return true, nil

	case "/model":
		if arg == "" {
			return true, fmt.Errorf("use the model selector or /model &lt;provider/model&gt;")
		}
		switch arg {
		case "+":
			if err := s.agent.Config.AddFavorite(s.agent.ModelID); err != nil {
				return true, err
			}
			if err := s.agent.Config.Save(); err != nil {
				return true, fmt.Errorf("cannot save config: %w", err)
			}
			s.sendBlock("system", `<span class="orange">⚡ Added to favorites: `+escapeHTML(s.agent.ModelID)+`</span>`)
			s.broadcastConfig()
			return true, nil
		case "-":
			removed, err := s.agent.Config.RemoveFavorite(s.agent.ModelID)
			if err != nil {
				return true, err
			}
			if !removed {
				s.sendBlock("system", `<span class="orange">⚡ Not in favorites: `+escapeHTML(s.agent.ModelID)+`</span>`)
				return true, nil
			}
			if err := s.agent.Config.Save(); err != nil {
				return true, fmt.Errorf("cannot save config: %w", err)
			}
			s.sendBlock("system", `<span class="orange">⚡ Removed from favorites: `+escapeHTML(s.agent.ModelID)+`</span>`)
			s.broadcastConfig()
			return true, nil
		default:
			if err := s.agent.SetModel(arg); err != nil {
				return true, err
			}
			s.sendBlock("system", `<span class="orange">⚡ Model set to: `+escapeHTML(arg)+`</span>`)
			s.broadcastConfig()
			return true, nil
		}

	case "/auth":
		return true, fmt.Errorf("/auth is not supported in the web transport; use the console")

	case "/exit":
		if err := s.agent.CloseSession(); err != nil {
			return true, fmt.Errorf("cannot close session: %w", err)
		}
		s.sendBlock("system", `<span class="orange">⚡ Goodbye.</span>`)
		return true, nil

	case "/help":
		s.sendBlock("system", `<span class="blue">Commands: /clear, /new, /model &lt;model&gt;, /model +, /model -, /cd &lt;path&gt;, /exit, /help</span>`)
		return true, nil

	default:
		return false, nil
	}
}

// resetConversation clears the active session and transcript state.
func (s *Server) resetConversation() error {
	if s.agent == nil {
		return fmt.Errorf("no active session")
	}
	if err := s.agent.ResetConversation(); err != nil {
		return err
	}
	s.mu.Lock()
	s.blocks = nil
	s.status = "Ready"
	s.mu.Unlock()
	return nil
}
