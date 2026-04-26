package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const (
	StorageEngineRedis    = "redis"
	StorageEnginePostgres = "postgres"
)

type Config struct {
	DiscordToken        string `envconfig:"DISCORD_BOT_TOKEN" required:"true"`
	DiscordGuildID      string `envconfig:"DISCORD_GUILD_ID" required:"true"`
	NewRelicAppName     string `envconfig:"NEW_RELIC_APP_NAME" default:"Discord IPTV Player - Remote Control"`
	NewRelicLicenseKey  string `envconfig:"NEW_RELIC_LICENSE_KEY" default:""`
	StorageEngine       string `envconfig:"STORAGE_ENGINE" default:"redis"`
	RedisAddress        string `envconfig:"REDIS_ADDRESS" default:"localhost:6379"`
	RedisPassword       string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB             int    `envconfig:"REDIS_DB" default:"0"`
	RedisPubSubChannel  string `envconfig:"REDIS_PUB_SUB_CHANNEL" default:"iptv"`
	PostgresDSN         string `envconfig:"POSTGRES_DSN" default:""`
	PostgresHost        string `envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort        string `envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresUser        string `envconfig:"POSTGRES_USER" default:"postgres"`
	PostgresPassword    string `envconfig:"POSTGRES_PASSWORD" default:""`
	PostgresDatabase    string `envconfig:"POSTGRES_DATABASE" default:"discord_iptv_player"`
	PostgresSSLMode     string `envconfig:"POSTGRES_SSLMODE" default:"disable"`
	PostgresMaxConns    int32  `envconfig:"POSTGRES_MAX_CONNS" default:"10"`
	PlaylistsConfigPath string `envconfig:"PLAYLISTS_CONFIG_PATH" default:"./playlists.yaml"`
	BlacklistedUsers    string `envconfig:"BLACKLISTED_USERS" default:""`
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	if _, err := cfg.NormalizedStorageEngine(); err != nil {
		return nil, err
	}

	log.Printf("configuration loaded")
	return &cfg, nil
}

func (c *Config) NormalizedStorageEngine() (string, error) {
	switch strings.ToLower(strings.TrimSpace(c.StorageEngine)) {
	case "", StorageEngineRedis:
		return StorageEngineRedis, nil
	case "pg", "pgsql", "postgresql", StorageEnginePostgres:
		return StorageEnginePostgres, nil
	default:
		return "", fmt.Errorf("unsupported STORAGE_ENGINE %q (use %q or %q)", c.StorageEngine, StorageEngineRedis, StorageEnginePostgres)
	}
}

func (c *Config) PostgresConnString() string {
	if strings.TrimSpace(c.PostgresDSN) != "" {
		return c.PostgresDSN
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(c.PostgresHost, c.PostgresPort),
		Path:   c.PostgresDatabase,
	}
	if c.PostgresPassword != "" {
		u.User = url.UserPassword(c.PostgresUser, c.PostgresPassword)
	} else {
		u.User = url.User(c.PostgresUser)
	}
	query := u.Query()
	query.Set("sslmode", c.PostgresSSLMode)
	u.RawQuery = query.Encode()
	return u.String()
}

// GetBlacklistedUsers returns a slice of blacklisted user IDs
func (c *Config) GetBlacklistedUsers() []string {
	if c.BlacklistedUsers == "" {
		return []string{}
	}

	userIDs := strings.Split(c.BlacklistedUsers, ",")
	var result []string
	for _, userID := range userIDs {
		trimmed := strings.TrimSpace(userID)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// IsUserBlacklisted checks if a user ID is in the blacklist
func (c *Config) IsUserBlacklisted(userID string) bool {
	blacklistedUsers := c.GetBlacklistedUsers()
	for _, blacklistedUser := range blacklistedUsers {
		if blacklistedUser == userID {
			return true
		}
	}
	return false
}
