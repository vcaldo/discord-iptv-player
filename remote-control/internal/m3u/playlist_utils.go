package m3u

import (
	"fmt"
	"os"

	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
	"gopkg.in/yaml.v3"
)

// LoadPlaylistsConfig loads and validates playlist configuration from a YAML file
func LoadPlaylistsConfig(configPath string) (*models.PlaylistsConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read playlists config file: %w", err)
	}

	var config models.PlaylistsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse playlists config: %w", err)
	}

	if err := validatePlaylistsConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid playlists config: %w", err)
	}

	return &config, nil
}

// validatePlaylistsConfig validates the playlist configuration for consistency and completeness
func validatePlaylistsConfig(config *models.PlaylistsConfig) error {
	if len(config.Playlists) == 0 {
		return fmt.Errorf("no playlists defined")
	}

	// Check for duplicate playlist names
	names := make(map[string]bool)
	enabledPlaylists := make(map[string]bool)

	for _, playlist := range config.Playlists {
		if playlist.Name == "" {
			return fmt.Errorf("playlist name cannot be empty")
		}

		if playlist.URL == "" {
			return fmt.Errorf("playlist '%s' URL cannot be empty", playlist.Name)
		}

		if playlist.MaxAgeDays <= 0 {
			return fmt.Errorf("playlist '%s' max_age_days must be positive", playlist.Name)
		}

		if names[playlist.Name] {
			return fmt.Errorf("duplicate playlist name: %s", playlist.Name)
		}
		names[playlist.Name] = true

		if playlist.Enabled {
			enabledPlaylists[playlist.Name] = true
		}

		if playlist.DisplayName == "" {
			// This will be handled when we access the playlist
		}
	}

	// Check that at least one playlist is enabled
	if len(enabledPlaylists) == 0 {
		return fmt.Errorf("at least one playlist must be enabled")
	}

	// Check that default playlist exists and is enabled
	if config.Settings.DefaultPlaylist == "" {
		return fmt.Errorf("default_playlist must be specified")
	}

	if !names[config.Settings.DefaultPlaylist] {
		return fmt.Errorf("default_playlist '%s' does not exist", config.Settings.DefaultPlaylist)
	}

	if !enabledPlaylists[config.Settings.DefaultPlaylist] {
		return fmt.Errorf("default_playlist '%s' must be enabled", config.Settings.DefaultPlaylist)
	}

	return nil
}
