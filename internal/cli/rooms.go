package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
)

func roomsCmd() *cli.Command {
	return &cli.Command{
		Name:        "rooms",
		Usage:       "List your rooms (name, id, members)",
		Description: "Fetch every room your account can see and print its name, id (streamkey) and members. The id can be used as --room or `w2g config set default_room <id>`.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "output json"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			app := appFrom(ctx)
			if err := app.requireLogin(); err != nil {
				return err
			}
			rooms, err := app.Client().Rooms(ctx)
			if err != nil {
				return err
			}

			if cmd.Bool("json") {
				enc := json.NewEncoder(app.Out())
				enc.SetIndent("", "  ")
				return enc.Encode(rooms)
			}

			fmt.Fprintf(app.Out(), "%d room(s)\n\n", len(rooms))
			tw := tabwriter.NewWriter(app.Out(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "  #\tNAME\tROOM ID\tMEMBERS\tOWNER\tDEFAULT")
			for i, r := range rooms {
				def := ""
				if r.StreamKey == app.Cfg.DefaultRoom {
					def = "*"
				}
				fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\n",
					i+1, r.PersistentName, r.StreamKey, strconv.Itoa(len(r.Users)), r.Owner(), def)
			}
			tw.Flush()
			if app.Cfg.DefaultRoom != "" {
				fmt.Fprintf(app.Out(), "\n(* = your configured default room)\n")
			}
			return nil
		},
	}
}
