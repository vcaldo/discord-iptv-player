import { Client } from "discord.js-selfbot-v13";
import { StreamOptions, MediaUdp } from "@dank074/discord-video-stream";
import { StatusProvider, ConnectionManager, StreamProvider } from "./discord/interfaces.js";
import { StatusManager } from "./discord/status-manager.js";
import { DiscordConnectionManager } from "./discord/connection-manager.js";
import { StreamService } from "./discord/stream-service.js";
import { Logger } from "../utils/logger.js";

/**
 * Main Discord service that coordinates connection, status management, and streaming
 */
export class DiscordService {
    private client: Client;
    private logger: Logger;
    private connectionManager: ConnectionManager;
    private statusManager: StatusProvider;
    private streamService: StreamProvider;

    constructor() {
        this.logger = new Logger('DiscordService');
        this.client = new Client();

        // Initialize the service components
        this.statusManager = new StatusManager(this.client);
        this.connectionManager = new DiscordConnectionManager(
            this.client,
            // Set idle status when connected
            () => this.statusManager.setIdleStatus()
        );
        this.streamService = new StreamService(this.client);

        // Start the connection process
        this.initialize();
    }

    /**
     * Initialize the Discord client connection
     */
    private async initialize(): Promise<void> {
        try {
            this.logger.log('Initializing Discord service...');
            await this.connectionManager.connect();
        } catch (error) {
            this.logger.error('Error initializing Discord service:', error);
        }
    }

    /**
     * Set idle/ready status
     */
    public setIdleStatus(): void {
        this.statusManager.setIdleStatus();
    }

    /**
     * Set watching status with the specified content name
     */
    public setWatchingStatus(name: string): void {
        this.statusManager.setWatchingStatus(name);
    }

    /**
     * Join a voice channel and prepare for streaming
     */
    public async joinVoiceChannel(streamOpts: StreamOptions): Promise<MediaUdp> {
        if (!this.isReady()) {
            this.logger.warn('Discord client not ready. Connecting before joining voice channel...');
            await this.connectionManager.connect();
        }

        return this.streamService.joinVoiceChannel(streamOpts);
    }

    /**
     * Leave the current voice channel
     */
    public leaveVoiceChannel(): void {
        this.streamService.leaveVoiceChannel();
    }

    /**
     * Start streaming the specified video
     */
    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        if (!this.isReady()) {
            throw new Error('Discord client not ready. Cannot start streaming.');
        }

        return this.streamService.startStreaming(video, udpConn);
    }

    /**
     * Check if the client is ready
     */
    public isReady(): boolean {
        return this.connectionManager.isConnected();
    }

    /**
     * Gracefully shut down the Discord client
     */
    public shutdown(): void {
        this.logger.log('Shutting down Discord service...');
        try {
            this.leaveVoiceChannel();
        } catch (error) {
            this.logger.error('Error leaving voice channel during shutdown:', error);
        }

        try {
            this.connectionManager.disconnect();
        } catch (error) {
            this.logger.error('Error disconnecting during shutdown:', error);
        }
    }
}
