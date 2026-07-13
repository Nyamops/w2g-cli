package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func cacheCmd() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Inspect or clear the on-disk HTTP cache",
		Description: `Manage the ETag-based response cache used by playlists/playlist/export.

    w2g cache path     print the cache directory
    w2g cache show      show entry count and size
    w2g cache clear     delete all cached responses`,
		Action: cacheShow,
		Commands: []*cli.Command{
			{
				Name:   "show",
				Usage:  "show entry count and size",
				Action: cacheShow,
			},
			{
				Name:  "path",
				Usage: "print the cache directory",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Fprintln(appFrom(ctx).Out(), appFrom(ctx).Cache.Dir())
					return nil
				},
			},
			{
				Name:  "clear",
				Usage: "delete all cached responses",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					app := appFrom(ctx)
					if err := app.Cache.Clear(); err != nil {
						return err
					}
					fmt.Fprintln(app.Out(), "Cache cleared.")
					return nil
				},
			},
		},
	}
}

func cacheShow(ctx context.Context, cmd *cli.Command) error {
	app := appFrom(ctx)
	n, b := app.Cache.Stats()
	fmt.Fprintf(app.Out(), "dir:     %s\n", app.Cache.Dir())
	fmt.Fprintf(app.Out(), "entries: %d\n", n)
	fmt.Fprintf(app.Out(), "size:    %s\n", humanBytes(b))
	return nil
}

func humanBytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := int64(n) / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
