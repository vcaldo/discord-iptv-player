package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

type Client struct {
	pg     *pgxpool.Pool
	config *config.Config
	nrApp  *newrelic.Application
}

func NewClient(ctx context.Context, cfg *config.Config, nrApp *newrelic.Application) (*Client, error) {
	txn := nrApp.StartTransaction("postgres:initialize-client")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	pgPool, err := newPostgresPool(ctx, cfg)
	if err != nil {
		txn.NoticeError(err)
		return nil, err
	}
	if err := migratePostgres(ctx, pgPool); err != nil {
		pgPool.Close()
		txn.NoticeError(err)
		return nil, err
	}

	log.Printf("successfully connected to PostgreSQL")

	return &Client{
		pg:     pgPool,
		config: cfg,
		nrApp:  nrApp,
	}, nil
}

func (c *Client) instrumentOperation(operationName string, fn func() error) error {
	txn := c.nrApp.StartTransaction("postgres:" + operationName)
	defer txn.End()

	err := fn()
	if err != nil {
		txn.NoticeError(err)
	}

	return err
}

// SetEx writes a string value at key with a TTL. Used by the metrics state
// writer so that a crashed process doesn't leave stale data sitting in storage.
func (c *Client) SetEx(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.pgSet(ctx, key, value, ttl)
}

// PingRaw exposes a thin Ping wrapper for callers that just need to check
// liveness without going through the instrumented operation pipeline.
func (c *Client) PingRaw(ctx context.Context) (string, error) {
	if err := c.pg.Ping(ctx); err != nil {
		return "", err
	}
	return "PONG", nil
}

func (c *Client) StorePlaylist(playlist *models.Playlist, guildID string) error {
	return c.pgStorePlaylist(playlist, guildID)
}

func (c *Client) GetPlaylist(guildID, playlistName string) (*models.Playlist, error) {
	return c.pgGetPlaylist(guildID, playlistName)
}

func (c *Client) GetPlaylistMetadata(guildID, playlistName string) (*models.Playlist, error) {
	return c.pgGetPlaylistMetadata(guildID, playlistName)
}

func (c *Client) ListPlaylists(guildID string) ([]string, error) {
	return c.pgListPlaylists(guildID)
}

func (c *Client) DeletePlaylist(guildID, playlistName string) error {
	return c.pgDeletePlaylist(guildID, playlistName)
}

func (c *Client) GetChannel(guildID, playlistName string, channelID string) (*models.TvChannel, error) {
	return c.pgGetChannel(guildID, playlistName, channelID)
}

func (c *Client) RemoteControlCommand(command *models.RemoteControlCommand) error {
	return c.instrumentOperation("remote-control-command", func() error {
		ctx := context.Background()
		jsonData, err := json.Marshal(command)
		if err != nil {
			return fmt.Errorf("failed to marshal remote control command: %w", err)
		}

		_, err = c.pg.Exec(ctx, `SELECT pg_notify($1, $2)`, c.config.ControlChannel, string(jsonData))
		if err != nil {
			return fmt.Errorf("failed to publish remote control command through PostgreSQL: %w", err)
		}

		log.Printf("published remote control command '%s' to PostgreSQL channel '%s'",
			command.Command, c.config.ControlChannel)
		return nil
	})
}

func (c *Client) GetCategories(guildID string, playlistName string) ([]string, error) {
	return c.pgGetCategories(guildID, playlistName)
}

func (c *Client) GetChannelsByCategory(guildID, playlistName, category string) ([]models.TvChannel, error) {
	return c.pgGetChannelsByCategory(guildID, playlistName, category)
}

func (c *Client) GetCategoryStats(guildID, playlistName string) (map[string]int, error) {
	return c.pgGetCategoryStats(guildID, playlistName)
}

func (c *Client) GetID(key string) string {
	id, err := c.pgGet(context.Background(), key)
	if err != nil {
		log.Printf("failed to get ID for key '%s': %v", key, err)
		return ""
	}
	return id
}

// SetCurrentPlaylist sets the current playlist for a guild.
func (c *Client) SetCurrentPlaylist(guildID, playlistName string) error {
	return c.pgSetCurrentPlaylist(guildID, playlistName)
}

// GetCurrentPlaylist gets the current playlist for a guild.
func (c *Client) GetCurrentPlaylist(guildID string) (string, error) {
	return c.pgGetCurrentPlaylist(guildID)
}

func (c *Client) Close() error {
	c.pg.Close()
	return nil
}

func (c *Client) SearchChannels(guildID, playlistName, query string) ([]models.TvChannel, error) {
	return c.pgSearchChannels(guildID, playlistName, query)
}

// Get retrieves a value by key from PostgreSQL.
func (c *Client) Get(key string) (string, error) {
	return c.pgGet(context.Background(), key)
}

// Set stores a key-value pair in PostgreSQL with optional expiration.
func (c *Client) Set(key, value string, expiration time.Duration) error {
	return c.pgSet(context.Background(), key, value, expiration)
}

// Del deletes one or more keys from PostgreSQL.
func (c *Client) Del(keys ...string) error {
	return c.pgDel(context.Background(), keys...)
}

// Keys finds all keys matching a wildcard pattern in PostgreSQL.
func (c *Client) Keys(pattern string) ([]string, error) {
	return c.pgKeys(context.Background(), pattern)
}
