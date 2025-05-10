// Initialize New Relic monitoring at the very start
import newrelic from 'newrelic';
import { StreamOptions, Utils } from "@dank074/discord-video-stream";
import config from "./config.js";
import { DiscordService } from "./services/discord.js";
import { RedisService } from "./services/redis.js";
import { RedisMessage } from "./types/types.js";
import { ShutdownHandler } from "./utils/shutdown.js";
import { YoutubeHelper } from "./utils/youtube.js";
import { ProcessManager } from "./utils/process-manager.js";
import { appLogger, logError, logInfo, logWarn, logDebug } from "./utils/logger.js";

// Configuration constants
const MAX_RETRY_ATTEMPTS = 3;
const RETRY_DELAY_MS = 2000;

// Application startup banner
logInfo("====================================");
logInfo(`Discord IPTV Player - Starting up...`);
logInfo(`Environment: ${config.isDevelopment() ? 'Development' : 'Production'}`);
logInfo(`Log level: ${config.isDevelopment() ? 'debug' : 'info'}`);
logInfo(`New Relic monitoring: Enabled`);
logInfo("====================================");

// Configure stream options
const streamOpts: StreamOptions = {
    width: config.width,
    height: config.height,
    fps: config.fps,
    bitrateKbps: config.bitrateKbps,
    maxBitrateKbps: config.maxBitrateKbps,
    hardwareAcceleratedDecoding: config.hardwareAcceleratedDecoding,
    videoCodec: Utils.normalizeVideoCodec(config.videoCodec),
    rtcpSenderReportEnabled: false,
    h26xPreset: 'ultrafast',
    minimizeLatency: false,
    forceChacha20Encryption: true
};

// Log stream configuration
logDebug("Stream configuration:", {
    width: config.width,
    height: config.height,
    fps: config.fps,
    bitrateKbps: config.bitrateKbps,
    maxBitrateKbps: config.maxBitrateKbps,
    hardwareAcceleration: config.hardwareAcceleratedDecoding,
    videoCodec: config.videoCodec
});

// Initialize services
logInfo("Initializing services...");
const discordService = new DiscordService();
const redisService = new RedisService();
const processManager = new ProcessManager();
const shutdownHandler = new ShutdownHandler(discordService, redisService, processManager);

// Set up global error handling
process.on('uncaughtException', (error) => {
    logError('Uncaught Exception - Application will continue running but may be unstable:', error);
});

process.on('unhandledRejection', (reason, promise) => {
    logError('Unhandled Promise Rejection:', { reason, promise });
});

// Setup shutdown handlers
shutdownHandler.setupShutdownHandlers();
logInfo("Shutdown handlers configured");

/**
 * Handles the play command with robust error handling and retries
 */
