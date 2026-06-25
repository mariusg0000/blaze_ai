#!/bin/bash
# Download YouTube audio as MP3
# Usage: ./download_mp3.sh <youtube-url> [output-dir]
# Default output dir: /mnt/DATA/Music/

URL="$1"
OUTDIR="${2:-/mnt/DATA/Music}"

if [ -z "$URL" ]; then
    echo "Usage: $0 <youtube-url> [output-dir]"
    exit 1
fi

mkdir -p "$OUTDIR"
cd "$OUTDIR"

yt-dlp -x --audio-format mp3 --no-playlist "$URL" -o "%(title)s.%(ext)s"
