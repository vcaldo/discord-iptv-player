package discord

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func DeregisterAllCommands(s *discordgo.Session, appID string) error {
	if err := DeregisterCommands(s, appID, ""); err != nil {
		return fmt.Errorf("failed to deregister global commands: %w", err)
	}

	var lastID string
	const limit = 100
	for {
		guilds, err := s.UserGuilds(limit, lastID, "", false)
		if err != nil {
			return fmt.Errorf("failed to fetch guilds: %w", err)
		}

		if len(guilds) == 0 {
			break
		}

		for _, guild := range guilds {
			if err := DeregisterCommands(s, appID, guild.ID); err != nil {
				return fmt.Errorf("failed to deregister commands for guild %s: %w", guild.ID, err)
			}
		}

		if len(guilds) < limit {
			break
		}
		lastID = guilds[len(guilds)-1].ID
	}

	log.Println("Successfully deregistered all commands")
	return nil
}

func DeregisterCommands(s *discordgo.Session, appID string, guildID string) error {
	commands, err := s.ApplicationCommands(appID, guildID)
	if err != nil {
		return fmt.Errorf("failed to fetch commands: %w", err)
	}

	if len(commands) == 0 {
		log.Printf("No commands to deregister for guild ID: %s\n", guildID)
		return nil
	}

	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(appID, guildID, cmd.ID)
		if err != nil {
			return fmt.Errorf("failed to delete command '%s': %w", cmd.Name, err)
		}
		log.Printf("Deregistered command: %s (Guild ID: %s)\n", cmd.Name, guildID)
		time.Sleep(500 * time.Millisecond) // Respect rate limits
	}

	return nil
}
