package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
)

func playlistsCmd() *cli.Command {
	return &cli.Command{
		Name:        "playlists",
		Aliases:     []string{"lists"},
		Usage:       "List the room's playlists (count, names, keys)",
		Description: "Fetch the room state and print how many playlists exist and their names/keys.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "output json"},
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
			state, err := app.Client().Playlists(ctx, room)
			if err != nil {
				return err
			}

			if cmd.Bool("json") {
				enc := json.NewEncoder(app.Out())
				enc.SetIndent("", "  ")
				return enc.Encode(state)
			}

			fmt.Fprintf(app.Out(), "Room %s — %d playlist(s)\n\n", room, len(state.Lists))
			tw := tabwriter.NewWriter(app.Out(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "  #\tTITLE\tKEY\tSHUFFLE\tACTIVE")
			for i, p := range state.Lists {
				active := ""
				if p.Key == state.SelectedPlaylist {
					active = "*"
				}
				fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n",
					i+1, p.Title, p.Key, shuffleLabel(p.Shuffle), active)
			}
			tw.Flush()
			if state.SelectedPlaylist != "" {
				fmt.Fprintf(app.Out(), "\n(* = currently active playlist)\n")
			}
			return nil
		},
	}
}

func shuffleLabel(s int) string {
	switch s {
	case 0:
		return "off"
	case 1:
		return "on"
	case 2:
		return "repeat"
	default:
		return strconv.Itoa(s)
	}
}