async function handlePlay(title: string, url: string, xcode_username?: string, xcode_password?: string) {
    const playTransaction = newrelic.startWebTransaction('handle-play', async function() {
        try {
            newrelic.addCustomAttribute('videoTitle', title);
            newrelic.addCustomAttribute('videoUrl', url);
            if (xcode_username) newrelic.addCustomAttribute('has_xcode_credentials', true);

            logInfo(`Attempting to play "${title}"`, {
                url,
                hasXcodeCredentials: !!xcode_username
            });

            if (!discordService.isReady()) {
                logWarn('Discord client not ready. Cannot play at the moment.');
                return;
            }

            // Stop any current stream but don't leave the voice channel
            logInfo('Stopping any existing stream without leaving voice channel...');
            await stopStreamOnly();
            // Increase wait time for Raspberry Pi to fully stop previous stream
            await new Promise(resolve => setTimeout(resolve, 2000));

            let videoUrl: string;
            try {
                await newrelic.startSegment('resolve-video-url', true, async () => {
                    logDebug('Resolving video URL...', { originalUrl: url });

                    // Handle YouTube links with the YouTube resolver
                    if (url.includes('youtube.com') || url.includes('youtu.be')) {
                        videoUrl = await YoutubeHelper.getVideoInternalUrl(url) ?? url;
                    } else {
                        // For other URLs, check if we need to handle xcode authentication
                        if (xcode_username && xcode_password) {
                            // Check if URL already contains credentials
                            if (!url.includes('@')) {
                                // Parse the URL
                                try {
                                    const urlObj = new URL(url);

                                    // Add xcode credentials to the URL
                                    urlObj.username = encodeURIComponent(xcode_username);
                                    urlObj.password = encodeURIComponent(xcode_password);

                                    videoUrl = urlObj.toString();
                                    logDebug('Added xcode credentials to URL');
                                } catch (urlError) {
                                    logError('Failed to parse URL for adding xcode credentials:', urlError);
                                    videoUrl = url; // Fall back to original URL
                                }
                            } else {
                                // URL already has credentials
                                videoUrl = url;
                                logDebug('URL already contains credentials');
                            }
                        } else {
                            videoUrl = url;
                        }
                    }

                    logInfo(`Resolved video URL successfully`);
                    logDebug(`Video URL details`, {
                        url: videoUrl.substring(0, videoUrl.indexOf('://') + 3) + '***' // Log only protocol for privacy
                    });
                });
            } catch (error) {
                newrelic.noticeError(error);
                logError('Failed to resolve video URL, using original as fallback:', error);

                // If we have xcode credentials but couldn't resolve properly, still try to add them
                if (xcode_username && xcode_password && !url.includes('@')) {
                    try {
                        const urlObj = new URL(url);
                        urlObj.username = encodeURIComponent(xcode_username);
                        urlObj.password = encodeURIComponent(xcode_password);
                        videoUrl = urlObj.toString();
                        logDebug('Added xcode credentials to fallback URL');
                    } catch (urlError) {
                        videoUrl = url;
                    }
                } else {
                    videoUrl = url;
                }

                logInfo(`Using modified URL as fallback`);
            }

            // Only join voice channel if not already in one
            let streamUdpConn;
            const inVoiceChannel = discordService.isInVoiceChannel();

            if (!inVoiceChannel) {
                logInfo('Not in voice channel, joining now...');
                await newrelic.startSegment('join-voice-channel', true, async () => {
                    streamUdpConn = await discordService.joinVoiceChannel(streamOpts);
                    logInfo('Successfully joined voice channel');
                });
            } else {
                logInfo('Already in voice channel, reusing connection...');
                streamUdpConn = discordService.getCurrentVoiceConnection();
                if (!streamUdpConn) {
                    logWarn('No existing voice connection found, joining channel again...');
                    streamUdpConn = await discordService.joinVoiceChannel(streamOpts);
                    logInfo('Successfully joined voice channel');
                }
            }

            logInfo(`Setting watching status to "${title}"`);
            discordService.setWatchingStatus(title);

            logInfo('Starting video stream...');
            await newrelic.startSegment('start-streaming', true, async () => {
                await discordService.startStreaming(videoUrl, streamUdpConn);
            });

            // Verify we're still in the voice channel after streaming
            if (!discordService.isInVoiceChannel()) {
                logWarn('Voice channel connection lost after streaming, attempting to reconnect...');
                await newrelic.startSegment('reconnect-voice-channel', true, async () => {
                    streamUdpConn = await discordService.joinVoiceChannel(streamOpts);
                    logInfo('Successfully reconnected to voice channel');
                });
            }

            logInfo(`Successfully playing "${title}"`);

        } catch (error) {
            newrelic.noticeError(error);
            logError(`Error during play operation for "${title}":`, error);

            // If error is critical, then stop completely - otherwise try to keep the connection
            const isCriticalError = error instanceof Error &&
                (error.message.includes('Guild not found') ||
                 error.message.includes('Discord client not ready'));

            if (isCriticalError) {
                logWarn('Critical error detected, stopping playback completely');
                await handleStop();
            } else {
                logInfo('Non-critical error, just stopping stream without leaving channel');
                await stopStreamOnly();
            }
        }
    });

    return playTransaction;
}

/**
 * Stops only the current stream without leaving the voice channel
 */
async function stopStreamOnly() {
    return newrelic.startWebTransaction('handle-stop-stream-only', async function() {
        try {
            logInfo("Stopping stream playback without leaving voice channel...");

            await newrelic.startSegment('kill-ffmpeg-processes', true, async () => {
                logInfo("Killing any running ffmpeg processes...");
                await processManager.killFfmpegProcesses();
            });

            discordService.setIdleStatus();

            logInfo("Successfully stopped stream playback");

            logDebug("Waiting 1500ms after stop stream command...");
            await new Promise(resolve => setTimeout(resolve, 1500));
        } catch (error) {
            newrelic.noticeError(error);
            logError("Error while stopping stream playback:", error);

            try {
                logInfo("Attempting to stop stream playback again after delay...");
                // Increase retry wait time for Raspberry Pi
                await new Promise(resolve => setTimeout(resolve, 2000));

                await processManager.killFfmpegProcesses();
                discordService.setIdleStatus();

                logInfo("Successfully stopped stream playback (second attempt)");
                logDebug("Waiting 1500ms after stop stream retry...");
                await new Promise(resolve => setTimeout(resolve, 1500));
            } catch (retryError) {
                newrelic.noticeError(retryError);
                logError("Failed to stop stream playback after retry:", retryError);

                try {
                    logInfo("Final attempt to kill ffmpeg processes...");
                    await processManager.killFfmpegProcesses();
                } catch (finalError) {
                    logError("Failed to kill ffmpeg processes in final attempt:", finalError);
                }
            }
        }
    });
}

