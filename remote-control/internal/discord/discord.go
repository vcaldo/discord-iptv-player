package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

type Bot struct {
	session *discordgo.Session
	config  *config.Config
}

func NewBot(cfg *config.Config, nrApp *newrelic.Application) (*Bot, error) {
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
	}, nil
}

func (b *Bot) commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Responds with pong to check if the bot is online",
		},
		{
			Name:        "play",
			Description: "Play a TV channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "channel",
					Description: "The TV channel to play",
					Required:    true,
				},
			},
		},
		{
			Name:        "stop",
			Description: "Stop currently playing TV channel",
		},
	}
}

func (b *Bot) registerCommands(ctx context.Context, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:register-all-commands")
	defer txn.End()

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

	time.Sleep(100 * time.Millisecond)

	return nil
}

func (b *Bot) Start(ctx context.Context, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:bot-startup")

	b.session.AddHandler(b.messageCreate)

	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			cmdTxn := nrApp.StartTransaction("discord:incoming-command")
			cmdTxn.AddAttribute("command_type", i.ApplicationCommandData().Name)

			err := handleApplicationCommand(ctx, s, i, nrApp)

			if err != nil {
				cmdTxn.NoticeError(err)
				log.Printf("error handling command: %v", err)
			}

			cmdTxn.End()
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

	log.Println("ending discord:bot-startup transaction")
	txn.End()

	// create a separate heartbeat transaction that periodically sends a heartbeat to New Relic
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				heartbeatTxn := nrApp.StartTransaction("discord:bot-heartbeat")
				heartbeatTxn.AddAttribute("session_id", b.session.State.SessionID)
				heartbeatTxn.AddAttribute("session_state", b.session.State)
				heartbeatTxn.End()
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Println("bot started successfully, waiting for shutdown signal")

	// Wait for context cancellation
	<-ctx.Done()

	// Create a shutdown transactiontransaction
	shutdownTxn := nrApp.StartTransaction("discord:bot-shutdown")
	defer shutdownTxn.End()

	closeErr := b.session.Close()
	if closeErr != nil {
		shutdownTxn.NoticeError(closeErr)
	}

	// Force this transaction to be sent by adding a delay
	time.Sleep(100 * time.Millisecond)

	return closeErr
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}
}
