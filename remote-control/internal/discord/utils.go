package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
)

func deregisterCommands(ctx context.Context, s *discordgo.Session) {
	commands, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		log.Println(err)
		return
	}

	for _, command := range commands {
		err = s.ApplicationCommandDelete(s.State.User.ID, "", command.ID)
		if err != nil {
			log.Println(err)
		}
		log.Printf("command deregistered: %s\n", command.Name)
	}

}
