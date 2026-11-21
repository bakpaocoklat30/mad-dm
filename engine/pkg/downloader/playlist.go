package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type PlaylistEntry struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Duration int64  `json:"duration"`
	Uploader string `json:"uploader"`
}

type PlaylistInfo struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Count   int             `json:"count"`
	Entries []PlaylistEntry `json:"entries"`
}

func IsPlaylistURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	q := u.Query()
	return q.Get("list") != "" || strings.Contains(u.Path, "/playlist")
}

func InspectPlaylist(rawURL string) (*PlaylistInfo, error) {
	ytPath := getYtDlpPath()
	if ytPath == "" {
		return nil, fmt.Errorf("streaming engine 'yt-dlp' not found in bin/ or PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	args := []string{
		"--flat-playlist",
		"--yes-playlist",
		"--dump-single-json",
		"--no-warnings",
	}

	if nodePath := getNodePath(); nodePath != "" {
		args = append(args, "--js-runtimes", "node:"+nodePath)
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, ytPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect playlist: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	var raw struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Type    string `json:"_type"`
		Entries []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Duration int64  `json:"duration"`
			Uploader string `json:"uploader"`
		} `json:"entries"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse playlist json: %w", err)
	}

	info := &PlaylistInfo{
		ID:    raw.ID,
		Title: raw.Title,
	}
	if info.Title == "" {
		info.Title = "YouTube Playlist"
	}

	idx := 0
	for _, e := range raw.Entries {
		if e.ID == "" && e.URL == "" {
			continue
		}
		itemURL := e.URL
		if !strings.HasPrefix(itemURL, "http") {
			itemURL = "https://www.youtube.com/watch?v=" + e.ID
		}
		itemTitle := e.Title
		if itemTitle == "" {
			itemTitle = "Track " + e.ID
		}
		info.Entries = append(info.Entries, PlaylistEntry{
			Index:    idx,
			ID:       e.ID,
			Title:    itemTitle,
			URL:      itemURL,
			Duration: e.Duration,
			Uploader: e.Uploader,
		})
		idx++
	}
	info.Count = len(info.Entries)

	return info, nil
}
