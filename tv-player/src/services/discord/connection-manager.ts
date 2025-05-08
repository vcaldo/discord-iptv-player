import { Client } from "discord.js-selfbot-v13";
import { ConnectionManager } from "./interfaces.js";
import { Logger } from "../../utils/logger.js";
import config from "../../config.js";

// Maximum number of retry attempts for connection operations
const MAX_RETRY_ATTEMPTS = 3;
// Delay between retry attempts in milliseconds
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

    /**
     * Sets up event handlers for connection events
     */
    private setupEventHandlers(): void {
        // Handle ready event
        this.client.on("ready", () => {
            if (this.client.user) {
                this.isConnectedState = true;
                this.reconnectAttempts = 0; // Reset counter on successful connection
                this.logger.log(`Connected as ${this.client.user.tag}`);

                // Call the onConnected callback if provided
                if (this.onConnected) {
                    this.onConnected();
                }
            }
        });

        // Handle disconnect event
        this.client.on("disconnect", (event) => {
            this.isConnectedState = false;
            this.logger.error(`Disconnected from Discord. Code: ${event.code}, Reason: ${event.reason}`);
            this.attemptReconnect();
        });

        // Handle error event
        this.client.on("error", (error) => {
            this.logger.error("Discord client error:", error);
            if (!this.isConnectedState) {
                this.attemptReconnect();
            }
        });
    }

    /**
     * Attempts to connect to Discord
     */
    public async connect(): Promise<void> {
        try {
            this.logger.log("Attempting to connect to Discord...");
            await this.client.login(config.token);
        } catch (error) {
            this.logger.error('Failed to login to Discord:', error);
            this.handleLoginFailure(error);
            throw error;
        }
    }

    /**
     * Handles specific login failure scenarios
     */
    private handleLoginFailure(error: any): void {
        // Handle specific error types with appropriate actions
        if (error.message?.includes('TOKEN_INVALID')) {
            this.logger.error('The provided token is invalid. Please check your configuration.');
        } else if (error.message?.includes('RATE_LIMITED')) {
            this.logger.error('Rate limited by Discord. Waiting before retry...');
            // Implement exponential backoff for rate limits
            const delayMs = RETRY_DELAY_MS * Math.pow(2, this.reconnectAttempts);
            this.attemptReconnect(delayMs);
        } else {
            this.attemptReconnect();
        }
    }

    /**
     * Attempts to reconnect to Discord with backoff
     */
    public attemptReconnect(delayMs = RETRY_DELAY_MS): void {
        if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
            this.reconnectAttempts++;
            this.logger.log(`Attempting to reconnect (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS}) in ${delayMs}ms...`);

            setTimeout(() => {
                if (!this.isConnectedState) {
                    this.client.login(config.token).catch(error => {
                        this.logger.error('Reconnection attempt failed:', error);
                        this.handleLoginFailure(error);
                    });
                }
            }, delayMs);
        } else {
            this.logger.error('Max reconnection attempts reached. Please check your configuration and network.');
        }
    }

    /**
     * Disconnects from Discord
     */
    public disconnect(): void {
        try {
            this.logger.log("Disconnecting from Discord...");
            this.client.destroy();
            this.isConnectedState = false;
        } catch (error) {
            this.logger.error("Error disconnecting from Discord:", error);
        }
    }

    /**
     * Returns whether the client is currently connected
     */
    public isConnected(): boolean {
        return this.isConnectedState && this.client.isReady();
    }
}