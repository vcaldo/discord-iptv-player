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
async function handlePlay(title: string, url: string) {
    const playTransaction = newrelic.startWebTransaction('handle-play', async function() {
        try {
            newrelic.addCustomAttribute('videoTitle', title);
            newrelic.addCustomAttribute('videoUrl', url);

            logInfo(`Attempting to play "${title}"`, { url });

            if (!discordService.isReady()) {
                logWarn('Discord client not ready. Cannot play at the moment.');
                return;
            }

            // Stop any current stream but don't leave the voice channel
            logInfo('Stopping any existing stream without leaving voice channel...');
            await stopStreamOnly();
            await new Promise(resolve => setTimeout(resolve, 500));

            let videoUrl: string;
            try {
                await newrelic.startSegment('resolve-video-url', true, async () => {
                    logDebug('Resolving video URL...', { originalUrl: url });
                    videoUrl = await YoutubeHelper.getVideoInternalUrl(url) ?? url;
                    logInfo(`Resolved video URL successfully`);
                    logDebug(`Video URL details`, { url: videoUrl });
                });
            } catch (error) {
                newrelic.noticeError(error);
                logError('Failed to resolve video URL, using original as fallback:', error);
                videoUrl = url;
                logInfo(`Using original URL as fallback: ${videoUrl}`);
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

            logDebug("Waiting 100ms after stop stream command...");
            await new Promise(resolve => setTimeout(resolve, 100));
        } catch (error) {
            newrelic.noticeError(error);
            logError("Error while stopping stream playback:", error);

            try {
                logInfo("Attempting to stop stream playback again after delay...");
                await new Promise(resolve => setTimeout(resolve, 500));

                await processManager.killFfmpegProcesses();
                discordService.setIdleStatus();

                logInfo("Successfully stopped stream playback (second attempt)");
                logDebug("Waiting 100ms after stop stream retry...");
                await new Promise(resolve => setTimeout(resolve, 100));
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

            logDebug("Waiting 100ms after stop command...");
            await new Promise(resolve => setTimeout(resolve, 100));
        } catch (error) {
            newrelic.noticeError(error);
            logError("Error while stopping playback:", error);

            try {
                logInfo("Attempting to stop playback again after delay...");
                await new Promise(resolve => setTimeout(resolve, 1000));

                discordService.leaveVoiceChannel();
                await processManager.killFfmpegProcesses();
                discordService.setIdleStatus();

                logInfo("Successfully stopped playing (second attempt)");
                logDebug("Waiting 100ms after stop command retry...");
                await new Promise(resolve => setTimeout(resolve, 100));
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
            const { command, title, url } = message;

            newrelic.addCustomAttribute('command', command || 'none');
            if (title) newrelic.addCustomAttribute('title', title);
            if (url) newrelic.addCustomAttribute('url', url);

            logInfo(`Received command: ${command}`, {
                channel: title || 'N/A',
                url: url || 'N/A'
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
                    await handlePlay(title, url);
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