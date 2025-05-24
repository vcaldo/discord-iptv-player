package m3u

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

// InitializePlaylists loads and initializes multiple playlists from YAML configuration
func InitializePlaylists(ctx context.Context, cfg *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("m3u:initialize-playlists")
	defer txn.End()
	// Load playlist configurations
	playlistsConfig, err := LoadPlaylistsConfig(cfg.PlaylistsConfigPath)
	if err != nil {
		txn.NoticeError(err)
		return fmt.Errorf("failed to load playlists config: %w", err)
	}

	if len(playlistsConfig.Playlists) == 0 {
		log.Println("no playlists configured, skipping playlist initialization")
		return nil
	}

	txn.AddAttribute("playlists_count", len(playlistsConfig.Playlists))

	// Initialize each playlist
	for _, playlistConfig := range playlistsConfig.Playlists {
		if !playlistConfig.Enabled {
			log.Printf("skipping disabled playlist: %s", playlistConfig.Name)
			continue
		}

		log.Printf("initializing playlist: %s", playlistConfig.Name)

		if err := initializeSinglePlaylist(ctx, cfg, redisClient, nrApp, playlistConfig); err != nil {
			log.Printf("error initializing playlist '%s': %v", playlistConfig.Name, err)
			txn.NoticeError(err)
			// Continue with other playlists instead of failing completely
			continue
		}
	}

	// Set default playlist if none is currently set
	currentPlaylist, err := redisClient.GetCurrentPlaylist(cfg.DiscordGuildID)
	if err != nil || currentPlaylist == "" {
		if defaultPlaylist := findDefaultPlaylist(playlistsConfig); defaultPlaylist != "" {
			log.Printf("setting default playlist to: %s", defaultPlaylist)
			if err := redisClient.SetCurrentPlaylist(cfg.DiscordGuildID, defaultPlaylist); err != nil {
				log.Printf("warning: failed to set default playlist: %v", err)
				txn.NoticeError(err)
			}
		}
	}

	log.Printf("playlist initialization completed")
	return nil
}

// initializeSinglePlaylist initializes a single playlist from its configuration
func initializeSinglePlaylist(ctx context.Context, cfg *config.Config, redisClient *redis.Client, nrApp *newrelic.Application, playlistConfig models.PlaylistConfig) error {
	txn := nrApp.StartTransaction("m3u:initialize-single-playlist")
	defer txn.End()

	txn.AddAttribute("playlist_name", playlistConfig.Name)

	segment := txn.StartSegment("check-playlist-in-redis")
	existingPlaylist, err := redisClient.GetPlaylist(cfg.DiscordGuildID, playlistConfig.Name)
	segment.End()

	refreshNeeded := true
	if err == nil {
		txn.AddAttribute("playlist.age_hours", time.Since(existingPlaylist.Updated).Hours())

		maxAgeDuration := time.Duration(playlistConfig.MaxAgeDays) * 24 * time.Hour
		if time.Since(existingPlaylist.Updated) < maxAgeDuration {
			log.Printf("using existing playlist '%s' from redis, updated %s ago", playlistConfig.Name, time.Since(existingPlaylist.Updated).Round(time.Second))
			refreshNeeded = false
		} else {
			log.Printf("playlist '%s' is older than max age (%d days), downloading new one", playlistConfig.Name, playlistConfig.MaxAgeDays)
		}
	} else {
		log.Printf("no playlist '%s' found in redis: %v. downloading new one", playlistConfig.Name, err)
		txn.NoticeError(err)
	}

	if !refreshNeeded {
		return nil
	}
	downloadSegment := txn.StartSegment("download-playlist")
	var playlist *models.Playlist

	if playlistConfig.URL != "" {
		playlist, err = GetPlaylist(ctx, playlistConfig.URL, playlistConfig.Name, nrApp)
	} else {
		err = fmt.Errorf("no URL configured for playlist '%s'", playlistConfig.Name)
	}
	downloadSegment.End()

	if err != nil {
		txn.NoticeError(err)
		return err
	}

	txn.AddAttribute("playlist.channels_count", len(playlist.Channels))

	if existingPlaylist != nil {
		deleteSegment := txn.StartSegment("delete-old-playlist")
		if err := redisClient.DeletePlaylist(cfg.DiscordGuildID, playlistConfig.Name); err != nil {
			log.Printf("warning: failed to delete old playlist '%s': %v", playlistConfig.Name, err)
			txn.NoticeError(err)
		}
		deleteSegment.End()
	}

	storeSegment := txn.StartSegment("store-new-playlist")
	if err := redisClient.StorePlaylist(playlist, cfg.DiscordGuildID); err != nil {
		txn.NoticeError(err)
		storeSegment.End()
		return err
	}
	storeSegment.End()

	log.Printf("playlist '%s' initialized with %d channels", playlistConfig.Name, len(playlist.Channels))
	return nil
}

// findDefaultPlaylist finds the default playlist from the configuration
func findDefaultPlaylist(playlistsConfig *models.PlaylistsConfig) string {
	if playlistsConfig.Settings.DefaultPlaylist != "" {
		for _, playlist := range playlistsConfig.Playlists {
			if playlist.Name == playlistsConfig.Settings.DefaultPlaylist && playlist.Enabled {
				return playlist.Name
			}
		}
	}

	// If no valid default configured, use the first enabled playlist
	for _, playlist := range playlistsConfig.Playlists {
		if playlist.Enabled {
			return playlist.Name
		}
	}

	return "default"
}

// Legacy function for backward compatibility - now deprecated
func InitializePlaylist(ctx context.Context, cfg *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) error {
	log.Println("Warning: InitializePlaylist is deprecated, use InitializePlaylists instead")
	return InitializePlaylists(ctx, cfg, redisClient, nrApp)
}
