package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"w2g-cli/internal/client"
)

func exportCmd() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export playlist(s) to text / json / csv / m3u",
		Description: `Dump playlist contents (titles, URLs, ...) to a file or stdout.

    w2g export "Playlist1"                           # text to stdout
    w2g export "Playlist1" -o playlist1.txt          # text to a file
    w2g export --all --format json -o all.json
    w2g export "Playlist1" --format m3u -o playlist1.m3u8

Formats:
    text human-readable list (default)
    json full structured data
    csv   playlist,#,title,url,id,votes
    m3u   an .m3u8 playlist of the URLs`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "all", Usage: "export every playlist in the room"},
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json|csv|m3u"},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file (default: stdout)"},
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

			type bundle struct {
				Playlist client.Playlist       `json:"playlist"`
				Items    []client.PlaylistItem `json:"items"`
			}
			var data []bundle
			for _, pl := range targets {
				items, err := cl.PlaylistItems(ctx, room, pl.Key)
				if err != nil {
					return fmt.Errorf("playlist %q: %w", pl.Title, err)
				}
				data = append(data, bundle{Playlist: pl, Items: items})
			}

			output := cmd.String("output")
			w, closeFn, err := openOutput(app.Out(), output)
			if err != nil {
				return err
			}
			defer closeFn()

			switch strings.ToLower(cmd.String("format")) {
			case "json":
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				if err := enc.Encode(data); err != nil {
					return err
				}
			case "csv":
				cw := csv.NewWriter(w)
				cw.Write([]string{"playlist", "index", "title", "url", "id", "votes"})
				for _, b := range data {
					for i, it := range b.Items {
						cw.Write([]string{
							b.Playlist.Title, strconv.Itoa(i + 1), it.Title,
							it.NormalizedURL(), strconv.FormatInt(it.ID, 10),
							strconv.Itoa(it.VoteCount),
						})
					}
				}
				cw.Flush()
				if err := cw.Error(); err != nil {
					return err
				}
			case "m3u":
				fmt.Fprintln(w, "#EXTM3U")
				for _, b := range data {
					fmt.Fprintf(w, "# Playlist: %s\n", b.Playlist.Title)
					for _, it := range b.Items {
						fmt.Fprintf(w, "#EXTINF:-1,%s\n%s\n", it.Title, it.NormalizedURL())
					}
				}
			case "text", "":
				for bi, b := range data {
					if bi > 0 {
						fmt.Fprintln(w)
					}
					fmt.Fprintf(w, "== %s (%s) — %d item(s) ==\n", b.Playlist.Title, b.Playlist.Key, len(b.Items))
					for i, it := range b.Items {
						fmt.Fprintf(w, "%3d. %s\n     %s\n", i+1, it.Title, it.NormalizedURL())
					}
				}
			default:
				return fmt.Errorf("unknown format %q (want text|json|csv|m3u)", cmd.String("format"))
			}

			if output != "" {
				fmt.Fprintf(app.Errw(), "Wrote %s\n", output)
			}
			return nil
		},
	}
}

func openOutput(defaultW io.Writer, path string) (io.Writer, func(), error) {
	if path == "" {
		return defaultW, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", path, err)
	}
	return f, func() { f.Close() }, nil
}
