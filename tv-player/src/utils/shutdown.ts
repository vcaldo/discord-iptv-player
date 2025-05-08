import { DiscordService } from "../services/discord.js";
import { RedisService } from "../services/redis.js";
import { ProcessManager } from "./process-manager.js"; // Added import
import { logInfo, logError } from "./logger.js"; // Added import for logging

export class ShutdownHandler {
    private services: {
        discord: DiscordService;
        redis: RedisService;
        processManager: ProcessManager; // Added processManager
    };

    constructor(
        discordService: DiscordService,
        redisService: RedisService,
        processManager: ProcessManager, // Added processManager parameter
    ) {
        this.services = {
            discord: discordService,
            redis: redisService,
            processManager: processManager, // Initialize processManager
        };
    }

    public setupShutdownHandlers() {
        // Handle graceful shutdown on SIGTERM and SIGINT
        process.on('SIGTERM', () => this.gracefulShutdown('SIGTERM'));
        process.on('SIGINT', () => this.gracefulShutdown('SIGINT'));
    }

    private async gracefulShutdown(signal: string) {
        logInfo(`\nReceived ${signal}. Starting graceful shutdown...`);

        try {
            // Stop playback and clean up resources
            logInfo('Stopping playback and cleaning up resources...');

            // Disconnect from voice channel if connected
            await this.services.discord.leaveVoiceChannel();
            logInfo('Disconnected from Discord voice channel');

            // Set the status back to idle
            await this.services.discord.setIdleStatus();
            logInfo('Discord status set to idle');

            // Disconnect Discord bot
            await this.services.discord.shutdown(); // Corrected to use shutdown method
            logInfo('Discord bot disconnected and offline');

            // Kill any running ffmpeg processes
            this.services.processManager.killFfmpegProcesses();
            logInfo('Killed running ffmpeg processes');

            // Disconnect Redis
            this.services.redis.disconnect();
            logInfo('Disconnected from Redis');

            logInfo('Graceful shutdown completed');
            process.exit(0);
        } catch (error) {
            logError('Error during graceful shutdown:', error);
            process.exit(1);
        }
    }
}