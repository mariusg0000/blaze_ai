# Session Decision Summary: xml-injected-content

Date: 2026-06-25 15:15
Base commit: 82e525d

## Context
User identified that injected skills content lacked clear delimiters in the prompt, causing section boundaries to bleed together. After trying ASCII box delimiters (────), user suggested XML wrappers since LLMs are trained to respect XML tag boundaries natively.

## Changes Made
- `internal/prompt/prompt.go`: Rewrote `buildSkillsSection` to output XML instead of Markdown. Available skills use `<available_skills><skill><name/><description/></skill></available_skills>`. Active skills use `<active_skills><skill><name/><behavior><![CDATA[...]]></behavior><data><![CDATA[...]]></data></skill></active_skills>`. Behavior and data content is wrapped in CDATA to preserve literal characters (&&, <, >) without escaping. Added `"html"` import for `html.EscapeString` on name/description fields. Rewrote `buildHostHelpersSection` to XML format (`<host_helpers_available>`, `<host_helpers_optional>`). Updated `buildHostHelpersAdvisory` to XML.
- `prompts/sysprompt.md`: Removed redundant "Available skills:" and "Active skills:" labels (XML tag names are self-describing). Removed "Available helpers:" and "Optional helpers:" labels. Updated all references from `## Active Skills` to `<active_skills>` in Active State Rules, Mandatory Skill Manager Gate, and Skill Retention sections.

## Decisions And Rationale
- XML over Markdown delimiters: LLMs natively understand XML boundaries. `<tag>...</tag>` is unambiguous where Markdown headings (###) can blend with injected content.
- CDATA over HTML escaping: Preserves `&&` in shell commands, `<` and `>` in code blocks. LLMs understand CDATA as raw text. Escaping would transform `&&` to `&amp;&amp;` which could confuse tool call generation.
- Labels removed from template: XML tag names (`<available_skills>`, `<active_skills>`, `<host_helpers_available>`) are self-describing. Redundant labels add tokens without value.

## Files Included
- `internal/prompt/prompt.go`: XML output for all injected sections
- `prompts/sysprompt.md`: Updated references, removed redundant labels
- `decisions/2026-06-25-1515-xml-injected-content.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
