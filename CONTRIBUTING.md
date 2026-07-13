# Contributing to w2g-cli

This is a small project, so the process is simple.
Bug reports, small fixes, and focused features are welcome.

## Getting set up

You need Go 1.26 or newer.

```sh
git clone https://github.com/Nyamops/w2g-cli
cd w2g-cli
go build -o w2g .
go test ./...
```

You also need `yt-dlp` and `ffmpeg` executables for `download` command.

## Before you open a pull request

Run these and make sure they pass:

```sh
go build ./...
go vet ./...
go test ./...
```

## Project layout

```
main.go                 entry point
internal/cli/           commands, flags, help text, dispatch (urfave/cli)
internal/client/        W2G API client and response models
internal/config/        config load/save (credentials + preferences)
internal/cache/         on-disk ETag HTTP cache
internal/downloader/    yt-dlp wrapper and resume manifest
internal/logging/       slog setup
```

## Code style

- Keep it simple. Prefer the standard library and a small diff over a new
  abstraction or dependency. Don't add a dependency for something a few lines
  can do.
- Follow the [Uber Go Style Guide](https://github.com/uber-go/guide) where it
  applies.

## Reporting bugs

Open an issue with:

- what you ran (the exact command),
- what happened versus what you expected,
- `w2g version` output, and `--log-level debug` output if it's relevant.

Since W2G's API is private and undocumented, some breakage comes from their side
changing. If a command suddenly returns "not authorized" or a decode error,
mention which endpoint (the command) so it's easy to trace.

## A note on scope

This is an unofficial client for personal use. Please don't send changes that abuses the W2G
service.
