import newrelic from "newrelic";
import { Client, CustomStatus, ActivityOptions } from "discord.js-selfbot-v13";
import { Streamer, prepareStream, playStream, Encoders } from "@dank074/discord-video-stream";
import { Redis } from "ioredis";

// --- Config ---
const TOKEN = requiredEnv("TOKEN");
const GUILD_ID = requiredEnv("GUILD_ID");
const REDIS_HOST = process.env.REDIS_HOST ?? "redis";
const REDIS_PORT = parseInt(process.env.REDIS_PORT ?? "6379", 10);
const REDIS_PASSWORD = process.env.REDIS_PASSWORD ?? "";
const REDIS_CHANNEL = process.env.REDIS_PUB_SUB_CHANNEL ?? "iptv";

const STREAM_WIDTH = parseInt(process.env.STREAM_WIDTH ?? "1920", 10);
const STREAM_HEIGHT = parseInt(process.env.STREAM_HEIGHT ?? "1080", 10);
const STREAM_FPS = parseInt(process.env.STREAM_FPS ?? "60", 10);
const STREAM_BITRATE_KBPS = parseInt(process.env.STREAM_BITRATE_KBPS ?? "1000", 10);
const STREAM_MAX_BITRATE_KBPS = parseInt(process.env.STREAM_MAX_BITRATE_KBPS ?? "4200", 10);
const STREAM_VIDEO_CODEC = (process.env.STREAM_VIDEO_CODEC ?? "H264") as "H264" | "H265" | "VP8" | "VP9" | "AV1";
const STREAM_HW_ACCEL = process.env.STREAM_HARDWARE_ACCELERATION === "true";

function requiredEnv(name: string): string {
    const value = process.env[name];
    if (!value) {
        console.error(`Missing required environment variable: ${name}`);
        process.exit(1);
    }
    return value;
}

// --- Types ---
interface RedisMessage {
    command: string;
    title: string;
    url: string;
    voice_channel_id: string;
}

// --- State ---
const client = new Client();
const streamer = new Streamer(client);
let currentAbortController: AbortController | null = null;
let inVoiceChannel = false;

// --- Discord ---
client.on("ready", async () => {
    console.log(`[discord] Logged in as ${client.user?.tag}`);
    setIdleStatus();

    // Save bot ID to redis for the remote-control service
    const botId = client.user?.id;
    if (botId) {
        try {
            await pubClient.set("tv_player_bot_id", botId);
            console.log(`[discord] Bot ID ${botId} saved to Redis`);
        } catch (err) {
            console.error("[discord] Failed to save bot ID to Redis:", err);
        }
    }
});

client.on("error", (err) => {
    console.error("[discord] Client error:", err);
});

// --- Status ---
function setIdleStatus() {
    try {
        const status = new CustomStatus(client).setEmoji("👌").setState("Ready to broadcast");
        client.user?.setActivity(status as unknown as ActivityOptions);
    } catch (err) {
        console.error("[status] Failed to set idle status:", err);
    }
}

function setWatchingStatus(title: string) {
    try {
        const status = new CustomStatus(client).setEmoji("📺").setState(title);
        client.user?.setActivity(status as unknown as ActivityOptions);
    } catch (err) {
        console.error("[status] Failed to set watching status:", err);
    }
}

// --- Streaming ---
async function handlePlay(title: string, url: string, voiceChannelId: string) {
    return newrelic.startWebTransaction("handle-play", async () => {
        try {
            newrelic.addCustomAttribute("videoTitle", title);
            newrelic.addCustomAttribute("videoUrl", url);
            newrelic.addCustomAttribute("voiceChannelId", voiceChannelId);

            console.log(`[play] Playing "${title}" from ${url} in channel ${voiceChannelId}`);

            if (!client.isReady()) {
                console.warn("[play] Discord client not ready, skipping");
                return;
            }

            // Stop current stream if any
            await stopStream();
            await sleep(500);

            // Join voice channel if not already in one
            if (!inVoiceChannel) {
                console.log(`[play] Joining voice channel ${voiceChannelId}`);
                try {
                    await newrelic.startSegment("join-voice-channel", true, async () => {
                        await streamer.joinVoice(GUILD_ID, voiceChannelId);
                        inVoiceChannel = true;
                        console.log("[play] Joined voice channel");
                    });
                } catch (err) {
                    newrelic.noticeError(err as Error);
                    console.error("[play] Failed to join voice channel:", err);
                    return;
                }
            }

            setWatchingStatus(title);

            // Prepare and play stream with the v6 API
            currentAbortController = new AbortController();
            const { signal } = currentAbortController;

            try {
                await newrelic.startSegment("start-streaming", true, async () => {
                    const { output } = prepareStream(url, {
                        width: STREAM_WIDTH,
                        height: STREAM_HEIGHT,
                        frameRate: STREAM_FPS,
                        videoCodec: STREAM_VIDEO_CODEC,
                        bitrateVideo: STREAM_BITRATE_KBPS,
                        bitrateVideoMax: STREAM_MAX_BITRATE_KBPS,
                        bitrateAudio: 128,
                        includeAudio: true,
                        hardwareAcceleratedDecoding: STREAM_HW_ACCEL,
                        minimizeLatency: true,
                        encoder: Encoders.software({
                            x264: { preset: "ultrafast", tune: "zerolatency" },
                            x265: { preset: "ultrafast", tune: "zerolatency" },
                        }),
                    }, signal);

                    await playStream(output, streamer, {
                        type: "go-live",
                    }, signal);
                });

                console.log(`[play] Stream ended for "${title}"`);
            } catch (err) {
                if (signal.aborted) {
                    console.log(`[play] Stream aborted for "${title}"`);
                } else {
                    newrelic.noticeError(err as Error);
                    console.error(`[play] Stream error for "${title}":`, err);
                }
            }
        } catch (err) {
            newrelic.noticeError(err as Error);
            console.error(`[play] Unexpected error for "${title}":`, err);
            await handleStop();
        }
    });
}

