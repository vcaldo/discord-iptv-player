package discord

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
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
	case models.TvCommand:
		return b.handleTvCommand(ctx, s, i, config, nrApp)
	case models.YoutubeCommand:
		return b.handleYoutubeCommand(ctx, s, i, config, nrApp)
	case models.StopCommand:
		return b.handleStopCommand(ctx, s, i, config, nrApp)
	case models.SearchCommand:
		return b.handleSearchCommand(ctx, s, i, config, nrApp)
	case models.CategoriesCommand:
		return b.handleCategoriesCommand(ctx, s, i, config, nrApp)
	case models.ListChannelsinCategoryCommand:
		return b.handleListChannelsInCategoryCommand(ctx, s, i, config, nrApp)
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

	remoteControlCommand := &models.RemoteControlCommand{
		Command:       models.PlayCommand,
		Title:         channel.Name,
		Url:           channel.Url,
		XcodeUsername: channel.XcodeUsername,
		XcodePassword: channel.XcodePassword,
	}

	err = b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error starting playback: %v", err),
		})
		return msgErr
	}

	// Use followup message since we already acknowledged the interaction
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Playing channel: %s - %s", channel.ID, channel.Name),
	})
	return err
}

func (b *Bot) handleYoutubeCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-youtube-command")
	defer txn.End()
	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var youtubeURL string
	if opt, ok := optionMap["url"]; ok {
		youtubeURL = opt.StringValue()
	} else {
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a YouTube URL to play",
		})
		return err
	}

	txn.AddAttribute("youtube_url", youtubeURL)

	titleSegment := txn.StartSegment("youtube_title_extraction")
	title, err := getYouTubeTitle(youtubeURL)
	if err != nil {
		txn.NoticeError(err)
		title = "YouTube Video"
		txn.AddAttribute("title_extraction_failed", true)
	} else {
		txn.AddAttribute("video_title", title)
	}
	titleSegment.End()

	remoteControlCommand := &models.RemoteControlCommand{
		Command: models.PlayCommand,
		Title:   title,
		Url:     youtubeURL,
	}

	err = b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error starting YouTube playback: %v", err),
		})
		return msgErr
	}

	// Use followup message since we already acknowledged the interaction
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("Playing YouTube video: %s (%s)", title, youtubeURL),
	})
	return err
}

