import { Client } from "discord.js-selfbot-v13";
import { StreamOptions, MediaUdp } from "@dank074/discord-video-stream";
import { StatusProvider, ConnectionManager, StreamProvider } from "./discord/interfaces.js";
import { StatusManager } from "./discord/status-manager.js";
import { DiscordConnectionManager } from "./discord/connection-manager.js";
import { StreamService } from "./discord/stream-service.js";
import { Logger } from "../utils/logger.js";
import newrelic from 'newrelic';

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

        this.statusManager = new StatusManager(this.client);
        this.connectionManager = new DiscordConnectionManager(
            this.client,
            () => this.statusManager.setIdleStatus()
        );
        this.streamService = new StreamService(this.client);

        this.initialize();
    }

    private async initialize(): Promise<void> {
        return newrelic.startBackgroundTransaction('discord:initialize-service', async () => {
            try {
                this.logger.log('Initializing Discord service...');
                await this.connectionManager.connect();
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error initializing Discord service:', error);
            }
        });
    }

    public setIdleStatus(): void {
        newrelic.startSegment('set-idle-status', true, () => {
            this.statusManager.setIdleStatus();
        });
    }

    public setWatchingStatus(name: string): void {
        newrelic.startSegment('set-watching-status', true, () => {
            newrelic.addCustomAttribute('content_name', name);
            this.statusManager.setWatchingStatus(name);
        });
    }

    public async joinVoiceChannel(streamOpts: StreamOptions, voice_channel_id?: string): Promise<MediaUdp> {
        return newrelic.startSegment('join-voice-channel', true, async () => {
            if (!this.isReady()) {
                this.logger.warn('Discord client not ready. Connecting before joining voice channel...');
                await this.connectionManager.connect();
            }

            return this.streamService.joinVoiceChannel(streamOpts, voice_channel_id);
        });
    }

    public leaveVoiceChannel(): void {
        newrelic.startSegment('leave-voice-channel', true, () => {
            this.streamService.leaveVoiceChannel();
        });
    }

    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        return newrelic.startSegment('start-streaming', true, async () => {
            if (!this.isReady()) {
                throw new Error('Discord client not ready. Cannot start streaming.');
            }

            newrelic.addCustomAttribute('video_url', video);
            return this.streamService.startStreaming(video, udpConn);
        });
    }

    public isReady(): boolean {
        return this.connectionManager.isConnected();
    }

    public isInVoiceChannel(): boolean {
        return newrelic.startSegment('check-in-voice-channel', true, () => {
            return this.streamService.isInVoiceChannel();
        });
    }

    public getCurrentVoiceConnection(): MediaUdp | null {
        return newrelic.startSegment('get-current-voice-connection', true, () => {
            return this.streamService.getCurrentVoiceConnection();
        });
    }

    public shutdown(): void {
        return newrelic.startBackgroundTransaction('discord:shutdown-service', () => {
            this.logger.log('Shutting down Discord service...');
            try {
                newrelic.startSegment('leave-voice-channel', true, () => {
                    this.leaveVoiceChannel();
                });
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error leaving voice channel during shutdown:', error);
            }

            try {
                newrelic.startSegment('disconnect-client', true, () => {
                    this.connectionManager.disconnect();
                });
            } catch (error) {
                newrelic.noticeError(error);
                this.logger.error('Error disconnecting during shutdown:', error);
            }
        });
    }
}
