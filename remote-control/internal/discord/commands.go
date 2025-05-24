package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

func (b *Bot) commands() []*discordgo.ApplicationCommand {
	// Get current playlist name from Redis
	currentPlaylistName, err := b.redis.GetCurrentPlaylist(b.config.DiscordGuildID)
	if err != nil {
		log.Printf("error getting current playlist from Redis: %v", err)
		// Fall back to a default value
		currentPlaylistName = "default"
	}

	playlist, err := b.redis.GetPlaylist(b.config.DiscordGuildID, currentPlaylistName)
	if err != nil {
		log.Printf("error getting playlist '%s' from Redis: %v", currentPlaylistName, err)
	}

	playlistLen := float64(50000)

	if err != nil {
		log.Printf("warning: could not get playlist to determine length: %v", err)
	} else {
		playlistLen = float64(len(playlist.Channels))
		log.Printf("setting tv command max channel to %d based on playlist '%s' length", int64(playlistLen), currentPlaylistName)
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        models.TvCommand,
			Description: "Play a TV channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "channel",
					Description: "The TV channel to play",
					Required:    true,
					MinValue:    &[]float64{1}[0],
					MaxValue:    playlistLen,
				},
			},
		},
		{
			Name:        models.YoutubeCommand,
			Description: "Play a YouTube video",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "The YouTube video URL to play",
					Required:    true,
				},
			},
		},
		{
			Name:        models.RestartCommand,
			Description: "Restart the bot",
		},
		{
			Name:        models.StopCommand,
			Description: "Stop currently playing TV channel",
		},
		{
			Name:        models.CategoriesCommand,
			Description: "Get the list of categories",
		},
		{
			Name:        models.ListChannelsinCategoryCommand,
			Description: "List channels in a category",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "category",
					Description:  "The category to search in",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        models.SearchCommand,
			Description: "Search for TV channels by name", Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "The channel name to search for",
					Required:    true,
				},
			},
		},
		{
			Name:        models.PlaylistCommand,
			Description: "Switch to a different playlist",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "name",
					Description:  "The playlist to switch to",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
	}
}

func (b *Bot) registerCommands(ctx context.Context, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:register-all-commands")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	// Uncomment the following line to deregister commands
	// deregisterCommands(ctx, b.session)

	log.Println("registering commands...")

	appID := b.session.State.User.ID

	for _, cmd := range b.commands() {
		segment := txn.StartSegment("register-command:" + cmd.Name)

		_, err := b.session.ApplicationCommandCreate(appID, "", cmd)
		if err != nil {
			segment.End()
			txn.NoticeError(err)
			log.Printf("error registering '%s' command: %v", cmd.Name, err)
			return err
		}
		log.Printf("successfully registered command: %s", cmd.Name)
		segment.End()
	}

	return nil
}
