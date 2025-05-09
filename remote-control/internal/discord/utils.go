package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/kkdai/youtube/v2"
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

// getYouTubeTitle extracts the title from a YouTube URL
func getYouTubeTitle(url string) (string, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(url)
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	return video.Title, nil
}
