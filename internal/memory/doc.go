// memory/doc.go — persistent memory file access.
// Reads app_home/memory/memory.md fresh on every prompt build. Memory updates are explicit
// via the shell tool or user action; the runtime never writes to memory automatically.
// Layer: memory access. Dependencies: none (file IO only).
package memory
