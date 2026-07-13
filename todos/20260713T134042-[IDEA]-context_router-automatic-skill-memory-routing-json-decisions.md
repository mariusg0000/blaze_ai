# TODO: Implement context router

## WHAT MUST BE DONE
Implement a fast secondary LLM context router that reviews recent conversation, available skill and memory-bank descriptions, and active state before each main LLM call, then returns structured load and unload decisions.

## WHY IT MUST BE DONE
Automatic context management could reduce manual activation work while keeping the main LLM responsible for user-facing output and manual controls.

## HOW IT MUST BE DONE
Add configurable router-model routing, strict JSON action parsing, prompt integration, and state application before final prompt construction. If no router model is configured, keep automatic management disabled and preserve manual behavior without fallback.
