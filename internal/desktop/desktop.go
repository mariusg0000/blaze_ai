// desktop.go — desktop companion startup, fixed-session wiring, and embedded window UI.
// Boots the singleton desktop transport from app_home/desktop, resumes the same
// session folder on every start, and hosts one local chat page inside a native
// desktop window with no browser chrome. The page renders Markdown (tables, code
// blocks, lists) over full-width console rows themed per block type with Monokai
// dark/light themes. Layer: transport runtime. Dependencies: internal/config,
// internal/platform, internal/runtime, internal/session.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	webview "github.com/webview/webview_go"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	runtimecore "blazeai/internal/runtime"
	"blazeai/internal/session"
)

// transcriptBlock is one chronological row in the desktop transcript.
//
// WHAT:  Carries a typed unit of conversation content (user, assistant, system, tool, reasoning).
// WHY:   The UI renders each block as one full-width row with a per-type background.
// PARAMS: Type — row kind; Prefix — short label like "You"; Text — raw markdown content.
type transcriptBlock struct {
	Type   string
	Prefix string
	Text   string
}

// Block type ids used by transcript rows.
const (
	blockUser      = "user"
	blockAssistant = "assistant"
	blockSystem    = "system"
	blockTool      = "tool"
	blockReasoning = "reasoning"
)

// blockPayload is one transcript block serialized to the JS frontend.
//
// WHAT:  JSON view of one transcript row sent via stateResponse.
// WHY:   The JS renderer rebuilds DOM rows from typed blocks instead of a flat string.
// PARAMS: Type — row kind; Prefix — label; Text — raw markdown content.
type blockPayload struct {
	Type   string `json:"type"`
	Prefix string `json:"prefix"`
	Text   string `json:"text"`
}

// desktopPageTemplate is the HTML shell with placeholders replaced at startup.
// __MARKED_JS__, __HIGHLIGHT_JS__, __DOMPURIFY_JS__, __HIGHLIGHT_DARK_CSS__, __HIGHLIGHT_LIGHT_CSS__.
const desktopPageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>BlazeAI Desktop</title>
  <style>
