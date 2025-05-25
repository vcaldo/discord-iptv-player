package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/html"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/m3u"
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
	case models.PlaylistCommand:
		return b.handlePlaylistCommand(ctx, s, i, config, nrApp)
	case models.RestartCommand:
		return b.handleRestartCommand(ctx, s, i, config, nrApp)
	case models.CatalogCommand:
		return b.handleCatalogCommand(ctx, s, i, config, nrApp)
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

	userVoiceState, err := getUserVoiceState(ctx, s, i, nrApp)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error getting user voice state: %v", err),
		})
		return msgErr
	}

	if userVoiceState == nil {
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "❌ You must be in a voice channel to use this command!",
		})
		return err
	}

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
	channel, err := b.redis.GetChannel(config.DiscordGuildID, b.getCurrentPlaylist(config), channelID)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error finding channel: %v", err),
		})
		return msgErr
	}

	txn.AddAttribute("channel_id", channelID)
	txn.AddAttribute("channel_name", channel.Name)

	remoteControlCommand := &models.RemoteControlCommand{
		Command:        models.PlayCommand,
		Title:          channel.Name,
		Url:            channel.Url,
		VoiceChannelID: userVoiceState.ChannelID,
	}

	err = b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error starting playback: %v", err),
		})
		return msgErr
	}

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
			Content: fmt.Sprintf("error starting YouTube playback: %v", err),
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

	userVoiceState, err := getUserVoiceState(ctx, s, i, nrApp)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error getting user voice state: %v", err),
		})
		return msgErr
	}

	if userVoiceState == nil {
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "❌ You must be in a voice channel to use this command!",
		})
		return err
	}

	remoteControlCommand := &models.RemoteControlCommand{
		Command: models.StopCommand,
	}

	err = b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error stopping playback: %v", err),
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
	playlist, err := b.redis.GetPlaylist(config.DiscordGuildID, b.getCurrentPlaylist(config))
	if err != nil {
		getPlaylistSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error retrieving playlist: %v", err),
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
	categories, err := b.redis.GetCategories(config.DiscordGuildID, b.getCurrentPlaylist(config))
	if err != nil {
		getSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error retrieving categories: %v", err),
		})
		return msgErr
	}
	getSegment.End()

	// Get category counts
	getStatsSegment := txn.StartSegment("get_category_stats")
	categoryStats, err := b.redis.GetCategoryStats(config.DiscordGuildID, b.getCurrentPlaylist(config))
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
	channels, err := b.redis.GetChannelsByCategory(config.DiscordGuildID, b.getCurrentPlaylist(config), category)
	if err != nil {
		getChannelsSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error retrieving channels for category '%s': %v", category, err),
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

func (b *Bot) handlePlaylistCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, cfg *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-playlist-command")
	defer txn.End()
	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var playlistName string
	if opt, ok := optionMap["name"]; ok {
		playlistName = opt.StringValue()
	} else {
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Please specify a playlist name",
		})
		return err
	}
	// Load playlist configurations to validate the playlist exists
	playlists, err := m3u.LoadPlaylistsConfig(cfg.PlaylistsConfigPath)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error loading playlist configurations: %v", err),
		})
		return msgErr
	}

	// Check if the playlist exists
	playlistExists := false
	for _, playlist := range playlists.Playlists {
		if playlist.Name == playlistName {
			playlistExists = true
			break
		}
	}

	if !playlistExists {
		availableNames := make([]string, 0, len(playlists.Playlists))
		for _, playlist := range playlists.Playlists {
			availableNames = append(availableNames, playlist.Name)
		}
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Playlist '%s' not found. Available playlists: %s", playlistName, strings.Join(availableNames, ", ")),
		})
		return err
	}

	// Set the current playlist in Redis
	err = b.redis.SetCurrentPlaylist(cfg.DiscordGuildID, playlistName)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error setting current playlist: %v", err),
		})
		return msgErr
	}

	txn.AddAttribute("playlist_name", playlistName)

	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("✅ Current playlist set to: **%s**", playlistName),
	})
	return err
}

