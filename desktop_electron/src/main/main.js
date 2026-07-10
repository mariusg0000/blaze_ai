// main.js — Electron desktop shell bootstrap.
// Starts the Go desktop backend, restores the last saved window geometry, and
// bridges renderer requests to the backend through ipcMain.
const { app, BrowserWindow, dialog, ipcMain } = require('electron');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const readline = require('node:readline');

let mainWindow = null;
let backend = null;
let boundsTimer = null;

class BackendClient {
  constructor(child) {
    this.child = child;
    this.nextID = 1;
    this.pending = new Map();

    const stdout = readline.createInterface({ input: child.stdout });
    stdout.on('line', (line) => this.handleLine(line));

    child.stderr.on('data', (chunk) => {
      const text = chunk.toString().trim();
      if (text) {
        console.error('[desktop-backend]', text);
      }
    });

    child.on('exit', (code, signal) => {
      const message = signal
        ? `desktop backend exited from signal ${signal}`
        : `desktop backend exited with code ${code}`;
      for (const pending of this.pending.values()) {
        pending.reject(new Error(message));
      }
      this.pending.clear();
      if (!app.isQuitting) {
        dialog.showErrorBox('BlazeAI Desktop', message);
        app.quit();
      }
    });
  }

  handleLine(line) {
    const text = String(line || '').trim();
    if (!text) {
      return;
    }
    let response;
    try {
      response = JSON.parse(text);
    } catch (error) {
      console.error('cannot parse backend response', error, text);
      return;
    }
    const pending = this.pending.get(response.id);
    if (!pending) {
      return;
    }
    this.pending.delete(response.id);
    if (!response.ok) {
      pending.reject(new Error(response.error || 'desktop backend request failed'));
      return;
    }
    pending.resolve(response.result);
  }

  call(method, params = {}) {
    const id = String(this.nextID++);
    const payload = JSON.stringify({ id, method, params });
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.child.stdin.write(payload + '\n', (error) => {
        if (!error) {
          return;
        }
        this.pending.delete(id);
        reject(error);
      });
    });
  }

  async dispose() {
    if (this.child.killed) {
      return;
    }
    this.child.kill();
  }
}

function backendExecutableName() {
  return process.platform === 'win32' ? 'blazeai-desktop-backend.exe' : 'blazeai-desktop-backend';
}

function resolveBackendPath() {
  const executableName = backendExecutableName();
  if (app.isPackaged) {
    const packagedPath = path.join(process.resourcesPath, 'backend', executableName);
    if (!fs.existsSync(packagedPath)) {
      throw new Error(`desktop backend binary missing: ${packagedPath}`);
    }
    return packagedPath;
  }
  const devPath = path.resolve(__dirname, '../../bin', executableName);
  if (!fs.existsSync(devPath)) {
    throw new Error(`desktop backend binary missing: ${devPath}. Run npm run build:backend first.`);
  }
  return devPath;
}

function spawnBackend() {
  const backendPath = resolveBackendPath();
  const args = [];
  if (process.env.BLAZEAI_DESKTOP_PROJECT) {
    args.push('-project', process.env.BLAZEAI_DESKTOP_PROJECT);
  }
  if (process.env.BLAZEAI_DESKTOP_LAST_SESSION === '1') {
    args.push('-last-session');
  }
  const child = spawn(backendPath, args, {
    stdio: ['pipe', 'pipe', 'pipe']
  });
  return new BackendClient(child);
}

function scheduleBoundsSave() {
  if (!mainWindow || !backend) {
    return;
  }
  if (boundsTimer) {
    clearTimeout(boundsTimer);
  }
  boundsTimer = setTimeout(() => {
    const bounds = mainWindow.getBounds();
    backend.call('set_window_bounds', bounds).catch((error) => {
      console.error('cannot persist window bounds', error);
    });
  }, 200);
}

async function createMainWindow() {
  backend = spawnBackend();
  const initialState = await backend.call('get_state');

  const browserWindowOptions = {
    width: 1100,
    height: 760,
    minWidth: 760,
    minHeight: 480,
    backgroundColor: '#272822',
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
      preload: path.join(__dirname, '../preload/preload.js')
    }
  };

  if (initialState.window && initialState.window.initialized) {
    browserWindowOptions.x = initialState.window.x;
    browserWindowOptions.y = initialState.window.y;
    browserWindowOptions.width = initialState.window.width;
    browserWindowOptions.height = initialState.window.height;
  }

  mainWindow = new BrowserWindow(browserWindowOptions);
  mainWindow.on('move', scheduleBoundsSave);
  mainWindow.on('resize', scheduleBoundsSave);
  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  await mainWindow.loadFile(path.join(__dirname, '../renderer/index.html'));
}

ipcMain.handle('desktop:call', async (_event, method, params) => {
  if (!backend) {
    throw new Error('desktop backend is not running');
  }
  return backend.call(method, params || {});
});

ipcMain.handle('desktop:pick-workdir', async (_event, options) => {
  if (!mainWindow) {
    throw new Error('desktop window is not ready');
  }
  const result = await dialog.showOpenDialog(mainWindow, {
    title: 'Select Work Directory',
    defaultPath: options && options.currentPath ? options.currentPath : undefined,
    properties: ['openDirectory']
  });
  if (result.canceled || result.filePaths.length === 0) {
    return null;
  }
  return {
    path: result.filePaths[0],
    resumeLastClean: Boolean(options && options.resumeLastClean)
  };
});

ipcMain.handle('desktop:cancel', async () => {
  if (!backend) {
    return false;
  }
  await backend.call('cancel', {});
  return true;
});

ipcMain.handle('desktop:quit', async () => {
  if (backend) {
    await backend.call('quit', {});
  }
  app.isQuitting = true;
  app.quit();
  return true;
});

app.on('window-all-closed', () => {
  app.isQuitting = true;
  app.quit();
});

app.on('before-quit', async () => {
  app.isQuitting = true;
  if (boundsTimer) {
    clearTimeout(boundsTimer);
    boundsTimer = null;
  }
  if (backend) {
    await backend.dispose();
    backend = null;
  }
});

app.whenReady().then(createMainWindow).catch((error) => {
  dialog.showErrorBox('BlazeAI Desktop', error.message);
  app.quit();
});
