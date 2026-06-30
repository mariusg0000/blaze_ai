// prompt/doc.go — prompt assembly from disk sources on every LLM call.
// Rebuilds the runtime prompt part in order: universal sysprompt, OS sysprompt,
// transport prompt, host helpers, skills section, specs.md, AGENTS.md.
// Skills include builtin, global, and project-scoped discovery.
// Replaces {VARIABLE_NAME} placeholders at build time.
// Layer: prompt construction. Dependencies: internal/skills, internal/platform.
package prompt
