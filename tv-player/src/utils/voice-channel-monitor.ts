import { logInfo, logWarn } from './logger.js';

export class VoiceChannelMonitor {
    private emptyChannelTimer: NodeJS.Timeout | null = null;
    private readonly EMPTY_CHANNEL_TIMEOUT = 60000; // 60 seconds ta em millisegundos

    constructor(
        private discordService: any,
        private handleStop: () => Promise<void>
    ) {}

    public isChannelEmpty(voiceConnection: any): boolean {
        const members = voiceConnection.channel.members.filter(
            (member: any) => !member.user.bot
        );
        return members.size === 0;
    }

    public startMonitoring(): void {
        const voiceConnection = this.discordService.getCurrentVoiceConnection();
        if (!voiceConnection) {
            logWarn('No voice connection to monitor');
            return;
        }

        // Monitor initial state
        if (this.isChannelEmpty(voiceConnection)) {
            this.startEmptyChannelTimer();
        }

        // Set up voice state update listener
        voiceConnection.channel.on('voiceStateUpdate', (_: any, __: any) => {
            this.checkChannelState();
        });
    }

    public stopMonitoring(): void {
        logInfo('Stopping voice channel monitoring');
        this.clearEmptyChannelTimer();
    }

    private checkChannelState(): void {
        const voiceConnection = this.discordService.getCurrentVoiceConnection();
        if (!voiceConnection) return;

        if (this.isChannelEmpty(voiceConnection)) {
            this.startEmptyChannelTimer();
        } else {
            this.clearEmptyChannelTimer();
        }
    }

    private startEmptyChannelTimer(): void {
        this.clearEmptyChannelTimer();
        logInfo('Starting empty channel timer...');
        
        this.emptyChannelTimer = setTimeout(async () => {
            const voiceConnection = this.discordService.getCurrentVoiceConnection();
            if (voiceConnection && this.isChannelEmpty(voiceConnection)) {
                logInfo('Channel empty for 60 seconds, stopping playback...');
                await this.handleStop(); // ficar vazio desconecta após 60s
            }
        }, this.EMPTY_CHANNEL_TIMEOUT);
    }

    private clearEmptyChannelTimer(): void {
        if (this.emptyChannelTimer) {
            clearTimeout(this.emptyChannelTimer);
            this.emptyChannelTimer = null;
        }
    }
}