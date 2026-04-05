package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
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
		Addr:            cfg.RedisAddress,
		Password:        cfg.RedisPassword,
		DB:              cfg.RedisDB,
		DialTimeout:     20 * time.Second,
		ReadTimeout:     2 * time.Minute,
		WriteTimeout:    10 * time.Minute,
		PoolSize:        20,
		PoolTimeout:     10 * time.Minute,
		MaxRetries:      10,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 10 * time.Minute,
	})

	var pong string
	var err error
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		segment := txn.StartSegment(fmt.Sprintf("redis:ping-attempt-%d", attempt))
		pong, err = rdb.Ping(ctx).Result()
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
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlist.Name)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)
		// We'll do the metadata and deletions first, then write channels in batches to avoid a single large write
		// causing network timeouts.
		// Execute metadata/deletes first
		metaPipe := c.rdb.Pipeline()
		metaPipe.HSet(ctx, playlistKey, "name", playlist.Name)
		metaPipe.HSet(ctx, playlistKey, "source", playlist.Source)
		metaPipe.HSet(ctx, playlistKey, "updated", playlist.Updated.Format(time.RFC3339))
		metaPipe.HSet(ctx, playlistKey, "length", len(playlist.Channels))

		// Delete existing channels, categories, and category counts before updating
		metaPipe.Del(ctx, channelsKey)
		metaPipe.Del(ctx, categoriesKey)
		metaPipe.Del(ctx, categoryCountsKey)

		// Delete existing category-to-channel mappings
		existingCategories, err := c.rdb.SMembers(ctx, categoriesKey).Result()
		if err == nil {
			for _, category := range existingCategories {
				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)
				metaPipe.Del(ctx, categoryChannelsKey)
			}
		}

		// Execute metadata deletions early so the heavy channel writes are separate
		if _, err := metaPipe.Exec(ctx); err != nil {
			return fmt.Errorf("failed to initialize playlist in Redis: %w", err)
		}

		// Track unique categories and their channel counts
		categoriesMap := make(map[string]struct{})
		categoryCountsMap := make(map[string]int)

		// Write channels in batches
		const batchSize = 500
		batchPipe := c.rdb.Pipeline()
		cmdsInBatch := 0

		// execWithRetries accepts any Pipeliner (Pipeline implements it) so we can pass batchPipe, metaPipe, etc.
		execWithRetries := func(p redis.Pipeliner) error {
			var err error
			maxAttempts := 3
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				_, err = p.Exec(ctx)
				if err == nil {
					return nil
				}
				log.Printf("redis pipeline exec attempt %d/%d failed: %v", attempt, maxAttempts, err)
				if attempt < maxAttempts {
					time.Sleep(time.Duration(attempt) * time.Second)
				}
			}
			return err
		}

		for i, channel := range playlist.Channels {
			channelIndex := i + 1
			channelKey := fmt.Sprintf("%s:%d", channelsKey, channelIndex)

			batchPipe.HSet(ctx, channelKey, "id", channel.ID)
			batchPipe.HSet(ctx, channelKey, "name", channel.Name)
			batchPipe.HSet(ctx, channelKey, "url", channel.Url)
			batchPipe.HSet(ctx, channelKey, "logo", channel.Logo)
			batchPipe.HSet(ctx, channelKey, "category", channel.Category)
			batchPipe.HSet(ctx, channelKey, "favorite", channel.Favorite)
			batchPipe.HSet(ctx, channelKey, "enabled", channel.Enabled)

			batchPipe.SAdd(ctx, channelsKey, channelIndex)

			if channel.Category != "" {
				categoriesMap[channel.Category] = struct{}{}
				categoryCountsMap[channel.Category]++

				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, channel.Category)
				batchPipe.SAdd(ctx, categoryChannelsKey, channelIndex)
			}

			cmdsInBatch++
			if cmdsInBatch >= batchSize {
				if err := execWithRetries(batchPipe); err != nil {
					return fmt.Errorf("failed to store playlist channels in Redis: %w", err)
				}
				// reset the batch
				batchPipe = c.rdb.Pipeline()
				cmdsInBatch = 0
			}
		}

		// Execute any remaining commands
		if cmdsInBatch > 0 {
			if err := execWithRetries(batchPipe); err != nil {
				return fmt.Errorf("failed to store playlist channels in Redis: %w", err)
			}
		}

		// Store categories and counts and add playlist name to guild set
		finalPipe := c.rdb.Pipeline()
		for category := range categoriesMap {
			finalPipe.SAdd(ctx, categoriesKey, category)
		}
		for category, count := range categoryCountsMap {
			finalPipe.HSet(ctx, categoryCountsKey, category, count)
		}
		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		finalPipe.SAdd(ctx, setKey, playlist.Name)

		if _, err := finalPipe.Exec(ctx); err != nil {
			return fmt.Errorf("failed to finalize playlist store in Redis: %w", err)
		}

		log.Printf("playlist '%s' stored successfully for guild %s with %d channels and %d categories",
			playlist.Name, guildID, len(playlist.Channels), len(categoriesMap))
		return nil
	})
}

