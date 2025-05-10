package models

import (
	"time"
)

type RemoteControlCommand struct {
	Command       string `json:"command"`
	Title         string `json:"title"`
	Url           string `json:"url"`
	XcodeUsername string `json:"xcode_username"`
	XcodePassword string `json:"xcode_password"`
}

type Playlist struct {
	Name     string      `json:"name"`
	Channels []TvChannel `json:"channels"`
	Source   string      `json:"source"`
	Updated  time.Time   `json:"updated"`
}

type TvChannel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Url           string `json:"url"`
	Logo          string `json:"logo"`
	Category      string `json:"category"`
	Favorite      bool   `json:"favorite"`
	Enabled       bool   `json:"enabled"`
	XcodeUsername string `json:"xcode_username"`
	XcodePassword string `json:"xcode_password"`
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
