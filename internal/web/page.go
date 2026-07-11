// page.go — single-page HTML application for the web transport.
// Inline CSS/JS with one embedded HTML template string served at /.
// Layer: transport output. Dependencies: none (pure string literal).
package web

const webPageHTML = `<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>BlazeAI Web</title>
<style>
  :root {
    --mono: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace;
    --font-size: 13.5px;
  }
  html[data-theme="dark"] {
    --bg: #272822;
    --text: #fbf4db;
    --user-text: #cde6ba;
    --orange: #f4a030;
    --blue: #66d9ef;
    --green: #a6e22e;
    --bright-green: #a6e22e;
    --purple: #ae81ff;
    --red: #f92672;
    --ctx: #66d9ef;
    --bright-blue: #66d9ef;
    --reasoning: #75715e;
    --border: #35362f;
    --panel: #1e1f1c;
    --panel-2: #32332e;
    --danger: #f92672;
  }
  html[data-theme="light"] {
    --bg: #fafafa;
    --text: #272822;
    --user-text: #4a7a2c;
    --orange: #c8701e;
    --blue: #2978a0;
    --green: #4a7a2c;
    --bright-green: #4a7a2c;
    --purple: #7a4a9c;
    --red: #a6276b;
    --ctx: #2978a0;
    --bright-blue: #2978a0;
    --reasoning: #a0988a;
    --border: #e4e2d5;
    --panel: #f4f2f0;
    --panel-2: #e4e2d5;
    --danger: #a6276b;
  }

  * { box-sizing: border-box; margin:0; padding:0; }
  body {
    font: var(--font-size)/1.45 var(--mono);
    background: var(--bg);
    color: var(--text);
    height: 100vh;
    display: grid;
    grid-template-rows: auto 1fr auto;
  }

  /* Top bar */
  .topbar {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 6px 10px;
    background: var(--panel);
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
    font-size: 12px;
  }
  .topbar .title {
    font-weight: 700;
    color: var(--orange);
    margin-right: 4px;
  }
  .topbar .workdir {
    flex: 1 1 120px;
    min-width: 0;
    color: var(--reasoning);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .topbar select,
  .topbar button {
    background: var(--panel-2);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 3px 7px;
    font: inherit;
    cursor: pointer;
    font-size: 11px;
  }
  .topbar select:focus,
  .topbar button:focus {
    outline: 1px solid var(--blue);
  }
  .topbar button { min-width: 22px; }
  .topbar .star-btn { font-size: 13px; padding: 2px 5px; }
  .topbar .checkbox-label {
    display: flex;
    align-items: center;
    gap: 3px;
    cursor: pointer;
    color: var(--reasoning);
    font-size: 11px;
  }
  .topbar .checkbox-label input { cursor: pointer; }

  /* Transcript */
  .transcript {
    overflow-y: auto;
    overflow-x: hidden;
    background: var(--bg);
    padding: 4px 0;
    scroll-behavior: smooth;
  }
  .row {
    padding: 4px 14px;
    white-space: pre-wrap;
    word-wrap: break-word;
    border-bottom: 1px solid var(--border);
  }
  .row-user { color: var(--user-text); }
  .row-system { color: var(--orange); border-bottom: 1px dashed var(--border); }
  .row-separator { border-bottom: none; padding: 2px 14px; }
  .row-reasoning { color: var(--reasoning); }
  .row-tool { color: var(--bright-green); }

  /* Inline styles */
  .bold { font-weight: 700; }
  .upper { text-transform: uppercase; }
  .orange { color: var(--orange); }
  .blue { color: var(--blue); }
  .green { color: var(--green); }
  .bright-green { color: var(--bright-green); }
  .purple { color: var(--purple); }
  .red { color: var(--red); }
  .ctx { color: var(--ctx); }
  .bright-blue { color: var(--bright-blue); }
  .reasoning { color: var(--reasoning); }
  .user-text { color: var(--user-text); }
  code {
    font-family: var(--mono);
    background: var(--panel-2);
    padding: 1px 4px;
    border-radius: 3px;
  }
  strong { color: var(--text); }
  em { color: var(--orange); }

  /* Footer */
  .footer {
    padding: 6px 10px;
    background: var(--panel);
    border-top: 1px solid var(--border);
  }
  .status {
    color: var(--bright-green);
    font-weight: 600;
    font-size: 12px;
    margin-bottom: 4px;
  }
  .status.error { color: var(--danger); }
  .composer {
    display: flex;
    align-items: stretch;
    gap: 6px;
  }
  .composer textarea {
    width: 100%;
    min-height: 28px;
    max-height: 200px;
    padding: 6px 8px;
    background: var(--panel-2);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 5px;
    resize: none;
    font: inherit;
    line-height: 1.5;
    overflow-y: auto;
    outline: none;
  }
  .composer textarea:focus { border-color: var(--blue); }
  .composer .send-btn {
    padding: 4px 12px;
    background: var(--blue);
    color: var(--bg);
    border: none;
    border-radius: 5px;
    font: inherit;
    font-weight: 700;
    cursor: pointer;
    font-size: 13px;
  }
  .composer .send-btn:disabled { opacity: 0.4; cursor: default; }
</style>
</head>
<body>
<div class="topbar">
  <span class="title">BlazeAI</span>
  <span id="workdir" class="workdir"></span>
  <select id="modeSelect" title="Work mode (Tab in console)"><option>--</option></select>
  <select id="modelSelect" title="Model (Ctrl+\ in console)"><option>--</option></select>
  <button id="starBtn" class="star-btn" title="Toggle favorite (Ctrl+F / Ctrl+R)">☆</button>
  <label class="checkbox-label" title="Reasoning display (Ctrl+T)">
    <input type="checkbox" id="reasoningCheck"> 🧠
  </label>
  <button id="clearBtn" title="Clear session (/clear)">Clear</button>
  <button id="themeBtn" title="Toggle theme">🌙</button>
</div>
<div id="transcript" class="transcript"></div>
<div class="footer">
  <div id="status" class="status">Ready</div>
  <div class="composer">
    <textarea id="input" placeholder="Type a message (Shift+Enter for newline)" rows="1"></textarea>
    <button id="sendBtn" class="send-btn" type="button">Send</button>
  </div>
</div>
<script>
var transcriptEl = document.getElementById('transcript');
var statusEl = document.getElementById('status');
var inputEl = document.getElementById('input');
var sendBtn = document.getElementById('sendBtn');
var workdirEl = document.getElementById('workdir');
var modeSelect = document.getElementById('modeSelect');
var modelSelect = document.getElementById('modelSelect');
var starBtn = document.getElementById('starBtn');
var reasoningCheck = document.getElementById('reasoningCheck');
var clearBtn = document.getElementById('clearBtn');
var themeBtn = document.getElementById('themeBtn');

var currentModel = '';
var currentFavorites = [];
var autoScroll = true;

// SSE connection.
var eventSource = new EventSource('/events');
var lastBlocks = []; // [{type, html}] for diff-based re-render.

eventSource.addEventListener('block', function(e) {
  var data = JSON.parse(e.data);
  if (data.streaming && data.index >= 0 && data.index < lastBlocks.length) {
    // Replace the exact server-side block index.
    lastBlocks[data.index] = {type: data.type, html: data.html};
  } else {
    // New block — append at the server's index. Reconnects normally replay in order.
    lastBlocks.push({type: data.type, html: data.html});
  }
  renderBlocks();
});

eventSource.addEventListener('status', function(e) {
  var data = JSON.parse(e.data);
  statusEl.textContent = data.text;
  statusEl.classList.toggle('error', /^Error/i.test(data.text));
  inputEl.disabled = data.busy;
  sendBtn.disabled = data.busy;
  clearBtn.disabled = data.busy;
});

eventSource.addEventListener('config', function(e) {
  var cfg = JSON.parse(e.data);
  currentModel = cfg.model;
  currentFavorites = cfg.favorites || [];

  // Populate mode dropdown.
  modeSelect.innerHTML = '';
  (cfg.modes || []).forEach(function(m) {
    var o = document.createElement('option');
    o.value = m; o.textContent = m;
    if (m === cfg.mode) o.selected = true;
    modeSelect.appendChild(o);
  });

  // Populate model dropdown: favorites first, then separator, then current if not in favorites.
  modelSelect.innerHTML = '';
  if (currentFavorites.length > 0) {
    currentFavorites.forEach(function(m) {
      var o = document.createElement('option');
      o.value = m; o.textContent = m;
      if (m === currentModel) o.selected = true;
      modelSelect.appendChild(o);
    });
    var sep = document.createElement('option');
    sep.disabled = true;
    sep.textContent = '──────────';
    modelSelect.appendChild(sep);
  }
  if (currentFavorites.indexOf(currentModel) === -1) {
    var o = document.createElement('option');
    o.value = currentModel; o.textContent = currentModel;
    o.selected = true;
    modelSelect.appendChild(o);
  }

  // Update star button.
  starBtn.textContent = currentFavorites.indexOf(currentModel) >= 0 ? '★' : '☆';

  // Update reasoning checkbox.
  reasoningCheck.checked = cfg.reasoning;

  // Update workdir.
  workdirEl.textContent = cfg.workdir;

  // Clear transcript on config events that signal a reset.
  if (cfg._clear) {
    lastBlocks = [];
    transcriptEl.innerHTML = '';
  }
});

eventSource.addEventListener('open', function() {
  statusEl.textContent = 'Connected';
});

eventSource.addEventListener('error', function() {
  statusEl.textContent = 'Disconnected';
  statusEl.classList.add('error');
});

// Re-render only changed blocks.
function renderBlocks() {
  var existing = transcriptEl.children;
  if (existing.length === lastBlocks.length) {
    updateRow(existing[lastBlocks.length - 1], lastBlocks[lastBlocks.length - 1]);
  } else if (existing.length < lastBlocks.length) {
    for (var i = existing.length; i < lastBlocks.length; i++) {
      transcriptEl.appendChild(buildRow(lastBlocks[i]));
    }
  } else {
    transcriptEl.innerHTML = '';
    lastBlocks.forEach(function(b) {
      transcriptEl.appendChild(buildRow(b));
    });
  }
  if (autoScroll) {
    transcriptEl.scrollTop = transcriptEl.scrollHeight;
  }
}

function buildRow(block) {
  var row = document.createElement('div');
  row.className = 'row row-' + block.type;
  row.innerHTML = block.html;
  return row;
}

function updateRow(row, block) {
  row.className = 'row row-' + block.type;
  row.innerHTML = block.html;
}

// Scroll tracking.
transcriptEl.addEventListener('scroll', function() {
  var atBottom = transcriptEl.scrollHeight - transcriptEl.scrollTop - transcriptEl.clientHeight < 40;
  autoScroll = atBottom;
});

// Send message.
function sendMessage() {
  var text = inputEl.value.trim();
  if (!text) return;
  inputEl.value = '';
  autoSizeInput();
  autoScroll = true;

  var form = new URLSearchParams();
  form.append('text', text);
  fetch('/input', {method: 'POST', body: form});
}

sendBtn.addEventListener('click', sendMessage);
inputEl.addEventListener('keydown', function(e) {
  if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault();
    if (!inputEl.disabled) sendMessage();
  }
});
inputEl.addEventListener('input', autoSizeInput);

function autoSizeInput() {
  inputEl.style.height = 'auto';
  var h = Math.min(inputEl.scrollHeight, 200);
  inputEl.style.height = Math.max(h, 28) + 'px';
}

// Mode change.
modeSelect.addEventListener('change', function() {
  var form = new URLSearchParams();
  form.append('name', modeSelect.value);
  fetch('/mode', {method: 'POST', body: form});
});

// Model change.
modelSelect.addEventListener('change', function() {
  var form = new URLSearchParams();
  form.append('id', modelSelect.value);
  fetch('/model', {method: 'POST', body: form});
});

// Toggle favorite.
starBtn.addEventListener('click', function() {
  fetch('/toggle-favorite', {method: 'POST'});
});

// Toggle reasoning.
reasoningCheck.addEventListener('change', function() {
  fetch('/toggle-reasoning', {method: 'POST'});
});

// Clear session.
clearBtn.addEventListener('click', function() {
  lastBlocks = [];
  transcriptEl.innerHTML = '';
  fetch('/clear', {method: 'POST'});
});

// Theme toggle (local storage only — not sent to server).
(function() {
  var theme = localStorage.getItem('blazeai-theme') || 'dark';
  document.documentElement.setAttribute('data-theme', theme);
  themeBtn.textContent = theme === 'dark' ? '🌙' : '☀️';
  themeBtn.addEventListener('click', function() {
    var next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('blazeai-theme', next);
    themeBtn.textContent = next === 'dark' ? '🌙' : '☀️';
  });
})();
</script>
</body>
</html>`
