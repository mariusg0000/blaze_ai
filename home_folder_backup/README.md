# Home Folder Backup

Manual snapshot of selected BlazeAI `app_home` folders into the repository for version control.

## Included Folders

| Source (app_home) | Purpose |
|---|---|
| `config/` | config.json, modes.json |
| `skills/` | Global custom skill folders |
| `projects/<name>/skills/` | Project-scoped skill folders (all projects) |
| `scripts/` | Custom scripts (venv/ excluded) |

## Excluded

- `scripts/venv/` — Python virtual environment, not tracked
- `backups/`, `projects/<name>/sessions/` — runtime data, not configuration

## Manual Backup

Run from the project root:

```bash
./home_folder_backup/backup_home.sh
```

Or specify a different source:

```bash
./home_folder_backup/backup_home.sh /custom/path/to/blazeai
```

The target is always this `home_folder_backup/` folder. Commit to git after the backup to version the configuration.
