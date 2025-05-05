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

func (b *Bot) Start(ctx context.Context, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:bot-startup")
	defer txn.End()

	ctx = newrelic.NewContext(ctx, txn)

	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			cmdTxn := nrApp.StartTransaction("discord:incoming-command")
			cmdTxn.AddAttribute("command_type", i.ApplicationCommandData().Name)

			cmdCtx := newrelic.NewContext(ctx, cmdTxn)
			err := handleApplicationCommand(cmdCtx, s, i, nrApp)
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
