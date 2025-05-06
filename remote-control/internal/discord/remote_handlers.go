package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

func (b *Bot) handleApplicationCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-application-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	switch i.ApplicationCommandData().Name {
	case "tv":
		return b.handleTvCommand(ctx, s, i, config, nrApp)
	case "stop":
		return b.handleStopCommand(ctx, s, i, nrApp)
	default:
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command",
			},
		})
	}
}

func (b *Bot) handleTvCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-tv-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var channelID string
	if opt, ok := optionMap["channel"]; ok {
		channelID = opt.StringValue()
	} else {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please specify a channel to play",
			},
		})
	}

	channel, err := b.redis.GetChannel(config.DiscordGuildID, config.PlaylistName, channelID)
	if err != nil {
		txn.NoticeError(err)
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Starting to play channel: %s", channel.Name),
		},
	})
}

func (b *Bot) handleStopCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-stop-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	// TODO: Implement actual TV player integration to stop playback

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Stopped playing TV channel",
		},
	})
}
