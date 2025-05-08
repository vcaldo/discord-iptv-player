import { Client, CustomStatus, ActivityOptions } from "discord.js-selfbot-v13";
import { StatusProvider } from "./interfaces.js";
import { Logger } from "../../utils/logger.js";

/**
 * Manages Discord user status functionality
 */
export class StatusManager implements StatusProvider {
    private client: Client;
    private logger: Logger;

    constructor(client: Client) {
        this.client = client;
        this.logger = new Logger('StatusManager');
    }

    /**
     * Creates a custom status with emoji and state text
     */
    private createCustomStatus(emoji: string, state: string): CustomStatus {
        try {
            return new CustomStatus(this.client).setEmoji(emoji).setState(state);
        } catch (error) {
            this.logger.error('Error creating custom status:', error);
            return new CustomStatus(this.client).setState('Online');
        }
    }

    /**
     * Sets the user status to idle/ready
     */
    public setIdleStatus(): void {
        try {
            const status = this.createCustomStatus('👌', 'Ready to broadcast');
            this.setActivity(status);
        } catch (error) {
            this.logger.error('Error setting idle status:', error);
            try {
                this.client.user?.setStatus('online');
            } catch (innerError) {
                this.logger.error('Failed to set fallback status:', innerError);
            }
        }
    }

    /**
     * Sets the user status to watching the specified content
     */
    public setWatchingStatus(name: string): void {
        try {
            const status = this.createCustomStatus('📺', `${name}`);
            this.setActivity(status);
        } catch (error) {
            this.logger.error('Error setting watching status:', error);
            try {
                const activityOptions: ActivityOptions = {
                    type: 'WATCHING',
                    name: name
                };
                this.client.user?.setActivity(activityOptions);
            } catch (innerError) {
                this.logger.error('Failed to set fallback watching status:', innerError);
            }
        }
    }

    private setActivity(status: CustomStatus): void {
        if (!this.client.user) {
            this.logger.warn('Cannot set activity: client user is not available');
            return;
        }

        const activityOptions = status as unknown as ActivityOptions;
        this.client.user.setActivity(activityOptions);
    }
}