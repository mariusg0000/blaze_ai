// app.js — renderer transcript UI for the Electron desktop shell.
// Polls the Go backend for state, renders typed transcript blocks as Markdown,
// and routes user actions through the preload bridge.
(function () {
  marked.setOptions({ gfm: true, breaks: true, headerIds: false, mangle: false });

  const transcriptEl = document.getElementById('transcript');
  const workdirEl = document.getElementById('workdir');
  const inputEl = document.getElementById('input');
  const sendEl = document.getElementById('send');
  const stopEl = document.getElementById('stop');
  const modeSelectEl = document.getElementById('modeSelect');
  const modelSelectEl = document.getElementById('modelSelect');
  const composerEl = document.getElementById('composer');
  const clearEl = document.getElementById('clear');
  const closeSessionEl = document.getElementById('closeSession');
  const quitEl = document.getElementById('quit');
  const themeEl = document.getElementById('theme');
  const fontSizeEl = document.getElementById('fontSize');
  const pickDirEl = document.getElementById('pickDir');
  const darkThemeLink = document.getElementById('hljs-dark');
  const lightThemeLink = document.getElementById('hljs-light');
  const statusTextEl = document.getElementById('statusText');
  const ctxLabelEl = document.getElementById('ctxLabel');
  const ctxFillEl = document.getElementById('ctxFill');

  let renderedBlocks = [];
  let lastModel = '';
  let lastMode = '';
  let lastTheme = '';
  let lastState = null;
  let requestInFlight = false;
  let autoScroll = true;

  function escapeHtml(text) {
    return String(text).replace(/[&<>"']/g, (char) => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;'
    }[char]));
  }

  function renderMarkdown(text) {
    if (!text) {
      return '';
    }
    const raw = marked.parse(text);
    return DOMPurify.sanitize(raw, { USE_PROFILES: { html: true } });
  }

  function rowClass(type) {
    return `row row-${type}`;
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    themeEl.textContent = theme === 'dark' ? '\u{1F319}' : '\u2600';
    darkThemeLink.disabled = theme !== 'dark';
    lightThemeLink.disabled = theme !== 'light';
  }

  function blocksChanged(blocks) {
    if (blocks.length !== renderedBlocks.length) {
      return true;
    }
    for (let index = 0; index < blocks.length; index += 1) {
      const current = blocks[index];
      const rendered = renderedBlocks[index];
      if (!rendered || rendered.type !== current.type || rendered.prefix !== current.prefix || rendered.length !== current.text.length) {
        return true;
      }
    }
    return false;
  }

  function renderRow(row, block) {
    const prefix = document.createElement('div');
    prefix.className = 'prefix';
    prefix.textContent = block.prefix || '';

    const content = document.createElement('div');
    content.className = 'content';
    content.innerHTML = block.type === 'tool' ? escapeHtml(block.text) : renderMarkdown(block.text);

    row.className = rowClass(block.type);
    row.innerHTML = '';
    if (block.prefix) {
      row.appendChild(prefix);
    }
    row.appendChild(content);
  }

  function renderBlocks(blocks) {
    transcriptEl.innerHTML = '';
    for (const block of blocks) {
      const row = document.createElement('div');
      renderRow(row, block);
      transcriptEl.appendChild(row);
    }
    hljs.highlightAll();
  }

  function syncModelOptions(models) {
    const currentOptions = Array.from(modelSelectEl.options).map((option) => option.value);
    const nextOptions = models || [];
    if (JSON.stringify(currentOptions) === JSON.stringify(nextOptions)) {
      return;
    }
    modelSelectEl.innerHTML = '';
    for (const model of nextOptions) {
      const option = document.createElement('option');
      option.value = model;
      option.textContent = model;
      modelSelectEl.appendChild(option);
    }
  }

  function syncModeOptions(names, selected) {
    const currentOptions = Array.from(modeSelectEl.options).map((o) => o.value);
    if (JSON.stringify(currentOptions) !== JSON.stringify(names)) {
      modeSelectEl.innerHTML = '';
      for (const name of names) {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = name;
        modeSelectEl.appendChild(option);
      }
    }
    if (selected) {
      modeSelectEl.value = selected;
    }
  }

  function applyState(state) {
    lastState = state;

    if (state.theme !== lastTheme) {
      applyTheme(state.theme);
      lastTheme = state.theme;
    }

    if (state.font_size > 0) {
      document.documentElement.style.setProperty('--font-size', `${state.font_size}px`);
      fontSizeEl.value = String(state.font_size);
    }

    if (state.reasoning_max_height > 0) {
      document.documentElement.style.setProperty('--reasoning-max-height', `${state.reasoning_max_height}px`);
    }

    if (state.blocks && blocksChanged(state.blocks)) {
      renderBlocks(state.blocks);
      renderedBlocks = state.blocks.map((block) => ({
        type: block.type,
        prefix: block.prefix,
        length: block.text.length
      }));
      if (autoScroll) {
        transcriptEl.scrollTop = transcriptEl.scrollHeight;
      }
      const reasoningContents = transcriptEl.querySelectorAll('.row-reasoning .content');
      for (const content of reasoningContents) {
        content.scrollTop = content.scrollHeight;
      }
    }

    workdirEl.textContent = state.workdir || '';

    statusTextEl.textContent = state.status || 'Ready';
    statusTextEl.className = 'status-text';
    statusTextEl.classList.toggle('error', /^error/i.test(state.status || ''));

    syncModeOptions(state.mode_names || [], state.mode_name || '');
    syncModelOptions(state.models);
    if (state.model !== lastModel) {
      modelSelectEl.value = state.model || '';
      lastModel = state.model || '';
    }
    if (state.mode_name !== lastMode) {
      modeSelectEl.value = state.mode_name || '';
      lastMode = state.mode_name || '';
    }

    if (state.max_context_tokens > 0) {
      const used = Math.min(state.used_context_tokens || 0, state.max_context_tokens);
      const pct = (used / state.max_context_tokens) * 100;
      ctxLabelEl.textContent = 'CTX ' + used.toLocaleString();
      ctxFillEl.style.width = Math.min(pct, 100) + '%';
    } else {
      ctxLabelEl.textContent = '';
      ctxFillEl.style.width = '0%';
    }

    const busy = Boolean(state.busy);
    inputEl.disabled = busy;
    sendEl.hidden = busy;
    stopEl.style.display = busy ? 'flex' : 'none';
    clearEl.disabled = busy;
    closeSessionEl.disabled = busy;

  }

  async function callBackend(method, params) {
    return window.blazeDesktop.call(method, params || {});
  }

  async function refresh() {
    if (requestInFlight) {
      return;
    }
    requestInFlight = true;
    try {
      applyState(await callBackend('get_state'));
    } catch (error) {
      statusTextEl.textContent = `Error: ${error.message}`;
      statusTextEl.classList.add('error');
    } finally {
      requestInFlight = false;
    }
  }

  async function runAction(action) {
	try {
	  await action();
	} catch (error) {
	  statusTextEl.textContent = `Error: ${error.message}`;
	  statusTextEl.classList.add('error');
	}
  }

  function autoSizeInput() {
    inputEl.style.height = 'auto';
    const height = Math.min(inputEl.scrollHeight, 200);
    inputEl.style.height = `${Math.max(height, 28)}px`;
  }

  transcriptEl.addEventListener('scroll', () => {
    const atBottom = transcriptEl.scrollHeight - transcriptEl.scrollTop - transcriptEl.clientHeight < 40;
    autoScroll = atBottom;
  });

  composerEl.addEventListener('submit', async (event) => {
    event.preventDefault();
    const text = inputEl.value.trim();
    if (!text) {
      return;
    }
    inputEl.value = '';
    autoSizeInput();
    autoScroll = true;
    try {
      applyState(await callBackend('send_message', { text }));
    } catch (error) {
      statusTextEl.textContent = `Error: ${error.message}`;
      statusTextEl.classList.add('error');
    }
  });

  clearEl.addEventListener('click', () => runAction(async () => {
    applyState(await callBackend('clear_session'));
  }));

  closeSessionEl.addEventListener('click', () => runAction(async () => {
    applyState(await callBackend('close_session'));
  }));

  modelSelectEl.addEventListener('change', () => runAction(async () => {
    applyState(await callBackend('change_model', { model: modelSelectEl.value }));
  }));

  modeSelectEl.addEventListener('change', () => runAction(async () => {
    applyState(await callBackend('change_mode', { name: modeSelectEl.value }));
  }));

  themeEl.addEventListener('click', () => runAction(async () => {
    const nextTheme = lastTheme === 'dark' ? 'light' : 'dark';
    applyState(await callBackend('set_theme', { theme: nextTheme }));
  }));

  fontSizeEl.addEventListener('change', () => runAction(async () => {
    const fontSize = Number.parseFloat(fontSizeEl.value);
    document.documentElement.style.setProperty('--font-size', `${fontSize}px`);
    applyState(await callBackend('set_font_size', { font_size: fontSize }));
  }));

  pickDirEl.addEventListener('click', () => runAction(async () => {
    const resumeLastClean = window.confirm('Resume the last clean session after switching to the selected project?\n\nChoose Cancel to start a new desktop session in that project.');
    const selected = await window.blazeDesktop.pickWorkDir({
      currentPath: lastState ? lastState.workdir_full : '',
      resumeLastClean
    });
    if (!selected) {
      return;
    }
    applyState(await callBackend('set_workdir', {
      path: selected.path,
      resume_last_clean: selected.resumeLastClean
    }));
  }));

  quitEl.addEventListener('click', () => runAction(async () => {
    await window.blazeDesktop.quit();
  }));

  stopEl.addEventListener('click', async () => {
    try {
      await window.blazeDesktop.cancel();
    } catch (error) {
      statusTextEl.textContent = `Error: ${error.message}`;
      statusTextEl.classList.add('error');
    }
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Tab' && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey) {
      event.preventDefault();
      runAction(async () => {
        applyState(await callBackend('next_mode'));
      });
    }
  });

  inputEl.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.metaKey && !event.altKey) {
      event.preventDefault();
      if (!inputEl.disabled) {
        composerEl.requestSubmit();
      }
    }
  });

  inputEl.addEventListener('input', autoSizeInput);

  refresh();
  setInterval(refresh, 700);
})();
