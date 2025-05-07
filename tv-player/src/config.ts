import dotenv from "dotenv"

dotenv.config()

export default {
    // Selfbot options
    token: process.env.TOKEN ?? (() => { throw new Error('TOKEN is required'); })(),
    guildId: process.env.GUILD_ID ?? (() => { throw new Error('GUILD_ID is required'); })(),
    videoChannelId: process.env.VIDEO_CHANNEL_ID ?? (() => { throw new Error('VIDEO_CHANNEL_ID is required'); })(),

    // Stream options
    respect_video_params: parseBoolean(process.env.STREAM_RESPECT_VIDEO_PARAMS ?? 'false'),
    width: parseInt(process.env.STREAM_WIDTH ?? '1920'),
    height: parseInt(process.env.STREAM_HEIGHT ?? '1080'),
    fps: parseInt(process.env.STREAM_FPS ?? '60'),
    bitrateKbps: parseInt(process.env.STREAM_BITRATE_KBPS ?? '1000'),
    maxBitrateKbps: parseInt(process.env.STREAM_MAX_BITRATE_KBPS ?? '4200'),
    hardwareAcceleratedDecoding: parseBoolean(process.env.STREAM_HARDWARE_ACCELERATION ?? 'false'),
    videoCodec: process.env.STREAM_VIDEO_CODEC ?? 'VP8',

    // Redis options
    redisHost: process.env.REDIS_HOST ?? (() => { throw new Error('REDIS_HOST is required'); })(),
    redisPort: parseInt(process.env.REDIS_PORT ?? (() => { throw new Error('REDIS_PORT is required'); })()),
    redisPassword: process.env.REDIS_PASSWORD ?? (() => { throw new Error('REDIS_PASSWORD is required'); })(),
}

function parseBoolean(value: string | undefined): boolean {
    if (typeof value !== "string") return false;
    return ["true", "1", "yes", "on"].includes(value.trim().toLowerCase());
}