func (b *Bot) handleStopCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-stop-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	remoteControlCommand := &models.RemoteControlCommand{
		Command: models.StopCommand,
	}

	err := b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error stopping playback: %v", err),
		})
		return msgErr
	}

	// Use followup message since we already acknowledged the interaction
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
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

	parseSegment := txn.StartSegment("parse_options")
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var searchQuery string
	if opt, ok := optionMap["name"]; ok {
		searchQuery = opt.StringValue()
	} else {
		parseSegment.End()
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a name to search for",
		})
		return err
	}
	parseSegment.End()

	// Get the playlist
	getPlaylistSegment := txn.StartSegment("get_playlist")
	playlist, err := b.redis.GetPlaylist(config.DiscordGuildID, config.PlaylistName)
	if err != nil {
		getPlaylistSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error retrieving playlist: %v", err),
		})
		return msgErr
	}
	getPlaylistSegment.End()

	// Search for channels
	searchSegment := txn.StartSegment("search_channels")
	var matchingChannels []string
	searchQueryLower := strings.ToLower(searchQuery)

	// Find the maximum length of channel ID for proper alignment
	prepareSegment := txn.StartSegment("max_id_length")
	maxIDLength := 0
	maxCategoryLength := 0
	for _, channel := range playlist.Channels {
		if strings.Contains(strings.ToLower(channel.Name), searchQueryLower) {
			if len(channel.ID) > maxIDLength {
				maxIDLength = len(channel.ID)
			}
			if len(channel.Category) > maxCategoryLength {
				maxCategoryLength = len(channel.Category)
			}
		}
	}
	prepareSegment.End()

	formatString := fmt.Sprintf("%%-%ds - %%-%ds - %%s", maxIDLength, maxCategoryLength)

	for _, channel := range playlist.Channels {
		if strings.Contains(strings.ToLower(channel.Name), searchQueryLower) {
			matchingChannels = append(matchingChannels, fmt.Sprintf(formatString,
				channel.ID, channel.Category, channel.Name))
		}
	}
	searchSegment.End()

	txn.AddAttribute("search_query", searchQuery)
	txn.AddAttribute("results_count", len(matchingChannels))

	if len(matchingChannels) == 0 {
		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("🙅‍♀️  No channels found matching `%s`", searchQuery),
		})
		return err
	}

	// Split results into batches to stay within Discord's 2000 character limit
	formatSegment := txn.StartSegment("format_results")
	header := fmt.Sprintf("🔎  Found **%d** channels matching `%s`:\n\n", len(matchingChannels), searchQuery)

	// Send results in batches of approximately 1700 characters (leaving room for headers)
	const maxBatchSize = 1700
	var currentBatch strings.Builder
	currentBatch.WriteString(header)
	currentBatch.WriteString("```\n") // Start code block
	formatSegment.End()

	sendSegment := txn.StartSegment("send_results")
	for idx, channel := range matchingChannels {
		// If adding this channel would exceed the batch size, send the current batch
		if currentBatch.Len() > 0 && currentBatch.Len()+len(channel)+10 > maxBatchSize { // Add 10 for the code block syntax
			// Close the code block
			currentBatch.WriteString("```")

			// Send the current batch
			_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: currentBatch.String(),
			})
			if err != nil {
				txn.NoticeError(err)
				sendSegment.End()
				return err
			}

			// Start a new batch
			currentBatch.Reset()

			// If this isn't the first channel, add a continuation header
			if idx > 0 {
				currentBatch.WriteString(fmt.Sprintf("Results for `%s` (continued):\n\n", searchQuery))
			}

			// Start new code block
			currentBatch.WriteString("```\n")
		}

		// Add the channel to the current batch
		currentBatch.WriteString(channel)
		currentBatch.WriteString("\n")
	}

	// Send any remaining channels in the final batch
	if currentBatch.Len() > 0 {
		// Close the code block
		currentBatch.WriteString("```")

		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: currentBatch.String(),
		})
		if err != nil {
			txn.NoticeError(err)
		}
	}
	sendSegment.End()

	return err
}

func (b *Bot) handleCategoriesCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-categories-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	// Get categories directly from Redis
	getSegment := txn.StartSegment("get_categories")
	categories, err := b.redis.GetCategories(config.DiscordGuildID, config.PlaylistName)
	if err != nil {
		getSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error retrieving categories: %v", err),
		})
		return msgErr
	}
	getSegment.End()

	// Get category counts
	getStatsSegment := txn.StartSegment("get_category_stats")
	categoryStats, err := b.redis.GetCategoryStats(config.DiscordGuildID, config.PlaylistName)
	if err != nil {
		getStatsSegment.End()
		txn.NoticeError(err)
		log.Printf("Warning: couldn't retrieve category stats: %v", err)
		// We'll continue without stats if we couldn't get them
	}
	getStatsSegment.End()

	txn.AddAttribute("categories_count", len(categories))

	if len(categories) == 0 {
		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "No categories found in the playlist.",
		})
		return err
	}

	// Format the response
	formatSegment := txn.StartSegment("format_results")
	header := fmt.Sprintf("📖  Found **%d** categories:\n\n", len(categories))

	// Sort categories alphabetically
	sort.Strings(categories)

	// Find the maximum length of category names for proper alignment
	maxCategoryLength := 0
	for _, category := range categories {
		if len(category) > maxCategoryLength {
			maxCategoryLength = len(category)
		}
	}

	formatString := fmt.Sprintf("%%-%ds  %%5d channels", maxCategoryLength)

	// Send results in batches to stay within Discord's 2000 character limit
	const maxBatchSize = 1700
	var currentBatch strings.Builder
	currentBatch.WriteString(header)
	currentBatch.WriteString("```\n") // Start code block
	formatSegment.End()

	sendSegment := txn.StartSegment("send_results")
	for idx, category := range categories {
		// Get the count for this category, default to 0 if not found
		count := 0
		if categoryStats != nil {
			if c, ok := categoryStats[category]; ok {
				count = c
			}
		}

		// Format the category entry with count
		categoryEntry := fmt.Sprintf(formatString, category, count)

		// If adding this category would exceed the batch size, send the current batch
		if currentBatch.Len() > 0 && currentBatch.Len()+len(categoryEntry)+10 > maxBatchSize {
			// Close the code block
			currentBatch.WriteString("```")

			// Send the current batch
			_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: currentBatch.String(),
			})
			if err != nil {
				txn.NoticeError(err)
				sendSegment.End()
				return err
			}

			// Start a new batch
			currentBatch.Reset()

			// If this isn't the first category, add a continuation header
			if idx > 0 {
				currentBatch.WriteString("Categories (continued):\n\n")
			}

			// Start new code block
			currentBatch.WriteString("```\n")
		}

		// Add the category to the current batch
		currentBatch.WriteString(categoryEntry)
		currentBatch.WriteString("\n")
	}

	// Send any remaining categories in the final batch
	if currentBatch.Len() > 0 {
		// Close the code block
		currentBatch.WriteString("```")

		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: currentBatch.String(),
		})
		if err != nil {
			txn.NoticeError(err)
		}
	}
	sendSegment.End()

	return err
}

func (b *Bot) handleListChannelsInCategoryCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-search-in-category-command")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	parseSegment := txn.StartSegment("parse_options")
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var category string
	if opt, ok := optionMap["category"]; ok {
		category = opt.StringValue()
	} else {
		parseSegment.End()
		// Use followup message since we already acknowledged the interaction
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a category to search in",
		})
		return err
	}
	parseSegment.End()

	// Get channels for the specified category
	getChannelsSegment := txn.StartSegment("get_channels_by_category")
	channels, err := b.redis.GetChannelsByCategory(config.DiscordGuildID, config.PlaylistName, category)
	if err != nil {
		getChannelsSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error retrieving channels for category '%s': %v", category, err),
		})
		return msgErr
	}
	getChannelsSegment.End()

	txn.AddAttribute("category", category)
	txn.AddAttribute("channels_count", len(channels))

	if len(channels) == 0 {
		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("📂  No channels found in category `%s`", category),
		})
		return err
	}

	// Format the results
	formatSegment := txn.StartSegment("format_results")
	header := fmt.Sprintf("📂  Found **%d** channels in category `%s`:\n\n", len(channels), category)

	// Sort channels by name for better readability
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})

	// Find maximum ID length for proper alignment
	maxIDLength := 0
	for _, channel := range channels {
		if len(channel.ID) > maxIDLength {
			maxIDLength = len(channel.ID)
		}
	}

	formatString := fmt.Sprintf("%%-%ds - %%s", maxIDLength)

	// Send results in batches to stay within Discord's 2000 character limit
	const maxBatchSize = 1700
	var currentBatch strings.Builder
	currentBatch.WriteString(header)
	currentBatch.WriteString("```\n") // Start code block
	formatSegment.End()

	sendSegment := txn.StartSegment("send_results")
	for idx, channel := range channels {
		channelEntry := fmt.Sprintf(formatString, channel.ID, channel.Name)

		// If adding this channel would exceed the batch size, send the current batch
		if currentBatch.Len() > 0 && currentBatch.Len()+len(channelEntry)+10 > maxBatchSize {
			// Close the code block
			currentBatch.WriteString("```")

			// Send the current batch
			_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: currentBatch.String(),
			})
			if err != nil {
				txn.NoticeError(err)
				sendSegment.End()
				return err
			}

			// Start a new batch
			currentBatch.Reset()

			// If this isn't the first channel, add a continuation header
			if idx > 0 {
				currentBatch.WriteString(fmt.Sprintf("Channels in category `%s` (continued):\n\n", category))
			}

			// Start new code block
			currentBatch.WriteString("```\n")
		}

		// Add the channel to the current batch
		currentBatch.WriteString(channelEntry)
		currentBatch.WriteString("\n")
	}

	// Send any remaining channels in the final batch
	if currentBatch.Len() > 0 {
		// Close the code block
		currentBatch.WriteString("```")

		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: currentBatch.String(),
		})
		if err != nil {
			txn.NoticeError(err)
		}
	}
	sendSegment.End()

	return err
}
