// prompt/doc.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in order: universal sysprompt, OS sysprompt,
// host helpers, memory.md, skills section, AGENTS.md. Replaces {VARIABLE_NAME} placeholders at build time.
// Layer: prompt construction. Dependencies: internal/skills, internal/memory, internal/platform.
package prompt
