# TODO: Implement ask_a_friend delegation

## WHAT MUST BE DONE
Implement a controlled one-shot `ask_a_friend` tool that lets the main BlazeAI model consult a configured secondary model using role-based routing, bounded context and output, one request only, no tool loops, no recursive delegation, and no fallback when the role is not configured.

## WHY IT MUST BE DONE
Focused second opinions, summarization, architecture review, debugging analysis, and session learning can benefit from a separate model while the main model remains in control.

## HOW IT MUST BE DONE
Implemented and verified in: `internal/tools/ask_friend.go`, `internal/tools/ask_friend_test.go`, `internal/llmcall/llmcall.go`, registered in `internal/runtime/runtime_test.go`, integrated in console, Telegram, and web transports. Supports role-based routing (advisor, summarization), input file up to 500000 bytes, timeout, strict validation, plain-text results, and clear configuration errors.
