import { StreamOptions, Utils } from "@dank074/discord-video-stream";
import config from "./config.js";
import { DiscordService } from "./services/discord.js";
import { RedisService } from "./services/redis.js";
import { RedisMessage } from "./types/types.js";
import { ShutdownHandler } from "./utils/shutdown.js";
import { YoutubeHelper } from "./utils/youtube.js";

// Maximum number of retry attempts for operations
const MAX_RETRY_ATTEMPTS = 3;
// Delay between retry attempts in milliseconds
const RETRY_DELAY_MS = 2000;

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

// Initialize services
const discordService = new DiscordService();
const redisService = new RedisService();
const shutdownHandler = new ShutdownHandler(discordService, redisService);

// Set up global error handling
process.on('uncaughtException', (error) => {
    console.error('Uncaught Exception:', error);
    // Log the error but keep the process running
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('Unhandled Rejection at:', promise, 'reason:', reason);
    // Log the error but keep the process running
});

// Setup shutdown handlers
shutdownHandler.setupShutdownHandlers();

/**
 * Handles the play command with robust error handling and retries
 * @param title The title of the video/stream
 * @param url The URL of the video/stream
 */
async function handlePlay(title: string, url: string) {
    let attempts = 0;

    while (attempts < MAX_RETRY_ATTEMPTS) {
        try {
            console.log(`Attempting to play "${title}" (${url}) - attempt ${attempts + 1}/${MAX_RETRY_ATTEMPTS}`);

            // Check if Discord client is ready
            if (!discordService.isReady()) {
                console.log('Discord client not ready. Waiting before attempting to play...');
                await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                attempts++;
                continue;
            }

            // Step 1: Get video URL (with fallback to original URL)
            let videoUrl: string;
            try {
                videoUrl = await YoutubeHelper.getVideoInternalUrl(url) ?? url;
                console.log(`Resolved video URL: ${videoUrl}`);
            } catch (error) {
                console.error('Failed to resolve video URL:', error);
                // Fallback to original URL on error
                videoUrl = url;
                console.log(`Using original URL as fallback: ${videoUrl}`);
            }

            // Step 2: Join voice channel
            const streamUdpConn = await discordService.joinVoiceChannel(streamOpts);

            // Step 3: Set status
            discordService.setWatchingStatus(title);

            // Step 4: Start streaming
            await discordService.startStreaming(videoUrl, streamUdpConn);
            console.log(`Successfully playing "${title}" from ${videoUrl}`);

            // If we reach here, everything succeeded
            return;

        } catch (error) {
            attempts++;
            console.error(`Error during play operation (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

            if (attempts >= MAX_RETRY_ATTEMPTS) {
                console.error(`Failed to play "${title}" after ${MAX_RETRY_ATTEMPTS} attempts`);
                // Clean up on failure
                try {
                    await handleStop();
                } catch (cleanupError) {
                    console.error('Error during cleanup after play failure:', cleanupError);
                }
                break;
            }

            // Exponential backoff before retry
            const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
            console.log(`Retrying play operation in ${delayMs}ms...`);
            await new Promise(resolve => setTimeout(resolve, delayMs));
        }
    }
}

/**
 * Handles the stop command with error handling
 */
async function handleStop() {
    try {
        discordService.leaveVoiceChannel();
        discordService.setIdleStatus();
        console.log("Successfully stopped playing");
    } catch (error) {
        console.error("Error while stopping playback:", error);
        // Try one more time with delay if initial attempt fails
        try {
            await new Promise(resolve => setTimeout(resolve, 1000));
            discordService.leaveVoiceChannel();
            discordService.setIdleStatus();
            console.log("Successfully stopped playing (second attempt)");
        } catch (retryError) {
            console.error("Failed to stop playback after retry:", retryError);
        }
    }
}

/**
 * Handles incoming Redis messages with error handling
 * @param message The Redis message containing command and parameters
 */
async function handleMessage(message: RedisMessage) {
    try {
        const { command, title, url } = message;
        console.log(`Received command: ${command} from channel: ${title || 'N/A'}`);

        if (!command) {
            console.error("Received message with no command");
            return;
        }

        switch (command.toLowerCase()) {
            case "play":
                if (!title || !url) {
                    console.error("Play command missing required parameters (title or url)");
                    return;
                }
                await handlePlay(title, url);
                break;

            case "stop":
                await handleStop();
                break;

            case "restart":
                console.log("Restarting application...");
                process.exit(0);
                break;

            default:
                console.warn(`Unknown command received: ${command}`);
                break;
        }
    } catch (error) {
        console.error("Error handling Redis message:", error);
    }
}

// Subscribe to Redis channel with error handling
try {
    console.log(`Subscribing to Redis channel: ${config.redisPubSubChannel}`);
    redisService.subscribe(config.redisPubSubChannel, handleMessage);
} catch (error) {
    console.error("Error subscribing to Redis channel:", error);
    // Try to reconnect and subscribe after delay
    setTimeout(() => {
        try {
            console.log(`Retrying subscription to Redis channel: ${config.redisPubSubChannel}`);
            redisService.subscribe(config.redisPubSubChannel, handleMessage);
        } catch (retryError) {
            console.error("Failed to subscribe to Redis channel after retry:", retryError);
            console.error("Application may not receive commands. Please restart the application.");
        }
    }, RETRY_DELAY_MS);
}