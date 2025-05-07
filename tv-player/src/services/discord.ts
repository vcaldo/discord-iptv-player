import { Client, CustomStatus, ActivityOptions } from "discord.js-selfbot-v13";
import { Streamer, StreamOptions, MediaUdp, streamLivestreamVideo } from "@dank074/discord-video-stream";
import config from "../config.js";

export class DiscordService {
    private streamer: Streamer;

    constructor() {
        this.streamer = new Streamer(new Client());
        this.setupClient();
    }

    private setupClient() {
        this.streamer.client.login(config.token);
        this.streamer.client.on("ready", () => {
            if (this.streamer.client.user) {
                console.log(`--- ${this.streamer.client.user.tag} is ready ---`);
                this.setIdleStatus();
            }
        });
    }

    private createCustomStatus(emoji: string, state: string): CustomStatus {
        return new CustomStatus(this.streamer.client).setEmoji(emoji).setState(state);
    }

    public setIdleStatus() {
        const status = this.createCustomStatus('👌', 'Ready to broadcast');
        this.streamer.client.user?.setActivity(status as unknown as ActivityOptions);
    }

    public setWatchingStatus(name: string) {
        const status = this.createCustomStatus('📺', `${name}`);
        this.streamer.client.user?.setActivity(status as unknown as ActivityOptions);    }

    public async joinVoiceChannel(streamOpts: StreamOptions) {
        await this.streamer.joinVoice(config.guildId, config.videoChannelId!, streamOpts);
        return await this.streamer.createStream(streamOpts);
    }

    public leaveVoiceChannel() {
        this.streamer.leaveVoice();
    }

    public async startStreaming(video: string, udpConn: MediaUdp): Promise<string> {
        console.log("Started streaming video");
        udpConn.mediaConnection.setSpeaking(true);
        udpConn.mediaConnection.setVideoStatus(true);

        try {
            const res = await streamLivestreamVideo(video, udpConn);
            console.log("Finished streaming video " + res);
            return res;
        } catch (error) {
            console.log("Error streaming video: ", error);
            throw error;
        } finally {
            udpConn.mediaConnection.setSpeaking(false);
            udpConn.mediaConnection.setVideoStatus(false);
        }
    }
}
