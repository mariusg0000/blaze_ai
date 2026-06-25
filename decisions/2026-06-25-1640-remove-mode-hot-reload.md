# Session Decision Summary: remove-mode-hot-reload

Date: 2026-06-25 16:40
Base commit: 91c5da5

## Context
Hot reload of modes.json was causing model changes in one BlazeAI instance to silently propagate to all running instances on the next Tab cycle or SetMode call. The user identified this as incorrect behavior — running sessions should be isolated and immune to config changes made from other instances. Mode and model changes must require an explicit app restart.

## Changes Made
- `internal/config/modes.go`: Removed `Reload()` method (27 lines). ModesConfig is now load-once at startup, immutable in-memory for the session lifetime.
- `internal/runtime/runtime.go`: Removed `ReloadModes()` method. Removed `a.ReloadModes()` call from both `SetMode()` and `NextMode()`. Modes are only read from disk once during agent initialization.
- `internal/config/modes_test.go`: Removed `TestModesReload` and `TestModesReloadInvalid` (68 lines). No hot-reload contract to test.
- `internal/runtime/runtime_test.go`: Removed `TestNextModeHotReloads` (42 lines). No reload path to exercise.
- `skills/customize_me.md`: Updated modes.json editing rules — all changes require restart with `-c` flag. Removed hot-reload and Tab-cycle-after-edit language. Simplified safety note.

## Decisions And Rationale
- Complete removal (not gating) because the user assessed that nothing should be hot-reloaded. Partial removal would leave dead code and misleading behavior.
- Restart with `-c` is the documented recovery path after config changes — this existed already for config.json changes and now applies to modes.json too.
- Session isolation is a fundamental correctness property: one session's model configuration must not drift due to another session's actions.

## Files Included
- `internal/config/modes.go`: Removed Reload()
- `internal/runtime/runtime.go`: Removed ReloadModes() and all calls
- `internal/config/modes_test.go`: Removed hot-reload tests
- `internal/runtime/runtime_test.go`: Removed hot-reload test
- `skills/customize_me.md`: Restart requirement, removed hot-reload docs
- `decisions/2026-06-25-1640-remove-mode-hot-reload.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
