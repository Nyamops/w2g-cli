package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Entry struct {
	URL  string `json:"url"`
	ETag string `json:"etag"`
	Body []byte `json:"body"`
}

type Cache struct {
	dir     string
	mu      sync.Mutex
	entries map[string]Entry
	loaded  bool
}

func DefaultDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "cache"), nil
}

func New(dir string) *Cache {
	return &Cache{dir: dir, entries: map[string]Entry{}}
}

func (c *Cache) Dir() string { return c.dir }

func (c *Cache) file() string { return filepath.Join(c.dir, "http-cache.json") }

func (c *Cache) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	data, err := os.ReadFile(c.file())
	if err != nil {
		return
	}
	var entries map[string]Entry
	if err := json.Unmarshal(data, &entries); err == nil && entries != nil {
		c.entries = entries
	}
}

func key(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) Get(url string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	e, ok := c.entries[key(url)]
	return e, ok
}

func (c *Cache) Put(url, etag string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	c.entries[key(url)] = Entry{URL: url, ETag: etag, Body: append([]byte(nil), body...)}
	_ = c.flushLocked()
}

func (c *Cache) flushLocked() error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.file(), data, 0o600)
}

func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]Entry{}
	c.loaded = true
	err := os.Remove(c.file())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *Cache) Stats() (count int, bytes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	for _, e := range c.entries {
		count++
		bytes += len(e.Body)
	}
	return
}
