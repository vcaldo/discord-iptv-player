import { DiscordService } from "../services/discord.js";
import { RedisService } from "../services/redis.js";
import { ProcessManager } from "./process-manager.js";
import { logInfo, logError } from "./logger.js";

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
        // Signal handler for all termination signals
        const handleSignal = async (signal: string) => {
            if (this.shuttingDown) return;
            this.shuttingDown = true;

            logInfo(`Signal ${signal} received. Initiating shutdown...`);

            try {
                await this.gracefulShutdown();
                process.exit(0);
            } catch (error) {
                logError(`Shutdown failed:`, error);
                process.exit(1);
            }
        };

        // Register handlers for important signals
        ['SIGTERM', 'SIGINT', 'SIGQUIT'].forEach(signal => {
            process.on(signal, () => handleSignal(signal));
        });

        logInfo('Shutdown handlers installed');
    }

    private async gracefulShutdown(): Promise<void> {
        try {
            this.services.discord.leaveVoiceChannel();
            this.services.discord.setIdleStatus();
            this.services.discord.shutdown();

            await this.services.processManager.killFfmpegProcesses();

            this.services.redis.disconnect();

            logInfo('Graceful shutdown completed');
        } catch (error) {
            logError('Error during shutdown:', error);
            throw error;
        }
    }
}