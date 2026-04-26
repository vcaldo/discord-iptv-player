package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

func newPostgresPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.PostgresConnString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse PostgreSQL configuration: %w", err)
	}
	if cfg.PostgresMaxConns > 0 {
		poolConfig.MaxConns = cfg.PostgresMaxConns
	}

	maxAttempts := 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL pool: %w", err)
		}
		if err := pool.Ping(ctx); err == nil {
			return pool, nil
		} else {
			pool.Close()
			lastErr = err
			log.Printf("PostgreSQL connection attempt %d/%d failed: %v. Retrying...", attempt, maxAttempts, err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", maxAttempts, lastErr)
}

func migratePostgres(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_trgm`,
		`CREATE TABLE IF NOT EXISTS playlists (
			guild_id text NOT NULL,
			name text NOT NULL,
			source text NOT NULL,
			updated_at timestamptz NOT NULL,
			length integer NOT NULL DEFAULT 0,
			created_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (guild_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_channels (
			guild_id text NOT NULL,
			playlist_name text NOT NULL,
			channel_id text NOT NULL,
			position integer NOT NULL,
			name text NOT NULL,
			url text NOT NULL,
			logo text NOT NULL DEFAULT '',
			category text NOT NULL DEFAULT '',
			favorite boolean NOT NULL DEFAULT false,
			enabled boolean NOT NULL DEFAULT true,
			PRIMARY KEY (guild_id, playlist_name, channel_id),
			FOREIGN KEY (guild_id, playlist_name) REFERENCES playlists (guild_id, name) ON DELETE CASCADE
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS playlist_channels_position_idx
			ON playlist_channels (guild_id, playlist_name, position)`,
		`CREATE INDEX IF NOT EXISTS playlist_channels_category_idx
			ON playlist_channels (guild_id, playlist_name, category)`,
		`CREATE INDEX IF NOT EXISTS playlist_channels_name_trgm_idx
			ON playlist_channels USING gin (lower(name) gin_trgm_ops)`,
		`CREATE TABLE IF NOT EXISTS current_playlists (
			guild_id text PRIMARY KEY,
			playlist_name text NOT NULL,
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS key_values (
			key text PRIMARY KEY,
			value text NOT NULL,
			expires_at timestamptz,
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS key_values_expires_at_idx
			ON key_values (expires_at) WHERE expires_at IS NOT NULL`,
	}

	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("failed to apply PostgreSQL schema statement %q: %w", statement, err)
		}
	}

	return nil
}

func (c *Client) pgStorePlaylist(playlist *models.Playlist, guildID string) error {
	return c.instrumentOperation("postgres-store-playlist", func() error {
		ctx := context.Background()
		tx, err := c.pg.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin PostgreSQL transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `
			INSERT INTO playlists (guild_id, name, source, updated_at, length)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (guild_id, name) DO UPDATE SET
				source = EXCLUDED.source,
				updated_at = EXCLUDED.updated_at,
				length = EXCLUDED.length`,
			guildID, playlist.Name, playlist.Source, playlist.Updated, len(playlist.Channels))
		if err != nil {
			return fmt.Errorf("failed to upsert playlist metadata in PostgreSQL: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM playlist_channels WHERE guild_id = $1 AND playlist_name = $2`, guildID, playlist.Name); err != nil {
			return fmt.Errorf("failed to delete old playlist channels from PostgreSQL: %w", err)
		}

		rows := make([][]any, 0, len(playlist.Channels))
		for i, channel := range playlist.Channels {
			channelID := channel.ID
			if channelID == "" {
				channelID = strconv.Itoa(i + 1)
			}
			rows = append(rows, []any{
				guildID,
				playlist.Name,
				channelID,
				i + 1,
				channel.Name,
				channel.Url,
				channel.Logo,
				channel.Category,
				channel.Favorite,
				channel.Enabled,
			})
		}

		if len(rows) > 0 {
			_, err = tx.CopyFrom(ctx,
				pgx.Identifier{"playlist_channels"},
				[]string{"guild_id", "playlist_name", "channel_id", "position", "name", "url", "logo", "category", "favorite", "enabled"},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				return fmt.Errorf("failed to copy playlist channels into PostgreSQL: %w", err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit PostgreSQL playlist store: %w", err)
		}

		return nil
	})
}

func (c *Client) pgGetPlaylist(guildID, playlistName string) (*models.Playlist, error) {
	var playlist *models.Playlist
	err := c.instrumentOperation("postgres-get-playlist", func() error {
		ctx := context.Background()
		var source string
		var updated time.Time
		err := c.pg.QueryRow(ctx, `
			SELECT source, updated_at
			FROM playlists
			WHERE guild_id = $1 AND name = $2`,
			guildID, playlistName).Scan(&source, &updated)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve playlist metadata from PostgreSQL: %w", err)
		}

		channels, err := c.pgQueryChannels(ctx, `
			SELECT channel_id, name, url, logo, category, favorite, enabled
			FROM playlist_channels
			WHERE guild_id = $1 AND playlist_name = $2
			ORDER BY position`,
			guildID, playlistName)
		if err != nil {
			return err
		}

		playlist = &models.Playlist{
			Name:     playlistName,
			Source:   source,
			Updated:  updated,
			Channels: channels,
		}
		return nil
	})

	return playlist, err
}

func (c *Client) pgGetPlaylistMetadata(guildID, playlistName string) (*models.Playlist, error) {
	var playlist *models.Playlist
	err := c.instrumentOperation("postgres-get-playlist-metadata", func() error {
		ctx := context.Background()
		var source string
		var updated time.Time
		err := c.pg.QueryRow(ctx, `
			SELECT source, updated_at
			FROM playlists
			WHERE guild_id = $1 AND name = $2`,
			guildID, playlistName).Scan(&source, &updated)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve playlist metadata from PostgreSQL: %w", err)
		}

		playlist = &models.Playlist{
			Name:     playlistName,
			Source:   source,
			Updated:  updated,
			Channels: []models.TvChannel{},
		}
		return nil
	})

	return playlist, err
}

func (c *Client) pgListPlaylists(guildID string) ([]string, error) {
	var playlistNames []string
	err := c.instrumentOperation("postgres-list-playlists", func() error {
		ctx := context.Background()
		rows, err := c.pg.Query(ctx, `
			SELECT name
			FROM playlists
			WHERE guild_id = $1
			ORDER BY name`,
			guildID)
		if err != nil {
			return fmt.Errorf("failed to list playlists from PostgreSQL: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return fmt.Errorf("failed to scan PostgreSQL playlist name: %w", err)
			}
			playlistNames = append(playlistNames, name)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to read PostgreSQL playlist names: %w", err)
		}

		return nil
	})

	return playlistNames, err
}

func (c *Client) pgDeletePlaylist(guildID, playlistName string) error {
	return c.instrumentOperation("postgres-delete-playlist", func() error {
		ctx := context.Background()
		tag, err := c.pg.Exec(ctx, `DELETE FROM playlists WHERE guild_id = $1 AND name = $2`, guildID, playlistName)
		if err != nil {
			return fmt.Errorf("failed to delete playlist from PostgreSQL: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
		}
		return nil
	})
}

func (c *Client) pgGetChannel(guildID, playlistName string, channelID string) (*models.TvChannel, error) {
	var channel *models.TvChannel
	err := c.instrumentOperation("postgres-get-channel", func() error {
		ctx := context.Background()
		lookupID := channelID
		channel = &models.TvChannel{}
		err := c.pg.QueryRow(ctx, `
			SELECT channel_id, name, url, logo, category, favorite, enabled
			FROM playlist_channels
			WHERE guild_id = $1 AND playlist_name = $2 AND channel_id = $3`,
			guildID, playlistName, lookupID).Scan(
			&channel.ID,
			&channel.Name,
			&channel.Url,
			&channel.Logo,
			&channel.Category,
			&channel.Favorite,
			&channel.Enabled,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("channel with ID '%s' not found in playlist '%s'", lookupID, playlistName)
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve channel from PostgreSQL: %w", err)
		}
		return nil
	})

	return channel, err
}

func (c *Client) pgGetCategories(guildID string, playlistName string) ([]string, error) {
	var categories []string
	err := c.instrumentOperation("postgres-get-categories", func() error {
		ctx := context.Background()
		if err := c.pgEnsurePlaylistExists(ctx, guildID, playlistName); err != nil {
			return err
		}

		rows, err := c.pg.Query(ctx, `
			SELECT DISTINCT category
			FROM playlist_channels
			WHERE guild_id = $1 AND playlist_name = $2 AND category <> ''
			ORDER BY category`,
			guildID, playlistName)
		if err != nil {
			return fmt.Errorf("failed to retrieve categories from PostgreSQL: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var category string
			if err := rows.Scan(&category); err != nil {
				return fmt.Errorf("failed to scan PostgreSQL category: %w", err)
			}
			categories = append(categories, category)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to read PostgreSQL categories: %w", err)
		}

		return nil
	})

	return categories, err
}

func (c *Client) pgGetChannelsByCategory(guildID, playlistName, category string) ([]models.TvChannel, error) {
	var channels []models.TvChannel
	err := c.instrumentOperation("postgres-get-channels-by-category", func() error {
		ctx := context.Background()
		if err := c.pgEnsurePlaylistExists(ctx, guildID, playlistName); err != nil {
			return err
		}

		var err error
		channels, err = c.pgQueryChannels(ctx, `
			SELECT channel_id, name, url, logo, category, favorite, enabled
			FROM playlist_channels
			WHERE guild_id = $1 AND playlist_name = $2 AND category = $3
			ORDER BY name`,
			guildID, playlistName, category)
		return err
	})

	return channels, err
}

func (c *Client) pgGetCategoryStats(guildID, playlistName string) (map[string]int, error) {
	categoryStats := make(map[string]int)
	err := c.instrumentOperation("postgres-get-category-stats", func() error {
		ctx := context.Background()
		if err := c.pgEnsurePlaylistExists(ctx, guildID, playlistName); err != nil {
			return err
		}

		rows, err := c.pg.Query(ctx, `
			SELECT category, count(*)
			FROM playlist_channels
			WHERE guild_id = $1 AND playlist_name = $2 AND category <> ''
			GROUP BY category`,
			guildID, playlistName)
		if err != nil {
			return fmt.Errorf("failed to retrieve category stats from PostgreSQL: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var category string
			var count int
			if err := rows.Scan(&category, &count); err != nil {
				return fmt.Errorf("failed to scan PostgreSQL category stats: %w", err)
			}
			categoryStats[category] = count
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to read PostgreSQL category stats: %w", err)
		}

		return nil
	})

	return categoryStats, err
}

func (c *Client) pgSearchChannels(guildID, playlistName, query string) ([]models.TvChannel, error) {
	var channels []models.TvChannel
	err := c.instrumentOperation("postgres-search-channels", func() error {
		ctx := context.Background()
		if err := c.pgEnsurePlaylistExists(ctx, guildID, playlistName); err != nil {
			return err
		}

		var err error
		channels, err = c.pgQueryChannels(ctx, `
			SELECT channel_id, name, url, logo, category, favorite, enabled
			FROM playlist_channels
			WHERE guild_id = $1
				AND playlist_name = $2
				AND lower(name) LIKE '%' || lower($3) || '%'
			ORDER BY position`,
			guildID, playlistName, strings.TrimSpace(query))
		return err
	})

	return channels, err
}

func (c *Client) pgSetCurrentPlaylist(guildID, playlistName string) error {
	return c.instrumentOperation("postgres-set-current-playlist", func() error {
		_, err := c.pg.Exec(context.Background(), `
			INSERT INTO current_playlists (guild_id, playlist_name, updated_at)
			VALUES ($1, $2, now())
			ON CONFLICT (guild_id) DO UPDATE SET
				playlist_name = EXCLUDED.playlist_name,
				updated_at = EXCLUDED.updated_at`,
			guildID, playlistName)
		if err != nil {
			return fmt.Errorf("failed to set current playlist in PostgreSQL: %w", err)
		}
		return nil
	})
}

func (c *Client) pgGetCurrentPlaylist(guildID string) (string, error) {
	var playlistName string
	err := c.instrumentOperation("postgres-get-current-playlist", func() error {
		err := c.pg.QueryRow(context.Background(), `
			SELECT playlist_name
			FROM current_playlists
			WHERE guild_id = $1`,
			guildID).Scan(&playlistName)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("current playlist not found for guild %s", guildID)
		}
		if err != nil {
			return fmt.Errorf("failed to get current playlist from PostgreSQL: %w", err)
		}
		return nil
	})

	return playlistName, err
}

func (c *Client) pgGet(ctx context.Context, key string) (string, error) {
	var value string
	err := c.pg.QueryRow(ctx, `
		SELECT value
		FROM key_values
		WHERE key = $1 AND (expires_at IS NULL OR expires_at > now())`,
		key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("key %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %q from PostgreSQL: %w", key, err)
	}
	return value, nil
}

func (c *Client) pgSet(ctx context.Context, key, value string, expiration time.Duration) error {
	var expiresAt any
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	_, err := c.pg.Exec(ctx, `
		INSERT INTO key_values (key, value, expires_at, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at,
			updated_at = EXCLUDED.updated_at`,
		key, value, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to set key %q in PostgreSQL: %w", key, err)
	}
	return nil
}

func (c *Client) pgDel(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := c.pg.Exec(ctx, `DELETE FROM key_values WHERE key = ANY($1)`, keys)
	if err != nil {
		return fmt.Errorf("failed to delete keys from PostgreSQL: %w", err)
	}
	return nil
}

func (c *Client) pgKeys(ctx context.Context, pattern string) ([]string, error) {
	rows, err := c.pg.Query(ctx, `
		SELECT key
		FROM key_values
		WHERE key LIKE $1 ESCAPE '\'
			AND (expires_at IS NULL OR expires_at > now())
		ORDER BY key`,
		redisPatternToSQLLike(pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to list keys from PostgreSQL: %w", err)
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read PostgreSQL keys: %w", err)
	}

	return keys, nil
}

func (c *Client) pgEnsurePlaylistExists(ctx context.Context, guildID, playlistName string) error {
	var exists bool
	if err := c.pg.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM playlists WHERE guild_id = $1 AND name = $2
		)`,
		guildID, playlistName).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check playlist existence in PostgreSQL: %w", err)
	}
	if !exists {
		return fmt.Errorf("playlist '%s' not found for guild %s", playlistName, guildID)
	}
	return nil
}

func (c *Client) pgQueryChannels(ctx context.Context, query string, args ...any) ([]models.TvChannel, error) {
	rows, err := c.pg.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels from PostgreSQL: %w", err)
	}
	defer rows.Close()

	channels := make([]models.TvChannel, 0)
	for rows.Next() {
		var channel models.TvChannel
		if err := rows.Scan(
			&channel.ID,
			&channel.Name,
			&channel.Url,
			&channel.Logo,
			&channel.Category,
			&channel.Favorite,
			&channel.Enabled,
		); err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL channel: %w", err)
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read PostgreSQL channels: %w", err)
	}

	return channels, nil
}

func redisPatternToSQLLike(pattern string) string {
	var builder strings.Builder
	for _, char := range pattern {
		switch char {
		case '*':
			builder.WriteByte('%')
		case '?':
			builder.WriteByte('_')
		case '%', '_', '\\':
			builder.WriteByte('\\')
			builder.WriteRune(char)
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
