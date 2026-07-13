# TODO: Implement runnable skills

## WHAT MUST BE DONE
Implement runnable skills using `[SYNTAX]` and `[CODE]` sections with a separate `run_skill` execution path, while keeping `load_skill` limited to prompt activation.

## WHY IT MUST BE DONE
Script-backed skills could provide reusable local automations without requiring every capability to become hardcoded Go-native tool code.

## HOW IT MUST BE DONE
Fully implemented: `RunSkillTool` in `internal/tools/skill_tools.go` with `name`, `arguments`, and `timeout`. Validates `[SYNTAX]` and `[CODE]` are present, supports `shell` only in v1, passes `BLAZE_SKILL_ARGS`/`BLAZE_SKILL_DIR`/`BLAZE_SKILL_ID`/`BLAZE_SKILL_NAME` env vars, enforces per-call timeout. Parser in `internal/skills/skills.go` supports `Syntax`, `Code`, `CodeLang`, `CodeError` fields. Registered in `internal/runtime/runtime.go`. Tested in `internal/tools/skill_tools_test.go` and `internal/tools/tools_test.go`.
