package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

func (b *Bot) isBotAlone(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error {
	txn := newrelic.FromContext(ctx)
	if txn == nil && nrApp != nil {
		txn = nrApp.StartTransaction("discord:is-bot-alone")
		defer txn.End()
	}

	botUserID := b.redis.GetID("tv_player_bot_id")
	if botUserID == "" {
		return fmt.Errorf("tv player bot ID not found in Redis")
	}

	guildID := config.DiscordGuildID

	guild, err := b.session.State.Guild(guildID)
	if err != nil {
		return fmt.Errorf("error getting guild %s: %w", guildID, err)
	}

	channelMembers := make(map[string][]*discordgo.User)
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != "" {
			user, err := b.session.User(vs.UserID)
			if err != nil {
				log.Printf("warning: could not get user info for %s: %v", vs.UserID, err)
				continue
			}
			channelMembers[vs.ChannelID] = append(channelMembers[vs.ChannelID], user)
		}
	}

	// Check if bot is alone in any voice channel
	for channelID, members := range channelMembers {
		if len(members) == 1 && members[0].ID == botUserID {
			log.Printf("TV Service is alone in channel %s, no one is watching...", channelID)
			remoteCommand := &models.RemoteControlCommand{
				Command: models.StopCommand,
			}

			err := b.redis.RemoteControlCommand(remoteCommand)
			if err != nil {
				log.Printf("error sending disconnect command: %v", err)
			}
		}
	}

	return nil
}
