[DESCRIPTION]
Load when the user wants to download YouTube videos as MP3 audio files, create playlists, or build a music library. Use for yt-dlp MP3 extraction, batch downloads, and managing local music collections.

[BEHAVIOR]

# YouTube Music Downloader

## Quick download
```bash
# Single track
{SKILL_DIR}/scripts/download_mp3.sh "https://www.youtube.com/watch?v=..."
```

## Manual yt-dlp usage
```bash
cd /mnt/DATA/Music
yt-dlp -x --audio-format mp3 --no-playlist "<url>" -o "%(title)s.%(ext)s"
```

## Flags
| Flag | Purpose |
|------|---------|
| `-x` | Extract audio |
| `--audio-format mp3` | Output format |
| `--no-playlist` | Single track only |
| `-o "%(title)s.%(ext)s"` | Output naming |

## Search & download from YouTube search
```bash
# Search first to find URLs
yt-dlp "ytsearch5:<query>" --no-playlist --print "%(title)s %(webpage_url)s"

# Then download desired ones
yt-dlp -x --audio-format mp3 --no-playlist "<url>" -o "%(title)s.%(ext)s"
```

## Batch download multiple tracks
```bash
# Sequential (recomandat)
for url in \
  "https://youtube.com/watch?v=URL1" \
  "https://youtube.com/watch?v=URL2"; do
  yt-dlp -x --audio-format mp3 --no-playlist "$url" -o "/mnt/DATA/Music/%(title)s.%(ext)s"
done

# Parallel (folosește -P / --paths pentru a evita CWD issues)
yt-dlp -x --audio-format mp3 --no-playlist -P "/mnt/DATA/Music" "URL1" &
yt-dlp -x --audio-format mp3 --no-playlist -P "/mnt/DATA/Music" "URL2" &
wait
```

## Output directory
Default: `/mnt/DATA/Music/`

## Post-processing: Normalization

Tracke de pe YouTube vin la nivele de volum diferite. După download, normalizează-le cu ffmpeg loudnorm (EBU R128, țintă -16 LUFS) pentru volum uniform.

### Procedură (two-pass loudnorm)

```bash
cd /mnt/DATA/Music
f="Nume fisier.mp3"

# Pass 1 - analiză
raw=$(ffmpeg -i "$f" -af loudnorm=I=-16:LRA=11:TP=-1.5:print_format=json -f null - 2>&1 \
  | grep -A 10 "Parsed_loudnorm" \
  | grep -E '"input_i"|"input_lra"|"input_tp"|"input_thresh"' \
  | sed 's/[",]//g; s/^[[:space:]]*//')
eval $(echo "$raw" | awk -F': ' '{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1); gsub(/^[[:space:]]+|[[:space:]]+$/, "", $2); print $1 "=" $2}')

# Pass 2 - normalizare (folosește nume temporar UNIC per fișier)
ffmpeg -y -i "$f" \
  -af loudnorm=I=-16:LRA=11:TP=-1.5:measured_I=$input_i:measured_LRA=$input_lra:measured_TP=$input_tp:measured_thresh=$input_thresh \
  -c:a libmp3lame -q:a 2 "${f%.mp3}_norm.mp3" && mv "${f%.mp3}_norm.mp3" "$f"
```

### Normalizare inline (one-pass, mai puțin precisă)
```bash
cd /mnt/DATA/Music
for f in *.mp3; do
  ffmpeg -y -i "$f" -af loudnorm=I=-16:LRA=11:TP=-1.5 -c:a libmp3lame -q:a 2 "${f%.mp3}_norm.mp3" && mv "${f%.mp3}_norm.mp3" "$f"
done
```

### Recomandare
Folosește two-pass când ai track-uri individuale. One-pass e suficient pentru batch-uri mari.

## Dependencies
- `yt-dlp` (latest via pip)
- `ffmpeg` (for audio conversion)
