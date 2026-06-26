# Compact-Language — specificație

compact_language := operator-based format for compressing natural language into structured, token-efficient instructions

## principii

- no prose → each info type has one format
- no narrative connectors → order + operators carry relations
- only notation from math, logic, code → no invented symbols
- one operator = one meaning
- consistency > extreme compression

## ierarhie și layout

- indent 2 spaces → hierarchy
- `- item` → unordered list, one per line
- `1. 2. 3.` → ordered list
- one line = one info unit

## escaping

- `{VAR}` → injectable, resolves at build
- `\{VAR\}` → literal, backslash stripped at build
- `\[BLOCK\]` → literal block
- `\\` → literal backslash
- reserved chars needing escape in text: `[` `]` `{` `}` `→` `!` `#` `@`

## operatori

### atribuire / definiție

- `=` — fact / value
  ex: `key = value`
- `:=` — nominal definition
  ex: `name := value`

### implicație / trigger

- `→` — if X then Y (most used operator)
  ex: `condition → action`
- compound conditions:
  - `cond1 ∧ cond2 → action` (and)
  - `cond1 ∨ cond2 → action` (or)

### condițional cu ramuri

- `cond ? a : b` — ternary
- multiple branches → separate lines with `→`, last with `else →`

### negare / prohibiție

- `!` prefix — do not / avoid
  ex: `!action`
- absolute prohibition → `! never action`
- intensity via words: `never` `avoid` `rarely`

### apartenență / clasificare

- `∈` — X is member of Y
  ex: `X ∈ Y`
- `∉` — X is not member of Y
  ex: `X ∉ Y`
- `⊆` — A is subcategory of B
  ex: `A ⊆ B`

### context / scope / tag

- `@` prefix — labels context, scope, or category
  ex: `@global` `@project` `@scope: global`
- `@` labels who the entity is `∈` asserts membership in a set

### exemplu

- `ex:` prefix — example
- placed below the rule it illustrates, at one deeper indent
- may use any operator

### comentariu / notă

- `#` prefix — explanatory note, rare
- if worth keeping permanently → becomes rule or fact

## keywords explicite

natural language prefixes for info types without a native symbol operator:

- `stop: condition` — stop when
- `valid: condition` — success confirmed when
- `fallback: action` — try if rest failed
- `first: action` — try first
- `prereq: condition` — required before continuing

## tipuri de informație → format

- trigger / activation → `condition → action`
- rule / constraint → `condition → action` `@` `!action`
- procedure → `1. 2. 3.` ordered steps, branches via `cond ? a : b` `@` lines with `cond →`
- fact / data → `key = value`
- definition → `name := value`
- example → `ex:` below illustrated rule
- warning / pitfall → `! action` + `never` / `avoid`
- scope / context → `@...` prefix
- fallback → `fallback: action` `@` `else → action`
- stop condition → `stop: condition`
- validation → `valid: condition`
- prerequisite → `prereq: condition`

## ce nu se comprimă

- trigger words in descriptions → natural language, must match user phrasing
- examples → natural language allowed, prefix `ex:`
- notes (`#`) → natural language allowed, rare
- ambiguous rule → rephrase mixed: compact structure + short clarification after `#`

## convenții de scriere

- names / scopes → lowercase, `_` for spaces
- actions → imperative: `read file` `ask user` `create folder`
- conditions → declarative: `file exists` `user confirms` `skill missing`
- ! no diacritics in compact content
