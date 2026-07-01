// app.js — renderer transcript UI for the Electron desktop shell.
// Polls the Go backend for state, renders typed transcript blocks as Markdown,
// and routes user actions through the preload bridge.
(function () {
  marked.setOptions({ gfm: true, breaks: true, headerIds: false, mangle: false });

  const transcriptEl = document.getElementById('transcript');
  const statusEl = document.getElementById('status');
  const workdirEl = document.getElementById('workdir');
  const inputEl = document.getElementById('input');
  const sendEl = document.getElementById('send');
  const modelEl = document.getElementById('model');
  const composerEl = document.getElementById('composer');
  const clearEl = document.getElementById('clear');
  const closeSessionEl = document.getElementById('closeSession');
  const quitEl = document.getElementById('quit');
  const themeEl = document.getElementById('theme');
  const fontSizeEl = document.getElementById('fontSize');
  const pickDirEl = document.getElementById('pickDir');
  const darkThemeLink = document.getElementById('hljs-dark');
  const lightThemeLink = document.getElementById('hljs-light');

  let renderedBlocks = [];
  let lastModel = '';
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
    row.appendChild(prefix);
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
    const currentOptions = Array.from(modelEl.options).map((option) => option.value);
    const nextOptions = models || [];
    if (JSON.stringify(currentOptions) === JSON.stringify(nextOptions)) {
      return;
    }
    modelEl.innerHTML = '';
    for (const model of nextOptions) {
      const option = document.createElement('option');
      option.value = model;
      option.textContent = model;
      modelEl.appendChild(option);
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
    }

    workdirEl.textContent = state.workdir || '';
    statusEl.textContent = state.status || 'Ready';
    statusEl.classList.toggle('error', /^error/i.test(state.status || ''));

    const busy = Boolean(state.busy);
    inputEl.disabled = busy;
    sendEl.disabled = busy;
    clearEl.disabled = busy;
    closeSessionEl.disabled = busy;

    syncModelOptions(state.models);
    if (state.model !== lastModel) {
      modelEl.value = state.model || '';
      lastModel = state.model || '';
    }
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
      statusEl.textContent = `Error: ${error.message}`;
      statusEl.classList.add('error');
    } finally {
      requestInFlight = false;
    }
  }

  async function runAction(action) {
	try {
	  await action();
	} catch (error) {
	  statusEl.textContent = `Error: ${error.message}`;
	  statusEl.classList.add('error');
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
      statusEl.textContent = `Error: ${error.message}`;
      statusEl.classList.add('error');
    }
  });

  clearEl.addEventListener('click', () => runAction(async () => {
    applyState(await callBackend('clear_session'));
  }));

  closeSessionEl.addEventListener('click', () => runAction(async () => {
    applyState(await callBackend('close_session'));
  }));

  modelEl.addEventListener('change', () => runAction(async () => {
    applyState(await callBackend('change_model', { model: modelEl.value }));
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
