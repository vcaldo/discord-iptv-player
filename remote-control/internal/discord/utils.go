package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/kkdai/youtube/v2"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func DeregisterCommands(ctx context.Context, s *discordgo.Session) {
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

func getYouTubeTitle(url string) (string, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(url)
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	return video.Title, nil
}

func getUserVoiceState(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, nrApp *newrelic.Application) (*discordgo.VoiceState, error) {
	txn := newrelic.FromContext(ctx)
	if txn == nil && nrApp != nil {
		txn = nrApp.StartTransaction("discord:get-user-voicestate")
		defer txn.End()
	}

	segment := txn.StartSegment("findUserVoiceState")
	defer segment.End()

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		txn.NoticeError(err)
		return nil, err
	}

	var userVoiceState *discordgo.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.UserID == i.Member.User.ID {
			userVoiceState = vs
			break
		}
	}

	return userVoiceState, nil
}
