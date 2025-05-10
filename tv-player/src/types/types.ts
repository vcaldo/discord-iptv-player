export interface RedisMessage {
    command: string;
    title: string;
    url: string;
    xcode_username?: string;
    xcode_password?: string;
}