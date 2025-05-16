package discord

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

type Bot struct {
	session *discordgo.Session
	config  *config.Config
	redis   *redis.Client
}

func NewBot(cfg *config.Config, redisClient *redis.Client, nrApp *newrelic.Application) (*Bot, error) {
	txn := nrApp.StartTransaction("discord:initialize-bot")
	defer txn.End()

	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		txn.NoticeError(err)
		return nil, err
	}

	session.Identify.Intents = discordgo.IntentGuilds |
		discordgo.IntentsGuildPresences |
		discordgo.IntentGuildMembers |
		discordgo.IntentGuildVoiceStates |
		discordgo.IntentMessageContent

	time.Sleep(100 * time.Millisecond)

	return &Bot{
		session: session,
		config:  cfg,
		redis:   redisClient,
	}, nil
}

func (b *Bot) Start(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:bot-startup")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			cmdTxn := nrApp.StartTransaction("discord:incoming-command")
			cmdTxn.AddAttribute("command_type", i.ApplicationCommandData().Name)

			cmdCtx := newrelic.NewContext(ctx, cmdTxn)
			err := b.handleApplicationCommand(cmdCtx, s, i, config, nrApp)
			if err != nil {
				cmdTxn.NoticeError(err)
				log.Printf("error handling command: %v", err)
			}

			cmdTxn.End()
		} else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
			autocompleteTxn := nrApp.StartTransaction("discord:autocomplete")
			autocompleteCtx := newrelic.NewContext(ctx, autocompleteTxn)

			err := b.handleAutocomplete(autocompleteCtx, s, i, config, nrApp)
			if err != nil {
				autocompleteTxn.NoticeError(err)
				log.Printf("error handling autocomplete: %v", err)
			}

			autocompleteTxn.End()
		}
	})

	if err := b.session.Open(); err != nil {
		txn.NoticeError(err)
		txn.End()
		return err
	}

	log.Println("bot connected successfully")

	if err := b.registerCommands(ctx, nrApp); err != nil {
		txn.NoticeError(err)
		log.Printf("error registering commands: %v", err)
	}

	log.Println("bot started successfully, waiting for shutdown signal")

	<-ctx.Done()

	shutdownTxn := nrApp.StartTransaction("discord:bot-shutdown")
	defer shutdownTxn.End()

	closeErr := b.session.Close()
	if closeErr != nil {
		shutdownTxn.NoticeError(closeErr)
	}

	return closeErr
}

// handleAutocomplete handles autocomplete interactions for Discord slash commands
func (b *Bot) handleAutocomplete(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, config *config.Config, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:handle-autocomplete")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	data := i.ApplicationCommandData()

	txn.AddAttribute("command_name", data.Name)

	// Handle different commands with autocomplete options
	switch data.Name {
	case models.ChannelsCommand:
		// Find the focused option (the one user is currently typing in)
		var focused *discordgo.ApplicationCommandInteractionDataOption
		var focusedValue string

		for _, opt := range data.Options {
			if opt.Focused {
				focused = opt
				focusedValue = opt.StringValue()
				break
			}
		}

		if focused == nil || focused.Name != "channel_name" {
			return nil // Not the option we're handling
		}

		txn.AddAttribute("input_value", focusedValue)

		// Use our new Redis Search functionality to get fuzzy matches
		searchSegment := txn.StartSegment("search_channels_by_name")
		channels, err := b.redis.SearchChannelsByName(config.DiscordGuildID, config.PlaylistName, focusedValue)
		if err != nil {
			searchSegment.End()
			txn.NoticeError(err)
			log.Printf("Error searching channels: %v", err)
			return err
		}
		searchSegment.End()

		// Format results for Discord autocomplete
		formatSegment := txn.StartSegment("format_results")
		var choices []*discordgo.ApplicationCommandOptionChoice

		// Add matching channels to choices (limit to 25 as per Discord's limit)
		maxChoices := 25
		for _, channel := range channels {
			// If we have 25 choices already, stop processing
			if len(choices) >= maxChoices {
				break
			}

			// Create a display name including the channel ID and category for extra context
			var displayName string
			if channel.Category != "" {
				displayName = fmt.Sprintf("%s - [%s] (%s)", channel.Name, channel.Category, channel.ID)
			} else {
				displayName = fmt.Sprintf("%s - (%s)", channel.Name, channel.ID)
			}

			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  displayName,
				Value: channel.ID, // Use the channel ID as the value
			})
		}
		formatSegment.End()

		txn.AddAttribute("choices_count", len(choices))

		// Respond with choices
		respondSegment := txn.StartSegment("send_response")
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
		respondSegment.End()

		if err != nil {
			txn.NoticeError(err)
			return err
		}

		return nil

	case models.ListChannelsinCategoryCommand:
		// Find the focused option (the one user is currently typing in)
		var focused *discordgo.ApplicationCommandInteractionDataOption
		var focusedValue string

		for _, opt := range data.Options {
			if opt.Focused {
				focused = opt
				focusedValue = opt.StringValue()
				break
			}
		}

		if focused == nil || focused.Name != "category" {
			return nil // Not the option we're handling
		}

		txn.AddAttribute("input_value", focusedValue)

		// Get categories from Redis
		getSegment := txn.StartSegment("get_categories")
		categories, err := b.redis.GetCategories(config.DiscordGuildID, config.PlaylistName)
		if err != nil {
			getSegment.End()
			txn.NoticeError(err)
			return err
		}
		getSegment.End()

		// Get category stats to show channel counts
		getStatsSegment := txn.StartSegment("get_category_stats")
		categoryStats, err := b.redis.GetCategoryStats(config.DiscordGuildID, config.PlaylistName)
		if err != nil {
			getStatsSegment.End()
			txn.NoticeError(err)
			log.Printf("Warning: couldn't retrieve category stats: %v", err)
			// Continue without stats if we couldn't get them
		}
		getStatsSegment.End()

		// Filter categories based on user input
		filterSegment := txn.StartSegment("filter_categories")
		var choices []*discordgo.ApplicationCommandOptionChoice

		// Lowercase the input for case-insensitive matching
		focusedValueLower := strings.ToLower(focusedValue)

		// Add matching categories to choices (limit to 25 as per Discord's limit)
		maxChoices := 25
		for _, category := range categories {
			// If we have 25 choices already, stop processing
			if len(choices) >= maxChoices {
				break
			}

			// Only add if it matches the user's input (empty input matches all)
			if focusedValue == "" || strings.Contains(strings.ToLower(category), focusedValueLower) {
				// Get channel count for this category
				count := 0
				if categoryStats != nil {
					if c, ok := categoryStats[category]; ok {
						count = c
					}
				}

				// Format the display name with channel count
				displayName := fmt.Sprintf("%s - (%d channels)", category, count)

				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  displayName,
					Value: category, // Still use the actual category name as the value
				})
			}
		}
		filterSegment.End()

		txn.AddAttribute("choices_count", len(choices))

		// Respond with choices
		respondSegment := txn.StartSegment("send_response")
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
		respondSegment.End()

		if err != nil {
			txn.NoticeError(err)
			return err
		}

		return nil
	}

	return nil
}
