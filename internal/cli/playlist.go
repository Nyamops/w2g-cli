package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"w2g-cli/internal/client"
)

func playlistCmd() *cli.Command {
	return &cli.Command{
		Name:  "playlist",
		Usage: "Show the contents of one playlist (by name or key)",
		Description: `Print every item in a playlist: title, URL and metadata.

The playlist can be given by its key or by (part of) its name:

    w2g playlist "Playlist1"
    w2g playlist rjqkl0vpyvr19wlnfiijlfps4lvmcsl8 --json
    w2g playlist "Playlist1" --urls        # just the URLs, one per line`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "output json"},
			&cli.BoolFlag{Name: "urls", Usage: "print only normalized URLs, one per line"},
			&cli.BoolFlag{Name: "activate", Usage: "also set this playlist as the room's active one"},
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
			if cmd.Args().Len() == 0 {
				return fmt.Errorf("usage: w2g playlist <name|key>")
			}
			cl := app.Client()
			state, err := cl.Playlists(ctx, room)
			if err != nil {
				return err
			}
			pl, err := resolvePlaylist(state, cmd.Args().First())
			if err != nil {
				return err
			}
			items, err := cl.PlaylistItems(ctx, room, pl.Key)
			if err != nil {
				return err
			}

			if cmd.Bool("activate") {
				if err := cl.SetActivePlaylist(ctx, room, pl.Key); err != nil {
					return fmt.Errorf("activate playlist: %w", err)
				}
				app.Log.Info("playlist activated", "title", pl.Title, "key", pl.Key)
			}

			switch {
			case cmd.Bool("json"):
				enc := json.NewEncoder(app.Out())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"title": pl.Title, "key": pl.Key, "count": len(items), "items": items,
				})
			case cmd.Bool("urls"):
				for _, it := range items {
					fmt.Fprintln(app.Out(), it.NormalizedURL())
				}
				return nil
			default:
				printPlaylistTable(app, pl, items)
				return nil
			}
		},
	}
}

func printPlaylistTable(app *App, pl *client.Playlist, items []client.PlaylistItem) {
	fmt.Fprintf(app.Out(), "%s (%s) — %d item(s)\n\n", pl.Title, pl.Key, len(items))
	tw := tabwriter.NewWriter(app.Out(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tTITLE\tURL")
	for i, it := range items {
		fmt.Fprintf(tw, "  %d\t%s\t%s\n", i+1, truncate(it.Title, 70), it.NormalizedURL())
	}
	tw.Flush()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