/**
 * Handles the stop command with error handling
 */
async function handleStop() {
    return newrelic.startWebTransaction('handle-stop', async function() {
        try {
            logInfo("Stopping playback...");

            discordService.leaveVoiceChannel();

            await newrelic.startSegment('kill-ffmpeg-processes', true, async () => {
                logInfo("Killing any running ffmpeg processes...");
                await processManager.killFfmpegProcesses();
            });

            discordService.setIdleStatus();

            logInfo("Successfully stopped playing");

            logDebug("Waiting 1500ms after stop command...");
            await new Promise(resolve => setTimeout(resolve, 1500));
        } catch (error) {
            newrelic.noticeError(error);
            logError("Error while stopping playback:", error);

            try {
                logInfo("Attempting to stop playback again after delay...");
                // Increase retry wait time for Raspberry Pi
                await new Promise(resolve => setTimeout(resolve, 3000));

                discordService.leaveVoiceChannel();
                await processManager.killFfmpegProcesses();
                discordService.setIdleStatus();

                logInfo("Successfully stopped playing (second attempt)");
                logDebug("Waiting 1500ms after stop command retry...");
                await new Promise(resolve => setTimeout(resolve, 1500));
            } catch (retryError) {
                newrelic.noticeError(retryError);
                logError("Failed to stop playback after retry:", retryError);

                try {
                    logInfo("Final attempt to kill ffmpeg processes...");
                    await processManager.killFfmpegProcesses();
                } catch (finalError) {
                    logError("Failed to kill ffmpeg processes in final attempt:", finalError);
                }
            }
        }
    });
}

/**
 * Handles incoming Redis messages
 */
async function handleMessage(message: RedisMessage) {
    return newrelic.startBackgroundTransaction('handle-message', 'Redis', async function() {
        try {
            const { command, title, url, xcode_username, xcode_password } = message;

            newrelic.addCustomAttribute('command', command || 'none');
            if (title) newrelic.addCustomAttribute('title', title);
            if (url) newrelic.addCustomAttribute('url', url);
            if (xcode_username) newrelic.addCustomAttribute('has_xcode_credentials', true);

            logInfo(`Received command: ${command}`, {
                channel: title || 'N/A',
                url: url || 'N/A',
                hasXcodeCredentials: !!xcode_username
            });

            if (!command) {
                logError("Received message with no command");
                return;
            }

            switch (command.toLowerCase()) {
                case "play":
                    if (!title || !url) {
                        logError("Play command missing required parameters", {
                            title: title || 'missing',
                            url: url || 'missing'
                        });
                        return;
                    }
                    await handlePlay(title, url, xcode_username, xcode_password);
                    break;

                case "stop":
                    await handleStop();
                    break;

                case "restart":
                    logInfo("Restarting application...");
                    process.exit(0);
                    break;

                default:
                    logWarn(`Unknown command received: ${command}`);
                    break;
            }
        } catch (error) {
            newrelic.noticeError(error);
            logError("Error handling Redis message:", error);
        }
    });
}

// Subscribe to Redis channel with error handling
try {
    logInfo(`Subscribing to Redis channel: ${config.redisPubSubChannel}`);
    redisService.subscribe(config.redisPubSubChannel, handleMessage);
} catch (error) {
    logError("Error subscribing to Redis channel:", error);
    setTimeout(() => {
        try {
            logInfo(`Retrying subscription to Redis channel: ${config.redisPubSubChannel}`);
            redisService.subscribe(config.redisPubSubChannel, handleMessage);
        } catch (retryError) {
            logError("Failed to subscribe to Redis channel after retry:", retryError);
            logError("Application may not receive commands. Please restart the application.");
        }
    }, RETRY_DELAY_MS);
}