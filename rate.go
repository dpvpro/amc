package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dbSubpath is the repository database downloaded to measure download speed.
const dbSubpath = "extra/os/x86_64/extra.db"

// rateMirrors measures the download rate of every mirror and stores it in
// m.rate. The number of parallel downloads is controlled by --threads.
func rateMirrors(mirrors []Mirror, opts *Options) {
	if len(mirrors) == 0 {
		return
	}
	workers := opts.Threads
	if workers <= 0 {
		workers = 1
	}
	infof(opts, "rating %d mirror(s) by download speed", len(mirrors))

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Go(func() {
			for i := range jobs {
				m := &mirrors[i]
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.DownloadTimeout)*time.Second)
				var rate, elapsed float64
				var err error
				if isRsync(m.URL) {
					elapsed, rate, err = rateRSync(ctx, m.URL, opts)
				} else {
					elapsed, rate, err = rateHTTP(ctx, m.URL, opts)
				}
				cancel()
				if err != nil {
					infof(opts, "failed to rate %s: %v", m.URL, err)
					m.rate = 0
					continue
				}
				m.rate = rate
				infof(opts, "%-40s %10.2f KiB/s %8.2f s", m.URL, rate/1024, elapsed)
			}
		})
	}
	for i := range mirrors {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func isRsync(mirrorURL string) bool {
	u, err := url.Parse(mirrorURL)
	if err != nil {
		return strings.HasPrefix(mirrorURL, "rsync://")
	}
	return u.Scheme == "rsync"
}

// rateHTTP downloads the database over HTTP(S) and returns elapsed time and
// bytes per second.
func rateHTTP(ctx context.Context, mirrorURL string, opts *Options) (float64, float64, error) {
	client := &http.Client{Timeout: time.Duration(opts.ConnectionTimeout) * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mirrorURL+dbSubpath, nil)
	if err != nil {
		return 0, 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	if err != nil {
		return 0, 0, err
	}
	if elapsed <= 0 {
		return 0, 0, fmt.Errorf("download finished instantly")
	}
	return elapsed, float64(n) / elapsed, nil
}

// rateRSync downloads the database via the system rsync binary and returns
// elapsed time and bytes per second.
func rateRSync(ctx context.Context, mirrorURL string, opts *Options) (float64, float64, error) {
	tmp, err := os.MkdirTemp("", "reflector-go-rsync-")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(tmp)

	cmd := exec.CommandContext(ctx, "rsync",
		"-avL", "--no-h", "--no-motd",
		fmt.Sprintf("--contimeout=%d", opts.ConnectionTimeout),
		mirrorURL+dbSubpath,
		tmp+string(filepath.Separator),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	elapsed := time.Since(start).Seconds()
	size := fileSize(filepath.Join(tmp, filepath.Base(dbSubpath)))
	if elapsed <= 0 {
		return 0, 0, fmt.Errorf("rsync finished instantly")
	}
	return elapsed, float64(size) / elapsed, nil
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}
