package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DiscordToken        string `envconfig:"DISCORD_BOT_TOKEN" required:"true"`
	DiscordGuildID      string `envconfig:"DISCORD_GUILD_ID" required:"true"`
	NewRelicAppName     string `envconfig:"NEW_RELIC_APP_NAME" default:"Discord IPTV Player - Remote Control"`
	NewRelicLicenseKey  string `envconfig:"NEW_RELIC_LICENSE_KEY" default:""`
	RedisAddress        string `envconfig:"REDIS_ADDRESS" default:"localhost:6379"`
	RedisPassword       string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB             int    `envconfig:"REDIS_DB" default:"0"`
	RedisPubSubChannel  string `envconfig:"REDIS_PUB_SUB_CHANNEL" default:"iptv"`
	PlaylistsConfigPath string `envconfig:"PLAYLISTS_CONFIG_PATH" default:"./playlists.yaml"`
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	log.Printf("configuration loaded")
	return &cfg, nil
}
