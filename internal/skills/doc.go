// skills/doc.go — skill discovery, parsing, validation, and scoping.
// Discovers skills from builtin (embedded), global (app_home/skills/), and project
// (app_home/projects/<project>/skills/) sources. Parses required [DESCRIPTION] and
// [BODY] sections and resolves names across scopes.
// Layer: skill management. Dependencies: internal/platform.
package skills
