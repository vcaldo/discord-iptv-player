package discord

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// stateKey is the storage key under which the bot publishes its current
// operational state for the nri-flex monitor to scrape via the discord-iptv
// monitor binary running on the host.
const stateKey = "remote_control:state"

// stateTTL must be longer than writeStateInterval so a momentary write blip
// does not cause readers to see "missing".
const writeStateInterval = 5 * time.Second
const stateTTL = 30 * time.Second

type remoteControlState struct {
	Timestamp           int64  `json:"timestamp"`
	StartTime           int64  `json:"start_time"`
	UptimeSec           int64  `json:"uptime_sec"`
	BotReady            bool   `json:"bot_ready"`
	BotUserID           string `json:"bot_user_id"`
	BotUserName         string `json:"bot_user_name"`
	GuildCount          int    `json:"guild_count"`
	HeartbeatLatencyMs  int64  `json:"heartbeat_latency_ms"`
	CommandsHandled     int64  `json:"commands_handled"`
	CommandErrors       int64  `json:"command_errors"`
	AutocompletesServed int64  `json:"autocompletes_served"`
	LastCommand         string `json:"last_command"`
	LastCommandAt       int64  `json:"last_command_at"`
	CurrentPlaylist     string `json:"current_playlist"`
}

// runStateWriter periodically serialises the bot's metric state and stores
// it in the selected storage engine so the host-side nri-flex collector can scrape it without
// having to touch Discord directly.
func (b *Bot) runStateWriter(ctx context.Context) {
	ticker := time.NewTicker(writeStateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.writeState(ctx); err != nil {
				log.Printf("state writer: %v", err)
			}
		}
	}
}

func (b *Bot) writeState(ctx context.Context) error {
	now := time.Now()
	st := remoteControlState{
		Timestamp:           now.Unix(),
		StartTime:           b.startTime.Unix(),
		UptimeSec:           int64(now.Sub(b.startTime).Seconds()),
		BotReady:            b.session != nil && b.session.State != nil && b.session.State.User != nil,
		CommandsHandled:     b.commandsHandled.Load(),
		CommandErrors:       b.commandErrors.Load(),
		AutocompletesServed: b.autocompletesServed.Load(),
		LastCommandAt:       b.lastCommandAt.Load(),
		CurrentPlaylist:     b.getCurrentPlaylist(b.config),
	}
	if v, ok := b.lastCommandName.Load().(string); ok {
		st.LastCommand = v
	}
	if st.BotReady {
		st.BotUserID = b.session.State.User.ID
		st.BotUserName = b.session.State.User.Username
		st.GuildCount = len(b.session.State.Guilds)
		st.HeartbeatLatencyMs = b.session.HeartbeatLatency().Milliseconds()
	}

	payload, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return b.redis.SetEx(ctx, stateKey, string(payload), stateTTL)
}
