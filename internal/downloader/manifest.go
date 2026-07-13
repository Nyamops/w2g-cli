package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const ManifestName = ".w2g-manifest.json"

type Record struct {
	VideoID    string    `json:"video_id"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	File       string    `json:"file"`
	Status     string    `json:"status"`
	DownloadAt time.Time `json:"downloaded_at"`
}

type Manifest struct {
	dir     string
	mu      sync.Mutex
	Records map[string]Record `json:"records"`
}

func LoadManifest(dir string) (*Manifest, error) {
	m := &Manifest{dir: dir, Records: map[string]Record{}}
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var loaded Manifest
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if loaded.Records != nil {
		m.Records = loaded.Records
	}
	return m, nil
}

func (m *Manifest) IsDone(videoID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.Records[videoID]
	return ok && r.Status == "done"
}

func (m *Manifest) File(videoID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Records[videoID].File
}

func (m *Manifest) MarkDone(videoID, title, url, file string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Records[videoID] = Record{
		VideoID:    videoID,
		Title:      title,
		URL:        url,
		File:       file,
		Status:     "done",
		DownloadAt: time.Now(),
	}
}

func (m *Manifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.dir, ManifestName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
