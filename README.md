# w2g — Watch2Gether CLI

A small command-line client for [Watch2Gether](https://w2g.tv), written in Go.
Sign in, list your rooms and their playlists, export them, and download the
audio from the YouTube links with `yt-dlp` and `ffmpeg`.

## Install

You need Go 1.26 or newer. The module pins `go 1.26.0`

```sh
git clone https://github.com/Nyamops/w2g-cli
cd w2g-cli
go build -o w2g .        # or: go build -o w2g.exe . on Windows
```

The binary is self-contained. Its config and cache files live next to it, so
you can drop `w2g` in a folder and it stays portable.

## Dependencies

- **Go 1.26+** to build.
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)** and **[ffmpeg](https://www.ffmpeg.org/)**, only for the `download`
  command. Put them on your
  `PATH`, or point the CLI at them (see [Configuration](#configuration)).

If `ffmpeg` is missing, downloads still work: you get the best available audio
stream as-is (for example `.webm` or `.m4a`) instead of a converted `.mp3`.

## Quick start

```sh
w2g login                         # sign in with your W2G email and password
w2g rooms                         # list your rooms
w2g join room1                    # pick a room by name or id; saved as default
w2g playlists                     # list that room's playlists
w2g playlist "Playlist1"          # show one playlist
w2g export --all -o lists.txt
w2g download "Playlist1" --limit 5
```

Run `w2g help` for a walkthrough, or `w2g <command> --help` for any command.

## Commands

| Command                | What it does                                               |
|------------------------|------------------------------------------------------------|
| `login`                | Sign in with email + password (or store a token directly). |
| `rooms`                | List your rooms with their ids and members.                |
| `join <name\|id>`      | Set a room as your default. Also joins by access key.      |
| `playlists`            | List the current room's playlists.                         |
| `playlist <name\|key>` | Show a playlist as a table, json, or bare URLs.            |
| `export`               | Dump one or all playlists to text, json, csv, or m3u.      |
| `download <name\|key>` | Download a playlist's audio via yt-dlp.                    |
| `cache`                | Show, clear, or locate the on-disk HTTP cache.             |
| `config`               | View or edit the saved configuration.                      |

Playlists and rooms can be given by their key/id or by (part of) their name.

## Downloads

```sh
w2g download "Playlist1"                     # -> ./w2g-downloads/<room id>/Playlist1/
w2g download "Playlist1" -o "D:/music"       # -> D:/music/<room id>/Playlist1/
w2g download --all --audio-format opus       # every playlist, one folder each
w2g download "Playlist1" --dry-run           # list what would be fetched
```

Files land under `<download dir>/<room id>/<playlist name>/`. Re-running skips
tracks already downloaded (tracked with a per-folder manifest and a yt-dlp
archive), so you only ever pull what's new. `--limit N` caps new tracks per run.

## Configuration

`w2g config show` prints the current config (secrets masked). `w2g config set
<key> <value>` changes one value. The file lives next to the binary; run
`w2g config path` to find it.

Common keys:

| key                          | meaning                                  | default           |
|------------------------------|------------------------------------------|-------------------|
| `default_room`               | room id used when `--room` is absent     | —                 |
| `ytdlp_path` / `ffmpeg_path` | tool locations                           | `PATH`            |
| `download_dir`               | output directory                         | `./w2g-downloads` |
| `audio_format`               | mp3, m4a, opus, …                        | `mp3`             |
| `bitrate`                    | audio quality (`0`–`10`, or e.g. `128K`) | `0` (best)        |
| `overwrite`                  | `skip` or `force` existing files         | `skip`            |
| `concurrency`                | parallel downloads                       | `3`               |
| `cookie_file`                | cookies.txt for yt-dlp                   | —                 |
| `proxy`                      | http(s) proxy URL                        | —                 |
| `ffmpeg_args` / `ytdlp_args` | extra args for the tools                 | —                 |
| `sponsor_block`              | drop SponsorBlock segments               | `false`           |

See `w2g config --help` for the full list, including `retries`, `timeout_secs`,
`output_template`, and `restrict_filenames`.

## Global flags

```
--room <id>        room id; overrides the configured default
--config <path>    use a specific config file
--ffmpeg <path>    ffmpeg binary or folder (overrides config)
--ytdlp <path>     yt-dlp binary (overrides config)
--log-level <lvl>  debug | info | warn | error
--log-file <path>  write logs to a file instead of stderr
-v, --verbose      info-level logging
-q, --quiet        errors only
--no-cache         bypass the HTTP cache for one call
--version          print version
```

Global flags go before the command, e.g. `w2g --room <id> playlists`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
