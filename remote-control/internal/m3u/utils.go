package m3u

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

func InitializePlaylist(ctx context.Context, config *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("m3u:initialize-playlist")
	defer txn.End()

	return nil
}
