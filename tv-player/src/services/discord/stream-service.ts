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

    // Retry configuration for streaming attempts
    private readonly MAX_RETRY_ATTEMPTS = 3;
    private readonly RETRY_DELAY_MS = 2000;

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
     * Starts streaming video content with automatic retry mechanism
     */
    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        return newrelic.startBackgroundTransaction('discord:start-streaming', async () => {
            this.logger.log("Starting to stream video:", video);
            newrelic.addCustomAttribute('video_url', video);

            let retryCount = 0;
            let lastError: any = null;

            // Main retry loop
            while (retryCount <= this.MAX_RETRY_ATTEMPTS) {
                try {
                    if (retryCount > 0) {
                        this.logger.log(`Retry attempt ${retryCount}/${this.MAX_RETRY_ATTEMPTS} for streaming video...`);
                        newrelic.addCustomAttribute('retry_count', retryCount);

                        // Wait before retrying
                        await new Promise(resolve => setTimeout(resolve, this.RETRY_DELAY_MS));

                        // Kill any existing ffmpeg processes before retry
                        await this.processManager.killFfmpegProcesses().catch(err => {
                            this.logger.error('Error killing ffmpeg processes before retry:', err);
                        });
                    }

                    // First, test the stream before attempting to play it
                    this.logger.log("Testing stream availability before playback...");
                    const streamTest = await this.processManager.testVideoStream(video, 10000);

                    if (!streamTest.success) {
                        // Log diagnostic information about why the stream test failed
                        this.logger.error("Stream test failed:", {
                            url: video,
                            error: streamTest.details.error || "Unknown error"
                        });

                        lastError = new Error(`Cannot play video: Stream test failed - ${streamTest.details.error || "Unknown error"}`);
                        retryCount++;
                        continue; // Skip to next retry attempt
                    }

                    // Log stream information
                    this.logger.info("Stream test successful:", {
                        format: streamTest.details.format,
                        resolution: streamTest.details.resolution,
                        codec: streamTest.details.codecInfo,
                        duration: streamTest.details.duration
                    });

                    // Make sure we're still connected before starting the stream
                    if (!this.isInVoiceChannel()) {
                        this.logger.warn('Not in voice channel when trying to start stream, attempting to reconnect...');
                        // We need to rejoin before streaming
                        const guild = this.streamer.client.guilds.cache.get(config.guildId);
                        if (!guild) {
                            lastError = new Error('Guild not found when trying to reconnect to voice channel');
                            retryCount++;
                            continue; // Skip to next retry attempt
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
                        // Ensure voice status is set properly
                        udpConn.mediaConnection.setSpeaking(true);
                        udpConn.mediaConnection.setVideoStatus(true);
                    });

                    try {
                        const res = await newrelic.startSegment('stream-video', true, async () => {
                            // We'll use the streamLivestreamVideo function but make sure to handle any disconnections
                            this.logger.log("Starting ffmpeg process for video:", video);

                            // Wrap streamLivestreamVideo with additional error handling for diagnostics
                            try {
                                return await streamLivestreamVideo(video, udpConn);
                            } catch (ffmpegError) {
                                // Enhanced error diagnostics
                                if (typeof ffmpegError === 'string' && ffmpegError.includes('ffmpeg exited with code')) {
                                    // Let's log more detailed error information
                                    this.logger.error('FFMPEG Error Details:', {
                                        error: ffmpegError,
                                        videoUrl: video,
                                        streamConfig: {
                                            width: config.width,
                                            height: config.height,
                                            fps: config.fps,
                                            bitrateKbps: config.bitrateKbps,
                                            maxBitrateKbps: config.maxBitrateKbps,
                                            codec: config.videoCodec,
                                            hardwareAcceleration: config.hardwareAcceleratedDecoding
                                        }
                                    });

                                    // Provide specific error information based on the stream URL
                                    if (video.includes('m3u8') || video.includes('.ts')) {
                                        this.logger.error('HLS/TS Stream Error: The stream may be unavailable or in an unsupported format. Consider verifying if the stream is accessible.');
                                    } else if (video.startsWith('rtmp://')) {
                                        this.logger.error('RTMP Stream Error: The RTMP stream may be offline or using an unsupported codec.');
                                    } else if (video.includes('youtube.com') || video.includes('youtu.be')) {
                                        this.logger.error('YouTube Stream Error: The YouTube stream may have been removed or is region-restricted.');
                                    } else {
                                        // Try to probe the stream to get more information
                                        this.logger.error('Unknown Stream Type Error: Attempting to verify stream availability...');

                                        // In a production environment, we might add ffprobe diagnostics here
                                        // For now, log that this could be a network or permission issue
                                        this.logger.error('Possible causes: Network issues, missing codec, or stream unavailability');
                                    }
                                }
                                throw ffmpegError; // Re-throw to be handled by outer try/catch
                            }
                        });

                        this.logger.log("Successfully finished streaming video:", res);

                        // Check if we're still connected after streaming
                        if (!this.isInVoiceChannel()) {
                            this.logger.warn('Voice channel connection lost after streaming, updating connection state');
                            this.currentStream = null;
                        }

                        return res;
                    } catch (streamError) {
                        // This inner catch block handles stream-specific errors within the retry loop
                        newrelic.noticeError(streamError);
                        this.logger.error(`Streaming process error (attempt ${retryCount + 1}/${this.MAX_RETRY_ATTEMPTS + 1}):`, streamError);

                        // Check if ffmpeg processes are still running and kill them
                        await this.processManager.killFfmpegProcesses().catch(killError => {
                            this.logger.error('Error killing ffmpeg processes after stream error:', killError);
                        });

                        // Store the error and continue to retry
                        lastError = streamError;
                        retryCount++;
                        continue;
                    }
                } catch (error) {
                    newrelic.noticeError(error);
                    this.logger.error(`Error streaming video (attempt ${retryCount + 1}/${this.MAX_RETRY_ATTEMPTS + 1}):`, error);

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

                    // Store the error and continue to retry
                    lastError = error;
                    retryCount++;
                    continue;
                }

                // If we reach here, streaming was successful
                if (retryCount > 0) {
                    this.logger.info(`Successfully played video after ${retryCount} retries`);
                }
                return `Stream completed successfully${retryCount > 0 ? ` after ${retryCount} retries` : ''}`;
            }

            // If we've exhausted all retry attempts, throw the last error
            const errorMsg = lastError instanceof Error ? lastError.message : String(lastError);
            throw new Error(`Failed to stream video after ${this.MAX_RETRY_ATTEMPTS} retry attempts: ${errorMsg}`);
        });
    }
}