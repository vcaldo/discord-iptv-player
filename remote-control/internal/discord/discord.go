package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

type Bot struct {
	session *discordgo.Session
	config  *config.Config
}

func NewBot(cfg *config.Config, nrApp *newrelic.Application) (*Bot, error) {
	txn := nrApp.StartTransaction("discord:new-bot")
	defer txn.End()

	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		txn.NoticeError(err)
		return nil, err
	}

	return &Bot{
		session: session,
		config:  cfg,
	}, nil
}

func (b *Bot) Start(ctx context.Context, nrApp *newrelic.Application) error {
	txn := nrApp.StartTransaction("discord:start-bot")
	defer txn.End()

	b.session.AddHandler(b.messageCreate)

	if err := b.session.Open(); err != nil {
		txn.NoticeError(err)
		return err
	}

	log.Println("bot started successfully")

	<-ctx.Done()
	closeErr := b.session.Close()
	if closeErr != nil {
		txn.NoticeError(closeErr)
	}
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
