import { Redis } from 'ioredis';
import config from '../config.js';
import { RedisMessage } from '../types/types.js';

// Maximum number of retry attempts for Redis operations
const MAX_RETRY_ATTEMPTS = 5;
// Delay between retry attempts in milliseconds
const RETRY_DELAY_MS = 2000;

export class RedisService {
    private redis!: Redis;
    private isConnected: boolean = false;
    private reconnectAttempts: number = 0;
    private subscriptions: Map<string, (message: RedisMessage) => Promise<void>> = new Map();

    constructor() {
        this.setupRedisClient();
    }

    private setupRedisClient() {
        try {
            this.redis = new Redis({
                host: config.redisHost,
                port: config.redisPort,
                password: config.redisPassword,
                retryStrategy: (times) => {
                    // Implement exponential backoff with max retries
                    if (times >= MAX_RETRY_ATTEMPTS) {
                        console.error(`Redis connection failed after ${times} attempts. No further retries.`);
                        return null; // Stop retrying
                    }
                    const delay = Math.min(RETRY_DELAY_MS * Math.pow(2, times), 30000); // Cap at 30 seconds
                    console.log(`Retrying Redis connection in ${delay}ms (attempt ${times + 1}/${MAX_RETRY_ATTEMPTS})...`);
                    return delay;
                }
            });

            this.setupEventHandlers();
        } catch (error) {
            console.error('Failed to create Redis client:', error);
            this.attemptReconnect();
        }
    }

    private setupEventHandlers() {
        // Handle successful connection
        this.redis.on('connect', () => {
            console.log('Connected to Redis server');
            this.isConnected = true;
            this.reconnectAttempts = 0;
            this.restoreSubscriptions();
        });

        // Handle connection errors
        this.redis.on('error', (error) => {
            console.error('Redis connection error:', error);
            if (this.isConnected) {
                this.isConnected = false;
            }
        });

        // Handle disconnection
        this.redis.on('close', () => {
            console.log('Redis connection closed');
            this.isConnected = false;
        });

        // Handle reconnection
        this.redis.on('reconnecting', () => {
            console.log('Attempting to reconnect to Redis...');
        });

        // Handle end of connection (final termination)
        this.redis.on('end', () => {
            console.log('Redis connection ended');
            this.isConnected = false;
            this.attemptReconnect();
        });
    }

