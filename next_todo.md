# Smart Input pentru BlazeAI

## Obiectiv

Adaugă input inteligent în consola REPL: istoric comenzi (săgeată sus/jos) și autocomplete contextual (Tab). Păstrează filosofia "super light" — o singură dependență externă.

## Library ales: `liner`

| | `liner` | `readline` (respins) |
|---|---|---|
| Deps runtime | `x/sys` + `go-runewidth` | `x/sys` doar |
| API | Simplu (`NewLiner`, `Prompt`, `Close`) | Complex (`Instance`, `Operation`, `Config`) |
| History | Manual (`ReadHistory`/`WriteHistory`) | Auto (`HistoryFile`) |
| Autocomplete | `SetCompleter(func(string) []string)` | `AutoCompleter` interface, tree-based |
| Menținut | Stabil, fără commituri din 2022 | Community PRs până în 2025 |

**De ce `liner`**: API mic, control manual asupra history-ului, potrivit filosofiei BlazeAI.

## Pași de implementare

### 1. Adaugă dependința `liner`

- `go get github.com/peterh/liner`
- Actualizează `go.mod` și `go.sum`

### 2. Rescrie `internal/console/reader.go`

- `Reader` devine wrapper peste `liner.Liner` în loc de `bufio.Scanner`
- Creează `NewReader(isTTY bool, historyPath string)` — inițializează liner, încarcă history
- `ReadLine()` apelează `liner.Prompt()` pe TTY, fallback la `bufio.Scanner` pe non-TTY
- `Close()` salvează history-ul și eliberează terminalul
- `SetCompleter(fn func(string) []string)` — setează autocomplete callback

### 3. Adaugă history persistence

- Fișier: `app_home/history` (o linie per comandă)
- Load: la pornirea REPL-ului, `liner.ReadHistory(file)`
- Save: la fiecare comandă, `liner.WriteHistory(file)` (append)
- Max entries: ~1000 (opțional, cu rotire)
- Non-TTY: nu se salvează/încarcă history

### 4. Adaugă autocomplete contextual

- `liner.SetCompleter(func(line string) []string)` cu logică:
  - Dacă linia începe cu `/` → lista slash commands: `/exit`, `/model`, `/cd`, `/clear`, `/new`
  - Dacă linia este `/model ` (cu spațiu) → lista favorite models din `Agent.Config.FavoriteModels`
  - Altfel → nimic (return nil)

### 5. Ajustează `internal/console/console.go`

- `Console.Run()`: înlocuiește channel-based input reader cu `liner.Prompt()` direct
- Elimină `startInputReader()` goroutine (liner blochează și gestionează inputul intern)
- `Console.Close()` apelează `Reader.Close()` pentru salvare history
- Abort mechanism: verifică dacă liner suportă Ctrl+C în timpul `Prompt()` (returnează `ErrPromptAborted`)
- Non-TTY: păstrează modelul curent cu `bufio.Scanner` și channel

### 6. Actualizează spec-urile

- `specs/03-interfaces.md`: adaugă mențiune despre history și autocomplete
- `next_todo.md`: marchează completarea

## Fișiere afectate

| Fișier | Modificare |
|---|---|
| `go.mod` | +1 dependență (`liner`) |
| `go.sum` | hash pentru `liner` + deps |
| `internal/console/reader.go` | Rescriere: `liner` în loc de `bufio.Scanner` |
| `internal/console/console.go` | Ajustare REPL loop, eliminare goroutine input |
| `specs/03-interfaces.md` | Documentare history + autocomplete |

## Constrângeri

- **Non-TTY**: fallback complet la `bufio.Scanner` (piped input/output)
- **Cross-platform**: `liner` suportă Linux, macOS, Windows
- **Nu schimbă** handler contract-ul (`OnContent`, `OnToolCall`, `OnToolResult`)
- **Nu schimbă** comportamentul de streaming, spinner, sau tool display
- **Fără bubbletea** — prea heavy pentru acest proiect

## Validare

1. `go build ./...` — compilație curată
2. `go test ./...` — toate testele trec
3. Manual: rulează `blazeai`, tastează `/` + Tab → autocomplete funcționează
4. Manual: săgeată sus → ultima comandă apare
5. Manual: ieși și reintră → history-ul persistă
6. Manual: `echo "test" | ./blazeai` → non-TTY merge ca înainte

## Risc

- `liner` deschide `/dev/tty` direct — trebuie verificat că nu conflict cu restul
- `liner` e neschimbat din 2022 — stabil dar fără suport activ
- Abort mechanism trebuie adaptat la `ErrPromptAborted` în loc de channel-based
