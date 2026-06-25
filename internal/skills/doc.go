// skills/doc.go — skill discovery, parsing, validation, scoping, and active list.
// Discovers skills from builtin (embedded), global (app_home/skills/), and project
// (workdir/.blazeai/skills/) sources. Parses [DESCRIPTION], [BEHAVIOR], [DATA].
// Maintains the in-memory active skills list and resolves names across scopes.
// Layer: skill management. Dependencies: internal/platform.
package skills
