// build-backend.mjs — builds the Go desktop backend into desktop_electron/bin/.
// Keeps the Electron shell and backend binary in one predictable layout for dev
// start and for packaging via electron-builder.
import { mkdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const appDir = resolve(scriptDir, '..');
const repoRoot = resolve(appDir, '..');
const outputDir = join(appDir, 'bin');
const executableName = process.platform === 'win32' ? 'blazeai-desktop-backend.exe' : 'blazeai-desktop-backend';
const outputPath = join(outputDir, executableName);

mkdirSync(outputDir, { recursive: true });

const result = spawnSync('go', ['build', '-o', outputPath, './cmd/blazeai-desktop-backend'], {
  cwd: repoRoot,
  stdio: 'inherit'
});

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
