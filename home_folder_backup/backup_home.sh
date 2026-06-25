#!/usr/bin/env bash
#
# backup_home.sh — Copy selected app_home folders into the repository backup directory.
#
# Backs up: config, global skills, project skills, scripts (excl. venv).
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

folders=("config" "skills")

for folder in "${folders[@]}"; do
  src="$SOURCE/$folder"
  dst="$BACKUP_DIR/$folder"
  if [ -d "$src" ]; then
    echo "  → $folder/"
    mkdir -p "$dst"
    cp -ruT "$src" "$dst"
  else
    echo "  ✗ $folder/ (skipped — not found)"
  fi
done

# project skills — backup each project's skills/ subfolder.
projects_src="$SOURCE/projects"
projects_dst="$BACKUP_DIR/projects"
if [ -d "$projects_src" ]; then
  for project_dir in "$projects_src"/*/; do
    [ -d "$project_dir" ] || continue
    project_name="$(basename "$project_dir")"
    skills_src="$project_dir/skills"
    if [ -d "$skills_src" ] && [ "$(ls -A "$skills_src" 2>/dev/null)" ]; then
      echo "  → projects/$project_name/skills/"
      skills_dst="$projects_dst/$project_name/skills"
      mkdir -p "$skills_dst"
      cp -ruT "$skills_src" "$skills_dst"
    fi
  done
else
  echo "  ✗ projects/ (skipped — not found)"
fi

# scripts/ — exclude venv/
src="$SOURCE/scripts"
dst="$BACKUP_DIR/scripts"
if [ -d "$src" ]; then
  echo "  → scripts/ (excluding venv/)"
  mkdir -p "$dst"
  for item in "$src"/*; do
    [ -e "$item" ] || continue
    name="$(basename "$item")"
    if [ "$name" = "venv" ]; then
      echo "      ⊘ scripts/venv/ (excluded)"
      continue
    fi
    cp -ru "$item" "$dst/"
  done
else
  echo "  ✗ scripts/ (skipped — not found)"
fi

echo ""
echo "Backup complete. Review changes and commit:"
echo "  git status $BACKUP_DIR"
