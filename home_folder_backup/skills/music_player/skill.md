[DESCRIPTION]
Load when the user wants to play, stop, or manage music playback, or create/use playlists. Use for mpv-based audio playback, playlist management, and library navigation. Library path, tracks, playlists, and radio stations in DATA.

[BEHAVIOR]

## Music Library Path

The library root is defined in the DATA section below as `music.library.path`.  
Default: `/mnt/DATA/Music`.

## Playback Engine

Use **mpv** — it's the most reliable CLI player available.  
Path: `/usr/bin/mpv`.

## Pre-flight Check (Always Run Before Starting Playback)

Before any play command, kill any existing mpv instance to prevent overlapping audio:

```bash
pkill mpv 2>/dev/null; sleep 0.2
```

This runs silently — if no mpv was running, nothing happens. Then start the new playback.

## Preferred Workflow

### 1. Playing a Single Track

Always kill existing mpv first, then daemonize properly with IPC socket:

```bash
pkill mpv 2>/dev/null; sleep 0.2
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket "/path/to/file.mp3" &>/dev/null &
```

**Confirm** playback started with:

```bash
pgrep mpv | head -1 && echo "PLAYING" || echo "STOPPED"
```

### 2. Playing a Playlist (Default: Loop)

Always use a temp playlist file — process substitution (`<(printf ...)`) is unreliable with `setsid` because the subshell exits before mpv reads the playlist.

Playlists loop by default (`--loop-playlist=inf` — repeats the whole playlist after it ends):

```bash
pkill mpv 2>/dev/null; sleep 0.2
printf '%s\n' /mnt/DATA/Music/*.mp3 > /tmp/mpv_playlist.txt
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket --loop-playlist=inf --playlist=/tmp/mpv_playlist.txt &>/dev/null &
```

For shuffle:

```bash
pkill mpv 2>/dev/null; sleep 0.2
printf '%s\n' /mnt/DATA/Music/*.mp3 > /tmp/mpv_playlist.txt
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket --shuffle --playlist=/tmp/mpv_playlist.txt &>/dev/null &
```

For shuffle + loop:
```bash
pkill mpv 2>/dev/null; sleep 0.2
printf '%s\n' /mnt/DATA/Music/*.mp3 > /tmp/mpv_playlist.txt
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket --shuffle --loop-playlist=inf --playlist=/tmp/mpv_playlist.txt &>/dev/null &
```

### 3. Playing a Named Playlist from DATA

If the DATA section defines a `music.playlist.<label>=<comma-separated tracks>`, extract track names and build a file list, then play with mpv.

**Procedure:**
1. Read the playlist value from the DATA section above.
2. Split by comma, trim whitespace.
3. Extract 1–3 most distinctive words from each track name (artist, main title word, unique keyword). Join all terms with `|`. Run a SINGLE `fd` command to find all matching files at once:

```bash
fd -t f -e mp3 "Term1|Term2|Term3|..." /mnt/DATA/Music/ > /tmp/mpv_playlist.txt
```

4. If some tracks were not found (fewer lines in `/tmp/mpv_playlist.txt` than expected), add broader fallback terms for the missing tracks and re-run the `fd` command appending to the file list.
5. Run the pre-flight kill, then play with `setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket --loop-playlist=inf --playlist=/tmp/mpv_playlist.txt &>/dev/null &`.

### 4. Playing Internet Radio

Radio station URLs are stored in the DATA section as `music.radio.<name>=<url>`.

**Procedure:**
1. Read `music.radio.<name>` from the DATA section above.
2. Run the pre-flight kill.
3. Play the stream:

```bash
pkill mpv 2>/dev/null; sleep 0.2
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket "<stream-url>" &>/dev/null &
```

4. Confirm playback.

