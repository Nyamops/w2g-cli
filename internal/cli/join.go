package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func joinCmd() *cli.Command {
	return &cli.Command{
		Name:      "join",
		Usage:     "Join/select a room by name, id, or access key (stores it as default)",
		ArgsUsage: "[name|id]",
		Description: `Select a room and save it as your default so later commands don't need --room.

Two ways:

  1. By name or id, for a room you are already a member of (see ` + "`w2g rooms`" + `):
        w2g join MyRoom
        w2g join 2qgg5hkuwfxzwbckth

  2. By access key, which joins via the W2G join_room endpoint and works even
     for a room you have not joined yet. Nickname and access key default to your
     config. Pass flags to override (and --save to persist them):
        w2g join --nickname "Nickname" --access-key "alnrtlaxht01hj4dozz2z1" --save`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "nickname", Usage: "your display name (default: config nickname)"},
			&cli.StringFlag{Name: "access-key", Usage: "the room access key (default: config access_key)"},
			&cli.StringFlag{Name: "slug", Value: "default", Usage: "room slug to join through"},
			&cli.BoolFlag{Name: "save", Usage: "persist nickname/access-key to config"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			app := appFrom(ctx)
			if err := app.requireLogin(); err != nil {
				return err
			}

			if q := cmd.Args().First(); q != "" {
				rooms, err := app.Client().Rooms(ctx)
				if err != nil {
					return err
				}
				r, err := resolveRoom(rooms, q)
				if err != nil {
					return err
				}
				app.Cfg.DefaultRoom = r.StreamKey
				if err := app.Cfg.Save(); err != nil {
					return err
				}
				fmt.Fprintf(app.Out(), "Selected room %q (%s). Set as default. Try `w2g playlists`.\n",
					r.PersistentName, r.StreamKey)
				return nil
			}

			nickname := cmd.String("nickname")
			accessKey := cmd.String("access-key")
			if nickname == "" {
				nickname = app.Cfg.Nickname
			}
			if accessKey == "" {
				accessKey = app.Cfg.AccessKey
			}
			nickname = strings.TrimSpace(nickname)
			accessKey = strings.TrimSpace(accessKey)
			if nickname == "" || accessKey == "" {
				return fmt.Errorf("both --nickname and --access-key are required (or set them in config)")
			}

			resp, err := app.Client().JoinRoom(ctx, cmd.String("slug"), nickname, accessKey)
			if err != nil {
				return err
			}
			if resp.StreamKey == "" {
				return fmt.Errorf("join succeeded but no room id was returned")
			}

			app.Cfg.DefaultRoom = resp.StreamKey
			if cmd.Bool("save") {
				app.Cfg.Nickname = nickname
				app.Cfg.AccessKey = accessKey
			}
			if err := app.Cfg.Save(); err != nil {
				return err
			}

			fmt.Fprintf(app.Out(), "Joined room %s\n", resp.StreamKey)
			fmt.Fprintf(app.Out(), "  owner: %v   plus: %v   pro: %v\n", resp.Owner, resp.Plus, resp.Pro)
			if resp.PersistentName != "" {
				fmt.Fprintf(app.Out(), "  name:  %s\n", resp.PersistentName)
			}
			fmt.Fprintf(app.Out(), "Set as default room. Try `w2g playlists`.\n")
			return nil
		},
	}
}
