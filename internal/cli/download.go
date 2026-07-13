package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"w2g-cli/internal/client"
	"w2g-cli/internal/downloader"
)

func downloadCmd() *cli.Command {
	return &cli.Command{
		Name:    "download",
		Aliases: []string{"dl"},
		Usage:   "Download a playlist's audio via yt-dlp/ffmpeg",
		Description: `Download audio for every track in a playlist.

Tracks are fetched from their YouTube URLs with yt-dlp and converted to audio
with ffmpeg. Already-downloaded tracks are skipped (tracked per output folder),
so re-running only grabs what's new. Use --limit to cap new downloads per run.

    w2g download "Playlist1"
    w2g download "Playlist1" --limit 5 -o "D:\music\playlist1"
    w2g download --all --audio-format opus
    w2g download "Playlist1" --dry-run        # list what would be downloaded

Requirements: yt-dlp and ffmpeg. Install them or set paths:
    w2g config set ytdlp_path C:\tools\yt-dlp.exe
    w2g config set ffmpeg_path C:\tools\ffmpeg\bin
or pass the global --ytdlp / --ffmpeg (before the command).

More download options (bitrate, cookie_file, proxy, overwrite, sponsor_block,
ffmpeg_args, ytdlp_args, ...) live in the config — see ` + "`w2g config --help`" + `.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "all", Usage: "download every playlist in the room"},
			&cli.IntFlag{Name: "limit", Usage: "max NEW tracks to download this run (0 = unlimited)"},
			&cli.StringFlag{Name: "output-dir", Aliases: []string{"o"}, Usage: "output directory (default: config or ./w2g-downloads)"},
			&cli.StringFlag{Name: "audio-format", Usage: "audio format: mp3|m4a|opus|... (default: config)"},
			&cli.IntFlag{Name: "concurrency", Usage: "parallel downloads (default: config)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "list what would be downloaded, don't download"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			app := appFrom(ctx)
			if err := app.requireLogin(); err != nil {
				return err
			}
			room, err := app.requireRoom()
			if err != nil {
				return err
			}
			all := cmd.Bool("all")
			if !all && cmd.Args().Len() == 0 {
				return fmt.Errorf("specify a playlist name/key or use --all")
			}

			cl := app.Client()
			state, err := cl.Playlists(ctx, room)
			if err != nil {
				return err
			}

			var targets []client.Playlist
			if all {
				targets = state.Lists
			} else {
				pl, err := resolvePlaylist(state, cmd.Args().First())
				if err != nil {
					return err
				}
				targets = []client.Playlist{*pl}
			}

			baseDir := cmd.String("output-dir")
			if baseDir == "" {
				baseDir = app.Cfg.ResolveDownloadDir()
			}
			format := pick(cmd.String("audio-format"), app.Cfg.AudioFormat)
			ytdlp := pick(app.ytdlpPath, app.Cfg.YtDlpPath)
			ffmpeg := pick(app.ffmpegPath, app.Cfg.FFmpegPath)
			conc := cmd.Int("concurrency")
			if conc <= 0 {
				conc = app.Cfg.Concurrency
			}
			limit := cmd.Int("limit")
			dryRun := cmd.Bool("dry-run")

			var grand downloader.Summary
			for _, pl := range targets {
				items, err := cl.PlaylistItems(ctx, room, pl.Key)
				if err != nil {
					return fmt.Errorf("playlist %q: %w", pl.Title, err)
				}
				dir := filepath.Join(baseDir, sanitize(room), sanitize(pl.Title))

				dlItems := make([]downloader.Item, 0, len(items))
				for _, it := range items {
					dlItems = append(dlItems, downloader.Item{
						ID: it.ID, Title: it.Title, URL: it.NormalizedURL(),
					})
				}

				if dryRun {
					fmt.Fprintf(app.Out(), "== %s -> %s (%d items) ==\n", pl.Title, dir, len(dlItems))
					for i, it := range dlItems {
						fmt.Fprintf(app.Out(), "%3d. %s\n     %s\n", i+1, it.Title, it.URL)
					}
					continue
				}

				dl, err := downloader.New(downloader.Options{
					YtDlpPath:         ytdlp,
					FFmpegPath:        ffmpeg,
					OutputDir:         dir,
					AudioFormat:       format,
					Concurrency:       conc,
					Retries:           app.Cfg.Retries,
					Limit:             limit,
					Logger:            app.Log,
					OutputTemplate:    app.Cfg.OutputTemplate,
					Bitrate:           app.Cfg.Bitrate,
					Overwrite:         app.Cfg.Overwrite,
					CookieFile:        app.Cfg.CookieFile,
					Proxy:             app.Cfg.Proxy,
					FFmpegArgs:        app.Cfg.FFmpegArgs,
					YtDlpArgs:         app.Cfg.YtDlpArgs,
					SponsorBlock:      app.Cfg.SponsorBlock,
					RestrictFilenames: app.Cfg.RestrictFilenames,
				})
				if err != nil {
					return err
				}

				fmt.Fprintf(app.Errw(), "Downloading %q -> %s\n", pl.Title, dir)
				sum, err := dl.Run(ctx, dlItems)
				if err != nil && ctx.Err() != nil {
					return fmt.Errorf("interrupted: %w", ctx.Err())
				}
				grand.Total += sum.Total
				grand.Downloaded += sum.Downloaded
				grand.Skipped += sum.Skipped
				grand.Failed += sum.Failed

				for _, r := range sum.Results {
					if r.Err != nil && !errors.Is(r.Err, downloader.ErrLimitReached) {
						fmt.Fprintf(app.Errw(), "  FAILED: %s: %v\n", r.Item.Title, r.Err)
					}
				}
			}

			if dryRun {
				return nil
			}
			fmt.Fprintf(app.Out(), "\nDone. downloaded=%d skipped=%d failed=%d total=%d\n",
				grand.Downloaded, grand.Skipped, grand.Failed, grand.Total)
			if grand.Failed > 0 {
				return fmt.Errorf("%d track(s) failed to download", grand.Failed)
			}
			return nil
		},
	}
}

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func sanitize(s string) string {
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, repl(r))
	}
	res := string(out)
	if res == "" {
		return "playlist"
	}
	return res
}
