package m3u

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type TvChannel struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Url      string `json:"url"`
	Logo     string `json:"logo"`
	Category string `json:"category"`
	Favorite bool   `json:"favorite"`
	Enabled  bool   `json:"enabled"`
}

type Playlist struct {
	Name     string      `json:"name"`
	Channels []TvChannel `json:"channels"`
	Source   string      `json:"source"`
	Updated  time.Time   `json:"updated"`
}

func GetPlaylist(ctx context.Context, url string, name string, nrApp *newrelic.Application) (*Playlist, error) {
	txn := nrApp.StartTransaction("m3u:get-playlist")
	defer txn.End()

	content, err := downloadPlaylist(ctx, url)
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to download playlist: %w", err)
	}

	playlist, err := parsePlaylist(ctx, content, name)
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	playlist.Source = url
	playlist.Updated = time.Now()

	return playlist, nil
}

func GetPlaylistFromFile(ctx context.Context, filePath string, nrApp *newrelic.Application) (*Playlist, error) {
	txn := nrApp.StartTransaction("m3u:get-playlist-from-file")
	defer txn.End()

	content, err := readFile(ctx, filePath)
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to read playlist file: %w", err)
	}

	name := filepath.Base(filePath)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	playlist, err := parsePlaylist(ctx, content, name)
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	playlist.Source = filePath
	playlist.Updated = time.Now()

	return playlist, nil
}

func downloadPlaylist(ctx context.Context, url string) (string, error) {
	txn := newrelic.FromContext(ctx)
	segment := txn.StartSegment("download-playlist")
	defer segment.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		txn.NoticeError(err)
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		txn.NoticeError(err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("failed to download playlist, status code: %d", resp.StatusCode)
		txn.NoticeError(err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		txn.NoticeError(err)
		return "", err
	}

	return string(body), nil
}

func readFile(ctx context.Context, filePath string) (string, error) {
	txn := newrelic.FromContext(ctx)
	segment := txn.StartSegment("read-file")
	defer segment.End()

	content, err := os.ReadFile(filePath)
	if err != nil {
		txn.NoticeError(err)
		return "", err
	}
	return string(content), nil
}

func parsePlaylist(ctx context.Context, content string, name string) (*Playlist, error) {
	txn := newrelic.FromContext(ctx)
	segment := txn.StartSegment("parse-playlist")
	defer segment.End()

	if !strings.HasPrefix(strings.TrimSpace(content), "#EXTM3U") {
		err := errors.New("invalid M3U format, missing #EXTM3U header")
		txn.NoticeError(err)
		return nil, err
	}

	playlist := &Playlist{
		Name:     name,
		Channels: []TvChannel{},
	}

	var currentChannel *TvChannel
	var channelID int64 = 1

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "#EXTM3U" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			// Parse channel info line
			currentChannel = &TvChannel{
				ID:       channelID,
				Favorite: false,
				Enabled:  true,
			}
			channelID++

			// Extract channel info
			infoLine := line[8:] // Remove #EXTINF:
			// Split by the first comma to separate duration and metadata
			parts := strings.SplitN(infoLine, ",", 2)
			if len(parts) < 2 {
				continue // Skip invalid lines
			}

			// Set name
			currentChannel.Name = strings.TrimSpace(parts[1])

			// Extract attributes from the first part
			attrPart := parts[0]
			attributes := extractAttributes(ctx, attrPart)

			// Set logo and category if available
			if logo, ok := attributes["tvg-logo"]; ok {
				currentChannel.Logo = logo
			}
			if category, ok := attributes["group-title"]; ok {
				currentChannel.Category = category
			}
		} else if !strings.HasPrefix(line, "#") && currentChannel != nil {
			// This is a URL line
			currentChannel.Url = line
			playlist.Channels = append(playlist.Channels, *currentChannel)
			currentChannel = nil
		}
	}

	if err := scanner.Err(); err != nil {
		txn.NoticeError(err)
		return nil, err
	}

	return playlist, nil
}

func extractAttributes(ctx context.Context, s string) map[string]string {
	txn := newrelic.FromContext(ctx)
	segment := txn.StartSegment("extract-attributes")
	defer segment.End()

	attributes := make(map[string]string)

	// Find all attribute patterns like key="value" or key='value'
	parts := strings.Split(s, " ")
	for _, part := range parts {
		// Check if it's an attribute
		if strings.Contains(part, "=") {
			keyVal := strings.SplitN(part, "=", 2)
			if len(keyVal) == 2 {
				key := strings.TrimSpace(keyVal[0])
				value := strings.TrimSpace(keyVal[1])

				// Remove quotes if present
				if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
					value[0] == '\'' && value[len(value)-1] == '\'') {
					value = value[1 : len(value)-1]
				}

				attributes[key] = value
			}
		} else if _, err := strconv.ParseFloat(part, 64); err == nil {
			// This is the duration part, we can ignore it
			continue
		}
	}

	return attributes
}
