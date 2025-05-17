import { Client } from "discord.js-selfbot-v13";
import { Streamer, StreamOptions, MediaUdp, streamLivestreamVideo } from "@dank074/discord-video-stream";
import { StreamProvider } from "./interfaces.js";
import { Logger } from "../../utils/logger.js";
import { ProcessManager } from "../../utils/process-manager.js";
import config from "../../config.js";
import newrelic from 'newrelic';

/**
 * Handles Discord streaming functionality
 */
export class StreamService implements StreamProvider {
    private streamer: Streamer;
    private logger: Logger;
    private processManager: ProcessManager;
    private currentStream: MediaUdp | null = null;

    constructor(client: Client) {
        this.streamer = new Streamer(client);
        this.logger = new Logger('StreamService');
        this.processManager = new ProcessManager();
    }

    /**
     * Joins a voice channel and creates a media stream
     */
    public async joinVoiceChannel(streamOpts: StreamOptions): Promise<MediaUdp> {
        return newrelic.startBackgroundTransaction('discord:join-voice-channel', async () => {
            newrelic.addCustomAttribute('guild_id', config.guildId);
            newrelic.addCustomAttribute('channel_id', config.videoChannelId);

            try {
                if (!this.streamer.client.isReady()) {
                    this.logger.error('Discord client not ready.');
                    throw new Error('Discord client not ready');
                }

                this.logger.log(`Joining voice channel in guild ${config.guildId}...`);

                await newrelic.startSegment('join-voice', true, async () => {
                    await this.streamer.joinVoice(config.guildId, config.videoChannelId, streamOpts);
                });

                this.logger.log('Creating stream...');

                const stream = await newrelic.startSegment('create-stream', true, async () => {
                    return await this.streamer.createStream(streamOpts);
                });

                this.logger.log('Successfully joined voice channel and created stream');
                this.currentStream = stream;
                return stream;
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error joining voice channel:', error);
                throw new Error(`Failed to join voice channel: ${error instanceof Error ? error.message : String(error)}`);
            }
        });
    }

    /**
     * Leaves the current voice channel
     */
    public leaveVoiceChannel(): void {
        newrelic.startBackgroundTransaction('discord:leave-voice-channel', () => {
            try {
                this.logger.log('Leaving voice channel...');

                newrelic.startSegment('leave-voice', true, () => {
                    this.streamer.leaveVoice();
                });

                this.currentStream = null;
                this.logger.log('Successfully left voice channel');
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error leaving voice channel:', error);

                try {
                    this.logger.log('Attempting force disconnect...');

                    newrelic.startSegment('force-disconnect', true, () => {
                        const guild = this.streamer.client.guilds.cache.get(config.guildId);
                        const voiceState = guild?.voiceStates?.cache.get(this.streamer.client.user?.id || '');
                        if (voiceState) voiceState.disconnect();
                    });
                    this.currentStream = null;
                } catch (innerError) {
                    newrelic.noticeError(innerError);
                    this.logger.error('Failed to force disconnect from voice channel:', innerError);
                }
            }
        });
    }

    /**
     * Checks if currently in a voice channel
     */
    public isInVoiceChannel(): boolean {
        try {
            const guild = this.streamer.client.guilds.cache.get(config.guildId);
            if (!guild) {
                this.logger.warn('Guild not found when checking voice state');
                return false;
            }

            const voiceState = guild.voiceStates?.cache.get(this.streamer.client.user?.id || '');
            return !!voiceState?.channelId;
        } catch (error) {
            newrelic.noticeError(error);
            this.logger.error('Error checking voice channel status:', error);
            return false;
        }
    }

    /**
     * Gets the current voice connection if available
     */
    public getCurrentVoiceConnection(): MediaUdp | null {
        if (this.currentStream && this.isInVoiceChannel()) {
            return this.currentStream;
        }
        return null;
    }

