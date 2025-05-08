import { Client } from "discord.js-selfbot-v13";
import { Streamer, StreamOptions, MediaUdp, streamLivestreamVideo } from "@dank074/discord-video-stream";
import { StreamProvider } from "./interfaces.js";
import { Logger } from "../../utils/logger.js";
import { ProcessManager } from "../../utils/process-manager.js";
import config from "../../config.js";
import newrelic from 'newrelic';

// Maximum number of retry attempts for streaming operations
const MAX_RETRY_ATTEMPTS = 3;
// Delay between retry attempts in milliseconds
const RETRY_DELAY_MS = 2000;

/**
 * Handles Discord streaming functionality
 */
export class StreamService implements StreamProvider {
    private streamer: Streamer;
    private logger: Logger;
    private processManager: ProcessManager;

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
            let attempts = 0;
            newrelic.addCustomAttribute('guild_id', config.guildId);
            newrelic.addCustomAttribute('channel_id', config.videoChannelId);

            while (attempts < MAX_RETRY_ATTEMPTS) {
                try {
                    // Check if client is ready before attempting to join
                    if (!this.streamer.client.isReady()) {
                        this.logger.log('Discord client not ready. Waiting before joining voice channel...');
                        await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                        attempts++;
                        continue;
                    }

                    this.logger.log(`Joining voice channel in guild ${config.guildId}...`);

                    // Create a segment for joining voice
                    await newrelic.startSegment('join-voice', true, async () => {
                        await this.streamer.joinVoice(config.guildId, config.videoChannelId, streamOpts);
                    });

                    this.logger.log('Creating stream...');

                    // Create a segment for creating the stream
                    const stream = await newrelic.startSegment('create-stream', true, async () => {
                        return await this.streamer.createStream(streamOpts);
                    });

                    this.logger.log('Successfully joined voice channel and created stream');
                    return stream;
                } catch (error) {
                    attempts++;
                    newrelic.noticeError(error);
                    newrelic.addCustomAttribute('attempt_number', attempts);

                    this.logger.error(`Error joining voice channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                    if (attempts >= MAX_RETRY_ATTEMPTS) {
                        this.logger.error('Failed to join voice channel after multiple attempts.');
                        throw new Error(`Failed to join voice channel: ${error instanceof Error ? error.message : String(error)}`);
                    }

                    // Wait before retry with exponential backoff
                    const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                    newrelic.addCustomAttribute('retry_delay_ms', delayMs);

                    this.logger.log(`Retrying in ${delayMs}ms...`);
                    await new Promise(resolve => setTimeout(resolve, delayMs));
                }
            }

            // This should not be reached due to the throw in the catch block,
            // but TypeScript requires a return statement
            throw new Error('Failed to join voice channel after multiple attempts');
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

                this.logger.log('Successfully left voice channel');
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error leaving voice channel:', error);

                // Try alternative cleanup if available
                try {
                    this.logger.log('Attempting force disconnect...');

                    // Create a segment for force disconnect
                    newrelic.startSegment('force-disconnect', true, () => {
                        // Force disconnect if the normal method fails
                        const guild = this.streamer.client.guilds.cache.get(config.guildId);
                        const voiceState = guild?.voiceStates?.cache.get(this.streamer.client.user?.id || '');
                        if (voiceState) voiceState.disconnect();
                    });
                } catch (innerError) {
                    newrelic.noticeError(innerError);
                    this.logger.error('Failed to force disconnect from voice channel:', innerError);
                }
            }
        });
    }

    /**
     * Starts streaming video content with retry mechanism
     */
    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        return newrelic.startBackgroundTransaction('discord:start-streaming', async () => {
            this.logger.log("Starting to stream video:", video);
            let attempts = 0;
            let success = false;

            newrelic.addCustomAttribute('video_url', video);

            while (attempts < MAX_RETRY_ATTEMPTS && !success) {
                try {
                    // Create a segment for setting up streaming
                    await newrelic.startSegment('setup-streaming', true, async () => {
                        udpConn.mediaConnection.setSpeaking(true);
                        udpConn.mediaConnection.setVideoStatus(true);
                    });

                    // Create a segment for the actual streaming
                    const res = await newrelic.startSegment('stream-video', true, async () => {
                        return await streamLivestreamVideo(video, udpConn);
                    });

                    this.logger.log("Successfully finished streaming video:", res);
                    success = true;
                    return res;
                } catch (error) {
                    attempts++;
                    newrelic.noticeError(error);
                    newrelic.addCustomAttribute('attempt_number', attempts);

                    this.logger.error(`Error streaming video (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                    if (attempts >= MAX_RETRY_ATTEMPTS) {
                        this.logger.error('Failed to stream video after multiple attempts');
                        throw new Error(`Failed to stream video: ${error instanceof Error ? error.message : String(error)}`);
                    }

                    // Wait before retry with exponential backoff
                    const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                    newrelic.addCustomAttribute('retry_delay_ms', delayMs);

                    this.logger.log(`Retrying stream in ${delayMs}ms...`);
                    await new Promise(resolve => setTimeout(resolve, delayMs));

                    // Reset connection state before retry
                    try {
                        newrelic.startSegment('reset-connection', true, () => {
                            udpConn.mediaConnection.setSpeaking(false);
                            udpConn.mediaConnection.setVideoStatus(false);
                        });
                        // Short pause before restarting
                        await new Promise(resolve => setTimeout(resolve, 500));
                    } catch (resetError) {
                        newrelic.noticeError(resetError);
                        this.logger.error('Error resetting connection state:', resetError);
                    }
                } finally {
                    if (!success) {
                        // Only reset if we're giving up
                        try {
                            udpConn.mediaConnection.setSpeaking(false);
                            udpConn.mediaConnection.setVideoStatus(false);
                        } catch (finalError) {
                            newrelic.noticeError(finalError);
                            this.logger.error('Error in finally block while cleaning up connection:', finalError);
                        }
                    }
                }
            }

            // This should not be reached due to the throw in the catch block
            throw new Error('Failed to stream video after multiple attempts');
        });
    }
}