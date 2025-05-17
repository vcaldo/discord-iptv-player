import { Redis } from 'ioredis';
import config from '../config.js';
import { RedisMessage } from '../types/types.js';
import { Logger } from '../utils/logger.js';
import newrelic from 'newrelic';

const MAX_RETRY_ATTEMPTS = 5;
const RETRY_DELAY_MS = 2000;

export class RedisService {
    private redis!: Redis;
    private isConnected: boolean = false;
    private reconnectAttempts: number = 0;
    private subscriptions: Map<string, (message: RedisMessage) => Promise<void>> = new Map();
    private logger: Logger;

    constructor() {
        this.logger = new Logger('RedisService');
        this.setupRedisClient();
    }

    private setupRedisClient() {
        try {
            this.logger.info('Creating Redis client...', {
                host: config.redisHost,
                port: config.redisPort,
                passwordProvided: !!config.redisPassword
            });

            this.redis = new Redis({
                host: config.redisHost,
                port: config.redisPort,
                password: config.redisPassword,
                retryStrategy: (times) => {
                    if (times >= MAX_RETRY_ATTEMPTS) {
                        this.logger.error(`Redis connection failed after ${times} attempts. No further retries.`);
                        return null;
                    }
                    const delay = Math.min(RETRY_DELAY_MS * Math.pow(2, times), 30000);
                    this.logger.warn(`Retrying Redis connection in ${delay}ms (attempt ${times + 1}/${MAX_RETRY_ATTEMPTS})...`);
                    return delay;
                }
            });

            this.setupEventHandlers();
        } catch (error) {
            this.logger.error('Failed to create Redis client:', error);
            this.attemptReconnect();
        }
    }

    private setupEventHandlers() {
        this.redis.on('connect', () => {
            this.logger.info('Connected to Redis server', {
                host: config.redisHost,
                port: config.redisPort
            });
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.restoreSubscriptions();
        });

        this.redis.on('error', (error) => {
            this.logger.error('Redis connection error:', error);
            if (this.isConnected) {
                this.isConnected = false;
            }
        });

        this.redis.on('close', () => {
            this.logger.warn('Redis connection closed');
            this.isConnected = false;
        });

        this.redis.on('reconnecting', () => {
            this.logger.info('Attempting to reconnect to Redis...');
        });

        this.redis.on('end', () => {
            this.logger.warn('Redis connection ended');
            this.isConnected = false;
            this.attemptReconnect();
        });
    }

