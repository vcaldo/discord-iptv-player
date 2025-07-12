package discord

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

func (b *Bot) isBotAlone(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error {
	log.Printf("Running periodic task: checking if TV player bot is alone in voice channel")

	txn := nrApp.StartTransaction("discord:check-bot-alone")
	defer txn.End()

	// Get guild information
	getGuildSegment := txn.StartSegment("get_guild")
	guild, err := b.session.State.Guild(config.DiscordGuildID)
	if err != nil {
		getGuildSegment.End()
		txn.NoticeError(err)
		return err
	}
	getGuildSegment.End()

	// Find TV player bot's voice state (not this remote control bot)
	getBotVoiceStateSegment := txn.StartSegment("get_tv_player_bot_voice_state")

	// Get TV player bot ID from Redis
	tvPlayerBotID := b.redis.GetID("tv_player_bot_id")
	if tvPlayerBotID == "" {
		getBotVoiceStateSegment.End()
		log.Printf("TV player bot ID not found in Redis, skipping alone check")
		txn.AddAttribute("tv_player_bot_id_found", false)
		return nil
	}

	log.Printf("Checking voice state for TV player bot ID: %s", tvPlayerBotID)
	txn.AddAttribute("tv_player_bot_id", tvPlayerBotID)

	var botVoiceState *discordgo.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.UserID == tvPlayerBotID {
			botVoiceState = vs
			break
		}
	}
	getBotVoiceStateSegment.End()

	// If TV player bot is not in any voice channel, nothing to check
	if botVoiceState == nil || botVoiceState.ChannelID == "" {
		log.Printf("TV player bot is not in any voice channel, skipping alone check")
		txn.AddAttribute("tv_player_bot_in_voice_channel", false)
		return nil
	}

	txn.AddAttribute("tv_player_bot_in_voice_channel", true)
	txn.AddAttribute("tv_player_bot_voice_channel_id", botVoiceState.ChannelID)

	// Count human users in the same voice channel
	checkUsersSegment := txn.StartSegment("check_human_users")
	humanUserCount := 0
	totalUsersInChannel := 0

	for _, vs := range guild.VoiceStates {
		// Skip if user is not in the same voice channel as the bot
		if vs.ChannelID != botVoiceState.ChannelID {
			continue
		}

		totalUsersInChannel++

		// Skip the TV player bot itself
		if vs.UserID == tvPlayerBotID {
			continue
		}

		// Check if this user is a bot/app
		member, err := b.session.State.Member(config.DiscordGuildID, vs.UserID)
		if err != nil {
			// If we can't get member info, try to fetch it from Discord API
			member, err = b.session.GuildMember(config.DiscordGuildID, vs.UserID)
			if err != nil {
				// If we still can't get member info, skip this user but log the error
				txn.NoticeError(err)
				continue
			}
		}

		// If the user is not a bot, count them as a human user
		if member.User != nil && !member.User.Bot {
			humanUserCount++
		}
	}
	checkUsersSegment.End()

	txn.AddAttribute("total_users_in_channel", totalUsersInChannel)
	txn.AddAttribute("human_users_in_channel", humanUserCount)
	txn.AddAttribute("bot_is_alone", humanUserCount == 0)

	// If there are no human users in the voice channel (TV player bot is alone)
	if humanUserCount == 0 {
		log.Printf("TV player bot is alone in voice channel %s, considering disconnect", botVoiceState.ChannelID)

		// Send stop command to trigger bot disconnect
		stopSegment := txn.StartSegment("send_stop_command")
		remoteControlCommand := &models.RemoteControlCommand{
			Command: models.StopCommand,
		}

		err = b.redis.RemoteControlCommand(remoteControlCommand)
		if err != nil {
			stopSegment.End()
			txn.NoticeError(err)
			return err
		}
		stopSegment.End()

		log.Printf("Stop command sent due to TV player bot being alone in voice channel")
	} else {
		log.Printf("TV player bot is not alone in voice channel %s, %d human users present", botVoiceState.ChannelID, humanUserCount)
	}

	return nil
}