func (c *Client) GetPlaylist(guildID, playlistName string) (*models.Playlist, error) {
	var playlist *models.Playlist
	err := c.instrumentOperation("get-playlist", func() error {
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		playlistData, err := c.rdb.HGetAll(ctx, playlistKey).Result()
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
		channelIndices, err := c.rdb.SMembers(ctx, channelsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel indices: %w", err)
		}

		// Retrieve each channel
		for _, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)

			// Get channel data
			channelData, err := c.rdb.HGetAll(ctx, channelKey).Result()
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

func (c *Client) GetPlaylistMetadata(guildID, playlistName string) (*models.Playlist, error) {
	var playlist *models.Playlist
	err := c.instrumentOperation("get-playlist-metadata", func() error {
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)

		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		playlistData, err := c.rdb.HGetAll(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve playlist metadata: %w", err)
		}

		playlist = &models.Playlist{
			Name:     playlistData["name"],
			Source:   playlistData["source"],
			Channels: []models.TvChannel{}, // Empty since we're not fetching channels
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

		log.Printf("retrieved playlist metadata for '%s' in guild %s (updated: %v)",
			playlist.Name, guildID, playlist.Updated)
		return nil
	})

	return playlist, err
}

func (c *Client) ListPlaylists(guildID string) ([]string, error) {
	var playlistNames []string

	err := c.instrumentOperation("list-playlists", func() error {
		ctx := context.Background()
		key := fmt.Sprintf("guild:%s:playlists", guildID)

		// Get all playlist names from the set
		var err error
		playlistNames, err = c.rdb.SMembers(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("failed to list playlists from Redis: %w", err)
		}

		return nil
	})

	return playlistNames, err
}

func (c *Client) DeletePlaylist(guildID, playlistName string) error {
	return c.instrumentOperation("delete-playlist", func() error {
		ctx := context.Background()
		// Create keys
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)

		// Check if playlist exists
		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Use pipeline for better performance
		pipe := c.rdb.Pipeline()

		// Get all categories to clean up their channel mappings
		categories, err := c.rdb.SMembers(ctx, categoriesKey).Result()
		if err == nil {
			for _, category := range categories {
				categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)
				pipe.Del(ctx, categoryChannelsKey)
			}
		}

		// Get all channel indices before deleting
		channelIndices, err := c.rdb.SMembers(ctx, channelsKey).Result()
		if err != nil {
			return fmt.Errorf("failed to retrieve channel indices: %w", err)
		}

		// Delete each channel entry
		for _, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)
			pipe.Del(ctx, channelKey)
		}

		// Delete the channels set
		pipe.Del(ctx, channelsKey)

		// Delete the categories set
		pipe.Del(ctx, categoriesKey)

		// Delete the category counts hash
		pipe.Del(ctx, categoryCountsKey)

		// Delete the playlist metadata
		pipe.Del(ctx, playlistKey)

		// Remove from the set of playlists
		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		pipe.SRem(ctx, setKey, playlistName)

		// Execute all commands in the pipeline
		_, err = pipe.Exec(ctx)
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
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)

		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		channelKey := fmt.Sprintf("%s:%s", channelsKey, channelID)

		exists, err = c.rdb.Exists(ctx, channelKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if channel exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("channel with ID '%s' not found in playlist '%s'", channelID, playlistName)
		}

		channelData, err := c.rdb.HGetAll(ctx, channelKey).Result()
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
		ctx := context.Background()
		jsonData, err := json.Marshal(command)
		if err != nil {
			return fmt.Errorf("failed to marshal remote control command: %w", err)
		}

		log.Printf("jsonData: %s", jsonData)
		err = c.rdb.Publish(ctx, c.config.RedisPubSubChannel, jsonData).Err()
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
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		categoriesKey := fmt.Sprintf("%s:categories", playlistKey)

		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Directly get categories from the set we created
		categories, err = c.rdb.SMembers(ctx, categoriesKey).Result()
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
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		channelsKey := fmt.Sprintf("%s:channels", playlistKey)
		categoryChannelsKey := fmt.Sprintf("%s:category:%s:channels", playlistKey, category)

		// Check if playlist exists
		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Get channel indices for the specified category
		channelIndices, err := c.rdb.SMembers(ctx, categoryChannelsKey).Result()
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
		cmds := make([]*redis.MapStringStringCmd, len(channelIndices))

		for i, indexStr := range channelIndices {
			channelKey := fmt.Sprintf("%s:%s", channelsKey, indexStr)
			cmds[i] = pipe.HGetAll(ctx, channelKey)
		}

		// Execute the pipeline
		_, err = pipe.Exec(ctx)
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
		ctx := context.Background()
		playlistKey := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		categoryCountsKey := fmt.Sprintf("%s:category-counts", playlistKey)

		// Check if playlist exists
		exists, err := c.rdb.Exists(ctx, playlistKey).Result()
		if err != nil {
			return fmt.Errorf("failed to check if playlist exists: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}

		// Get all category counts from the hash
		categoryCountsMap, err := c.rdb.HGetAll(ctx, categoryCountsKey).Result()
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

func (c *Client) GetID(key string) string {
	var id string
	err := c.instrumentOperation("get-id", func() error {
		var err error
		id, err = c.rdb.Get(context.Background(), key).Result()
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("failed to get ID for key '%s': %v", key, err)
		return ""
	}

	return id
}

// SetCurrentPlaylist sets the current playlist for a guild
func (c *Client) SetCurrentPlaylist(guildID, playlistName string) error {
	return c.instrumentOperation("set-current-playlist", func() error {
		key := fmt.Sprintf("guild:%s:current_playlist", guildID)

		err := c.rdb.Set(context.Background(), key, playlistName, 0).Err()
		if err != nil {
			return fmt.Errorf("failed to set current playlist: %w", err)
		}

		log.Printf("set current playlist to '%s' for guild %s", playlistName, guildID)
		return nil
	})
}

// GetCurrentPlaylist gets the current playlist for a guild
func (c *Client) GetCurrentPlaylist(guildID string) (string, error) {
	var playlistName string

	err := c.instrumentOperation("get-current-playlist", func() error {
		key := fmt.Sprintf("guild:%s:current_playlist", guildID)

		var err error
		playlistName, err = c.rdb.Get(context.Background(), key).Result()
		if err != nil {
			return fmt.Errorf("failed to get current playlist: %w", err)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	log.Printf("retrieved current playlist '%s' for guild %s", playlistName, guildID)
	return playlistName, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// Get retrieves a value by key from Redis
func (c *Client) Get(key string) (string, error) {
	var value string
	err := c.instrumentOperation("get", func() error {
		var err error
		value, err = c.rdb.Get(context.Background(), key).Result()
		if err != nil {
			return err
		}
		return nil
	})
	return value, err
}

// Set stores a key-value pair in Redis with optional expiration
func (c *Client) Set(key, value string, expiration time.Duration) error {
	return c.instrumentOperation("set", func() error {
		return c.rdb.Set(context.Background(), key, value, expiration).Err()
	})
}

// Del deletes one or more keys from Redis
func (c *Client) Del(keys ...string) error {
	return c.instrumentOperation("del", func() error {
		return c.rdb.Del(context.Background(), keys...).Err()
	})
}

// Keys finds all keys matching a pattern
func (c *Client) Keys(pattern string) ([]string, error) {
	var keys []string
	err := c.instrumentOperation("keys", func() error {
		var err error
		keys, err = c.rdb.Keys(context.Background(), pattern).Result()
		if err != nil {
			return err
		}
		return nil
	})
	return keys, err
}
