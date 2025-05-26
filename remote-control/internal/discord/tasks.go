package discord

import (
	"context"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

func (b *Bot) checkExpiredBotDelays(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error {
	// ToDo
	return nil
}