    private attemptReconnect(delayMs = RETRY_DELAY_MS) {
        if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
            this.reconnectAttempts++;
            console.log(`Attempting to reconnect to Redis (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS}) in ${delayMs}ms...`);

            setTimeout(() => {
                if (!this.isConnected) {
                    try {
                        this.setupRedisClient();
                    } catch (error) {
                        console.error('Error during Redis reconnection attempt:', error);
                        // Increase delay for next retry with exponential backoff
                        const nextDelayMs = Math.min(delayMs * 2, 30000); // Cap at 30 seconds
                        this.attemptReconnect(nextDelayMs);
                    }
                }
            }, delayMs);
        } else {
            console.error(`Failed to reconnect to Redis after ${MAX_RETRY_ATTEMPTS} attempts.`);
            console.error('Redis functionality is degraded. Application may not receive commands.');
        }
    }

    // Restore all active subscriptions after reconnection
    private async restoreSubscriptions() {
        if (this.subscriptions.size > 0) {
            console.log(`Restoring ${this.subscriptions.size} Redis subscription(s)...`);

            for (const [channel, handler] of this.subscriptions.entries()) {
                try {
                    await this.subscribeToChannel(channel);
                    console.log(`Restored subscription to channel: ${channel}`);
                } catch (error) {
                    console.error(`Failed to restore subscription to channel ${channel}:`, error);
                }
            }
        }
    }

    private async subscribeToChannel(channel: string): Promise<void> {
        return new Promise((resolve, reject) => {
            this.redis.subscribe(channel, (err) => {
                if (err) {
                    console.error(`Failed to subscribe to Redis channel ${channel}:`, err);
                    reject(err);
                } else {
                    console.log(`Subscribed to Redis channel: ${channel}`);
                    resolve();
                }
            });
        });
    }

    public async subscribe(pubSubChannel: string, messageHandler: (message: RedisMessage) => Promise<void>) {
        // Store the subscription for reconnection purposes
        this.subscriptions.set(pubSubChannel, messageHandler);

        let attempts = 0;
        let subscribed = false;

        while (attempts < MAX_RETRY_ATTEMPTS && !subscribed) {
            try {
                // Wait for connection if not connected
                if (!this.isConnected) {
                    console.log('Redis not connected. Waiting before attempting to subscribe...');
                    await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                    attempts++;
                    continue;
                }

                // Subscribe to the channel
                await this.subscribeToChannel(pubSubChannel);
                subscribed = true;

                // Only set up the message handler once
                if (!this.redis.listenerCount('message')) {
                    this.redis.on('message', async (receivedChannel: string, message: string) => {
                        const handler = this.subscriptions.get(receivedChannel);
                        if (handler) {
                            try {
                                console.log(`Received message on channel ${receivedChannel}: ${message}`);
                                const parsedMessage = this.parseMessage(message);
                                if (parsedMessage) {
                                    await handler(parsedMessage);
                                }
                            } catch (error) {
                                console.error(`Error processing Redis message on channel ${receivedChannel}:`, error);
                            }
                        }
                    });
                }

            } catch (error) {
                attempts++;
                console.error(`Error subscribing to Redis channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                if (attempts >= MAX_RETRY_ATTEMPTS) {
                    console.error(`Failed to subscribe to Redis channel ${pubSubChannel} after multiple attempts`);
                    const errorMessage = error instanceof Error ? error.message : String(error);
                    throw new Error(`Failed to subscribe to Redis channel: ${errorMessage}`);
                }

                // Wait before retry with exponential backoff
                const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                console.log(`Retrying subscription in ${delayMs}ms...`);
                await new Promise(resolve => setTimeout(resolve, delayMs));
            }
        }
    }

    private parseMessage(message: string): RedisMessage | null {
        try {
            const parsedMessage = JSON.parse(message) as RedisMessage;

            // Validate required fields
            if (!parsedMessage.command) {
                console.error('Received invalid Redis message: missing command field');
                return null;
            }

            return parsedMessage;
        } catch (error) {
            console.error('Error parsing Redis message:', error, 'Raw message:', message);
            return null;
        }
    }

    public async publish(channel: string, message: RedisMessage): Promise<boolean> {
        let attempts = 0;

        while (attempts < MAX_RETRY_ATTEMPTS) {
            try {
                // Wait for connection if not connected
                if (!this.isConnected) {
                    console.log('Redis not connected. Waiting before attempting to publish...');
                    await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                    attempts++;
                    continue;
                }

                const result = await this.redis.publish(channel, JSON.stringify(message));
                console.log(`Published message to channel ${channel}, received by ${result} subscriber(s)`);
                return result > 0;

            } catch (error) {
                attempts++;
                console.error(`Error publishing to Redis channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                if (attempts >= MAX_RETRY_ATTEMPTS) {
                    console.error(`Failed to publish to Redis channel ${channel} after multiple attempts`);
                    return false;
                }

                // Wait before retry with exponential backoff
                const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                console.log(`Retrying publish in ${delayMs}ms...`);
                await new Promise(resolve => setTimeout(resolve, delayMs));
            }
        }

        return false;
    }

    public disconnect() {
        try {
            if (this.isConnected) {
                this.redis.disconnect();
                console.log('Redis disconnected successfully');
            }
        } catch (error) {
            console.error('Error disconnecting from Redis:', error);
        } finally {
            this.isConnected = false;
            this.subscriptions.clear();
        }
    }

    public isReady(): boolean {
        return this.isConnected && this.redis.status === 'ready';
    }
}
