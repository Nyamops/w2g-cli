package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/urfave/cli/v3"

	"w2g-cli/internal/config"
)

func configCmd() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "View or edit configuration",
		Description: `Inspect and modify the persisted configuration.

    w2g config path                 print the config file location
    w2g config show                 print the config (secrets masked)
    w2g config get download_dir     print one value
    w2g config set retries 5        set one value and save

Editable keys:
    lang, default_room, nickname, access_key, ffmpeg_path, ytdlp_path,
    download_dir, audio_format, user_agent, concurrency, retries, timeout_secs

Download keys (yt-dlp):
    output_template   yt-dlp output template (default: %(title)s [%(id)s].%(ext)s)
    bitrate           audio quality: 0-10 (VBR) or e.g. 128K (default: 0 = best)
    overwrite         skip|force existing files (default: skip)
    cookie_file       path to a cookies.txt for yt-dlp
    proxy             http(s) proxy URL
    ffmpeg_args       extra args passed to ffmpeg during extraction
    ytdlp_args        extra args appended to the yt-dlp command
    sponsor_block     true|false: remove SponsorBlock segments (needs ffmpeg)
    restrict_filenames true|false: ASCII-safe file names

Credentials (remember_user_token, w2g_session_id) are set via ` + "`w2g login`" + `.`,
		Action: configShow,
		Commands: []*cli.Command{
			{
				Name:   "show",
				Usage:  "print the config (secrets masked)",
				Action: configShow,
			},
			{
				Name:  "path",
				Usage: "print the config file location",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					app := appFrom(ctx)
					fmt.Fprintln(app.Out(), app.Cfg.Path())
					return nil
				},
			},
			{
				Name:            "get",
				Usage:           "print one value",
				SkipFlagParsing: true,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					app := appFrom(ctx)
					if cmd.Args().Len() < 1 {
						return fmt.Errorf("usage: w2g config get KEY")
					}
					v, err := configGet(app, cmd.Args().First())
					if err != nil {
						return err
					}
					fmt.Fprintln(app.Out(), v)
					return nil
				},
			},
			{
				Name:            "set",
				Usage:           "set one value and save",
				SkipFlagParsing: true,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					app := appFrom(ctx)
					if cmd.Args().Len() < 2 {
						return fmt.Errorf("usage: w2g config set KEY VALUE")
					}
					key, value := cmd.Args().Get(0), cmd.Args().Get(1)
					if err := app.Cfg.Set(key, value); err != nil {
						return err
					}
					if err := app.Cfg.Save(); err != nil {
						return err
					}
					fmt.Fprintf(app.Out(), "Set %s. Saved to %s\n", key, app.Cfg.Path())
					return nil
				},
			},
		},
	}
}

func configShow(ctx context.Context, cmd *cli.Command) error {
	app := appFrom(ctx)
	enc := json.NewEncoder(app.Out())
	enc.SetIndent("", "  ")
	return enc.Encode(app.Cfg.Redacted())
}

func configGet(app *App, key string) (string, error) {
	c := app.Cfg
	switch key {
	case "lang":
		return c.Lang, nil
	case "default_room", "room":
		return c.DefaultRoom, nil
	case "nickname":
		return c.Nickname, nil
	case "access_key":
		return config.Mask(c.AccessKey), nil
	case "ffmpeg_path":
		return c.FFmpegPath, nil
	case "ytdlp_path":
		return c.YtDlpPath, nil
	case "download_dir":
		return c.ResolveDownloadDir(), nil
	case "audio_format":
		return c.AudioFormat, nil
	case "user_agent":
		return c.UserAgent, nil
	case "output_template":
		return c.OutputTemplate, nil
	case "bitrate":
		return c.Bitrate, nil
	case "cookie_file":
		return c.CookieFile, nil
	case "proxy":
		return c.Proxy, nil
	case "ffmpeg_args":
		return c.FFmpegArgs, nil
	case "ytdlp_args":
		return c.YtDlpArgs, nil
	case "overwrite":
		return c.Overwrite, nil
	case "sponsor_block":
		return strconv.FormatBool(c.SponsorBlock), nil
	case "restrict_filenames":
		return strconv.FormatBool(c.RestrictFilenames), nil
	case "concurrency":
		return strconv.Itoa(c.Concurrency), nil
	case "retries":
		return strconv.Itoa(c.Retries), nil
	case "timeout_secs":
		return strconv.Itoa(c.TimeoutSecs), nil
	default:
		return "", fmt.Errorf("unknown config key %q", key)
	}
}
