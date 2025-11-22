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
	"regexp"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

func GetPlaylist(ctx context.Context, url string, name string, nrApp *newrelic.Application) (*models.Playlist, error) {
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

func GetPlaylistFromFile(ctx context.Context, filePath string, nrApp *newrelic.Application) (*models.Playlist, error) {
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

	// Add headers to mimic a web browser to avoid 403 Forbidden errors
	req.Header.Set("User-Agent", "discord-iptv-player/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	client := &http.Client{
		Timeout: 600 * time.Second,
	}
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

func parsePlaylist(ctx context.Context, content string, name string) (*models.Playlist, error) {
	txn := newrelic.FromContext(ctx)
	segment := txn.StartSegment("parse-playlist")
	defer segment.End()

	if !strings.HasPrefix(strings.TrimSpace(content), "#EXTM3U") {
		err := errors.New("invalid M3U format, missing #EXTM3U header")
		txn.NoticeError(err)
		return nil, err
	}

	playlist := &models.Playlist{
		Name:     name,
		Channels: []models.TvChannel{},
	}

	var currentChannel *models.TvChannel
	var channelID int64 = 1

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "#EXTM3U" {
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			// Parse channel info line
			currentChannel = &models.TvChannel{
				ID:       fmt.Sprintf("%d", channelID),
				Favorite: false,
				Enabled:  true,
			}
			channelID++

			// Extract channel info
			infoLine := line[8:] // Remove #EXTINF:

			// Find the last comma that's not inside quotes to separate attributes from channel name
			attrPart, channelName := splitAttributesAndName(infoLine)
			if channelName == "" {
				continue
			}

			// Set name
			currentChannel.Name = strings.TrimSpace(channelName)

			// Extract attributes from the first part
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

	// Regular expression to find attributes, handling quoted values with spaces
	attributePattern := regexp.MustCompile(`([a-zA-Z0-9-]+)=("([^"]*)"|'([^']*)'|[^ ]*)`)
	matches := attributePattern.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		key := match[1]
		var value string

		// Check which capture group has the value (quoted or unquoted)
		if match[3] != "" {
			// Double-quoted value
			value = match[3]
		} else if match[4] != "" {
			// Single-quoted value
			value = match[4]
		} else {
			// Unquoted value
			value = match[2]
			// Remove quotes if present for unquoted pattern
			if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
				value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		attributes[key] = value
	}

	return attributes
}

// splitAttributesAndName splits the EXTINF line into attributes part and channel name
// It finds the last comma that's not inside quotes to properly separate them
func splitAttributesAndName(infoLine string) (string, string) {
	var attrPart, channelName string
	inQuotes := false
	quoteChar := byte(0)
	lastCommaPos := -1

	// Find the last comma that's not inside quotes
	for i := 0; i < len(infoLine); i++ {
		char := infoLine[i]

		if !inQuotes && (char == '"' || char == '\'') {
			inQuotes = true
			quoteChar = char
		} else if inQuotes && char == quoteChar {
			inQuotes = false
			quoteChar = 0
		} else if !inQuotes && char == ',' {
			lastCommaPos = i
		}
	}

	if lastCommaPos == -1 {
		// No comma found outside quotes, invalid format
		return "", ""
	}

	attrPart = infoLine[:lastCommaPos]
	channelName = infoLine[lastCommaPos+1:]

	return attrPart, channelName
}
