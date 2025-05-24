package models

// PlaylistConfig represents a single playlist configuration
type PlaylistConfig struct {
	Name        string `yaml:"name"`
	DisplayName string `yaml:"display_name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	MaxAgeDays  int    `yaml:"max_age_days"`
	Enabled     bool   `yaml:"enabled"`
}

// PlaylistsSettings represents global playlist settings
type PlaylistsSettings struct {
	DefaultPlaylist string `yaml:"default_playlist"`
}

// PlaylistsConfig represents the complete playlist configuration structure
type PlaylistsConfig struct {
	Playlists []PlaylistConfig  `yaml:"playlists"`
	Settings  PlaylistsSettings `yaml:"settings"`
}

// GetDisplayName returns the display name of the playlist, falling back to the name if not set
func (p *PlaylistConfig) GetDisplayName() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Name
}
