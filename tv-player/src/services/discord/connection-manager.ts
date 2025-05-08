import { Client } from "discord.js-selfbot-v13";
import { ConnectionManager } from "./interfaces.js";
import { Logger } from "../../utils/logger.js";
import config from "../../config.js";
import newrelic from 'newrelic';

const MAX_RETRY_ATTEMPTS = 3;
const RETRY_DELAY_MS = 2000;

/**
 * Manages Discord connection state and reconnection logic
 */
export class DiscordConnectionManager implements ConnectionManager {
    private client: Client;
    private logger: Logger;
    private isConnectedState: boolean = false;
    private reconnectAttempts: number = 0;
    private onConnected?: () => void;

    constructor(client: Client, onConnected?: () => void) {
        this.client = client;
        this.logger = new Logger('ConnectionManager');
        this.onConnected = onConnected;
        this.setupEventHandlers();
    }

    private setupEventHandlers(): void {
        this.client.on("ready", () => {
            if (this.client.user) {
                this.isConnectedState = true;
                this.reconnectAttempts = 0;
                this.logger.log(`Connected as ${this.client.user.tag}`);

                if (this.onConnected) {
                    this.onConnected();
                }
            }
        });

        this.client.on("disconnect", (event) => {
            this.isConnectedState = false;
            this.logger.error(`Disconnected from Discord. Code: ${event.code}, Reason: ${event.reason}`);
            this.attemptReconnect();
        });

        this.client.on("error", (error) => {
            this.logger.error("Discord client error:", error);
            if (!this.isConnectedState) {
                this.attemptReconnect();
            }
        });
    }

    public async connect(): Promise<void> {
        return newrelic.startBackgroundTransaction('discord:connect', async () => {
            try {
                this.logger.log("Attempting to connect to Discord...");

                await newrelic.startSegment('login', true, async () => {
                    await this.client.login(config.token);
                });
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Failed to login to Discord:', error);
                this.handleLoginFailure(error);
                throw error;
            }
        });
    }

    private handleLoginFailure(error: any): void {
        newrelic.startBackgroundTransaction('discord:handle-login-failure', () => {
            if (error.message) {
                newrelic.addCustomAttribute('error_message', error.message);
            }

            if (error.message?.includes('TOKEN_INVALID')) {
                this.logger.error('The provided token is invalid. Please check your configuration.');
            } else if (error.message?.includes('RATE_LIMITED')) {
                this.logger.error('Rate limited by Discord. Waiting before retry...');
                const delayMs = RETRY_DELAY_MS * Math.pow(2, this.reconnectAttempts);
                this.attemptReconnect(delayMs);
            } else {
                this.attemptReconnect();
            }
        });
    }

    public attemptReconnect(delayMs = RETRY_DELAY_MS): void {
        newrelic.startBackgroundTransaction('discord:reconnect', () => {
            newrelic.addCustomAttribute('attempt_number', this.reconnectAttempts + 1);
            newrelic.addCustomAttribute('delay_ms', delayMs);

            if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
                this.reconnectAttempts++;
                this.logger.log(`Attempting to reconnect (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS}) in ${delayMs}ms...`);

                setTimeout(() => {
                    if (!this.isConnectedState) {
                        newrelic.startSegment('login-retry', true, async () => {
                            try {
                                await this.client.login(config.token);
                            } catch (error) {
                                newrelic.noticeError(error);
                                this.logger.error('Reconnection attempt failed:', error);
                                this.handleLoginFailure(error);
                            }
                        });
                    }
                }, delayMs);
            } else {
                this.logger.error('Max reconnection attempts reached. Please check your configuration and network.');
            }
        });
    }

    public disconnect(): void {
        newrelic.startBackgroundTransaction('discord:disconnect', () => {
            try {
                this.logger.log("Disconnecting from Discord...");

                newrelic.startSegment('destroy-client', true, () => {
                    this.client.destroy();
                });

                this.isConnectedState = false;
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error("Error disconnecting from Discord:", error);
            }
        });
    }

    public isConnected(): boolean {
        return this.isConnectedState && this.client.isReady();
    }
}