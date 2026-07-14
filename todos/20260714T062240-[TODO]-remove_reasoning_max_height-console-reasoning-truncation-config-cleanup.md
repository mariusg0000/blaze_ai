# TODO: Eliminate ReasoningMaxHeight

## WHAT MUST BE DONE
Remove the `ReasoningMaxHeight` config field, its validation, its default value, and the truncation logic that uses it in `OnReasoning`. Reasoning output should display in full without a line limit. Keep the `ShowReasoning` toggle (`Ctrl+T`) which controls visibility independently.

## WHY IT MUST BE DONE
The line-limit truncation of reasoning is an unnecessary constraint. Users should see the complete reasoning output when reasoning display is enabled.

## HOW IT MUST BE DONE
1. Remove `ReasoningMaxHeight int` field and its `json:"reasoning_max_height"` tag from `internal/config/config.go` (line 154).
2. Remove the default value `ReasoningMaxHeight: 150` from the config constructor (line 199).
3. Remove the validation block for `ReasoningMaxHeight < 0` (lines 400-401).
4. In `internal/console/console.go` `OnReasoning()` (line 654+), remove the `maxHeight` variable, the early-return when `reasoningLines >= maxHeight`, and the truncation logic that prints `[...truncated]`. The method should simply stream all reasoning chunks with the `🧠` prefix and line tracking, without any cutoff.
