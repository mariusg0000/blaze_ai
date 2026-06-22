// skills/doc.go — skill discovery, validation, and active skills list management.
// Discovers builtin and custom skills from disk, validates [DESCRIPTION] and [DETAILS] sections,
// maintains the in-memory active skills list, and resolves collisions (custom wins over builtin).
// Layer: skill management. Dependencies: internal/platform (app home path for custom skills).
package skills
