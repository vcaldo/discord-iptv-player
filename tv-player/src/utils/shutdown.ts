import { DiscordService } from "../services/discord.js";
import { RedisService } from "../services/redis.js";

export class ShutdownHandler {
    private services: {
        discord: DiscordService;
        redis: RedisService;
    };

    constructor(
        discordService: DiscordService,
        redisService: RedisService,
    ) {
        this.services = {
            discord: discordService,
            redis: redisService,
        };
    }

    public setupShutdownHandlers() {
        // Handle graceful shutdown on SIGTERM and SIGINT
        process.on('SIGTERM', () => this.gracefulShutdown('SIGTERM'));
        process.on('SIGINT', () => this.gracefulShutdown('SIGINT'));
    }

    private async gracefulShutdown(signal: string) {
        console.log(`\nReceived ${signal}. Starting graceful shutdown...`);

        try {
            // Disconnect from voice channel if connected
            this.services.discord.leaveVoiceChannel();
            console.log('Disconnected from Discord voice channel');

            // Disconnect Redis
            this.services.redis.disconnect();
            console.log('Disconnected from Redis');

            console.log('Graceful shutdown completed');
            process.exit(0);
        } catch (error) {
            console.error('Error during graceful shutdown:', error);
            process.exit(1);
        }
    }
} 