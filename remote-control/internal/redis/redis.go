package redis

import (
	"context"
	"encoding/json"
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

	// Set up Redis client with timeout options
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddress,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
		MaxRetries:   5,
		MaxConnAge:   0,
		IdleTimeout:  5 * time.Minute,
	})

	var pong string
	var err error
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		segment := txn.StartSegment(fmt.Sprintf("redis:ping-attempt-%d", attempt))
		pong, err = rdb.Ping().Result()
		segment.End()

		if err == nil {
			break
		}

		log.Printf("Redis connection attempt %d/%d failed: %v. Retrying...", attempt, maxAttempts, err)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	if err != nil {
		txn.NoticeError(err)
		return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxAttempts, err)
	}

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
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)

		pipe := c.rdb.Pipeline()

		pipe.HSet(playlistKey, "name", playlist.Name)
		pipe.HSet(playlistKey, "source", playlist.Source)
		pipe.HSet(playlistKey, "updated", playlist.Updated.Format(time.RFC3339))
		pipe.HSet(playlistKey, "length", len(playlist.Channels))

		// Delete existing channels, categories, and category counts before updating
		pipe.Del(channelsKey)
		pipe.Del(categoriesKey)
		pipe.Del(categoryCountsKey)

		// Delete existing category-to-channel mappings
		// We need to get existing categories first to clean up their mappings
		existingCategories, err := c.rdb.SMembers(categoriesKey).Result()
		if err == nil {
			for _, category := range existingCategories {
				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)
				pipe.Del(categoryChannelsKey)
			}
		}

		// Track unique categories and their channel counts
		categoriesMap := make(map[string]struct{})
		categoryCountsMap := make(map[string]int)

		for i, channel := range playlist.Channels {
			channelIndex := i + 1
			channelKey := fmt.Sprintf("%s:%d", channelsKey, channelIndex)

			pipe.HSet(channelKey, "id", channel.ID)
			pipe.HSet(channelKey, "name", channel.Name)
			pipe.HSet(channelKey, "url", channel.Url)
			pipe.HSet(channelKey, "logo", channel.Logo)
			pipe.HSet(channelKey, "category", channel.Category)
			pipe.HSet(channelKey, "favorite", channel.Favorite)
			pipe.HSet(channelKey, "enabled", channel.Enabled)

			pipe.SAdd(channelsKey, channelIndex)

			// Add category to tracking map if it's not empty
			if channel.Category != "" {
				categoriesMap[channel.Category] = struct{}{}
				categoryCountsMap[channel.Category]++

				// Add channel to category-specific set for easy lookup
				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, channel.Category)
				pipe.SAdd(categoryChannelsKey, channelIndex)
			}
		}

		// Store all unique categories in a separate set
		for category := range categoriesMap {
			pipe.SAdd(categoriesKey, category)
		}

		// Store category counts in a hash
		for category, count := range categoryCountsMap {
			pipe.HSet(categoryCountsKey, category, count)
		}

		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		pipe.SAdd(setKey, playlist.Name)

		_, err = pipe.Exec()
		if err != nil {
			return fmt.Errorf("failed to store playlist in Redis: %w", err)
		}

		log.Printf("playlist '%s' stored successfully for guild %s with %d channels and %d categories",
			playlist.Name, guildID, len(playlist.Channels), len(categoriesMap))
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

		playlistData, err := c.rdb.HGetAll(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve playlist data: %w", err)
		}

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

			// Get ID as string
			if idStr, ok := channelData["id"]; ok && idStr != "" {
				channel.ID = idStr
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

		// Log playlist retrieval with length info (either from stored length or calculated)
		var length int64
		if lengthStr, ok := playlistData["length"]; ok && lengthStr != "" {
			if parsedLength, err := strconv.ParseInt(lengthStr, 10, 64); err == nil {
				length = parsedLength
			} else {
				length = int64(len(playlist.Channels))
				log.Printf("warning: failed to parse playlist length from Redis: %v", err)
			}
		} else {
			length = int64(len(playlist.Channels))
		}

		log.Printf("retrieved playlist '%s' for guild %s with %d channels",
			playlist.Name, guildID, length)
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
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)

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

		// Get all categories to clean up their channel mappings
		categories, err := c.rdb.SMembers(categoriesKey).Result()
		if err == nil {
			for _, category := range categories {
				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)
				pipe.Del(categoryChannelsKey)
			}
		}

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

		// Delete the categories set
		pipe.Del(categoriesKey)

		// Delete the category counts hash
		pipe.Del(categoryCountsKey)

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

		log.Printf("playlist '%s' deleted successfully for guild %s", playlistName, guildID)
		return nil
	})
}

