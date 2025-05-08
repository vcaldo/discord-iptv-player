import { Client, CustomStatus, ActivityOptions } from "discord.js-selfbot-v13";
import { Streamer, StreamOptions, MediaUdp, streamLivestreamVideo } from "@dank074/discord-video-stream";
import config from "../config.js";

// Maximum number of retry attempts for operations
const MAX_RETRY_ATTEMPTS = 3;
// Delay between retry attempts in milliseconds
const RETRY_DELAY_MS = 2000;

export class DiscordService {
    private streamer: Streamer;
    private isConnected: boolean = false;
    private reconnectAttempts: number = 0;

    constructor() {
        this.streamer = new Streamer(new Client());
        this.setupClient();
    }

    private setupClient() {
        try {
            // Setup event handlers before login
            this.setupEventHandlers();

            // Attempt to login
            this.streamer.client.login(config.token).catch(error => {
                console.error('Failed to login to Discord:', error);
                this.handleLoginFailure(error);
            });
        } catch (error) {
            console.error('Error during client setup:', error);
            // Attempt to recover by restarting the setup process
            if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
                this.reconnectAttempts++;
                console.log(`Attempting to reconnect (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS})...`);
                setTimeout(() => this.setupClient(), RETRY_DELAY_MS);
            } else {
                console.error('Max reconnection attempts reached. Please check your configuration and network.');
            }
        }
    }

    private setupEventHandlers() {
        // Handle ready event
        this.streamer.client.on("ready", () => {
            if (this.streamer.client.user) {
                this.isConnected = true;
                this.reconnectAttempts = 0; // Reset counter on successful connection
                console.log(`--- ${this.streamer.client.user.tag} is ready ---`);
                this.setIdleStatus();
            }
        });

        // Handle disconnect event
        this.streamer.client.on("disconnect", (event) => {
            this.isConnected = false;
            console.error(`Disconnected from Discord. Code: ${event.code}, Reason: ${event.reason}`);
            this.attemptReconnect();
        });

        // Handle error event
        this.streamer.client.on("error", (error) => {
            console.error("Discord client error:", error);
            if (!this.isConnected) {
                this.attemptReconnect();
            }
        });
    }

    private handleLoginFailure(error: any) {
        // Handle specific error types with appropriate actions
        if (error.message?.includes('TOKEN_INVALID')) {
            console.error('The provided token is invalid. Please check your configuration.');
        } else if (error.message?.includes('RATE_LIMITED')) {
            console.error('Rate limited by Discord. Waiting before retry...');
            // Implement exponential backoff for rate limits
            const delayMs = RETRY_DELAY_MS * Math.pow(2, this.reconnectAttempts);
            this.attemptReconnect(delayMs);
        } else {
            this.attemptReconnect();
        }
    }

    private attemptReconnect(delayMs = RETRY_DELAY_MS) {
        if (this.reconnectAttempts < MAX_RETRY_ATTEMPTS) {
            this.reconnectAttempts++;
            console.log(`Attempting to reconnect (${this.reconnectAttempts}/${MAX_RETRY_ATTEMPTS}) in ${delayMs}ms...`);
            setTimeout(() => {
                if (!this.isConnected) {
                    this.streamer.client.login(config.token).catch(error => {
                        console.error('Reconnection attempt failed:', error);
                        this.handleLoginFailure(error);
                    });
                }
            }, delayMs);
        } else {
            console.error('Max reconnection attempts reached. Please check your configuration and network.');
        }
    }

    private createCustomStatus(emoji: string, state: string): CustomStatus {
        try {
            return new CustomStatus(this.streamer.client).setEmoji(emoji).setState(state);
        } catch (error) {
            console.error('Error creating custom status:', error);
            // Fallback to a simple status in case of error
            return new CustomStatus(this.streamer.client).setState('Online');
        }
    }

    public setIdleStatus() {
        try {
            const status = this.createCustomStatus('👌', 'Ready to broadcast');
            this.streamer.client.user?.setActivity(status as unknown as ActivityOptions);
        } catch (error) {
            console.error('Error setting idle status:', error);
            // Try a simpler approach as fallback
            try {
                this.streamer.client.user?.setStatus('online');
            } catch (innerError) {
                console.error('Failed to set fallback status:', innerError);
            }
        }
    }

    public setWatchingStatus(name: string) {
        try {
            const status = this.createCustomStatus('📺', `${name}`);
            this.streamer.client.user?.setActivity(status as unknown as ActivityOptions);
        } catch (error) {
            console.error('Error setting watching status:', error);
            // Try a simpler approach as fallback
            try {
                this.streamer.client.user?.setActivity({
                    type: 'WATCHING',
                    name: name
                } as unknown as ActivityOptions);
            } catch (innerError) {
                console.error('Failed to set fallback watching status:', innerError);
            }
        }
    }

    public async joinVoiceChannel(streamOpts: StreamOptions): Promise<MediaUdp> {
        let attempts = 0;

        while (attempts < MAX_RETRY_ATTEMPTS) {
            try {
                // Check if client is connected before attempting to join
                if (!this.isConnected) {
                    console.log('Discord client not connected. Waiting before joining voice channel...');
                    await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS));
                    attempts++;
                    continue;
                }

                await this.streamer.joinVoice(config.guildId, config.videoChannelId!, streamOpts);
                const stream = await this.streamer.createStream(streamOpts);
                console.log('Successfully joined voice channel and created stream');
                return stream;
            } catch (error) {
                attempts++;
                console.error(`Error joining voice channel (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                if (attempts >= MAX_RETRY_ATTEMPTS) {
                    console.error('Failed to join voice channel after multiple attempts.');
                    throw new Error(`Failed to join voice channel: ${error instanceof Error ? error.message : String(error)}`);
                }

                // Wait before retry with exponential backoff
                const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                console.log(`Retrying in ${delayMs}ms...`);
                await new Promise(resolve => setTimeout(resolve, delayMs));
            }
        }

        // This should not be reached due to the throw in the catch block,
        // but TypeScript requires a return statement
        throw new Error('Failed to join voice channel after multiple attempts');
    }

    public leaveVoiceChannel() {
        try {
            this.streamer.leaveVoice();
            console.log('Successfully left voice channel');
        } catch (error) {
            console.error('Error leaving voice channel:', error);
            // Try alternative cleanup if available
            try {
                // Force disconnect if the normal method fails
                const guild = this.streamer.client.guilds.cache.get(config.guildId);
                const voiceState = guild?.voiceStates?.cache.get(this.streamer.client.user?.id || '');
                if (voiceState) voiceState.disconnect();
            } catch (innerError) {
                console.error('Failed to force disconnect from voice channel:', innerError);
            }
        }
    }

    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        console.log("Starting to stream video:", video);
        let attempts = 0;
        let success = false;

        while (attempts < MAX_RETRY_ATTEMPTS && !success) {
            try {
                udpConn.mediaConnection.setSpeaking(true);
                udpConn.mediaConnection.setVideoStatus(true);

                const res = await streamLivestreamVideo(video, udpConn);
                console.log("Successfully finished streaming video:", res);
                success = true;
                return res;
            } catch (error) {
                attempts++;
                console.error(`Error streaming video (attempt ${attempts}/${MAX_RETRY_ATTEMPTS}):`, error);

                if (attempts >= MAX_RETRY_ATTEMPTS) {
                    console.error('Failed to stream video after multiple attempts');
                    throw new Error(`Failed to stream video: ${error instanceof Error ? error.message : String(error)}`);
                }

                // Wait before retry with exponential backoff
                const delayMs = RETRY_DELAY_MS * Math.pow(1.5, attempts - 1);
                console.log(`Retrying stream in ${delayMs}ms...`);
                await new Promise(resolve => setTimeout(resolve, delayMs));

                // Reset connection state before retry
                try {
                    udpConn.mediaConnection.setSpeaking(false);
                    udpConn.mediaConnection.setVideoStatus(false);
                    // Short pause before restarting
                    await new Promise(resolve => setTimeout(resolve, 500));
                } catch (resetError) {
                    console.error('Error resetting connection state:', resetError);
                }
            } finally {
                if (!success) {
                    // Only reset if we're giving up or succeeded
                    try {
                        udpConn.mediaConnection.setSpeaking(false);
                        udpConn.mediaConnection.setVideoStatus(false);
                    } catch (finalError) {
                        console.error('Error in finally block while cleaning up connection:', finalError);
                    }
                }
            }
        }

        // This should not be reached due to the throw in the catch block
        throw new Error('Failed to stream video after multiple attempts');
    }

    // Check if the client is ready before performing operations
    public isReady(): boolean {
        return this.isConnected && this.streamer.client.isReady();
    }
}