async function stopStream() {
    return newrelic.startWebTransaction("handle-stop-stream", async () => {
        if (currentAbortController) {
            currentAbortController.abort();
            currentAbortController = null;
        }

        try {
            streamer.stopStream();
        } catch {
            // ignore if no active stream
        }

        setIdleStatus();
        await sleep(100);
    });
}

async function handleStop() {
    return newrelic.startWebTransaction("handle-stop", async () => {
        console.log("[stop] Stopping playback");
        await stopStream();

        try {
            streamer.leaveVoice();
        } catch {
            // ignore
        }
        inVoiceChannel = false;

        console.log("[stop] Playback stopped");
    });
}

// --- Redis ---
const pubClient = new Redis({
    host: REDIS_HOST,
    port: REDIS_PORT,
    password: REDIS_PASSWORD || undefined,
    retryStrategy: (times) => {
        if (times >= 10) return null;
        return Math.min(times * 1000, 30000);
    },
});

const subClient = pubClient.duplicate({ enableReadyCheck: false });

pubClient.on("connect", () => console.log("[redis] Connected (pub)"));
pubClient.on("error", (err) => console.error("[redis] Pub client error:", err));
subClient.on("connect", () => console.log("[redis] Connected (sub)"));
subClient.on("error", (err) => console.error("[redis] Sub client error:", err));

subClient.subscribe(REDIS_CHANNEL, (err) => {
    if (err) {
        console.error(`[redis] Failed to subscribe to ${REDIS_CHANNEL}:`, err);
        return;
    }
    console.log(`[redis] Subscribed to channel: ${REDIS_CHANNEL}`);
});

subClient.on("message", async (_channel: string, raw: string) => {
    await newrelic.startBackgroundTransaction("handle-message", "Redis", async () => {
        let msg: RedisMessage;
        try {
            msg = JSON.parse(raw);
        } catch {
            console.error("[redis] Failed to parse message:", raw.substring(0, 100));
            return;
        }

        if (!msg.command) {
            console.error("[redis] Message missing command field");
            return;
        }

        newrelic.addCustomAttribute("command", msg.command);
        if (msg.title) newrelic.addCustomAttribute("title", msg.title);
        if (msg.url) newrelic.addCustomAttribute("url", msg.url);

        console.log(`[redis] Received command: ${msg.command}`, {
            title: msg.title,
            url: msg.url?.substring(0, 50),
            channel: msg.voice_channel_id,
        });

        try {
            switch (msg.command.toLowerCase()) {
                case "play":
                    if (!msg.title || !msg.url) {
                        console.error("[play] Missing title or url");
                        return;
                    }
                    await handlePlay(msg.title, msg.url, msg.voice_channel_id);
                    break;

                case "stop":
                    await handleStop();
                    break;

                case "restart":
                    console.log("[restart] Restarting...");
                    process.exit(0);
                    break;

                default:
                    console.warn(`[redis] Unknown command: ${msg.command}`);
            }
        } catch (err) {
            newrelic.noticeError(err as Error);
            console.error("[redis] Error handling message:", err);
        }
    });
});

// --- Shutdown ---
let shuttingDown = false;

async function shutdown(signal: string) {
    if (shuttingDown) return;
    shuttingDown = true;

    console.log(`[shutdown] ${signal} received, shutting down...`);

    try {
        await stopStream();
        streamer.leaveVoice();
    } catch {
        // ignore
    }

    try {
        client.destroy();
    } catch {
        // ignore
    }

    try {
        pubClient.disconnect();
        subClient.disconnect();
    } catch {
        // ignore
    }

    console.log("[shutdown] Done");
    process.exit(0);
}

process.on("SIGTERM", () => shutdown("SIGTERM"));
process.on("SIGINT", () => shutdown("SIGINT"));

process.on("uncaughtException", (err) => {
    console.error("[fatal] Uncaught exception:", err);
});

process.on("unhandledRejection", (reason) => {
    console.error("[fatal] Unhandled rejection:", reason);
});

// --- Start ---
console.log("=== tv-player2 starting ===");
console.log(`Stream: ${STREAM_WIDTH}x${STREAM_HEIGHT}@${STREAM_FPS}fps ${STREAM_VIDEO_CODEC} ${STREAM_BITRATE_KBPS}kbps`);
console.log(`Redis: ${REDIS_HOST}:${REDIS_PORT} channel=${REDIS_CHANNEL}`);
console.log("New Relic monitoring: Enabled");

client.login(TOKEN).catch((err) => {
    console.error("[discord] Login failed:", err);
    process.exit(1);
});

// --- Util ---
function sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
