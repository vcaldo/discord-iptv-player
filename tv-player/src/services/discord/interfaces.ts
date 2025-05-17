import { StreamOptions, MediaUdp } from "@dank074/discord-video-stream";

// Define clear interfaces for the status system
export interface StatusProvider {
    setIdleStatus(): void;
    setWatchingStatus(name: string): void;
}

// Define clear interfaces for connection management
export interface ConnectionManager {
    isConnected(): boolean;
    connect(): Promise<void>;
    disconnect(): void;
    attemptReconnect(delayMs?: number): void;
}

// Define interfaces for streaming functionality
export interface StreamProvider {
    joinVoiceChannel(streamOpts: StreamOptions, voice_channel_id: string): Promise<MediaUdp>;
    leaveVoiceChannel(): void;
    startStreaming(video: string, udpConn: MediaUdp, voice_channel_id: string): Promise<string>;
    isInVoiceChannel(): boolean;
    getCurrentVoiceConnection(): MediaUdp | null;
}