func (b *Bot) handleRestartCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-restart-command")
	defer txn.End()
	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	userVoiceState, err := getUserVoiceState(ctx, s, i, nrApp)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error getting user voice state: %v", err),
		})
		return msgErr
	}

	if userVoiceState == nil {
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "❌ You must be in a voice channel to use this command!",
		})
		return err
	}

	// Send restart command
	remoteControlCommand := &models.RemoteControlCommand{
		Command: models.RestartCommand,
	}

	err = b.redis.RemoteControlCommand(remoteControlCommand)
	if err != nil {
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("error restarting bot: %v", err),
		})
		return msgErr
	}

	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{Content: "Bot is restarting... Please wait a moment."})
	return err
}

func (b *Bot) handleCatalogCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-catalog-command")
	defer txn.End()
	ctx = newrelic.NewContext(ctx, txn)

	txn.AddAttribute("user_id", i.Member.User.ID)
	txn.AddAttribute("user_name", i.Member.User.Username)

	// Get the current playlist name
	currentPlaylist := b.getCurrentPlaylist(config)
	txn.AddAttribute("playlist_name", currentPlaylist)

	// Get the playlist from Redis
	getPlaylistSegment := txn.StartSegment("get_playlist")
	playlist, err := b.redis.GetPlaylist(config.DiscordGuildID, currentPlaylist)
	if err != nil {
		getPlaylistSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error retrieving playlist: %v", err),
		})
		return msgErr
	}
	getPlaylistSegment.End()

	if len(playlist.Channels) == 0 {
		_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "No channels found in the current playlist.",
		})
		return err
	}
	// Generate HTML catalog
	generateSegment := txn.StartSegment("generate_html")
	generator := html.NewCatalogGenerator()

	// Organize channels by category
	categoryChannels := make(map[string][]models.TvChannel)
	categoriesSet := make(map[string]bool)

	for _, channel := range playlist.Channels {
		category := channel.Category
		if category == "" {
			category = "Uncategorized"
		}
		categoryChannels[category] = append(categoryChannels[category], channel)
		categoriesSet[category] = true
	}

	// Extract sorted categories
	categories := make([]string, 0, len(categoriesSet))
	for category := range categoriesSet {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	htmlContent := generator.GenerateHTML(playlist, categories, categoryChannels)
	generateSegment.End()

	filename := fmt.Sprintf("%s-catalog-%s.html", currentPlaylist, time.Now().Format("2006-01-02"))

	fileSizeBytes := len(htmlContent)
	fileSizeKB := float64(fileSizeBytes) / 1024
	log.Printf("Generated catalog HTML file: %s, Size: %d bytes (%.2f KB), Channels: %d, Categories: %d",
		filename, fileSizeBytes, fileSizeKB, len(playlist.Channels), len(categories))

	fileSegment := txn.StartSegment("create_temp_file")
	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		fileSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error creating catalog file: %v", err),
		})
		return msgErr
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.WriteString(tempFile, htmlContent)
	if err != nil {
		fileSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error writing catalog file: %v", err),
		})
		return msgErr
	}

	// Reset file pointer to beginning
	_, err = tempFile.Seek(0, 0)
	if err != nil {
		fileSegment.End()
		txn.NoticeError(err)
		_, msgErr := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("Error preparing catalog file: %v", err),
		})
		return msgErr
	}
	fileSegment.End()
	sendSegment := txn.StartSegment("send_file")
	_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("📒 Generated catalog for playlist **%s** with **%d** channels organized by **%d** categories.\nDownload this file and open it in your browser to view the interactive catalog.",
			currentPlaylist, len(playlist.Channels), len(categories)),
		Files: []*discordgo.File{
			{
				Name:   filename,
				Reader: tempFile,
			},
		},
	})
	sendSegment.End()
	if err != nil {
		txn.NoticeError(err)
		return err
	}

	txn.AddAttribute("channels_count", len(playlist.Channels))
	txn.AddAttribute("categories_count", len(categories))
	txn.AddAttribute("file_size_bytes", fileSizeBytes)
	txn.AddAttribute("file_size_kb", fileSizeKB)
	return nil
}
