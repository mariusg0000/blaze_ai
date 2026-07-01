// handler_test.go — tests for compact desktop tool activity rendering.
// Verifies that the desktop transport mirrors tool activity as summarized status
// lines instead of raw tool result dumps. Layer: transport tests.
package desktop

import (
	"strings"
	"testing"
)

type handlerSinkStub struct {
	activity  []string
	status    []string
	busy      []bool
	content   strings.Builder
	started   int
	finished  int
	reasoning strings.Builder
}

func (s *handlerSinkStub) AppendUser(text string)       { _, _ = s.content.WriteString("U:" + text) }
func (s *handlerSinkStub) AppendSystem(text string)     { _, _ = s.content.WriteString("S:" + text) }
func (s *handlerSinkStub) StartAssistant()              { s.started++ }
func (s *handlerSinkStub) AppendAssistant(delta string) { _, _ = s.content.WriteString(delta) }
func (s *handlerSinkStub) FinishAssistant()             { s.finished++ }
func (s *handlerSinkStub) StartReasoning()              {}
func (s *handlerSinkStub) AppendReasoning(delta string) { _, _ = s.reasoning.WriteString(delta) }
func (s *handlerSinkStub) FinishReasoning()             {}
func (s *handlerSinkStub) SetToolActivity(text string)    { s.activity = append(s.activity, text) }
func (s *handlerSinkStub) SetStatus(text string)          { s.status = append(s.status, text) }
func (s *handlerSinkStub) SetBusy(active bool)            { s.busy = append(s.busy, active) }

func TestHandlerToolActivitySummarizesSuccess(t *testing.T) {
	sink := &handlerSinkStub{}
	h := NewHandler(sink)
	h.BeginTurn()
	h.OnToolCall("shell", "Check signal-cli installation and if it is usable")
	h.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	if len(sink.activity) != 2 {
		t.Fatalf("activity updates = %d, want 2", len(sink.activity))
	}
	if sink.activity[0] != "💻 Check signal-cli installation and if it is usable..." {
		t.Fatalf("pending activity = %q", sink.activity[0])
	}
	if sink.activity[1] != "💻 Check signal-cli installation and if it is usable ✅" {
		t.Fatalf("completed activity = %q", sink.activity[1])
	}
}

func TestHandlerToolActivitySummarizesErrors(t *testing.T) {
	sink := &handlerSinkStub{}
	h := NewHandler(sink)
	h.BeginTurn()
	h.OnToolCall("shell", "Run failing command")
	h.OnToolResult("shell", "exit_code: 1\nstdout:\n\nstderr:\nvery detailed failure output that should be summarized for desktop display\n")
	got := sink.activity[len(sink.activity)-1]
	if !strings.Contains(got, "❌") {
		t.Fatalf("completed activity = %q, want error badge", got)
	}
	if strings.Contains(got, "stdout:") || strings.Contains(got, "stderr:") {
		t.Fatalf("completed activity leaked raw tool sections: %q", got)
	}
	if !strings.Contains(got, "very detailed failure output") {
		t.Fatalf("completed activity missing summarized detail: %q", got)
	}
}

func TestHandlerReasoningFeedsReasoningBlock(t *testing.T) {
	sink := &handlerSinkStub{}
	h := NewHandler(sink)
	h.BeginTurn()
	h.OnReasoning("thinking step")
	h.OnReasoning(" more")
	if sink.reasoning.String() != "thinking step more" {
		t.Fatalf("reasoning content = %q, want %q", sink.reasoning.String(), "thinking step more")
	}
	h.FinishTurn(nil)
}

func TestHandlerContentClosesReasoningBeforeAssistant(t *testing.T) {
	sink := &handlerSinkStub{}
	h := NewHandler(sink)
	h.BeginTurn()
	h.OnReasoning("why")
	h.OnContent("answer")
	if sink.started != 1 {
		t.Fatalf("assistant started = %d, want 1 (reasoning swaps to assistant)", sink.started)
	}
	if sink.reasoning.String() != "why" {
		t.Fatalf("reasoning lost content: %q", sink.reasoning.String())
	}
}
