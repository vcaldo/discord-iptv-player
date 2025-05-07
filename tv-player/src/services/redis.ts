import { Redis } from 'ioredis';
import config from '../config.js';
import { RedisMessage } from '../types/types.js';

export class RedisService {
    private redis: Redis;

    constructor() {
        this.redis = new Redis({
            host: config.redisHost,
            port: config.redisPort,
            password: config.redisPassword
        });
    }

    public async subscribe(pubSubChannel: string, messageHandler: (message: RedisMessage) => Promise<void>) {
        this.redis.subscribe(pubSubChannel, (err) => {
            if (err) {
                console.error('Failed to subscribe to Redis channel:', err);
                return;
            }
            console.log(`Subscribed to Redis channel: ${pubSubChannel}`);
        });

        this.redis.on('message', async (receivedChannel: string, message: string) => {
            if (receivedChannel === pubSubChannel) {
                try {
                    console.log("Received message: " + message);
                    const parsedMessage = JSON.parse(message) as RedisMessage;
                    console.log(parsedMessage);
                    await messageHandler(parsedMessage);
                } catch (error) {
                    console.error('Error processing Redis message:', error);
                }
            }
        });
    }

    public disconnect() {
        this.redis.disconnect();
    }
}
