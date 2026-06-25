[DESCRIPTION]
Load when the user asks for web search, news, internet research, or DuckDuckGo results. Use for `ddgs` text, news, image, and video searches without an API key.

[BEHAVIOR]
Free web search using DuckDuckGo via the `ddgs` CLI installed in the BlazeAI venv. **No API key required.**

Wrapper script: `/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh` — adaugă automat un delay de 1s între căutări (configurabil via `DDGS_DELAY`) pentru a evita rate limiting-ul DuckDuckGo.

## Quick reference

| Method | Use case | Fields |
|--------|----------|--------|
| `text` | General research, companies | title, href, body |
| `news` | Current events, updates | date, title, source, body, url |
| `images` | Visuals, diagrams | title, image, thumbnail, url |
| `videos` | Tutorials, demos | title, content, duration, provider |

## CLI usage patterns

### Text search (general)
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "python async programming" -m 5
```

### News search
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh news -k "AI regulation 2026" -m 5
```

### Image search
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh images -k "semiconductor chip" -m 5
```

### Video search
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh videos -k "FastAPI tutorial" -m 5
```

### With region / time / safe search
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "best restaurants" -m 5 -r us-en
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "latest AI news" -m 5 -t w
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "python" -m 5 -s off
```

### JSON output for structured processing (pipe to jq)
- For `text`, `images`, `videos`: `-o json` writes to stdout — pipe to `jq` directly.
- For `news`: `-o json` writes to a FILE, not stdout. Use one of:
  ```bash
  # Option A: redirect to file, then read and pipe
  /home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh news -k "AI regulation" -m 5 -o json > /tmp/ddgs.json 2>&1 && jq '.[] | {title, url}' /tmp/ddgs.json

  # Option B: skip JSON, use plain text output (simpler, faster)
  /home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh news -k "AI regulation" -m 5
  ```

```bash
# text output pipes to jq normally
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "fastapi tutorial" -m 5 -o json | jq '.[].title'
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "fastapi tutorial" -m 5 -o json | jq '.[] | {title, href}'
```

### Custom delay (override default 1s)
```bash
DDGS_DELAY=3 /home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "query" -m 5
DDGS_DELAY=0 /home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "query" -m 5   # fara delay
```

### Extract content from a result URL
```bash
curl -sL "<url>" | head -c 5000
# or using ddgs extract:
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh extract "<url>"
```

## Common workflow

1. **Search** with ddgs to find relevant URLs:
```bash
/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh text -k "fastapi deployment guide" -m 5 -o json | jq '.[] | {title, href}'
```

2. **Extract** content from the best URL:
```bash
curl -sL "<url>" | head -c 5000
```

## CLI flags summary

| Flag | Description | Example |
|------|-------------|---------|
| `-k` | Keywords (query) — required | `-k "search terms"` |
| `-m` | Max results | `-m 5` |
| `-r` | Region | `-r us-en` |
| `-t` | Time limit | `-t w` (week) |
| `-s` | Safe search | `-s off` |
| `-o` | Output format | `-o json` |

## Limitations

- **Rate limiting**: DuckDuckGo poate bloca după requesturi rapide. Wrapper-ul adaugă 1s delay implicit.
- **No full content**: ddgs returnează doar snippet-uri. Folosește `curl` pentru conținut complet.
- **Blocked IPs**: Unele cloud IP-uri pot fi blocate. Încearcă alte cuvinte cheie sau așteaptă.
- **Field variability**: Câmpurile pot varia între versiuni. Folosește `jq` defensiv (`.body?`).

## When to stop debugging

- If a search already returned useful results (non-empty, relevant), **answer the user immediately** with the findings.
- Do NOT add extra tool calls to debug the JSON format, the wrapper, or the exit code.
- An `exit_code != 0` with useful stdout is still a valid result — use the data.
- Only retry or debug if the output is genuinely empty or irrelevant.
- One successful search is enough. Do not re-search unless the user asks for different keywords or more results.

## Pitfalls

- **Package name**: `pip install ddgs` (nu `duckduckgo-search`).
- **Rezultate goale**: Posibil rate limiting — wrapper-ul ajută, dar nu garantează.
- **`news -o json` writes to file, NOT stdout**. Pipe-ul direct (`| jq`) eșuează. Folosește output text sau redirect > file apoi citește.
- **Folosește mereu calea completă** `/home/marius/blazeai/skills/duckduckgo_search/scripts/duckduckgo.sh`.

## Validated with

`ddgs==9.14.4` în BlazeAI venv pe Python 3.10. Toate metodele (text, news, images, videos) confirmate cu `-o json`.
