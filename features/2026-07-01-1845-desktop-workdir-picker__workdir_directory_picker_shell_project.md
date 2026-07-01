## Feature Description
Desktop transport work directory change via native GTK directory picker, with ShellTool now respecting the agent's current project directory.

## Rationale And Implementation
User requested a button to open a native file dialog and change the working folder at runtime. The directory picker uses GTK's file chooser dialog and is persisted to config.json atomically. The ShellTool was missing the workdir closure — commands ran in the process's current directory instead of the project directory, which broke the workdir change UX.

## Modified Files
- internal/desktop/config.go: added SaveTo for atomic config persistence after workdir change
- internal/desktop/desktop.go: added pickWorkDir Go binding, folder button in topbar, workdir update via agent.SetWorkDir
- internal/desktop/platform_linux.go: added desktopPickDirectory C function wrapping GTK file chooser dialog
- internal/runtime/runtime.go: wired ShellTool with agent.WorkDir closure
- internal/tools/shell.go: added workDir func() string accessor so shell commands run in the project directory
