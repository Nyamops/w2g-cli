package downloader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	ID    int64
	Title string
	URL   string
}

type Options struct {
	YtDlpPath   string
	FFmpegPath  string
	OutputDir   string
	AudioFormat string
	Concurrency int
	Retries     int
	Limit       int
	Logger      *slog.Logger

	OutputTemplate    string
	Bitrate           string
	Overwrite         string
	CookieFile        string
	Proxy             string
	FFmpegArgs        string
	YtDlpArgs         string
	SponsorBlock      bool
	RestrictFilenames bool
}

type Result struct {
	Item    Item
	Skipped bool
	File    string
	Err     error
}

type Summary struct {
	Total      int
	Downloaded int
	Skipped    int
	Failed     int
	Results    []Result
}

type Downloader struct {
	opts      Options
	log       *slog.Logger
	ytdlp     string
	ffmpegLoc string
	hasFFmpeg bool
	manifest  *Manifest
	mu        sync.Mutex
}

func New(opts Options) (*Downloader, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if opts.AudioFormat == "" {
		opts.AudioFormat = "mp3"
	}
	if opts.OutputTemplate == "" {
		opts.OutputTemplate = "%(title)s [%(id)s].%(ext)s"
	}
	if opts.Bitrate == "" {
		opts.Bitrate = "0"
	}
	if opts.Overwrite == "" {
		opts.Overwrite = "skip"
	}

	ytdlp, err := resolveBinary(opts.YtDlpPath, "yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("yt-dlp not found: %w\n"+
			"Install it (https://github.com/yt-dlp/yt-dlp) or set its path with "+
			"`w2g config set ytdlp_path <path>` / --ytdlp", err)
	}

	if opts.FFmpegPath != "" {
		if _, err := os.Stat(opts.FFmpegPath); err != nil {
			return nil, fmt.Errorf("ffmpeg path %q is not accessible: %w", opts.FFmpegPath, err)
		}
	}
	ffmpegLoc, hasFFmpeg := resolveFFmpeg(opts.FFmpegPath, ytdlp)
	if !hasFFmpeg {
		opts.Logger.Warn("ffmpeg not found — downloading best audio without conversion " +
			"(files keep their original format, e.g. .webm/.m4a). " +
			"Install ffmpeg or set its path (`w2g config set ffmpeg_path <path>`) to convert to " +
			opts.AudioFormat)
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	man, err := LoadManifest(opts.OutputDir)
	if err != nil {
		return nil, err
	}

	return &Downloader{
		opts:      opts,
		log:       opts.Logger,
		ytdlp:     ytdlp,
		ffmpegLoc: ffmpegLoc,
		hasFFmpeg: hasFFmpeg,
		manifest:  man,
	}, nil
}

func resolveFFmpeg(explicit, ytdlpPath string) (location string, found bool) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, true
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, true
	}
	if ytdlpPath != "" {
		dir := filepath.Dir(ytdlpPath)
		for _, name := range []string{"ffmpeg.exe", "ffmpeg"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return dir, true
			}
		}
	}
	return "", false
}

func resolveBinary(explicit, name string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("configured path %q: %w", explicit, err)
		}
		return explicit, nil
	}
	return exec.LookPath(name)
}

func (d *Downloader) archivePath() string {
	return filepath.Join(d.opts.OutputDir, ".yt-dlp-archive.txt")
}

