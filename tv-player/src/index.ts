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

// Maximum number of retry attempts for operations
const MAX_RETRY_ATTEMPTS = 3;
// Delay between retry attempts in milliseconds
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

    /**
     * Enables the sending of RTCP sender reports. These reports assist the receiver in synchronizing audio and video frames.
     * In certain uncommon scenarios, disabling this feature might be beneficial.
     */
    rtcpSenderReportEnabled: false,
    /**
     * Specifies the encoding preset for H264 or H265 codecs. Faster presets result in lower quality.
     * Available presets include: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, and veryslow.
     */
    h26xPreset: 'ultrafast',
    /**
     * Configures ffmpeg parameters to minimize latency and expedite video output.
     * Note: This may occasionally cause video output lag.
     */
    minimizeLatency: false,
    /**
     * Forces the use of ChaCha20-Poly1305 encryption, which is generally faster than AES-256-GCM,
     * except when AES-NI is utilized.
     */
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
const shutdownHandler = new ShutdownHandler(discordService, redisService);

// Set up global error handling
process.on('uncaughtException', (error) => {
    logError('Uncaught Exception - Application will continue running but may be unstable:', error);
    // Log the error but keep the process running
});

process.on('unhandledRejection', (reason, promise) => {
    logError('Unhandled Promise Rejection:', { reason, promise });
    // Log the error but keep the process running
});

// Setup shutdown handlers
shutdownHandler.setupShutdownHandlers();
logInfo("Shutdown handlers configured");

/**
 * Handles the play command with robust error handling and retries
 * @param title The title of the video/stream
 * @param url The URL of the video/stream
 */
async function handlePlay(title: string, url: string) {
    // Create New Relic transaction for play operation
    const playTransaction = newrelic.startWebTransaction('handlePlay', async function() {
        let attempts = 0;

        while (attempts < MAX_RETRY_ATTEMPTS) {
            try {
                // Add custom attributes to the transaction
                newrelic.addCustomAttribute('videoTitle', title);
                newrelic.addCustomAttribute('videoUrl', url);
                newrelic.addCustomAttribute('attemptNumber', attempts + 1);

                logInfo(`Attempting to play "${title}" - attempt ${attempts + 1}/${MAX_RETRY_ATTEMPTS}`, { url });

                // Check if Discord client is ready
                if (!discordService.isReady()) {
                    logWarn('Discord client not ready. Waiting before attempting to play...');
                    await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                    attempts++;
                    continue;
                }

                // Step 1: Stop any existing stream first
                logInfo('Stopping any existing stream...');
                await handleStop();
                // Small delay to ensure cleanup is complete
                await new Promise(resolve => setTimeout(resolve, 500));

                // Step 2: Get video URL (with fallback to original URL)
                let videoUrl: string;
                try {
                    // Create New Relic segment for URL resolution
                    await newrelic.startSegment('resolve-video-url', true, async () => {
                        logDebug('Resolving video URL...', { originalUrl: url });
                        videoUrl = await YoutubeHelper.getVideoInternalUrl(url) ?? url;
                        logInfo(`Resolved video URL successfully`);
                        logDebug(`Video URL details`, { url: videoUrl });
                    });
                } catch (error) {
                    newrelic.noticeError(error);
                    logError('Failed to resolve video URL, using original as fallback:', error);
                    // Fallback to original URL on error
                    videoUrl = url;
                    logInfo(`Using original URL as fallback: ${videoUrl}`);
                }

                // Step 3: Join voice channel
                logInfo('Joining voice channel...');
                // Create New Relic segment for voice channel joining
                await newrelic.startSegment('join-voice-channel', true, async () => {
                    const streamUdpConn = await discordService.joinVoiceChannel(streamOpts);
                    logInfo('Successfully joined voice channel');

                    // Step 4: Set status
                    logInfo(`Setting watching status to "${title}"`);
                    discordService.setWatchingStatus(title);

                    // Step 5: Start streaming
                    logInfo('Starting video stream...');
                    // Create New Relic segment for streaming
                    await newrelic.startSegment('start-streaming', true, async () => {
                        await discordService.startStreaming(videoUrl, streamUdpConn);
                    });
                    logInfo(`Successfully playing "${title}"`);
                });

                // If we reach here, everything succeeded
                return;
            } catch (error) {
                // Report errors to New Relic
                newrelic.noticeError(error);
                attempts++;
                logError(`Error during play operation (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                if (attempts >= MAX_RETRY_ATTEMPTS) {
                    logError(`Failed to play "${title}" after ${MAX_RETRY_ATTEMPTS} attempts`);
                    // Clean up on failure
                    try {
                        await handleStop();
                    } catch (cleanupError) {
                        newrelic.noticeError(cleanupError);
                        logError('Error during cleanup after play failure:', cleanupError);
                    }
                    break;
                }

                // Exponential backoff before retry
                const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                logInfo(`Retrying play operation in ${delayMs}ms...`);
                await new Promise(resolve => setTimeout(resolve, delayMs));
            }
        }
    });

    return playTransaction;
}

/**
 * Handles the stop command with error handling
 */
async function handleStop() {
    // Create New Relic transaction for stop operation
    return newrelic.startWebTransaction('handle-stop', async function() {
        try {
            logInfo("Stopping playback...");

            // First leave the voice channel
            discordService.leaveVoiceChannel();

            // Then kill any running ffmpeg processes
            await newrelic.startSegment('kill-ffmpeg-processes', true, async () => {
                logInfo("Killing any running ffmpeg processes...");
                await processManager.killFfmpegProcesses();
            });

            // Finally, set the status back to idle
            discordService.setIdleStatus();

            logInfo("Successfully stopped playing");
        } catch (error) {
            // Report error to New Relic
            newrelic.noticeError(error);
            logError("Error while stopping playback:", error);

            // Try one more time with delay if initial attempt fails
            try {
                logInfo("Attempting to stop playback again after delay...");
                await new Promise(resolve => setTimeout(resolve, 1000));

                discordService.leaveVoiceChannel();

                // Make sure to kill ffmpeg processes even in the retry
                await processManager.killFfmpegProcesses();

                discordService.setIdleStatus();
                logInfo("Successfully stopped playing (second attempt)");
            } catch (retryError) {
                // Report retry error to New Relic
                newrelic.noticeError(retryError);
                logError("Failed to stop playback after retry:", retryError);

                // Last attempt to at least kill ffmpeg processes
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
 * Handles incoming Redis messages with error handling
 * @param message The Redis message containing command and parameters
 */
async function handleMessage(message: RedisMessage) {
    // Create New Relic transaction for message handling
    return newrelic.startBackgroundTransaction('handle-message', 'Redis', async function() {
        try {
            const { command, title, url } = message;

            // Add custom attributes to the New Relic transaction
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
            // Report error to New Relic
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
    // Try to reconnect and subscribe after delay
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