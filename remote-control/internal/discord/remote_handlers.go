package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

func (b *Bot) handleApplicationCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-application-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	// Immediately acknowledge the interaction to prevent timeout
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		txn.NoticeError(err)
		return fmt.Errorf("failed to acknowledge interaction: %w", err)
	}

	switch i.ApplicationCommandData().Name {
	case "tv":
		return b.handleTvCommand(ctx, s, i, config, nrApp)
	case "stop":
		return b.handleStopCommand(ctx, s, i, nrApp)
	case "search":
		return b.handleSearchCommand(ctx, s, i, config, nrApp)
	default:
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Unknown command",
		})
		return err
	}
}

func (b *Bot) handleTvCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-tv-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var channelID string
	if opt, ok := optionMap["channel"]; ok {
		channelID = fmt.Sprintf("%d", opt.IntValue())
	} else {
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a channel to play",
		})
		return err
	}

	channel, err := b.redis.GetChannel(config.DiscordGuildID, config.PlaylistName, channelID)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error finding channel: %v", err),
		})
		return msgErr
	}

	txn.AddAttribute("channel_id", channelID)
	txn.AddAttribute("channel_name", channel.Name)

	// Use followup message since we already acknowledged the interaction
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Starting to play channel: %s", channel.Name),
	})
	return err
}

func (b *Bot) handleStopCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-stop-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	// TODO: Implement actual TV player integration to stop playback

	// Use followup message since we already acknowledged the interaction
	_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: "Stopped playing TV channel",
	})
	return err
}

func (b *Bot) handleSearchCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-search-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var searchQuery string
	if opt, ok := optionMap["name"]; ok {
		searchQuery = opt.StringValue()
	} else {
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a name to search for",
		})
		return err
	}

	// Get the playlist
	playlist, err := b.redis.GetPlaylist(config.DiscordGuildID, config.PlaylistName)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error retrieving playlist: %v", err),
		})
		return msgErr
	}

	// Search for channels
	var matchingChannels []string
	searchQueryLower := strings.ToLower(searchQuery)

	for _, channel := range playlist.Channels {
		if strings.Contains(strings.ToLower(channel.Name), searchQueryLower) {
			matchingChannels = append(matchingChannels, fmt.Sprintf("• **%s** (ID: %s) - Category: %s",
				channel.Name, channel.ID, channel.Category))

			// Limit results to avoid message size limits
			if len(matchingChannels) >= 25 {
				break
			}
		}
	}

	var responseContent string
	if len(matchingChannels) > 0 {
		responseContent = fmt.Sprintf("Found %d channels matching '%s':\n\n%s",
			len(matchingChannels), searchQuery, strings.Join(matchingChannels, "\n"))

		// Add note if results were limited
		if len(matchingChannels) == 25 && len(playlist.Channels) > 25 {
			responseContent += "\n\n*Results limited to 25 channels. Use a more specific search term to narrow results.*"
		}
	} else {
		responseContent = fmt.Sprintf("No channels found matching '%s'", searchQuery)
	}

	txn.AddAttribute("search_query", searchQuery)
	txn.AddAttribute("results_count", len(matchingChannels))

	// Use followup message since we already acknowledged the interaction
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: responseContent,
	})
	return err
}