[DATA]
music.library.path=/mnt/DATA/Music
music.download.tool=yt-dlp
music.download.format=mp3
music.download.settings=-x --audio-format mp3 --no-playlist
music.track.01=Skulgard - Spit My Name
music.track.02=Eldruna - Wellerman
music.track.03=Eiffel 65 - Blue
music.track.04=RAVNIR x Eldruna - Amber Road
music.track.05=Kriegswolf - Tochter
music.track.06=No Throne Above Me
music.track.07=Frozen Winds Rise
music.track.08=RAVNIR x Eldruna - Vølven
music.track.09=ODIN'S GATE - Varkul Iron
music.track.10=If You Feel It in Your Blood - Valhalla Awaits
music.playlist.2025-06-24=Skulgard - Spit My Name, RAVNIR x Eldruna - Amber Road, Kriegswolf - Tochter, No Throne Above Me, Frozen Winds Rise, RAVNIR x Eldruna - Vølven
music.playlist.viking=Skulgard - Spit My Name, Eldruna - Wellerman, RAVNIR x Eldruna - Amber Road, Kriegswolf - Tochter, No Throne Above Me, Frozen Winds Rise, RAVNIR x Eldruna - Vølven, ODIN'S GATE - Varkul Iron, If You Feel It in Your Blood - Valhalla Awaits
music.radio.rockfm-romania=https://live.rockfm.ro/rockfm.aacp
music.radio.magicfm-romania=https://live.magicfm.ro/magicfm.aacp

### 5. Stopping Playback

```bash
pkill mpv
```

Check with `pgrep mpv` before reporting.

### 6. Volume Control

mpv does not expose volume change remotely by default. Use `pactl` (PulseAudio) to adjust system/application volume.

- **Check current volume:** `pactl get-sink-volume @DEFAULT_SINK@`
- **Set to 50%:** `pactl set-sink-volume @DEFAULT_SINK@ 50%`
- **Set to 33% (one third):** `pactl set-sink-volume @DEFAULT_SINK@ 33%`
- **Increase by 10%:** `pactl set-sink-volume @DEFAULT_SINK@ +10%`
- **Decrease by 10%:** `pactl set-sink-volume @DEFAULT_SINK@ -10%`

If the system uses PipeWire instead of PulseAudio, `wpctl` may also work:

- **Check:** `wpctl get-volume @DEFAULT_AUDIO_SINK@`
- **Set to 50%:** `wpctl set-volume @DEFAULT_AUDIO_SINK@ 0.5`

Test `wpctl` availability first (`which wpctl`) before using it — it is not installed on all systems.

### 7. Checking Playback Status

```bash
if pgrep mpv >/dev/null; then echo "PLAYING (PID $(pgrep mpv | head -1))"; else echo "STOPPED"; fi
```

## Known Pitfalls

### Pitfall 1: mpv Blocks the Terminal

Running `mpv ...` without `setsid` and `&>/dev/null &` blocks the shell tool until timeout (60s default). Always use:

```bash
setsid mpv --no-terminal --really-quiet --input-ipc-server=/tmp/mpv-socket "<file>" &>/dev/null &
```

**Do not use** bare `mpv ... &` — it may still hold the terminal in some configurations.

### Pitfall 2: Overlapping Audio if Pre-flight is Skipped

If you skip the pre-flight `pkill mpv` before starting a new track, two mpv instances will play simultaneously. Always run the pre-flight check before any new playback.

### Pitfall 3: Filename Matching for Playlists

Track names in the skill data may not match filenames exactly (e.g., extra artist/title formatting, special characters). Use `fd` substring search as a fallback:

```bash
fd -t f -e mp3 "Wellerman" /mnt/DATA/Music/
```

When searching for multiple tracks, batch patterns in a SINGLE `fd` command using `|` as separator. Do NOT run separate `fd` calls per track — it wastes shell tool calls and inflates context with redundant tool results:

```bash
fd -t f -e mp3 "Skulgard|Eldruna|RAVNIR|Kriegswolf" /mnt/DATA/Music/ > /tmp/mpv_playlist.txt
```

If some tracks are not found with the first batch, add broader search terms and re-run once more.

### Pitfall 4: `wpctl` Not Available on PulseAudio-Only Systems

`wpctl` is a PipeWire tool and is not installed on systems running pure PulseAudio. Do not assume `wpctl` exists. Always check with `which wpctl` or fall back to `pactl` immediately. On this system, `pactl` is the reliable volume control.

### 5: Process Substitution (`<()`) Fails with `setsid`

Using `--playlist=<(printf '%s\n' ...)` combined with `setsid` is unreliable. The subshell that creates the FIFO exits before mpv finishes reading it, resulting in an empty or partial playlist. Always write a physical temp file (`/tmp/mpv_playlist.txt`) when using `setsid`.

**Do not use:**
```bash
setsid mpv --playlist=<(printf '%s\n' *.mp3) ...   # unreliable
```

**Use instead:**
```bash
printf '%s\n' *.mp3 > /tmp/mpv_playlist.txt
setsid mpv --playlist=/tmp/mpv_playlist.txt ...
```

### Pitfall 6: `--loop=inf` Loops Current File, Not the Playlist

