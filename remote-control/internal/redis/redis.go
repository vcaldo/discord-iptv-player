package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/m3u"
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

func (c *Client) StorePlaylist(playlist *m3u.Playlist, guildID string) error {
	return c.instrumentOperation("store-playlist", func() error {
		// Convert playlist to JSON
		playlistJSON, err := json.Marshal(playlist)
		if err != nil {
			return fmt.Errorf("failed to marshal playlist: %w", err)
		}

		// Store playlist with guild-specific key
		key := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlist.Name)

		// Store with expiry time (30 days)
		err = c.rdb.Set(key, playlistJSON, 30*24*time.Hour).Err()
		if err != nil {
			return fmt.Errorf("failed to store playlist in Redis: %w", err)
		}

		// Store playlist name in a set for easy listing
		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		err = c.rdb.SAdd(setKey, playlist.Name).Err()
		if err != nil {
			return fmt.Errorf("failed to add playlist to set: %w", err)
		}

		log.Printf("Playlist '%s' stored successfully for guild %s", playlist.Name, guildID)
		return nil
	})
}

func (c *Client) GetPlaylist(guildID, playlistName string) (*m3u.Playlist, error) {
	var playlist *m3u.Playlist

	err := c.instrumentOperation("get-playlist", func() error {
		key := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)

		// Get playlist JSON from Redis
		playlistJSON, err := c.rdb.Get(key).Bytes()
		if err != nil {
			if err == redis.Nil {
				return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
			}
			return fmt.Errorf("failed to retrieve playlist from Redis: %w", err)
		}

		// Unmarshal JSON into playlist struct
		playlist = &m3u.Playlist{}
		if err := json.Unmarshal(playlistJSON, playlist); err != nil {
			return fmt.Errorf("failed to unmarshal playlist: %w", err)
		}

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
		// Delete the playlist itself
		key := fmt.Sprintf("guild:%s:playlist:%s", guildID, playlistName)
		err := c.rdb.Del(key).Err()
		if err != nil {
			return fmt.Errorf("failed to delete playlist from Redis: %w", err)
		}

		// Remove from the set of playlists
		setKey := fmt.Sprintf("guild:%s:playlists", guildID)
		err = c.rdb.SRem(setKey, playlistName).Err()
		if err != nil {
			return fmt.Errorf("failed to remove playlist from set: %w", err)
		}

		log.Printf("Playlist '%s' deleted successfully for guild %s", playlistName, guildID)
		return nil
	})
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
