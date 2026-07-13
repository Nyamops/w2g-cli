package cli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/urfave/cli/v3"
)

func versionCmd() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print version information",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Fprintf(appFrom(ctx).Out(), "w2g %s (%s/%s, %s)\n",
				version, runtime.GOOS, runtime.GOARCH, runtime.Version())
			return nil
		},
	}
}
