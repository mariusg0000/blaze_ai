// preload.js — safe renderer bridge for the desktop shell.
// Exposes a narrow IPC API for backend calls, workdir picking, and app quit.
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('blazeDesktop', {
  call(method, params) {
    return ipcRenderer.invoke('desktop:call', method, params || {});
  },
  pickWorkDir(options) {
    return ipcRenderer.invoke('desktop:pick-workdir', options || {});
  },
  quit() {
    return ipcRenderer.invoke('desktop:quit');
  }
});
