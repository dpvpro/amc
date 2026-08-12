package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// MirrorStatus is the top-level mirror status document from the Arch Linux
// Mirror Status API.
type MirrorStatus struct {
	Version   int      `json:"version"`
	LastCheck string   `json:"last_check"`
	URLs      []Mirror `json:"urls"`
}

// Mirror is a single mirror entry.
type Mirror struct {
	URL           string  `json:"url"`
	Protocol      string  `json:"protocol"`
	LastSync      *string `json:"last_sync"` // may be null
	CompletionPct float64 `json:"completion_pct"`
	Delay         float64 `json:"delay"`
	DurationAvg   float64 `json:"duration_avg"`
	Score         float64 `json:"score"`
	Country       string  `json:"country"`
	CountryCode   string  `json:"country_code"`
	Isos          bool    `json:"isos"`
	IPv4          bool    `json:"ipv4"`
	IPv6          bool    `json:"ipv6"`
	Details       string  `json:"details"`

	lastSyncTime time.Time
	rate         float64
}

// parseLastSync converts the "last_sync" timestamp (with or without
// microseconds) to a time.Time.
func parseLastSync(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse last_sync %q", s)
}

// cachePathFor returns the cache file for the given status URL. The default
// URL reuses the well-known name so other tools can find it.
func cachePathFor(url string) string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = dir
		} else {
			base = "."
		}
	}
	if url == defaultURL {
		return filepath.Join(base, "mirrorstatus.json")
	}
	name := base64.RawURLEncoding.EncodeToString([]byte(url)) + ".json"
	return filepath.Join(base, "reflector-go", name)
}

// fetchStatus retrieves the mirror status, reusing a local cache within the
// cache timeout. It returns the status and the time it was last retrieved.
func fetchStatus(opts *Options) (*MirrorStatus, time.Time, error) {
	path := cachePathFor(opts.URL)

	if st, err := os.Stat(path); err == nil {
		if time.Since(st.ModTime()) < time.Duration(opts.CacheTimeout)*time.Second {
			status, err := readStatus(path)
			return status, st.ModTime(), err
		}
	}

	data, err := download(opts.URL, opts.ConnectionTimeout)
	if err != nil {
		return nil, time.Time{}, err
	}
	var status MirrorStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, time.Time{}, fmt.Errorf("invalid mirror status JSON: %w", err)
	}
	if err := writeCache(path, data); err != nil {
		return nil, time.Time{}, err
	}
	return &status, time.Now(), nil
}

func readStatus(path string) (*MirrorStatus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var status MirrorStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("invalid cached mirror status: %w", err)
	}
	return &status, nil
}

func download(url string, timeoutSec int) ([]byte, error) {
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve mirror status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve mirror status: HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// writeCache stores data in a temp file and renames it into place to avoid
// leaving a partial file behind.
func writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mirrorstatus-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
