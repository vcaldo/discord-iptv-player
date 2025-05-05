package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func handleApplicationCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-application-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	switch i.ApplicationCommandData().Name {
	case "tv":
		return handleTvCommand(ctx, s, i, nrApp)
	case "stop":
		return handleStopCommand(ctx, s, i, nrApp)
	default:
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command",
			},
		})
	}
}

func handleTvCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-tv-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var channelName string
	if opt, ok := optionMap["channel"]; ok {
		channelName = opt.StringValue()
	} else {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please specify a channel to play",
			},
		})
	}

	// TODO: Implement actual TV player integration

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Starting to play channel: %s", channelName),
		},
	})
}

func handleStopCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-stop-command")
	defer txn.End()

	// Associate context with transaction
	ctx = newrelic.NewContext(ctx, txn)

	// TODO: Implement actual TV player integration to stop playback

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Stopped playing TV channel",
		},
	})
}
