package m3u

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

func InitializePlaylist(ctx context.Context, config *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("m3u:initialize-playlist")
	defer txn.End()

	segment := txn.StartSegment("check-playlist-in-redis")

	// Check if playlist exists in Redis
	existingPlaylist, err := redisClient.GetPlaylist(config.DiscordGuildID, config.PlaylistName)
	segment.End()

	refreshNeeded := true
	if err == nil {
		// Add playlist age as a custom attribute for monitoring
		txn.AddAttribute("playlist.age_hours", time.Since(existingPlaylist.Updated).Hours())

		// Playlist exists, check if it's older than the max age
		maxAgeDuration := time.Duration(config.PlaylistMaxAgeDays) * 24 * time.Hour
		if time.Since(existingPlaylist.Updated) < maxAgeDuration {
			// Playlist is still fresh, no need to download
			log.Printf("using existing playlist from redis, updated %s ago", time.Since(existingPlaylist.Updated).Round(time.Second))
			refreshNeeded = false
		} else {
			log.Printf("playlist is older than max age (%d days), downloading new one", config.PlaylistMaxAgeDays)
		}
	} else {
		log.Printf("no playlist found in redis: %v. downloading new one", err)
		txn.NoticeError(err)
	}

	if !refreshNeeded {
		return nil
	}

	// Parse multiple playlist URLs if present
	playlistURLs := strings.Split(config.PlaylistURL, "|")
	// Trim spaces from each name
	for i, url := range playlistURLs {
		playlistURLs[i] = strings.TrimSpace(url)

	}

	// Log if multiple playlists are specified

	log.Printf("found %d playlists to process", len(playlistURLs))

	// Download and parse each playlist}}
	downloadSegment := txn.StartSegment("download-playlists")
	var allPlaylists []*models.Playlist
	var totalChannels int

	for _, url := range playlistURLs {
		playlist, err := DownloadPlaylist(ctx, url, config.PlaylistName, nrApp)
		if err != nil {
			log.Printf("error downloading playlist from %s: %v", url, err)
			txn.NoticeError(err)
			continue // Skip this playlist but try others
		}

		log.Printf("downloaded playlist from %s with %d channels", url, len(playlist.Channels))
		allPlaylists = append(allPlaylists, playlist)
		totalChannels += len(playlist.Channels)
	}
	downloadSegment.End()

	if len(allPlaylists) == 0 {
		err := fmt.Errorf("failed to download any playlists")
		txn.NoticeError(err)
		return err
	}

	playlist := &models.Playlist{
		Name:     config.PlaylistName,
		Channels: make([]models.TvChannel, 0, totalChannels),
		Source:   config.PlaylistURL,
		Updated:  time.Now(),
	}

	for _, p := range allPlaylists {
		playlist.Channels = append(playlist.Channels, p.Channels...)
	}

	txn.AddAttribute("playlist.channels_count", len(playlist.Channels))

	// Delete the old playlist if it exists
	if existingPlaylist != nil {
		deleteSegment := txn.StartSegment("delete-old-playlist")
		if err := redisClient.DeletePlaylist(config.DiscordGuildID, config.PlaylistName); err != nil {
			log.Printf("warning: failed to delete old playlist: %v", err)
			txn.NoticeError(err)
			// Continue anyway, we'll just overwrite it
		}
		deleteSegment.End()
	}

	// Store the new playlist in Redis
	storeSegment := txn.StartSegment("store-new-playlist")
	if err := redisClient.StorePlaylist(playlist, config.DiscordGuildID); err != nil {
		txn.NoticeError(err)
		storeSegment.End()
		return err
	}
	storeSegment.End()

	log.Printf("playlist initialized with %d channels", len(playlist.Channels))
	return nil
}
