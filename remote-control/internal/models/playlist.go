package models

import (
	"time"
)

type RemoteControlCommand struct {
	RedisPubSubChannel string     `json:"redis_channel"`
	Command            string     `json:"command"`
	TvChannel          *TvChannel `json:"tv_channel"`
}

type Playlist struct {
	Name     string      `json:"name"`
	Channels []TvChannel `json:"channels"`
	Source   string      `json:"source"`
	Updated  time.Time   `json:"updated"`
}

type TvChannel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Url      string `json:"url"`
	Logo     string `json:"logo"`
	Category string `json:"category"`
	Favorite bool   `json:"favorite"`
	Enabled  bool   `json:"enabled"`
}

type YoutubeVideo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	Duration    string `json:"duration"`
	Url         string `json:"url"`
	Channel     string `json:"channel"`
}

// Legacy ChannelCommand struct
type ChannelCommand struct {
	Command string `json:"command"`
	Tittle  string `json:"title"`
	URL     string `json:"url"`
}
