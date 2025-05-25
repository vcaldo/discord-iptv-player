package discord

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/kkdai/youtube/v2"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
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

func isAnyoneWatching(ctx context.Context, s *discordgo.Session, v *discordgo.VoiceStateUpdate, redisClient *redis.Client, nrApp *newrelic.Application) bool {
	txn := newrelic.FromContext(ctx)
	if txn == nil && nrApp != nil {
		txn = nrApp.StartTransaction("discord:is-anyone-watching")
		defer txn.End()
	}
	segment := txn.StartSegment("isAnyoneWatching")
	defer segment.End()

	botUserID := redisClient.GetID("tv_player_bot_id")
	if botUserID == "" {
		log.Printf("warning: tv player bot ID not found in Redis")
		return true
	}

	channelIDToCheck := v.ChannelID
	if v.BeforeUpdate != nil && v.BeforeUpdate.ChannelID != "" {
		channelIDToCheck = v.BeforeUpdate.ChannelID
	}

	voiceChannelMembers, err := getChannelMembers(ctx, s, v.GuildID, channelIDToCheck, nrApp)
	if err != nil {
		log.Printf("error getting channel members: %v", err)
		return true
	}
	if len(voiceChannelMembers) == 1 {
		log.Printf("only one member in channel %s", channelIDToCheck)
		if voiceChannelMembers[0].ID == botUserID {
			log.Printf("TV Service is alone in the channel, leaving...")
			return false
		}
	}
	return true
}

func getChannelMembers(ctx context.Context, s *discordgo.Session, guildID string, channelID string, nrApp *newrelic.Application) ([]*discordgo.User, error) {
	txn := newrelic.FromContext(ctx)
	if txn == nil && nrApp != nil {
		txn = nrApp.StartTransaction("discord:get-channel-members")
		defer txn.End()
	}

	segment := txn.StartSegment("findChannelMembers")
	defer segment.End()

	guild, err := s.State.Guild(guildID)
	if err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	var members []*discordgo.User
	for _, vs := range guild.VoiceStates {
		log.Printf("user: %s", vs.UserID)
		if vs.ChannelID == channelID {
			user, err := s.User(vs.UserID)
			if err != nil {
				log.Printf("warning: could not get user info for %s: %v", vs.UserID, err)
				continue
			}
			members = append(members, user)
		}
	}

	if txn != nil {
		txn.AddAttribute("channel_id", channelID)
		txn.AddAttribute("member_count", len(members))
	}

	return members, nil
}