func (d *Downloader) Run(ctx context.Context, items []Item) (*Summary, error) {
	sum := &Summary{Total: len(items), Results: make([]Result, len(items))}

	type job struct {
		idx  int
		item Item
	}
	var jobs []job
	newCount := 0
	for i, it := range items {
		vid := VideoID(it.URL)
		if d.opts.Overwrite != "force" && vid != "" && d.manifest.IsDone(vid) {
			sum.Results[i] = Result{Item: it, Skipped: true, File: d.manifest.File(vid)}
			sum.Skipped++
			continue
		}
		if d.opts.Limit > 0 && newCount >= d.opts.Limit {
			sum.Results[i] = Result{Item: it, Skipped: true, Err: ErrLimitReached}
			sum.Skipped++
			continue
		}
		newCount++
		jobs = append(jobs, job{idx: i, item: it})
	}

	d.log.Info("download plan",
		"total", len(items), "to_download", len(jobs),
		"already_done", sum.Skipped, "concurrency", d.opts.Concurrency)

	jobCh := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < d.opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				res := d.download(ctx, j.item)
				d.mu.Lock()
				sum.Results[j.idx] = res
				if res.Err != nil {
					sum.Failed++
				} else {
					sum.Downloaded++
				}
				d.mu.Unlock()
			}
		}()
	}

	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case jobCh <- j:
		}
	}
	close(jobCh)
	wg.Wait()

	if err := d.manifest.Save(); err != nil {
		d.log.Warn("failed to save manifest", "err", err)
	}
	return sum, ctx.Err()
}

var ErrLimitReached = errors.New("skipped: per-run limit reached")

func (d *Downloader) download(ctx context.Context, it Item) Result {
	u := it.URL
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	vid := VideoID(u)

	args := []string{
		"--no-playlist",
		"--retries", strconv.Itoa(d.opts.Retries),
		"-o", filepath.Join(d.opts.OutputDir, d.opts.OutputTemplate),
		"--print", "after_move:filepath",
		"--no-simulate",
	}
	if d.opts.Overwrite == "force" {
		args = append(args, "--force-overwrites")
	} else {
		args = append(args, "--no-overwrites", "--download-archive", d.archivePath())
	}
	if d.opts.Proxy != "" {
		args = append(args, "--proxy", d.opts.Proxy)
	}
	if d.opts.CookieFile != "" {
		args = append(args, "--cookies", d.opts.CookieFile)
	}
	if d.opts.RestrictFilenames {
		args = append(args, "--restrict-filenames")
	}
	if d.hasFFmpeg {
		args = append(args,
			"-x",
			"--audio-format", d.opts.AudioFormat,
			"--audio-quality", d.opts.Bitrate,
			"--ffmpeg-location", d.ffmpegLoc,
		)
		if d.opts.SponsorBlock {
			args = append(args, "--sponsorblock-remove", "default")
		}
		if d.opts.FFmpegArgs != "" {
			args = append(args, "--postprocessor-args", "ffmpeg:"+d.opts.FFmpegArgs)
		}
	} else {
		args = append(args, "-f", "bestaudio/best")
	}
	if extra := strings.Fields(d.opts.YtDlpArgs); len(extra) > 0 {
		args = append(args, extra...)
	}
	args = append(args, u)

	d.log.Info("downloading", "title", it.Title, "url", u)
	d.log.Debug("yt-dlp invocation", "bin", d.ytdlp, "args", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, d.ytdlp, args...)
	out, err := cmd.CombinedOutput()
	outStr := string(out)
	if err != nil {
		return Result{Item: it, Err: fmt.Errorf("yt-dlp failed: %w\n%s", err, lastLines(outStr, 12))}
	}

	file := lastFilepathLine(outStr)
	if vid != "" {
		d.manifest.MarkDone(vid, it.Title, u, file)
	}
	d.log.Info("downloaded", "title", it.Title, "file", file)
	return Result{Item: it, File: file}
}

func VideoID(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	switch {
	case host == "youtu.be":
		return strings.Trim(u.Path, "/")
	case strings.HasSuffix(host, "youtube.com"):
		if v := u.Query().Get("v"); v != "" {
			return v
		}

		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "v") {
			return parts[1]
		}
	}
	return ""
}

func lastFilepathLine(out string) string {
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	last := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if line != "" && (filepath.IsAbs(line) || strings.Contains(line, string(os.PathSeparator))) {
			last = line
		}
	}
	return last
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
