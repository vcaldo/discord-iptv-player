import { DiscordService } from "../services/discord.js";
import { RedisService } from "../services/redis.js";
import { ProcessManager } from "./process-manager.js";
import { logInfo, logError, logWarn, flushLogs } from "./logger.js";

export class ShutdownHandler {
    private services: {
        discord: DiscordService;
        redis: RedisService;
        processManager: ProcessManager;
    };
    private shuttingDown: boolean = false;

    constructor(
        discordService: DiscordService,
        redisService: RedisService,
        processManager: ProcessManager,
    ) {
        this.services = {
            discord: discordService,
            redis: redisService,
            processManager: processManager,
        };
    }

    public setupShutdownHandlers() {
        const handleSignal = async (signal: string) => {
            // Prevent multiple shutdown attempts
            if (this.shuttingDown) {
                logWarn(`Shutdown already in progress. Ignoring additional ${signal} signal.`);
                await flushLogs(); // Make sure this warning is visible
                return;
            }

            this.shuttingDown = true;

            try {
                // Log with timestamp for Docker logs
                logInfo(`[${new Date().toISOString()}] Signal ${signal} received. Initiating graceful shutdown...`);
                await flushLogs(); // Ensure the initial shutdown message is visible

                // Use a timeout to ensure shutdown completes even if something hangs
                const shutdownTimeout = setTimeout(() => {
                    logWarn(`Shutdown timed out after 10 seconds. Forcing exit...`);
                    process.exit(1);
                }, 10000);

                await this.gracefulShutdown(signal);

                // Clear timeout as shutdown completed successfully
                clearTimeout(shutdownTimeout);

                logInfo(`[${new Date().toISOString()}] Graceful shutdown for ${signal} completed successfully. Exiting process.`);
                await flushLogs(); // Ensure the final shutdown message is visible

                // Small delay to ensure logs are flushed before exit
                setTimeout(() => {
                    process.exit(0);
                }, 500);
            } catch (error) {
                logError(`[${new Date().toISOString()}] Graceful shutdown for ${signal} failed:`, error);
                await flushLogs(); // Ensure the error message is visible

                // Force exit after error with small delay to flush logs
                setTimeout(() => {
                    process.exit(1);
                }, 500);
            }
        };

        // Handle standard termination signals
        process.on('SIGTERM', () => handleSignal('SIGTERM'));
        process.on('SIGINT', () => handleSignal('SIGINT'));

        // Handle additional signals sent by Docker
        process.on('SIGQUIT', () => handleSignal('SIGQUIT'));

        // Handle Node.js-specific events that might occur during shutdown
        process.on('beforeExit', () => handleSignal('beforeExit'));

        logInfo('Shutdown handlers installed for signals: SIGTERM, SIGINT, SIGQUIT and beforeExit');
    }

    private async gracefulShutdown(signal: string): Promise<void> {
        logInfo(`[${new Date().toISOString()}] Starting graceful shutdown tasks for signal: ${signal}...`);
        await flushLogs();

        try {
            // Stop playback and clean up resources
            logInfo('[SHUTDOWN] Stopping playback and cleaning up resources...');
            await flushLogs();

            // Disconnect from voice channel if connected
            this.services.discord.leaveVoiceChannel();
            logInfo('[SHUTDOWN] Disconnected from Discord voice channel');
            await flushLogs();

            // Set the status back to idle
            this.services.discord.setIdleStatus();
            logInfo('[SHUTDOWN] Discord status set to idle');
            await flushLogs();

            // Disconnect Discord bot
            this.services.discord.shutdown();
            logInfo('[SHUTDOWN] Discord bot disconnected and offline');
            await flushLogs();

            // Kill any running ffmpeg processes
            logInfo('[SHUTDOWN] Killing running ffmpeg processes...');
            await this.services.processManager.killFfmpegProcesses();
            logInfo('[SHUTDOWN] Killed running ffmpeg processes');
            await flushLogs();

            // Disconnect Redis
            logInfo('[SHUTDOWN] Disconnecting from Redis...');
            this.services.redis.disconnect();
            logInfo('[SHUTDOWN] Disconnected from Redis');
            await flushLogs();

            logInfo('[SHUTDOWN] All graceful shutdown tasks completed.');
            await flushLogs();
        } catch (error) {
            logError('[SHUTDOWN] Error during graceful shutdown tasks:', error);
            await flushLogs();
            throw error;
        }
    }
}