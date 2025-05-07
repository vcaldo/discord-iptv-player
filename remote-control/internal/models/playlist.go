package models

import (
	"time"
)

type TvChannel struct {
	ID       string `json:"id"`
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
