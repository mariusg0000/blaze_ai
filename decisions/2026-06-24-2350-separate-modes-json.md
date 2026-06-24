# Session Decision Summary: separate modes.json with atomic saves and corruption fallback

Date: 2026-06-24 23:50
Base commit: 8c1b227

## Context
Config.json was the single file holding everything: providers, API keys, roles, compaction, AND modes. When the LLM edited modes via the customize_me skill and corrupted the JSON, the entire config was broken — including the API key and provider data. Modes are frequently edited; provider data is rarely touched. They should not share the same file.

## Changes Made
Extracted work modes from config.json into a separate `modes.json` file with atomic writes and corruption fallback.

### New files
- `internal/config/modes.go` (259 lines) — ModesConfig struct, Load, Save (atomic temp→validate→rename), Reload (hot-reload), MigrateFromConfig (one-time extraction from config.json), Validate, validateBasic, DefaultMode
- `internal/config/modes_test.go` (278 lines) — 14 tests: default mode, save/load roundtrip, missing file fallback, corrupted JSON fallback, empty array fallback, duplicate name validation, bad format validation, unknown provider validation, dangling last_mode validation, hot reload, invalid reload rejection, atomic save, migration no-op

### Modified files
- `internal/config/config.go` — removed Modes/LastMode fields from Config struct; removed ReloadModesFromDisk() method; removed mode validation from Validate(); updated Config struct doc
- `internal/config/config_test.go` — removed 8 mode-related tests (moved to modes_test.go); updated validConfig() helper to exclude modes
- `internal/runtime/runtime.go` — Agent gets `Modes *config.ModesConfig` field; NewAgent loads from modes.json with migration; SetMode/NextMode/SetModel save to modes.json instead of config.json; ReloadModes delegates to Modes.Reload()
- `internal/runtime/runtime_test.go` — all mode tests adapted to use agent.Modes instead of agent.Config.Modes; TestNewAgentWithMode and TestNewAgentWithModeFallbackToFirstMode now write modes.json to HOME before NewAgent
- `internal/console/console_test.go` — TestPromptLabelWithMode uses agent.Modes.Modes
- `firstrun.go` — saves default mode via config.DefaultMode().Save() separately from config.json
- `skills/customize_me.md` — updated to document modes.json location, standalone JSON structure, atomic save safety, config.json vs modes.json editing separation

## Decisions And Rationale
- **Atomic writes**: write to .tmp → re-read and validate JSON → os.Rename. Prevents half-written corruption if the process crashes mid-write. If rename can't happen (different filesystem), temp file is cleaned up.
- **Fallback on corruption**: LoadModes() never returns an error for corruption — it falls back to DefaultMode with the given model ID. This means a corrupted modes.json won't crash the app. The valid config.json remains untouched.
- **Reload strict**: Reload() (hot-reload) DOES return errors on corruption, preserving the in-memory state. This is the "reject bad new data" behavior at runtime.
- **Migration**: MigrateFromConfig() reads the raw config JSON (preserving all other fields via map[string]json.RawMessage), extracts modes/last_mode if present, writes modes.json, strips them from config.json on disk. One-time, idempotent (existing modes.json means no-op).
- **Config.Load unchanged**: Load() parses the config struct without modes — any legacy modes keys in the raw JSON are silently ignored. This means old configs work without migration.

## Implementation Approach
Greenfield modes.go package in the config package. The ModesConfig type is self-contained with its own file path, load, save, and validate logic. The runtime Agent holds a pointer to a ModesConfig, replacing all cfg.Modes/cfg.LastMode accesses. SetMode and NextMode call modes.Save() (which does atomic write), not cfg.Save() (which writes providers/keys).

## Alternatives Considered
- Keeping modes in config.json with a backup-on-save pattern: rejected because it doesn't isolate the corruption — a corrupted config still loses provider/role data.
- Making modes.json optional with a no-config fallback: rejected because the app MUST have at least one mode. The fallback is to DefaultMode with the default role model.

## Files Included
- internal/config/modes.go: new — ModesConfig with atomic save, fallback load, migration, hot-reload
- internal/config/modes_test.go: new — 14 tests for modes operations
- internal/config/config.go: removed modes/last_mode from Config struct and validation
- internal/config/config_test.go: removed old mode tests (moved to modes_test.go)
- internal/runtime/runtime.go: Agent.Modes field, NewAgent uses LoadModes, SetMode/NextMode use modes.Save
- internal/runtime/runtime_test.go: adapted to agent.Modes API
- internal/console/console_test.go: adapted to agent.Modes API
- firstrun.go: saves modes separately via DefaultMode().Save()
- skills/customize_me.md: documented modes.json location, structure, atomic safety

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
