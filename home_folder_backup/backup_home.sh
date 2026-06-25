#!/usr/bin/env bash
#
# backup_home.sh — Copy selected app_home folders into the repository backup directory.
#
# Usage:
#   ./home_folder_backup/backup_home.sh              # default source: ~/blazeai
#   ./home_folder_backup/backup_home.sh /custom/app  # custom source

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKUP_DIR="$SCRIPT_DIR"

SOURCE="${1:-$HOME/blazeai}"

if [ ! -d "$SOURCE" ]; then
  echo "ERROR: Source directory not found: $SOURCE"
  exit 1
fi

echo "Backing up from: $SOURCE"
echo "Backing up to:   $BACKUP_DIR"

folders=("config" "memories" "skills")

for folder in "${folders[@]}"; do
  src="$SOURCE/$folder"
  dst="$BACKUP_DIR/$folder"
  if [ -d "$src" ]; then
    echo "  → $folder/"
    rm -rf "$dst"
    cp -r "$src" "$dst"
  else
    echo "  ✗ $folder/ (skipped — not found)"
  fi
done

# scripts/ — exclude venv/
src="$SOURCE/scripts"
dst="$BACKUP_DIR/scripts"
if [ -d "$src" ]; then
  echo "  → scripts/ (excluding venv/)"
  rm -rf "$dst"
  mkdir -p "$dst"
  for item in "$src"/*; do
    [ -e "$item" ] || continue
    name="$(basename "$item")"
    if [ "$name" = "venv" ]; then
      echo "      ⊘ scripts/venv/ (excluded)"
      continue
    fi
    cp -r "$item" "$dst/"
  done
else
  echo "  ✗ scripts/ (skipped — not found)"
fi

echo ""
echo "Backup complete. Review changes and commit:"
echo "  git status $BACKUP_DIR"
