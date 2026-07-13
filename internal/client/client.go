package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"w2g-cli/internal/cache"
)

const (
	apiBase = "https://w2g-api.w2g.tv"
	origin  = "https://w2g.tv"
	referer = "https://w2g.tv/"
)

type Credentials struct {
	RememberToken string
	SessionID     string
	Lang          string
}

type Options struct {
	Creds     Credentials
	UserAgent string
	Timeout   time.Duration
	Retries   int
	Cache     *cache.Cache
	NoCache   bool
	Logger    *slog.Logger
}

type Client struct {
	http    *http.Client
	opts    Options
	log     *slog.Logger
	retries int
}

type APIError struct {
	Status int
	Method string
	URL    string
	Body   string
}

var _ error = (*APIError)(nil)

func (e *APIError) Error() string {
	msg := fmt.Sprintf("w2g API %s %s -> HTTP %d", e.Method, e.URL, e.Status)
	if b := strings.TrimSpace(e.Body); b != "" {
		if len(b) > 300 {
			b = b[:300] + "…"
		}
		msg += ": " + b
	}
	return msg
}

func New(opts Options) *Client {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "w2g-cli"
	}
	return &Client{
		http:    &http.Client{Timeout: opts.Timeout},
		opts:    opts,
		log:     opts.Logger,
		retries: opts.Retries,
	}
}

func (c *Client) cookieHeader() string {
	var parts []string
	lang := c.opts.Creds.Lang
	if lang == "" {
		lang = "en"
	}
	parts = append(parts, "w2glang="+lang)
	if t := c.opts.Creds.RememberToken; t != "" {
		parts = append(parts, "remember_user_token="+t)
	}
	if s := c.opts.Creds.SessionID; s != "" {
		parts = append(parts, "w2g_session_id="+s)
	}
	parts = append(parts, "w2g_auth=1")
	return strings.Join(parts, "; ")
}

func (c *Client) newRequest(ctx context.Context, method, fullURL string, body []byte) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", c.opts.UserAgent)
	req.Header.Set("Cookie", c.cookieHeader())
	return req, nil
}

func (c *Client) doWithRetry(ctx context.Context, method, fullURL string, body []byte, etag string) (*http.Response, []byte, error) {
	var lastErr error
	attempts := c.retries + 1
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 300 * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			c.log.Debug("retrying request", "method", method, "url", fullURL, "attempt", attempt+1, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := c.newRequest(ctx, method, fullURL, body)
		if err != nil {
			return nil, nil, err
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}

		c.log.Debug("http request", "method", method, "url", fullURL)
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, nil, err
			}
			continue
		}

		if resp.StatusCode == http.StatusNotModified {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return resp, nil, nil
		}

		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &APIError{Status: resp.StatusCode, Method: method, URL: fullURL, Body: string(data)}
			c.log.Warn("retryable http status", "status", resp.StatusCode, "url", fullURL)
			continue
		}
		return resp, data, nil
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, nil, lastErr
}

func (c *Client) get(ctx context.Context, fullURL string) ([]byte, error) {
	var etag string
	useCache := c.opts.Cache != nil && !c.opts.NoCache
	var cached cache.Entry
	if useCache {
		if e, ok := c.opts.Cache.Get(fullURL); ok {
			cached = e
			etag = e.ETag
		}
	}

	resp, data, err := c.doWithRetry(ctx, http.MethodGet, fullURL, nil, etag)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		c.log.Debug("cache hit (304)", "url", fullURL)
		return cached.Body, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.statusError(http.MethodGet, fullURL, resp.StatusCode, data)
	}
	if useCache {
		if tag := resp.Header.Get("ETag"); tag != "" {
			c.opts.Cache.Put(fullURL, tag, data)
		}
	}
	return data, nil
}

func (c *Client) post(ctx context.Context, fullURL string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	resp, data, err := c.doWithRetry(ctx, http.MethodPost, fullURL, body, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.statusError(http.MethodPost, fullURL, resp.StatusCode, data)
	}
	return data, nil
}

func (c *Client) statusError(method, fullURL string, status int, body []byte) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("not authorized (HTTP %d) — your login may have expired. Run `w2g login` again: %w",
			status, &APIError{Status: status, Method: method, URL: fullURL, Body: string(body)})
	}
	return &APIError{Status: status, Method: method, URL: fullURL, Body: string(body)}
}

type SignInResult struct {
	RememberToken string
	SessionID     string
}

func (c *Client) SignIn(ctx context.Context, email, password string) (*SignInResult, error) {
	payload := map[string]any{
		"user": map[string]any{
			"email":       email,
			"password":    password,
			"remember_me": 1,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode sign_in body: %w", err)
	}
	u := apiBase + "/auth/sign_in.json"
	resp, data, err := c.doWithRetry(ctx, http.MethodPost, u, body, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("sign in failed: check your email and password")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Method: http.MethodPost, URL: u, Body: string(data)}
	}

	res := &SignInResult{}
	for _, ck := range resp.Cookies() {
		switch ck.Name {
		case "remember_user_token":
			res.RememberToken = ck.Value
		case "w2g_session_id":
			res.SessionID = ck.Value
		}
	}
	if res.RememberToken == "" {
		return nil, fmt.Errorf("sign in succeeded but no remember_user_token cookie was returned")
	}
	return res, nil
}

func (c *Client) JoinRoom(ctx context.Context, slug, nickname, accessKey string) (*JoinResponse, error) {
	if slug == "" {
		slug = "default"
	}
	u := fmt.Sprintf("%s/rooms/%s/join_room", apiBase, url.PathEscape(slug))
	payload := map[string]string{"nname": nickname, "access_key": accessKey}
	data, err := c.post(ctx, u, payload)
	if err != nil {
		return nil, err
	}
	var out JoinResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode join_room response: %w", err)
	}
	return &out, nil
}

func (c *Client) Rooms(ctx context.Context) ([]Room, error) {
	data, err := c.get(ctx, apiBase+"/streams")
	if err != nil {
		return nil, err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil {
		return nil, fmt.Errorf("decode streams: %w", err)
	}
	return rooms, nil
}

func (c *Client) Playlists(ctx context.Context, streamKey string) (*PlaylistsState, error) {
	u := fmt.Sprintf("%s/rooms/%s/sync_state", apiBase, url.PathEscape(streamKey))
	data, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parsePlaylistsFromState(data)
}

func (c *Client) PlaylistItems(ctx context.Context, streamKey, playlistKey string) ([]PlaylistItem, error) {
	u := fmt.Sprintf("%s/streams/%s/playlists/%s/playlist_items",
		apiBase, url.PathEscape(streamKey), url.PathEscape(playlistKey))
	data, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var items []PlaylistItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode playlist_items: %w", err)
	}
	return items, nil
}

func (c *Client) SetActivePlaylist(ctx context.Context, streamKey, playlistKey string) error {
	u := fmt.Sprintf("%s/rooms/%s/playlists/sync_update", apiBase, url.PathEscape(streamKey))
	payload := map[string]string{"active_list": playlistKey}
	_, err := c.post(ctx, u, payload)
	return err
}