    /**
     * Starts streaming video content with retry mechanism
     */
    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        return newrelic.startBackgroundTransaction('discord:start-streaming', async () => {
            this.logger.log("Starting to stream video:", video);
            newrelic.addCustomAttribute('video_url', video);

            // Always inject a custom User-Agent for IPTV
            const userAgent = "VLC/3.0.18 LibVLC/3.0.18";
            // ffmpeg supports passing headers via URL options: https://ffmpeg.org/ffmpeg-protocols.html#http
            // This will work for most HTTP/HTTPS streams
            let videoWithUserAgent = video;
            if (video.startsWith('http://') || video.startsWith('https://')) {
                // Append user-agent as ffmpeg URL option
                videoWithUserAgent = `${video}?user_agent=${encodeURIComponent(userAgent)}`;
            }

            try {
                // Make sure we're still connected before starting the stream
                if (!this.isInVoiceChannel()) {
                    this.logger.warn('Not in voice channel when trying to start stream, attempting to reconnect...');
                    // We need to rejoin before streaming
                    const guild = this.streamer.client.guilds.cache.get(config.guildId);
                    if (!guild) {
                        throw new Error('Guild not found when trying to reconnect to voice channel');
                    }

                    await this.streamer.joinVoice(config.guildId, config.videoChannelId, {
                        width: config.width,
                        height: config.height,
                        fps: config.fps,
                        bitrateKbps: config.bitrateKbps,
                        maxBitrateKbps: config.maxBitrateKbps,
                        hardwareAcceleratedDecoding: config.hardwareAcceleratedDecoding,
                        videoCodec: config.videoCodec as any,
                        rtcpSenderReportEnabled: false,
                        h26xPreset: 'ultrafast',
                        minimizeLatency: false,
                        forceChacha20Encryption: true
                    });

                    // Get a new stream after rejoining
                    udpConn = await this.streamer.createStream({
                        width: config.width,
                        height: config.height,
                        fps: config.fps,
                        bitrateKbps: config.bitrateKbps,
                        maxBitrateKbps: config.maxBitrateKbps,
                        hardwareAcceleratedDecoding: config.hardwareAcceleratedDecoding,
                        videoCodec: config.videoCodec as any,
                        rtcpSenderReportEnabled: false,
                        h26xPreset: 'ultrafast',
                        minimizeLatency: false,
                        forceChacha20Encryption: true
                    });
                    this.currentStream = udpConn;
                }

                await newrelic.startSegment('setup-streaming', true, async () => {
                    udpConn.mediaConnection.setSpeaking(true);
                    udpConn.mediaConnection.setVideoStatus(true);
                });

                const res = await newrelic.startSegment('stream-video', true, async () => {
                    // Use the video URL with injected user-agent
                    return await streamLivestreamVideo(videoWithUserAgent, udpConn);
                });

                this.logger.log("Successfully finished streaming video:", res);

                // Check if we're still connected after streaming
                if (!this.isInVoiceChannel()) {
                    this.logger.warn('Voice channel connection lost after streaming, updating connection state');
                    this.currentStream = null;
                }

                return res;
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error streaming video:', error);

                try {
                    // Clean up connection state but don't disconnect
                    if (udpConn && udpConn.mediaConnection) {
                        udpConn.mediaConnection.setSpeaking(false);
                        udpConn.mediaConnection.setVideoStatus(false);
                    }
                } catch (cleanupError) {
                    newrelic.noticeError(cleanupError);
                    this.logger.error('Error cleaning up connection state after streaming error:', cleanupError);
                }

                // Check connection status
                if (this.isInVoiceChannel()) {
                    this.logger.info('Still connected to voice channel after streaming error');
                } else {
                    this.logger.warn('Voice channel connection lost after streaming error');
                    this.currentStream = null;
                }

                throw new Error(`Failed to stream video: ${error instanceof Error ? error.message : String(error)}`);
            }
        });
    }
}