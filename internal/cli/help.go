package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func helpCmd() *cli.Command {
	return &cli.Command{
		Name:            "help",
		Aliases:         []string{"h"},
		Usage:           "Show commands, then a step-by-step getting-started guide",
		ArgsUsage:       "[command]",
		SkipFlagParsing: true,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if name := cmd.Args().First(); name != "" {
				return cli.ShowCommandHelp(ctx, cmd.Root(), name)
			}
			if err := cli.ShowRootCommandHelp(cmd); err != nil {
				return err
			}
			fmt.Fprint(appFrom(ctx).Out(), "\n"+guideText)
			return nil
		},
	}
}

const guideText = `W2G CLI — getting started
=========================

1) Log in
     w2g login                                  # prompts for email + password
     w2g login --email you@example.com --password "..."

2) Pick a room
     w2g rooms                                  # list your rooms (name, id, members)
     w2g join room1                             # select by name or id. Saved as default
   Override per command with --room <id>.

3) Use it
     w2g playlists                              # list the room's playlists
     w2g playlist "Playlist1"                   # show a playlist (by name or key)
     w2g playlist "Playlist1" --json            # machine-readable
     w2g export --all -o lists.txt              # dump every playlist as text
     w2g download "Playlist1"                   # download audio (needs yt-dlp + ffmpeg)

Downloads
---------
Downloading pulls audio from the YouTube links via yt-dlp (which uses ffmpeg).
Requirements: yt-dlp and ffmpeg. Install them or set paths:
    w2g config set ytdlp_path C:\tools\yt-dlp.exe
    w2g config set ffmpeg_path C:\tools\ffmpeg\bin
or pass the global --ytdlp / --ffmpeg (before the command).

Already-downloaded tracks are skipped, so re-running download only fetches
what's new. --limit N caps new songs per run. More options (bitrate, proxy,
cookie_file, ...) live in the config — see w2g config --help.
`