    private attemptReconnect(delayMs = RETRY_DELAY_MS) {
        if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
            this.reconnectAttempts++;
            this.logger.info(`Attempting to reconnect to Redis (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS}) in ${delayMs}ms...`);

            setTimeout(() => {
                if (!this.isConnected) {
                    try {
                        this.setupRedisClient();
                    } catch (error) {
                        this.logger.error('Error during Redis reconnection attempt:', error);
                        const nextDelayMs = Math.min(delayMs * 2, 30000);
                        this.attemptReconnect(nextDelayMs);
                    }
                }
            }, delayMs);
        } else {
            this.logger.error(`Failed to reconnect to Redis after ${MAX_RETRY_ATTEMPTS} attempts.`);
            this.logger.error('Redis functionality is degraded. Application may not receive commands.');
        }
    }

    private async restoreSubscriptions() {
        if (this.subscriptions.size > 0) {
            this.logger.info(`Restoring ${this.subscriptions.size} Redis subscription(s)...`);

            for (const [channel, handler] of this.subscriptions.entries()) {
                try {
                    await this.subscribeToChannel(channel);
                    this.logger.info(`Restored subscription to channel: ${channel}`);
                } catch (error) {
                    this.logger.error(`Failed to restore subscription to channel ${channel}:`, error);
                }
            }
        }
    }

    private async subscribeToChannel(channel: string): Promise<void> {
        return new Promise((resolve, reject) => {
            this.redis.subscribe(channel, (err) => {
                if (err) {
                    this.logger.error(`Failed to subscribe to Redis channel ${channel}:`, err);
                    reject(err);
                } else {
                    this.logger.info(`Subscribed to Redis channel: ${channel}`);
                    resolve();
                }
            });
        });
    }

    public async subscribe(pubSubChannel: string, messageHandler: (message: RedisMessage) => Promise<void>) {
        return newrelic.startBackgroundTransaction('pub-sub:subscribe', 'Redis', async () => {
            newrelic.addCustomAttribute('channel', pubSubChannel);

            this.subscriptions.set(pubSubChannel, messageHandler);

            let attempts = 0;
            let subscribed = false;

            while (attempts < MAX_RETRY_ATTEMPTS && !subscribed) {
                try {
                    if (!this.isConnected) {
                        this.logger.warn('Redis not connected. Waiting before attempting to subscribe...');
                        await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                        attempts++;
                        continue;
                    }

                    await newrelic.startSegment('subscribe-to-channel', true, async () => {
                        await this.subscribeToChannel(pubSubChannel);
                    });
                    subscribed = true;

                    if (!this.redis.listenerCount('message')) {
                        this.logger.debug('Setting up Redis message handler');
                        this.redis.on('message', async (receivedChannel: string, message: string) => {
                            const handler = this.subscriptions.get(receivedChannel);
                            if (handler) {
                                try {
                                    this.logger.debug(`Received message on channel ${receivedChannel}`, { messageLength: message.length });
                                    this.logger.debug(`Message content: ${message.substring(0, 100)}${message.length > 100 ? '...' : ''}`);

                                    const parsedMessage = await newrelic.startSegment('parse-redis-message', true, async () => {
                                        return this.parseMessage(message);
                                    });

                                    if (parsedMessage) {
                                        await handler(parsedMessage);
                                    }
                                } catch (error) {
                                    newrelic.noticeError(error);
                                    this.logger.error(`Error processing Redis message on channel ${receivedChannel}:`, error);
                                }
                            }
                        });
                    }

                } catch (error) {
                    newrelic.noticeError(error);
                    attempts++;
                    this.logger.error(`Error subscribing to Redis channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                    if (attempts >= MAX_RETRY_ATTEMPTS) {
                        this.logger.error(`Failed to subscribe to Redis channel ${pubSubChannel} after multiple attempts`);
                        const errorMessage = error instanceof Error ? error.message : String(error);
                        throw new Error(`Failed to subscribe to Redis channel: ${errorMessage}`);
                    }

                    const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                    this.logger.info(`Retrying subscription in ${delayMs}ms...`);
                    await new Promise(resolve => setTimeout(resolve, delayMs));
                }
            }
        });
    }

    private parseMessage(message: string): RedisMessage | null {
        try {
            const parsedMessage = JSON.parse(message) as RedisMessage;

            if (!parsedMessage.command) {
                this.logger.error('Received invalid Redis message: missing command field', {
                    messagePreview: message.substring(0, 100)
                });
                return null;
            }

            this.logger.debug('Successfully parsed Redis message', {
                command: parsedMessage.command,
                title: parsedMessage.title || 'N/A'
            });
            return parsedMessage;
        } catch (error) {
            this.logger.error('Error parsing Redis message:', error, {
                rawMessage: message.substring(0, 100) + (message.length > 100 ? '...' : '')
            });
            return null;
        }
    }

    public async publish(channel: string, message: RedisMessage): Promise<boolean> {
        return newrelic.startBackgroundTransaction('pub-sub:publish', 'Redis', async () => {
            newrelic.addCustomAttribute('channel', channel);
            newrelic.addCustomAttribute('command', message.command);

            let attempts = 0;

            while (attempts < MAX_RETRY_ATTEMPTS) {
                try {
                    if (!this.isConnected) {
                        this.logger.warn('Redis not connected. Waiting before attempting to publish...');
                        await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                        attempts++;
                        continue;
                    }

                    const stringifiedMessage = JSON.stringify(message);
                    this.logger.debug(`Publishing message to channel ${channel}`, {
                        command: message.command,
                        messageLength: stringifiedMessage.length
                    });

                    const result = await newrelic.startSegment('redis-publish', true, async () => {
                        return await this.redis.publish(channel, stringifiedMessage);
                    });

                    this.logger.info(`Published message to channel ${channel}, received by ${result} subscriber(s)`, {
                        command: message.command
                    });
                    return result > 0;

                } catch (error: unknown) {
                    newrelic.noticeError(error);
                    attempts++;
                    this.logger.error(`Error publishing to Redis channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                    if (attempts >= MAX_RETRY_ATTEMPTS) {
                        this.logger.error(`Failed to publish to Redis channel ${channel} after multiple attempts`);
                        return false;
                    }

                    const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                    this.logger.info(`Retrying publish in ${delayMs}ms...`);
                    await new Promise(resolve => setTimeout(resolve, delayMs));
                }
            }

            return false;
        });
    }

    public disconnect() {
        try {
            if (this.isConnected) {
                this.logger.info('Disconnecting from Redis');
                this.redis.disconnect();
                this.logger.info('Redis disconnected successfully');
            }
        } catch (error) {
            this.logger.error('Error disconnecting from Redis:', error);
        } finally {
            this.isConnected = false;
            this.subscriptions.clear();
        }
    }

    public isReady(): boolean {
        return this.isConnected && this.redis.status === 'ready';
    }

    /**
     * Sets a value in Redis
     * @param key The key to set
     * @param value The value to store
     */
    public async set(key: string, value: string): Promise<void> {
        try {
            await this.redis.set(key, value);
        } catch (error) {
            throw new Error(`Failed to set Redis key ${key}: ${error}`);
        }
    }
}
