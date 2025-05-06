package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func (b *Bot) commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "tv",
			Description: "Play a TV channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "channel",
					Description: "The TV channel to play",
					Required:    true,
					MinValue:    &[]float64{0}[0],
					MaxValue:    float64(100),
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

	ctx = newrelic.NewContext(ctx, txn)

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
