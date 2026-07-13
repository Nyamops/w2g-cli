package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

type Config struct {
	RememberToken string `json:"remember_user_token,omitempty"`

	SessionID string `json:"w2g_session_id,omitempty"`

	Lang string `json:"lang,omitempty"`

	DefaultRoom string `json:"default_room,omitempty"`

	Nickname  string `json:"nickname,omitempty"`
	AccessKey string `json:"access_key,omitempty"`

	FFmpegPath string `json:"ffmpeg_path,omitempty"`
	YtDlpPath  string `json:"ytdlp_path,omitempty"`

	DownloadDir string `json:"download_dir,omitempty"`
	AudioFormat string `json:"audio_format,omitempty"`
	Concurrency int    `json:"concurrency,omitempty"`
	Retries     int    `json:"retries,omitempty"`
	TimeoutSecs int    `json:"timeout_secs,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`

	OutputTemplate    string `json:"output_template,omitempty"`
	Bitrate           string `json:"bitrate,omitempty"`
	Overwrite         string `json:"overwrite,omitempty"`
	CookieFile        string `json:"cookie_file,omitempty"`
	Proxy             string `json:"proxy,omitempty"`
	FFmpegArgs        string `json:"ffmpeg_args,omitempty"`
	YtDlpArgs         string `json:"ytdlp_args,omitempty"`
	SponsorBlock      bool   `json:"sponsor_block,omitempty"`
	RestrictFilenames bool   `json:"restrict_filenames,omitempty"`

	path string `json:"-"`
}

func Defaults() *Config {
	return &Config{
		Lang:        "en",
		AudioFormat: "mp3",
		Concurrency: 3,
		Retries:     3,
		TimeoutSecs: 30,
		UserAgent:   DefaultUserAgent,
		Overwrite:   "skip",
	}
}

func DefaultPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "config.json"), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	cfg := Defaults()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()
	cfg.path = path
	return cfg, nil
}

func (c *Config) applyDefaults() {
	d := Defaults()
	if c.Lang == "" {
		c.Lang = d.Lang
	}
	if c.AudioFormat == "" {
		c.AudioFormat = d.AudioFormat
	}
	if c.Concurrency <= 0 {
		c.Concurrency = d.Concurrency
	}
	if c.Retries < 0 {
		c.Retries = d.Retries
	}
	if c.TimeoutSecs <= 0 {
		c.TimeoutSecs = d.TimeoutSecs
	}
	if c.UserAgent == "" {
		c.UserAgent = d.UserAgent
	}
	if c.Overwrite == "" {
		c.Overwrite = d.Overwrite
	}
}

func (c *Config) Path() string { return c.path }

func (c *Config) Save() error {
	if c.path == "" {
		p, err := DefaultPath()
		if err != nil {
			return err
		}
		c.path = p
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", c.path, err)
	}
	return nil
}

func (c *Config) LoggedIn() bool { return c.RememberToken != "" }

func (c *Config) ResolveDownloadDir() string {
	if c.DownloadDir != "" {
		return c.DownloadDir
	}
	return "w2g-downloads"
}

func (c *Config) Set(key, value string) error {
	switch key {
	case "lang":
		c.Lang = value
	case "default_room", "room":
		c.DefaultRoom = value
	case "nickname":
		c.Nickname = value
	case "access_key":
		c.AccessKey = value
	case "ffmpeg_path":
		c.FFmpegPath = value
	case "ytdlp_path":
		c.YtDlpPath = value
	case "download_dir":
		c.DownloadDir = value
	case "audio_format":
		c.AudioFormat = value
	case "user_agent":
		c.UserAgent = value
	case "output_template":
		c.OutputTemplate = value
	case "bitrate":
		c.Bitrate = value
	case "cookie_file":
		c.CookieFile = value
	case "proxy":
		c.Proxy = value
	case "ffmpeg_args":
		c.FFmpegArgs = value
	case "ytdlp_args":
		c.YtDlpArgs = value
	case "overwrite":
		if value != "skip" && value != "force" {
			return fmt.Errorf("config overwrite must be skip or force")
		}
		c.Overwrite = value
	case "sponsor_block":
		return setBool(&c.SponsorBlock, key, value)
	case "restrict_filenames":
		return setBool(&c.RestrictFilenames, key, value)
	case "concurrency":
		return setInt(&c.Concurrency, key, value, 1)
	case "retries":
		return setInt(&c.Retries, key, value, 0)
	case "timeout_secs":
		return setInt(&c.TimeoutSecs, key, value, 1)
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func setBool(dst *bool, key, value string) error {
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("config %s must be true or false: %w", key, err)
	}
	*dst = b
	return nil
}

func setInt(dst *int, key, value string, min int) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("config %s must be an integer: %w", key, err)
	}
	if n < min {
		return fmt.Errorf("config %s must be >= %d", key, min)
	}
	*dst = n
	return nil
}

func (c *Config) Redacted() *Config {
	cp := *c
	cp.RememberToken = Mask(c.RememberToken)
	cp.SessionID = Mask(c.SessionID)
	cp.AccessKey = Mask(c.AccessKey)
	return &cp
}

func Mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "…" + s[len(s)-4:]
}
