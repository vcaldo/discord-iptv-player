package m3u

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

func GetPlaylist(url string, name string) (*Playlist, error) {
	content, err := downloadPlaylist(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download playlist: %w", err)
	}

	playlist, err := parsePlaylist(content, name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	playlist.Source = url
	playlist.Updated = time.Now()

	return playlist, nil
}

func GetPlaylistFromFile(filePath string) (*Playlist, error) {
	content, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read playlist file: %w", err)
	}

	name := filepath.Base(filePath)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	playlist, err := parsePlaylist(content, name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	playlist.Source = filePath
	playlist.Updated = time.Now()

	return playlist, nil
}

func downloadPlaylist(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download playlist, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func readFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func parsePlaylist(content string, name string) (*Playlist, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "#EXTM3U") {
		return nil, errors.New("invalid M3U format, missing #EXTM3U header")
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
			attributes := extractAttributes(attrPart)

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
		return nil, err
	}

	return playlist, nil
}

func extractAttributes(s string) map[string]string {
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