__HIGHLIGHT_DARK_CSS__
__HIGHLIGHT_LIGHT_CSS__
    :root {
      --mono: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace;
      --font-size: 13.5px;
    }
    html[data-theme="dark"] {
      --bg: #272822;
      --row-user: #2b2c27;
      --row-assistant: #272822;
      --row-system: #232420;
      --row-tool: #2d2f29;
      --row-reasoning: #28241f;
      --text: #fbf4db;
      --text-muted: #75715e;
      --text-dim: #a6a297;
      --accent: #66d9ef;
      --accent-2: #a6e22e;
      --user-text: #cde6ba;
      --reasoning-text: #99948a;
      --tool-prefix: #5abdde;
      --tool-text: #add8e6;
      --border: #35362f;
      --panel: #1e1f1c;
      --panel-2: #32332e;
      --danger: #f92672;
      --scrollbar-track: #1e1f1c;
      --scrollbar-thumb: #49483e;
      --scrollbar-thumb-hover: #66d9ef;
    }
    html[data-theme="light"] {
      --bg: #fafafa;
      --row-user: #fcfcf8;
      --row-assistant: #fafafa;
      --row-system: #f6f5f0;
      --row-tool: #f9faf5;
      --row-reasoning: #faf5f0;
      --text: #272822;
      --text-muted: #8a8a7a;
      --text-dim: #5a5648;
      --accent: #2978a0;
      --accent-2: #4a7a2c;
      --user-text: #4a7a2c;
      --reasoning-text: #a0988a;
      --tool-prefix: #297898;
      --tool-text: #3a80a0;
      --border: #e4e2d5;
      --panel: #f4f2f0;
      --panel-2: #e4e2d5;
      --danger: #a6276b;
      --scrollbar-track: #f4f2ea;
      --scrollbar-thumb: #c9c3ac;
      --scrollbar-thumb-hover: #2978a0;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: var(--font-size, 13.5px)/1.25 var(--mono);
      background: var(--bg);
      color: var(--text);
    }
    .app {
      height: 100vh;
      display: grid;
      grid-template-rows: auto 1fr auto;
    }
    .topbar {
      display: flex;
      gap: 10px;
      align-items: center;
      padding: 8px 12px;
      background: var(--panel);
      border-bottom: 1px solid var(--border);
      flex-wrap: wrap;
    }
    .topbar .title {
      font-weight: 700;
      margin-right: 4px;
    }
    html[data-theme="dark"] .topbar .title { color: #f4a030; }
    html[data-theme="light"] .topbar .title { color: #c8701e; }
    .topbar .workdir {
      flex: 1 1 200px;
      min-width: 0;
      color: var(--text-dim);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .topbar select,
    .topbar button {
      background: var(--panel-2);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 5px 9px;
      font: inherit;
      cursor: pointer;
    }
    .topbar button.danger { color: var(--danger); }
    .topbar .theme-btn { font-size: 15px; padding: 4px 8px; }
    .pickdir-btn { background: none; border: none; cursor: pointer; font-size: 14px; padding: 2px 4px; line-height: 1; color: var(--text-dim); }
    .pickdir-btn:hover { color: var(--accent); }
    .fs-control { display: flex; align-items: center; gap: 4px; cursor: pointer; }
    .fs-label { font-size: 11px; color: var(--text-dim); font-weight: 700; }
    .fs-control select {
      background: var(--panel-2); color: var(--text);
      border: 1px solid var(--border); border-radius: 6px;
      padding: 3px 6px; font: inherit; font-size: 11px;
      cursor: pointer;
    }
    .fs-control select:focus { outline: 1px solid var(--accent); }

    .transcript {
      margin: 0;
      overflow-y: auto;
      overflow-x: hidden;
      background: var(--bg);
      scrollbar-width: thin;
      scrollbar-color: var(--scrollbar-thumb) var(--scrollbar-track);
    }
    .transcript::-webkit-scrollbar { width: 10px; }
    .transcript::-webkit-scrollbar-track { background: var(--scrollbar-track); }
    .transcript::-webkit-scrollbar-thumb {
      background: var(--scrollbar-thumb);
      border-radius: 5px;
    }
    .transcript::-webkit-scrollbar-thumb:hover { background: var(--scrollbar-thumb-hover); }

    .row {
      padding: 8px 16px;
      border-bottom: 1px solid var(--border);
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .row-user { background: var(--row-user); color: var(--user-text); }
    .row-assistant { background: var(--row-assistant); color: var(--text); }
    .row-system { background: var(--row-system); color: var(--text-dim); }
    .row-tool { background: var(--row-tool); color: var(--tool-text); line-height: 1; }
    .row-reasoning { background: var(--row-reasoning); color: var(--reasoning-text); line-height: 1; }

    .row .prefix {
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
    }
    .row-user .prefix { color: var(--accent-2); }
    .row-assistant .prefix { color: inherit; }
    html[data-theme="dark"] .row-assistant .prefix { color: #f4a030; }
    html[data-theme="light"] .row-assistant .prefix { color: #c8701e; }
    .row-reasoning .prefix { color: var(--reasoning-text); }
    .row-tool .prefix { color: var(--tool-prefix); }

    .row-tool .content { white-space: pre-wrap; }
    .row-reasoning .content { max-height: var(--reasoning-max-height); overflow-y: auto; }
    .row .content { width: 100%; }
    .row .content > *:first-child { margin-top: 0; }
    .row .content > *:last-child { margin-bottom: 0; }

    .content p { margin: 6px 0; white-space: pre-wrap; }
    .content h1, .content h2, .content h3, .content h4, .content h5, .content h6 {
      margin: 10px 0 6px; line-height: 1.3; color: var(--accent);
    }
    .content h1 { font-size: 1.35em; }
    .content h2 { font-size: 1.2em; }
    .content h3 { font-size: 1.08em; }
    .content ul, .content ol { margin: 6px 0; padding-left: 22px; }
    .content li { margin: 2px 0; }
    .content blockquote {
      margin: 6px 0;
      padding: 4px 12px;
      border-left: 3px solid var(--text-muted);
      color: var(--text-dim);
    }
    .content a { color: var(--accent); text-decoration: underline; }
    .content strong { color: var(--text); }
    .content code {
      font-family: var(--mono);
      background: var(--panel-2);
      padding: 1px 5px;
      border-radius: 4px;
      font-size: 0.92em;
    }
    .content pre {
      margin: 8px 0;
      padding: 0;
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 6px;
      overflow: hidden;
    }
    .content pre code {
      display: block;
      padding: 10px 12px;
      background: transparent;
      overflow-x: auto;
      font-size: 12.5px;
      line-height: 1.5;
    }
    .content table {
      border-collapse: collapse;
      margin: 8px 0;
      width: 100%;
      font-size: 12.5px;
    }
    .content th, .content td {
      border: 1px solid var(--border);
      padding: 5px 9px;
      text-align: left;
      vertical-align: top;
    }
    .content th {
      background: var(--panel-2);
      color: var(--accent);
      font-weight: 700;
    }
    .content tr:nth-child(even) td { background: rgba(0,0,0,0.06); }
    html[data-theme="light"] .content tr:nth-child(even) td { background: rgba(0,0,0,0.03); }
    .content hr { border: none; border-top: 1px solid var(--border); margin: 10px 0; }
    .content img { max-width: 100%; }

    .content img { max-width: 100%; }

    .footer {
      padding: 8px 12px;
      background: var(--panel);
      border-top: 1px solid var(--border);
    }
    .status {
      color: var(--accent-2);
      font-weight: 600;
      font-size: 12px;
      margin-bottom: 6px;
    }
    .status.error { color: var(--danger); }
    .composer {
      position: relative;
      display: flex;
      align-items: stretch;
    }
    .composer textarea {
      width: 100%;
      min-height: 28px;
      max-height: 200px;
      padding: 8px 38px 8px 10px;
      background: var(--panel-2);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      resize: none;
      font: inherit;
      line-height: 1.5;
      overflow-y: auto;
    }
    .composer textarea:focus { outline: 1px solid var(--accent); }
    .send-overlay {
      position: absolute;
      right: 6px;
      bottom: 6px;
      width: 26px;
      height: 26px;
      border: none;
      border-radius: 6px;
      background: var(--accent);
      color: var(--bg);
      font-size: 14px;
      font-weight: 700;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 0;
      line-height: 1;
    }
    .send-overlay:disabled { opacity: 0.4; cursor: default; }
    .send-overlay:hover:not(:disabled) { filter: brightness(1.15); }

    @media (max-width: 720px) {
      .topbar { flex-direction: column; align-items: stretch; }
      .topbar .workdir { flex: 0; }
    }
  </style>
</head>
<body>
  <div class="app">
    <div class="topbar">
      <span class="title">BlazeAI</span>
      <span id="workdir" class="workdir"></span>
      <button id="pickDir" class="pickdir-btn" title="Change work directory">&#x1F4C2;</button>
      <button id="theme" class="theme-btn" title="Toggle theme"></button>
      <label class="fs-control" title="Font size">
        <span class="fs-label">Aa</span>
        <select id="fontSize">
          <option value="11">11px</option>
          <option value="12">12px</option>
          <option value="13.5" selected>13.5px</option>
          <option value="15">15px</option>
          <option value="17">17px</option>
        </select>
      </label>
      <label>
        <span style="color:var(--text-dim);font-size:11px;">model</span>
        <select id="model"></select>
      </label>
      <button id="clear">Clear</button>
      <button id="closeSession">Close</button>
      <button id="quit" class="danger">Quit</button>
    </div>
    <div id="transcript" class="transcript"></div>
    <div class="footer">
      <div id="status" class="status">Ready</div>
      <form id="composer" class="composer">
        <textarea id="input" placeholder="Type a message or /help (Shift+Enter for newline)" rows="1"></textarea>
        <button id="send" class="send-overlay" type="submit" title="Send (Enter)">&#8629;</button>
      </form>
    </div>
  </div>
  <script>
__MARKED_JS__
__HIGHLIGHT_JS__
__DOMPURIFY_JS__
  </script>
  <script>
    // highlight.js theme stylesheets: two <style> elements, one disabled at a time.
    var hljsDarkStyle = document.createElement("style");
    hljsDarkStyle.textContent = "__HIGHLIGHT_DARK_CSS__";
    document.head.appendChild(hljsDarkStyle);
    var hljsLightStyle = document.createElement("style");
    hljsLightStyle.textContent = "__HIGHLIGHT_LIGHT_CSS__";
    document.head.appendChild(hljsLightStyle);
    function applyHljsTheme(theme) {
      hljsDarkStyle.sheet.disabled = (theme !== "dark");
      hljsLightStyle.sheet.disabled = (theme !== "light");
    }
  </script>
  <script>
    marked.setOptions({ gfm: true, breaks: true, headerIds: false, mangle: false });

    var transcriptEl = document.getElementById('transcript');
    var statusEl = document.getElementById('status');
    var workdirEl = document.getElementById('workdir');
    var inputEl = document.getElementById('input');
    var sendEl = document.getElementById('send');
    var modelEl = document.getElementById('model');
    var composerEl = document.getElementById('composer');
    var clearEl = document.getElementById('clear');
    var closeSessionEl = document.getElementById('closeSession');
    var quitEl = document.getElementById('quit');
    var themeEl = document.getElementById('theme');
    var fontSizeEl = document.getElementById('fontSize');
    var pickDirEl = document.getElementById('pickDir');

    var renderedBlocks = []; // {type, prefix, html} last applied
    var lastModel = '';
    var lastTheme = '';
    var requestInFlight = false;
    var autoScroll = true;

    function escapeHtml(s) {
      return s.replace(/[&<>"']/g, function (c) {
        return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c];
      });
    }

    function renderMarkdown(text) {
      if (!text) return '';
      var raw = marked.parse(text);
      return DOMPurify.sanitize(raw, { USE_PROFILES: { html: true } });
    }

    function rowClass(type) {
      return 'row row-' + type;
    }

    function applyState(state) {
      if (state.theme !== lastTheme) {
        document.documentElement.setAttribute('data-theme', state.theme);
        themeEl.textContent = state.theme === 'dark' ? '\u{1F319}' : '\u2600';
        applyHljsTheme(state.theme);
        lastTheme = state.theme;
      }
      if (state.font_size > 0) {
        document.documentElement.style.setProperty('--font-size', state.font_size + 'px');
        fontSizeEl.value = String(state.font_size);
      }
      if (state.reasoning_max_height > 0) {
        document.documentElement.style.setProperty('--reasoning-max-height', state.reasoning_max_height + 'px');
      }

      if (state.blocks && blocksChanged(state.blocks)) {
        renderBlocks(state.blocks);
        renderedBlocks = state.blocks.map(function (b) {
          return { type: b.type, prefix: b.prefix, length: b.text.length };
        });
        if (autoScroll) {
          transcriptEl.scrollTop = transcriptEl.scrollHeight;
        }
      }
      workdirEl.textContent = state.workdir;
      var busy = state.busy;
      inputEl.disabled = busy;
      sendEl.disabled = busy;
      clearEl.disabled = busy;
      closeSessionEl.disabled = busy;
      statusEl.textContent = state.status;
      statusEl.classList.toggle('error', /^error/i.test(state.status));

      var options = modelEl.options;
      if (JSON.stringify(Array.from(options).map(function (o) { return o.value; })) !== JSON.stringify(state.models || [])) {
        modelEl.innerHTML = '';
        (state.models || []).forEach(function (m) {
          var o = document.createElement('option');
          o.value = m; o.textContent = m;
          modelEl.appendChild(o);
        });
      }
      if (state.model !== lastModel) {
        modelEl.value = state.model;
        lastModel = state.model;
      }
    }

    function blocksChanged(blocks) {
      if (blocks.length !== renderedBlocks.length) return true;
      for (var i = 0; i < blocks.length; i++) {
        var r = renderedBlocks[i];
        if (!r || r.type !== blocks[i].type || r.prefix !== blocks[i].prefix || r.length !== blocks[i].text.length) {
          return true;
        }
      }
      return false;
    }

    function renderBlocks(blocks) {
      // Diff: rebuild only if count changed; otherwise update last block in place.
      var existing = transcriptEl.children;
      if (existing.length === blocks.length) {
        updateRow(existing[blocks.length - 1], blocks[blocks.length - 1]);
      } else if (existing.length < blocks.length) {
        for (var i = existing.length; i < blocks.length; i++) {
          var row = document.createElement('div');
          row.className = rowClass(blocks[i].type);
          renderRow(row, blocks[i]);
          transcriptEl.appendChild(row);
        }
      } else {
        transcriptEl.innerHTML = '';
        blocks.forEach(function (b) {
          var row = document.createElement('div');
          row.className = rowClass(b.type);
          renderRow(row, b);
          transcriptEl.appendChild(row);
        });
      }
      hljs.highlightAll();
    }

    function renderRow(row, block) {
      var prefix = document.createElement('div');
      prefix.className = 'prefix';
      prefix.textContent = block.prefix || '';
      var content = document.createElement('div');
      content.className = 'content';
      content.innerHTML = block.type === 'tool' ? escapeHtml(block.text) : renderMarkdown(block.text);
      row.innerHTML = '';
      row.appendChild(prefix);
      row.appendChild(content);
    }

    function updateRow(row, block) {
      row.className = rowClass(block.type);
      var content = row.querySelector('.content');
      if (!content) {
        renderRow(row, block);
        return;
      }
      content.innerHTML = block.type === 'tool' ? escapeHtml(block.text) : renderMarkdown(block.text);
      if (block.type === 'reasoning') content.scrollTop = content.scrollHeight;
      var prefix = row.querySelector('.prefix');
      if (prefix) prefix.textContent = block.prefix || '';
    }

    transcriptEl.addEventListener('scroll', function () {
      var atBottom = transcriptEl.scrollHeight - transcriptEl.scrollTop - transcriptEl.clientHeight < 40;
      autoScroll = atBottom;
    });

    async function refresh() {
      if (requestInFlight) return;
      requestInFlight = true;
      try {
        applyState(await getState());
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
        statusEl.classList.add('error');
      } finally {
        requestInFlight = false;
      }
    }

    composerEl.addEventListener('submit', async (event) => {
      event.preventDefault();
      var text = inputEl.value.trim();
      if (!text) return;
      inputEl.value = '';
      autoSizeInput();
      autoScroll = true;
      try {
        applyState(await sendMessage(text));
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
        statusEl.classList.add('error');
      }
    });

    clearEl.addEventListener('click', async () => {
      try { applyState(await clearSession()); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });

    closeSessionEl.addEventListener('click', async () => {
      try { applyState(await closeSession()); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });

    pickDirEl.addEventListener('click', async () => {
      try { applyState(await pickWorkDir()); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });
    quitEl.addEventListener('click', async () => {
      try { await quitApp(); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });

    modelEl.addEventListener('change', async () => {
      try { applyState(await changeModel(modelEl.value)); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });

    themeEl.addEventListener('click', async () => {
      var next = (lastTheme === 'dark') ? 'light' : 'dark';
      try { applyState(await setTheme(next)); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });
    fontSizeEl.addEventListener('change', async () => {
      var nextSize = parseFloat(fontSizeEl.value);
      document.documentElement.style.setProperty('--font-size', nextSize + 'px');
      try { applyState(await setFontSize(nextSize)); }
      catch (err) { statusEl.textContent = 'Error: ' + err.message; statusEl.classList.add('error'); }
    });

    inputEl.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey) {
        event.preventDefault();
        if (!inputEl.disabled) composerEl.requestSubmit();
      }
    });
    inputEl.addEventListener('input', autoSizeInput);

    function autoSizeInput() {
      inputEl.style.height = 'auto';
      var h = Math.min(inputEl.scrollHeight, 200);
      inputEl.style.height = Math.max(h, 28) + 'px';
    }

    refresh();
    setInterval(refresh, 700);
  </script>
</body>
</html>`

const (
	defaultDesktopWindowWidth  = 1100
	defaultDesktopWindowHeight = 760
	windowStateWriteDelay      = 350 * time.Millisecond
)

// stateResponse is the JSON payload pushed to the JS frontend.
//
// WHAT:  Carries the full transcript as typed blocks plus status, model, and theme.
// WHY:   The renderer rebuilds DOM rows from blocks to apply per-type backgrounds and markdown.
// PARAMS: Blocks — typed transcript rows; Status — footer line; Busy — blocks input;
// Model/Models — current and selectable model ids; WorkDir — runtime project path; Theme — active theme id.
type stateResponse struct {
	Blocks             []blockPayload `json:"blocks"`
	Status             string         `json:"status"`
	Busy               bool           `json:"busy"`
	Model              string         `json:"model"`
	Models             []string       `json:"models"`
	WorkDir            string         `json:"workdir"`
	Theme              string         `json:"theme"`
	FontSize           float64        `json:"font_size"`
	ReasoningMaxHeight float64        `json:"reasoning_max_height"`
}

// buildDesktopPage injects embedded vendor JS/CSS into the page template.
//
// WHAT:  Replaces placeholders with embedded marked, highlight, DOMPurify, and Monokai themes.
// WHY:   The page must render Markdown offline with no network access at runtime.
// RETURNS: ready HTML string; error if any vendor asset cannot be read.
func buildDesktopPage() (string, error) {
	markedJS, err := vendorFile("marked.min.js")
	if err != nil {
		return "", fmt.Errorf("cannot embed marked.js: %w", err)
	}
	highlightJS, err := vendorFile("highlight.min.js")
	if err != nil {
		return "", fmt.Errorf("cannot embed highlight.js: %w", err)
	}
	dompurifyJS, err := vendorFile("dompurify.min.js")
	if err != nil {
		return "", fmt.Errorf("cannot embed dompurify.js: %w", err)
	}
	darkCSS, err := vendorFile("highlight-monoaki-dark.css")
	if err != nil {
		return "", fmt.Errorf("cannot embed monokai dark css: %w", err)
	}
	lightCSS, err := vendorFile("highlight-monoaki-light.css")
	if err != nil {
		return "", fmt.Errorf("cannot embed monokai light css: %w", err)
	}
	r := strings.NewReplacer(
		"__MARKED_JS__", string(markedJS),
		"__HIGHLIGHT_JS__", string(highlightJS),
		"__DOMPURIFY_JS__", string(dompurifyJS),
		"__HIGHLIGHT_DARK_CSS__", string(darkCSS),
		"__HIGHLIGHT_LIGHT_CSS__", string(lightCSS),
	)
	return r.Replace(desktopPageTemplate), nil
}

// Run starts the singleton desktop companion transport and blocks until the app exits.
//
// WHAT:  Boots one embedded desktop chat window over the shared runtime core.
// WHY:   The desktop transport provides a persistent singleton conversation like Telegram.
// PARAMS: ctx — process lifetime context; cfg — loaded global runtime config; osType — detected OS;
// promptsFS — embedded prompt filesystem.
// RETURNS: error if startup or embedded window wiring fails.
func Run(ctx context.Context, cfg *config.Config, osType platform.OS, promptsFS fs.FS) error {
	desktopCfg, configPath, err := LoadConfig()
	if err != nil {
		return err
	}
	state, statePath, err := LoadState(cfg)
	if err != nil {
		return err
	}
	instanceDir, err := InstanceDir()
	if err != nil {
		return err
	}
	sessDir := filepath.Join(instanceDir, "session")
	sess, resumed, err := openDesktopSession(sessDir)
	if err != nil {
		return err
	}
	agent, err := runtimecore.NewAgent(cfg, sess, osType, promptsFS, desktopCfg.WorkDir, nil, "desktop")
	if err != nil {
		return fmt.Errorf("cannot create desktop agent: %w", err)
	}
	agent.Builder.TransportContext = strings.TrimSpace(`Desktop companion transport is active.
One singleton desktop app owns this transport.
Exactly one fixed session is resumed on every start.
Do not create or refer to multiple desktop conversations.
Replies are shown in one dedicated desktop window, not a terminal.
The desktop window renders full Markdown: headings, bold, italic, inline code, fenced code blocks with syntax highlighting, tables, lists, blockquotes, links, horizontal rules, and images displayed inline.
Use tables, code blocks, and structured formatting freely — the interface will render them correctly.
Avoid raw plain-text dumps when a table or a code block communicates the information more clearly.`)
	if resumed && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for desktop resume: %w", err)
		}
	}
	if err := agent.SetModelLocal(state.SelectedModelValue()); err != nil {
		return fmt.Errorf("cannot apply desktop model: %w", err)
	}

	page, err := buildDesktopPage()
	if err != nil {
		return err
	}

	ui := newDesktopUI(agent, cfg, desktopCfg, state, statePath, configPath)
	ui.loadSessionTranscript()
	view := webview.New(false)
	if view == nil {
		return fmt.Errorf("cannot create desktop window")
	}
	destroyed := false
	defer func() {
		if !destroyed {
			view.Destroy()
		}
	}()
	view.SetTitle("BlazeAI Desktop")
	initialBounds := WindowBounds{Width: defaultDesktopWindowWidth, Height: defaultDesktopWindowHeight}
	if storedBounds, ok := state.WindowBoundsValue(); ok {
		initialBounds.Width = storedBounds.Width
		initialBounds.Height = storedBounds.Height
	}
	view.SetSize(initialBounds.Width, initialBounds.Height, webview.HintNone)
	view.SetHtml(page)
	if err := ui.attach(view); err != nil {
		return err
	}
	platformUI, err := startDesktopPlatform(view, ui, desktopCfg, osType)
	if err != nil {
		return err
	}
	defer platformUI.Shutdown()

	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		<-ctx.Done()
		view.Terminate()
	}()
	go func() {
		<-ui.quitCh
		view.Terminate()
	}()
	view.Run()
	view.Destroy()
	destroyed = true
	if err := ui.flushState(); err != nil {
		return err
	}
	return nil
}

func openDesktopSession(sessionDir string) (*session.Session, bool, error) {
	sess, err := session.Load(sessionDir)
	if err == nil {
		return sess, true, nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return nil, false, fmt.Errorf("cannot load desktop session: %w", err)
	}
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, false, fmt.Errorf("cannot create desktop session folder: %w", err)
	}
	sess = &session.Session{
		Messages:      []session.Message{},
		ClosedCleanly: false,
		Folder:        sessionDir,
	}
	if err := sess.Save(); err != nil {
		return nil, false, fmt.Errorf("cannot create desktop session: %w", err)
	}
	return sess, false, nil
}

type desktopUI struct {
	agent       *runtimecore.Agent
	cfg         *config.Config
	desktopCfg  *Config
	state       *State
	statePath   string
	configPath  string
	handler     *Handler
	view       webview.WebView
	quitCh     chan struct{}
	quitOnce   sync.Once

	mu             sync.Mutex
	blocks         []transcriptBlock
	status         string
	busy           bool
	saveTimer      *time.Timer
	activeTool     int
	activeReply    int
	activeReasoning int
}

func newDesktopUI(agent *runtimecore.Agent, cfg *config.Config, desktopCfg *Config, state *State, statePath, configPath string) *desktopUI {
	ui := &desktopUI{
		agent:           agent,
		cfg:             cfg,
		desktopCfg:      desktopCfg,
		state:           state,
		statePath:       statePath,
		configPath:      configPath,
		quitCh:          make(chan struct{}),
		status:          "Ready",
		activeTool:      -1,
		activeReply:     -1,
		activeReasoning: -1,
	}
	ui.handler = NewHandler(ui)
	agent.Handler = ui.handler
	return ui
}

func (ui *desktopUI) attach(view webview.WebView) error {
	ui.view = view
	binds := []struct {
		name string
		fn   interface{}
	}{
		{"getState", ui.getState},
		{"sendMessage", ui.sendMessage},
		{"changeModel", ui.changeModel},
		{"clearSession", ui.clearSession},
		{"closeSession", ui.closeSession},
		{"quitApp", ui.quitApp},
		{"setTheme", ui.setTheme},
		{"setFontSize", ui.setFontSize},
		{"pickWorkDir", ui.pickWorkDir},
	}
	for _, b := range binds {
		if err := view.Bind(b.name, b.fn); err != nil {
			return fmt.Errorf("cannot bind %s: %w", b.name, err)
		}
	}
	return nil
}

func (ui *desktopUI) getState() stateResponse {
	return ui.snapshot()
}

func (ui *desktopUI) sendMessage(text string) (stateResponse, error) {
	if err := ui.submitInput(strings.TrimSpace(text)); err != nil {
		return stateResponse{}, err
	}
	return ui.snapshot(), nil
}

func (ui *desktopUI) changeModel(modelID string) (stateResponse, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return stateResponse{}, fmt.Errorf("model is required")
	}
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before changing the model")
	}
	if err := ui.agent.SetModelLocal(modelID); err != nil {
		return stateResponse{}, err
	}
	ui.state.SetSelectedModel(modelID)
	if err := ui.flushState(); err != nil {
		return stateResponse{}, err
	}
	ui.AppendSystem("Model set to: " + modelID)
	ui.SetStatus("Ready")
	return ui.snapshot(), nil
}

func (ui *desktopUI) setTheme(theme string) (stateResponse, error) {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return stateResponse{}, fmt.Errorf("theme is required")
	}
	if err := validateTheme(theme); err != nil {
		return stateResponse{}, err
	}
	ui.state.SetTheme(theme)
	if err := ui.flushState(); err != nil {
		return stateResponse{}, err
	}
	return ui.snapshot(), nil
}

func (ui *desktopUI) pickWorkDir() (stateResponse, error) {
	if ui.view == nil {
		return stateResponse{}, fmt.Errorf("window not ready")
	}
	selected, err := pickDirectoryNative("Select Work Directory", ui.agent.WorkDir)
	if err != nil {
		return stateResponse{}, err
	}
	selected = strings.TrimSpace(selected)
	if selected == "" || selected == ui.agent.WorkDir {
		return ui.snapshot(), nil
	}
	if err := ui.agent.SetWorkDir(selected); err != nil {
		return stateResponse{}, err
	}
	ui.desktopCfg.WorkDir = selected
	if err := ui.desktopCfg.SaveTo(ui.configPath); err != nil {
		return stateResponse{}, err
	}
	ui.AppendSystem("Work directory changed to: " + selected)
	return ui.snapshot(), nil
}

func (ui *desktopUI) setFontSize(fs float64) (stateResponse, error) {
	if fs <= 0 {
		return stateResponse{}, fmt.Errorf("font size must be greater than zero")
	}
	if err := validateFontSize(fs); err != nil {
		return stateResponse{}, err
	}
	ui.state.SetFontSize(fs)
	if err := ui.flushState(); err != nil {
		return stateResponse{}, err
	}
	return ui.snapshot(), nil
}

func (ui *desktopUI) clearSession() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before clearing the fixed session")
	}
	if err := ui.agent.ResetConversation(); err != nil {
		return stateResponse{}, err
	}
	ui.resetTranscript()
	ui.AppendSystem("Session cleared.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) closeSession() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before closing the session")
	}
	if err := ui.agent.CloseSession(); err != nil {
		return stateResponse{}, err
	}
	ui.AppendSystem("Session closed cleanly. Desktop app stays online.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) quitApp() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before quitting")
	}
	ui.requestQuit("Desktop app is shutting down.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) submitInput(text string) error {
	if text == "" {
		return fmt.Errorf("message is empty")
	}
	if ui.isBusy() {
		return fmt.Errorf("wait for the current turn to finish before sending another message")
	}
	ui.AppendUser(text)

	if strings.HasPrefix(text, "/") {
		handled, response, err := HandleCommand(text, ui.agent, ui.cfg, ui.state, ui.statePath)
		if err != nil {
			return err
		}
		if handled {
			if strings.HasPrefix(text, "/clear") || strings.HasPrefix(text, "/new") {
				ui.resetTranscript()
			}
			if response != "" {
				ui.AppendSystem(response)
			}
			return nil
		}
	}

	ui.handler.BeginTurn()
	go func(input string) {
		err := ui.agent.RunTurn(context.Background(), input)
		ui.handler.FinishTurn(err)
		if err != nil {
			ui.AppendSystem("Error: " + err.Error())
		}
	}(text)
	return nil
}

func (ui *desktopUI) loadSessionTranscript() {
	ui.resetTranscript()
	activity := &toolActivity{}
	for _, msg := range ui.agent.Session.Messages {
		switch msg.Role {
		case "user":
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				ui.AppendUser(text)
				activity.Reset()
			}
		case "assistant":
			if reasoning := strings.TrimSpace(msg.Reasoning); reasoning != "" {
				ui.StartReasoning()
				ui.AppendReasoning(reasoning)
				ui.FinishReasoning()
			}
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				ui.StartAssistant()
				ui.AppendAssistant(text)
				ui.FinishAssistant()
				activity.Reset()
			}
			for _, call := range decodeReplayToolCalls(msg.ToolCalls, ui.agent.Tools) {
				activity.AddCall(call.callID, call.name, call.args)
				ui.SetToolActivity(activity.Render())
			}
		case "tool":
			activity.ApplyResult(msg.ToolCallID, msg.Name, messageText(msg.Content))
			ui.SetToolActivity(activity.Render())
		}
	}
	ui.SetStatus("Ready")
}

func (ui *desktopUI) snapshot() stateResponse {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	blocks := make([]blockPayload, 0, len(ui.blocks))
	for _, b := range ui.blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		blocks = append(blocks, blockPayload{Type: b.Type, Prefix: b.Prefix, Text: text})
	}
	return stateResponse{
		Blocks:             blocks,
		Status:             ui.status,
		Busy:               ui.busy,
		Model:              ui.agent.ModelID,
		Models:             modelOptions(ui.cfg, ui.agent.ModelID),
		WorkDir:            truncatePath(ui.agent.WorkDir, 72),
		Theme:              ui.state.ThemeValue(),
		FontSize:           ui.state.FontSizeValue(),
		ReasoningMaxHeight: ui.desktopCfg.ReasoningMaxHeightValue(),
	}
}

func (ui *desktopUI) resetTranscript() {
	ui.mu.Lock()
	ui.blocks = nil
	ui.activeTool = -1
	ui.activeReply = -1
	ui.activeReasoning = -1
	ui.mu.Unlock()
}

func (ui *desktopUI) isBusy() bool {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	return ui.busy
}

func (ui *desktopUI) appendSection(blockType, prefix, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ui.mu.Lock()
	ui.activeTool = -1
	ui.activeReply = -1
	ui.activeReasoning = -1
	ui.blocks = append(ui.blocks, transcriptBlock{Type: blockType, Prefix: prefix, Text: text})
	ui.mu.Unlock()
}

// AppendUser adds one user message block to the transcript.
func (ui *desktopUI) AppendUser(text string) {
	ui.appendSection(blockUser, "You", text)
}

// AppendSystem adds one local desktop status block to the transcript.
func (ui *desktopUI) AppendSystem(text string) {
	ui.appendSection(blockSystem, "System", text)
}

// StartAssistant opens a new assistant block if one is not active yet.
// If a reasoning block is active, it is closed first.
func (ui *desktopUI) StartAssistant() {
	ui.mu.Lock()
	if ui.activeReply >= 0 {
		ui.mu.Unlock()
		return
	}
	if ui.activeReasoning >= 0 {
		ui.activeReasoning = -1
	}
	ui.activeTool = -1
	ui.blocks = append(ui.blocks, transcriptBlock{Type: blockAssistant, Prefix: "BlazeAI"})
	ui.activeReply = len(ui.blocks) - 1
	ui.mu.Unlock()
}

// AppendAssistant appends streamed assistant text to the active assistant block.
func (ui *desktopUI) AppendAssistant(delta string) {
	ui.mu.Lock()
	if ui.activeReply < 0 {
		ui.blocks = append(ui.blocks, transcriptBlock{Type: blockAssistant, Prefix: "BlazeAI"})
		ui.activeReply = len(ui.blocks) - 1
	}
	ui.blocks[ui.activeReply].Text += delta
	ui.mu.Unlock()
}

// FinishAssistant closes the current streamed assistant block.
func (ui *desktopUI) FinishAssistant() {
	ui.mu.Lock()
	ui.activeReply = -1
	ui.mu.Unlock()
}

// StartReasoning opens a new reasoning block if one is not active yet.
func (ui *desktopUI) StartReasoning() {
	ui.mu.Lock()
	if ui.activeReasoning >= 0 {
		ui.mu.Unlock()
		return
	}
	ui.activeTool = -1
	ui.activeReply = -1
	ui.blocks = append(ui.blocks, transcriptBlock{Type: blockReasoning, Prefix: "Reasoning"})
	ui.activeReasoning = len(ui.blocks) - 1
	ui.mu.Unlock()
}

// AppendReasoning appends streamed reasoning text to the active reasoning block.
func (ui *desktopUI) AppendReasoning(delta string) {
	ui.mu.Lock()
	if ui.activeReasoning < 0 {
		ui.blocks = append(ui.blocks, transcriptBlock{Type: blockReasoning, Prefix: "Reasoning"})
		ui.activeReasoning = len(ui.blocks) - 1
	}
	ui.blocks[ui.activeReasoning].Text += delta
	ui.mu.Unlock()
}

// FinishReasoning closes the current streamed reasoning block.
func (ui *desktopUI) FinishReasoning() {
	ui.mu.Lock()
	ui.activeReasoning = -1
	ui.mu.Unlock()
}

// SetToolActivity replaces or creates the current editable tool activity block.
func (ui *desktopUI) SetToolActivity(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ui.mu.Lock()
	ui.activeReply = -1
	ui.activeReasoning = -1
	if ui.activeTool >= 0 && ui.activeTool < len(ui.blocks) {
		ui.blocks[ui.activeTool].Text = text
	} else {
		ui.blocks = append(ui.blocks, transcriptBlock{Type: blockTool, Prefix: "Tool", Text: text})
		ui.activeTool = len(ui.blocks) - 1
	}
	ui.mu.Unlock()
}

// SetStatus updates the footer status line.
func (ui *desktopUI) SetStatus(text string) {
	ui.mu.Lock()
	ui.status = text
	ui.mu.Unlock()
}

// SetBusy toggles whether the desktop transport accepts another turn.
func (ui *desktopUI) SetBusy(active bool) {
	ui.mu.Lock()
	ui.busy = active
	ui.mu.Unlock()
}

func (ui *desktopUI) rememberWindowBounds(bounds WindowBounds) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if !ui.state.UpdateWindowBounds(bounds) {
		return
	}
	ui.mu.Lock()
	if ui.saveTimer == nil {
		ui.saveTimer = time.AfterFunc(windowStateWriteDelay, func() {
			_ = ui.flushState()
		})
	} else {
		ui.saveTimer.Reset(windowStateWriteDelay)
	}
	ui.mu.Unlock()
}

func (ui *desktopUI) flushState() error {
	ui.mu.Lock()
	if ui.saveTimer != nil {
		ui.saveTimer.Stop()
	}
	ui.mu.Unlock()
	if err := ui.state.SaveTo(ui.statePath, ui.cfg); err != nil {
		return fmt.Errorf("cannot persist desktop state: %w", err)
	}
	return nil
}

func (ui *desktopUI) requestQuit(message string) {
	if strings.TrimSpace(message) != "" {
		ui.AppendSystem(message)
	}
	ui.quitOnce.Do(func() { close(ui.quitCh) })
}

func modelOptions(cfg *config.Config, selected string) []string {
	seen := map[string]struct{}{}
	options := make([]string, 0, len(cfg.FavoriteModels)+1)
	for _, modelID := range cfg.FavoriteModels {
		if strings.TrimSpace(modelID) == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		options = append(options, modelID)
	}
	if strings.TrimSpace(selected) != "" {
		if _, ok := seen[selected]; !ok {
			options = append(options, selected)
		}
	}
	return options
}

func truncatePath(path string, max int) string {
	if max <= 0 || len(path) <= max {
		return path
	}
	if max <= 3 {
		return path[:max]
	}
	return "..." + path[len(path)-max+3:]
}

func messageText(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := messageTextPart(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(v)
	}
}

func messageTextPart(item interface{}) string {
	m, ok := item.(map[string]interface{})
	if !ok {
		return fmt.Sprint(item)
	}
	typeName, _ := m["type"].(string)
	switch typeName {
	case "text":
		text, _ := m["text"].(string)
		return text
	case "input_text":
		text, _ := m["text"].(string)
		return text
	case "image_url", "input_image":
		return "[image omitted]"
	default:
		return ""
	}
}