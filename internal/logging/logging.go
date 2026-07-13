package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	Level   string
	File    string
	Verbose bool
	Quiet   bool
}

func Setup(opts Options) (*slog.Logger, io.Closer, error) {
	level, err := parseLevel(opts)
	if err != nil {
		return nil, nopCloser{}, err
	}

	var out io.Writer = os.Stderr
	var closer io.Closer = nopCloser{}
	if opts.File != "" {
		f, err := os.OpenFile(opts.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nopCloser{}, fmt.Errorf("open log file %s: %w", opts.File, err)
		}
		out = f
		closer = f
	}

	handler := slog.NewTextHandler(out, &slog.HandlerOptions{Level: level})
	return slog.New(handler), closer, nil
}

func parseLevel(opts Options) (slog.Level, error) {
	if opts.Level == "" {
		switch {
		case opts.Quiet:
			return slog.LevelError, nil
		case opts.Verbose:
			return slog.LevelInfo, nil
		default:

			return slog.LevelWarn, nil
		}
	}
	switch strings.ToLower(opts.Level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (want debug|info|warn|error)", opts.Level)
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
