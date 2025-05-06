package redis

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

type Client struct {
	rdb    *redis.Client
	config *config.Config
	nrApp  *newrelic.Application
}

func NewClient(ctx context.Context, cfg *config.Config, nrApp *newrelic.Application) (*Client, error) {
	txn := nrApp.StartTransaction("redis:initialize-client")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("redis_address", cfg.RedisAddress)
	txn.AddAttribute("redis_db", cfg.RedisDB)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	segment := txn.StartSegment("redis:ping")
	pong, err := rdb.Ping().Result()
	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	segment.End()

	log.Printf("successfully connected to Redis: %s", pong)

	return &Client{
		rdb:    rdb,
		config: cfg,
		nrApp:  nrApp,
	}, nil
}

func (c *Client) instrumentOperation(operationName string, fn func() error) error {
	txn := c.nrApp.StartTransaction("redis:" + operationName)
	defer txn.End()

	err := fn()
	if err != nil {
		txn.NoticeError(err)
	}

	return err
}

func (c *Client) StorePlaylist(playlist *models.Playlist, guildID string) error {
	return c.instrumentOperation("store-playlist", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlist.Name)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		pipe := c.rdb.Pipeline()

		pipe.HSet(playlistKey, "name", playlist.Name)
		pipe.HSet(playlistKey, "source", playlist.Source)
		pipe.HSet(playlistKey, "updated", playlist.Updated.Format(time.RFC3339))

		pipe.Del(channelsKey)

		for i, channel := range playlist.Channels {
			channelKey := fmt.Sprintf("%s:%d", channelsKey, i)

			pipe.HSet(channelKey, "id", channel.ID)
			pipe.HSet(channelKey, "name", channel.Name)
			pipe.HSet(channelKey, "url", channel.Url)
			pipe.HSet(channelKey, "logo", channel.Logo)
			pipe.HSet(channelKey, "category", channel.Category)
			pipe.HSet(channelKey, "favorite", channel.Favorite)
			pipe.HSet(channelKey, "enabled", channel.Enabled)

			pipe.SAdd(channelsKey, i)
		}

		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		pipe.SAdd(setKey, playlist.Name)

		_, err := pipe.Exec()
		if err != nil {
			return fmt.Errorf("failed to store playlist in Redis: %w", err)
		}

		log.Printf("Playlist '%s' stored successfully for guild %s with %d channels",
			playlist.Name, guildID, len(playlist.Channels))
		return nil
	})
}

func (c *Client) GetPlaylist(guildID, playlistName string) (*models.Playlist, error) {
	var playlist *models.Playlist

	err := c.instrumentOperation("get-playlist", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Get playlist metadata from hash
		playlistData, err := c.rdb.HGetAll(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve playlist data: %w", err)
		}

		// Initialize playlist
		playlist = &models.Playlist{
			Name:     playlistData["name"],
			Source:   playlistData["source"],
			Channels: []models.TvChannel{},
		}

		// Parse updated time
		if updated, ok := playlistData["updated"]; ok && updated != "" {
			parsedTime, err := time.Parse(time.RFC3339, updated)
			if err != nil {
				log.Printf("Warning: failed to parse updated time: %v", err)
			} else {
				playlist.Updated = parsedTime
			}
		}

		// Get channel indices from set
		channelIndices, err := c.rdb.SMembers(channelsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel indices: %w", err)
		}

		// Retrieve each channel
		for _, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)

			// Get channel data
			channelData, err := c.rdb.HGetAll(channelKey).Result()
			if err != nil {
				log.Printf("Warning: failed to retrieve channel data for index %s: %v", indexStr, err)
				continue
			}

			// Create channel object
			channel := models.TvChannel{
				Name:     channelData["name"],
				Url:      channelData["url"],
				Logo:     channelData["logo"],
				Category: channelData["category"],
			}

			// Parse ID
			if idStr, ok := channelData["id"]; ok && idStr != "" {
				if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
					channel.ID = id
				}
			}

			// Parse boolean fields
			if favoriteStr, ok := channelData["favorite"]; ok {
				channel.Favorite = favoriteStr == "1" || favoriteStr == "true"
			}

			if enabledStr, ok := channelData["enabled"]; ok {
				channel.Enabled = enabledStr == "1" || enabledStr == "true"
			}

			// Add channel to playlist
			playlist.Channels = append(playlist.Channels, channel)
		}

		log.Printf("Retrieved playlist '%s' for guild %s with %d channels",
			playlist.Name, guildID, len(playlist.Channels))
		return nil
	})

	return playlist, err
}

func (c *Client) ListPlaylists(guildID string) ([]string, error) {
	var playlistNames []string

	err := c.instrumentOperation("list-playlists", func() error {
		key := fmt.Sprintf("guild:%s:playlists", guildID)

		// Get all playlist names from the set
		var err error
		playlistNames, err = c.rdb.SMembers(key).Result()
		if err != nil {
			return fmt.Errorf("failed to list playlists from Redis: %w", err)
		}

		return nil
	})

	return playlistNames, err
}

func (c *Client) DeletePlaylist(guildID, playlistName string) error {
	return c.instrumentOperation("delete-playlist", func() error {
		// Create keys
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		// Check if playlist exists
		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Use pipeline for better performance
		pipe := c.rdb.Pipeline()

		// Get all channel indices before deleting
		channelIndices, err := c.rdb.SMembers(channelsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel indices: %w", err)
		}

		// Delete each channel entry
		for _, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)
			pipe.Del(channelKey)
		}

		// Delete the channels set
		pipe.Del(channelsKey)

		// Delete the playlist metadata
		pipe.Del(playlistKey)

		// Remove from the set of playlists
		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		pipe.SRem(setKey, playlistName)

		// Execute all commands in the pipeline
		_, err = pipe.Exec()
		if err != nil {
			return fmt.Errorf("failed to delete playlist from Redis: %w", err)
		}

		log.Printf("Playlist '%s' deleted successfully for guild %s", playlistName, guildID)
		return nil
	})
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
