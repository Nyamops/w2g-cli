package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"w2g-cli/internal/cache"
	"w2g-cli/internal/client"
	"w2g-cli/internal/config"
	"w2g-cli/internal/logging"
)

var version = "0.1.0"

type App struct {
	Cfg   *config.Config
	Log   *slog.Logger
	Cache *cache.Cache

	room       string
	noCache    bool
	ffmpegPath string
	ytdlpPath  string

	in     io.Reader
	out    io.Writer
	err    io.Writer
	closer io.Closer
}

func (a *App) Out() io.Writer  { return a.out }
func (a *App) Errw() io.Writer { return a.err }

func (a *App) Room() string {
	if a.room != "" {
		return a.room
	}
	return a.Cfg.DefaultRoom
}

func (a *App) Client() *client.Client {
	return client.New(client.Options{
		Creds: client.Credentials{
			RememberToken: a.Cfg.RememberToken,
			SessionID:     a.Cfg.SessionID,
			Lang:          a.Cfg.Lang,
		},
		UserAgent: a.Cfg.UserAgent,
		Timeout:   time.Duration(a.Cfg.TimeoutSecs) * time.Second,
		Retries:   a.Cfg.Retries,
		Cache:     a.Cache,
		NoCache:   a.noCache,
		Logger:    a.Log,
	})
}

func (a *App) requireLogin() error {
	if !a.Cfg.LoggedIn() {
		return fmt.Errorf("not logged in — run `w2g login` first (see `w2g help`)")
	}
	return nil
}

func (a *App) requireRoom() (string, error) {
	r := a.Room()
	if r == "" {
		return "", fmt.Errorf("no room set — pass --room <id>, run `w2g join`, or `w2g config set default_room <id>`")
	}
	return r, nil
}

type appKey struct{}

func appFrom(ctx context.Context) *App {
	a, _ := ctx.Value(appKey{}).(*App)
	return a
}

func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "config", Usage: "config file location (default: OS user config dir)"},
		&cli.StringFlag{Name: "room", Usage: "room id (streamkey); overrides the configured default"},
		&cli.StringFlag{Name: "log-level", Usage: "debug|info|warn|error"},
		&cli.StringFlag{Name: "log-file", Usage: "write logs to a file instead of stderr"},
		&cli.StringFlag{Name: "ffmpeg", Usage: "path to ffmpeg binary or folder (overrides config ffmpeg_path)"},
		&cli.StringFlag{Name: "ytdlp", Usage: "path to yt-dlp (overrides config ytdlp_path)"},
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "info-level logging"},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "error-only logging"},
		&cli.BoolFlag{Name: "no-cache", Usage: "bypass the on-disk HTTP cache"},
	}
}

func setup(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	logger, closer, err := logging.Setup(logging.Options{
		Level:   cmd.String("log-level"),
		File:    cmd.String("log-file"),
		Verbose: cmd.Bool("verbose"),
		Quiet:   cmd.Bool("quiet"),
	})
	if err != nil {
		return ctx, err
	}

	cfg, err := config.Load(cmd.String("config"))
	if err != nil {
		closer.Close()
		return ctx, err
	}

	cacheDir, _ := cache.DefaultDir()
	app := &App{
		Cfg:        cfg,
		Log:        logger,
		Cache:      cache.New(cacheDir),
		room:       cmd.String("room"),
		noCache:    cmd.Bool("no-cache"),
		ffmpegPath: cmd.String("ffmpeg"),
		ytdlpPath:  cmd.String("ytdlp"),
		in:         os.Stdin,
		out:        os.Stdout,
		err:        os.Stderr,
		closer:     closer,
	}
	return context.WithValue(ctx, appKey{}, app), nil
}

func teardown(ctx context.Context, cmd *cli.Command) error {
	if app := appFrom(ctx); app != nil && app.closer != nil {
		app.closer.Close()
	}
	return nil
}

func Execute() int {
	root := &cli.Command{
		Name:           "w2g",
		Usage:          "command-line client for Watch2Gether (W2G)",
		Version:        version,
		Description:    "Start with `w2g help` for how to log in, then `w2g login`, `w2g join`, `w2g playlists`.",
		Flags:          globalFlags(),
		Before:         setup,
		After:          teardown,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		Commands: []*cli.Command{
			loginCmd(),
			helpCmd(),
			joinCmd(),
			roomsCmd(),
			playlistsCmd(),
			playlistCmd(),
			exportCmd(),
			downloadCmd(),
			cacheCmd(),
			configCmd(),
			versionCmd(),
		},
	}

	ctx, stop := signalContext()
	defer stop()

	if err := root.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