`--loop=inf` (alias `--loop-file=inf`) loops the **current file** infinitely — it never advances to the next track in the playlist.
To loop the entire playlist, use **`--loop-playlist=inf`**.

**Wrong (stuck on first track):**
```bash
setsid mpv --loop=inf --playlist=/tmp/mpv_playlist.txt ...   # infinite loop of file 1
```

**Correct (loops through all tracks):**
```bash
setsid mpv --loop-playlist=inf --playlist=/tmp/mpv_playlist.txt ...
```

### Pitfall 7: Missing `socat` for IPC Control

To control mpv remotely via IPC, `socat` must be installed. Check with `which socat` and install if missing:

```bash
sudo apt install -y socat
```

Without `socat`, the IPC socket cannot be queried or controlled.

---

## IPC Remote Control

Every playback command in this skill now includes `--input-ipc-server=/tmp/mpv-socket`, enabling remote control via the Unix socket.

### Prerequisites

`socat` must be installed (`which socat`). See Pitfall 7 above.

### Available IPC Commands

Send commands via `socat`:

| Command | Action |
|---------|--------|
| `echo 'playlist-next' \| socat - /tmp/mpv-socket` | Next track |
| `echo 'playlist-prev' \| socat - /tmp/mpv-socket` | Previous track |
| `echo 'cycle pause' \| socat - /tmp/mpv-socket` | Pause / resume |
| `echo 'stop' \| socat - /tmp/mpv-socket` | Stop playback |
| `echo '{"command":["get_property","filename"]}' \| socat - /tmp/mpv-socket` | Get current filename |
| `echo '{"command":["get_property","time-pos"]}' \| socat - /tmp/mpv-socket` | Get current position (seconds) |
| `echo '{"command":["get_property","duration"]}' \| socat - /tmp/mpv-socket` | Get track duration (seconds) |
| `echo '{"command":["set","volume","50"]}' \| socat - /tmp/mpv-socket` | Set volume to 50% (mpv internal) |

The JSON commands return structured responses with `"error":"success"` on success.

### Quick Check — What's Playing

```bash
echo '{"command":["get_property","filename"]}' | socat - /tmp/mpv-socket 2>/dev/null
```

This returns the current filename and is the canonical way to check playback state instead of guessing from the playlist.

## Playlist Management

### Creating a New Playlist in Skill Data

Playlist data must be stored in the DATA section at the bottom of this skill.

Format for playlists in skill data:

```
music.playlist.<label>=<Track Artist - Title, Track2 Artist - Title, ...>
```

Examples:
```
music.playlist.favorites=Wardruna - Kvitravn, Heilung - Krigsgaldr, Danheim - Berserkir
music.playlist.2025-06-24=Skulgard - Spit My Name, Wardruna - Kvitravn, ...
```

### Steps to Create a Playlist

1. Ask the user which tracks and what label/name.
2. Validate that the tracks exist in the library (use `fd` or `ls`).
3. Append the new `music.playlist.<label>=<tracks>` entry to the DATA section at the bottom of this skill.
4. Confirm the playlist is saved.

### Playing a Named Playlist

1. Read the playlist line from the DATA section above.
2. Split the comma-separated track names.
3. For each track, find the matching file in the library.
4. Build and play the file list.
5. Play using the default looping command from section 2 (includes `--loop-playlist=inf`).

## First Commands to Try

| Intent | Command |
|--------|---------|
| List library | `ls /mnt/DATA/Music/` |
| Show playlists | Read `music.playlist.*` keys from DATA section |
| Show radio stations | Read `music.radio.*` keys from DATA section |
| Play single | `setsid mpv ...` pattern |
| Play all loop | `setsid mpv --loop-playlist=inf --playlist=/tmp/mpv_playlist.txt` |
| Play all shuffle | `setsid mpv --shuffle --playlist=/tmp/mpv_playlist.txt` |
| Play radio | `setsid mpv "<stream-url>"` pattern |
| Stop | `pkill mpv` |
| Volume 50% | `pactl set-sink-volume @DEFAULT_SINK@ 50%` |
| Next track | `echo 'playlist-next' \| socat - /tmp/mpv-socket` |
| Current track | `echo '{"command":["get_property","filename"]}' \| socat - /tmp/mpv-socket` |

## Safety

- Never kill processes without warning if other audio apps may be using mpv (rare on this system).
- Verify the file exists before playing.
- Quote paths to handle spaces or special characters in filenames.