func (c *Client) GetChannel(guildID, playlistName string, channelID string) (*models.TvChannel, error) {
	var channel *models.TvChannel

	err := c.instrumentOperation("get-channel", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		channelKey := fmt.Sprintf("%s:%s", channelsKey, channelID)

		exists, err = c.rdb.Exists(channelKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if channel exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("channel with ID '%s' not found in playlist '%s'", channelID, playlistName)
		}

		channelData, err := c.rdb.HGetAll(channelKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel data: %w", err)
		}

		channel = &models.TvChannel{
			ID:       channelID,
			Name:     channelData["name"],
			Url:      channelData["url"],
			Logo:     channelData["logo"],
			Category: channelData["category"],
			Favorite: channelData["favorite"] == "1" || channelData["favorite"] == "true",
			Enabled:  channelData["enabled"] == "1" || channelData["enabled"] == "true",
		}

		log.Printf("retrieved channel '%s' from playlist '%s' for guild %s",
			channel.Name, playlistName, guildID)

		return nil
	})

	return channel, err
}

func (c *Client) RemoteControlCommand(command *models.RemoteControlCommand) error {
	return c.instrumentOperation("remote-control-command", func() error {
		jsonData, err := json.Marshal(command)
		if err != nil {
			return fmt.Errorf("failed to marshal remote control command: %w", err)
		}

		log.Printf("jsonData: %s", jsonData)
		err = c.rdb.Publish(c.config.RedisPubSubChannel, jsonData).Err()
		if err != nil {
			return fmt.Errorf("failed to publish remote control command: %w", err)
		}
		log.Printf("published remote control command '%s' to channel '%s'",
			command.Command, c.config.RedisPubSubChannel)
		return nil
	})
}

func (c *Client) GetCategories(guildID string, playlistName string) ([]string, error) {
	var categories []string

	err := c.instrumentOperation("get-categories-from-set", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)

		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Directly get categories from the set we created
		categories, err = c.rdb.SMembers(categoriesKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve categories: %w", err)
		}

		log.Printf("retrieved %d categories from set for playlist '%s' in guild %s",
			len(categories), playlistName, guildID)

		return nil
	})

	return categories, err
}

func (c *Client) GetChannelsByCategory(guildID, playlistName, category string) ([]models.TvChannel, error) {
	var channels []models.TvChannel

	err := c.instrumentOperation("get-channels-by-category", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)
		categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)

		// Check if playlist exists
		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Get channel indices for the specified category
		channelIndices, err := c.rdb.SMembers(categoryChannelsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel indices for category '%s': %w", category, err)
		}

		// If no channels found for the category
		if len(channelIndices) == 0 {
			log.Printf("no channels found for category '%s' in playlist '%s'", category, playlistName)
			return nil
		}

		// Retrieve each channel
		// Use a Redis pipeline to batch HGetAll commands
		pipe := c.rdb.Pipeline()
		cmds := make([]*redis.StringStringMapCmd, len(channelIndices))

		for i, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)
			cmds[i] = pipe.HGetAll(channelKey)
		}

		// Execute the pipeline
		_, err = pipe.Exec()
		if err != nil {
			return fmt.Errorf("failed to execute Redis pipeline: %w", err)
		}

		// Process the results
		for i, cmd := range cmds {
			channelData, err := cmd.Result()
			if err != nil {
				log.Printf("Warning: failed to retrieve channel data for index %s: %v", channelIndices[i], err)
				continue
			}

			// Create channel object
			channel := models.TvChannel{
				Name:     channelData["name"],
				Url:      channelData["url"],
				Logo:     channelData["logo"],
				Category: channelData["category"],
			}

			// Get ID as string
			if idStr, ok := channelData["id"]; ok && idStr != "" {
				channel.ID = idStr
			}

			// Parse boolean fields
			if favoriteStr, ok := channelData["favorite"]; ok {
				channel.Favorite = favoriteStr == "1" || favoriteStr == "true"
			}

			if enabledStr, ok := channelData["enabled"]; ok {
				channel.Enabled = enabledStr == "1" || enabledStr == "true"
			}

			// Add channel to results
			channels = append(channels, channel)
		}

		log.Printf("retrieved %d channels for category '%s' in playlist '%s' for guild %s",
			len(channels), category, playlistName, guildID)
		return nil
	})

	return channels, err
}

func (c *Client) GetCategoryStats(guildID, playlistName string) (map[string]int, error) {
	var categoryStats map[string]int

	err := c.instrumentOperation("get-category-stats", func() error {
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)

		// Check if playlist exists
		exists, err := c.rdb.Exists(playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Get all category counts from the hash
		categoryCountsMap, err := c.rdb.HGetAll(categoryCountsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve category counts: %w", err)
		}

		// Convert string counts to integers
		categoryStats = make(map[string]int, len(categoryCountsMap))
		for category, countStr := range categoryCountsMap {
			count, err := strconv.Atoi(countStr)
			if err != nil {
				log.Printf("Warning: failed to parse count for category '%s': %v", category, err)
				categoryStats[category] = 0
			} else {
				categoryStats[category] = count
			}
		}

		log.Printf("retrieved stats for %d categories from playlist '%s' in guild %s",
			len(categoryStats), playlistName, guildID)
		return nil
	})

	return categoryStats, err
}

func (c *Client) GetKey(key string) string {
	val, err := c.rdb.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("key %s not found in Redis", key)
			return ""
		}
		log.Printf("error getting key %s from Redis: %v", key, err)
		return ""
	}
	return val
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
