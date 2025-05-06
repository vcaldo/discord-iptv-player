package m3u

import (
	"context"
	"log"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

func InitializePlaylist(ctx context.Context, config *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("m3u:initialize-playlist")
	defer txn.End()

	// Default guild ID - can be changed if you need to store playlists per guild
	// const defaultGuildID = "default"
	// const defaultPlaylistName = "default"

	// // If no playlist URL is configured, nothing to do
	// if config.PlaylistURL == "" {
	// 	log.Println("no playlist url configured, skipping playlist initialization")
	// 	return nil
	// }

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

	// Download and parse the playlist
	downloadSegment := txn.StartSegment("download-playlist")
	playlist, err := GetPlaylist(ctx, config.PlaylistURL, config.PlaylistName, nrApp)
	downloadSegment.End()

	if err != nil {
		txn.NoticeError(err)
		return err
	}

	txn.AddAttribute("playlist.channels_count", len(playlist.Channels))

	// If we have an existing playlist and the number of channels is the same,
	// the playlist might not have changed, so we can keep the old one to avoid
	// unnecessary updates and data transfer
	if existingPlaylist != nil && len(existingPlaylist.Channels) == len(playlist.Channels) {
		log.Println("new playlist has the same number of channels as the existing one, keeping the existing playlist")
		// Update the timestamp to reset the age check
		existingPlaylist.Updated = time.Now()

		storeSegment := txn.StartSegment("store-playlist")
		err := redisClient.StorePlaylist(existingPlaylist, config.DiscordGuildID)
		storeSegment.End()

		if err != nil {
			txn.NoticeError(err)
		}
		return err
	}

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
