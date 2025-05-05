package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DiscordToken       string `envconfig:"DISCORD_BOT_TOKEN" required:"true"`
	NewRelicAppName    string `envconfig:"NEW_RELIC_APP_NAME" default:"Discord IPTV Player"`
	NewRelicLicenseKey string `envconfig:"NEW_RELIC_LICENSE_KEY" default:""`
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
