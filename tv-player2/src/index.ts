import newrelic from "newrelic";
import { Client, CustomStatus, ActivityOptions } from "discord.js-selfbot-v13";
import { Streamer, prepareStream, playStream, Encoders } from "@dank074/discord-video-stream";
import pg from "pg";
import type { PoolClient } from "pg";

const { Pool } = pg;

// --- Config ---
const TOKEN = requiredEnv("TOKEN");
const GUILD_ID = requiredEnv("GUILD_ID");
const CONTROL_CHANNEL = normalizeControlChannel(process.env.CONTROL_CHANNEL ?? "iptv");
const POSTGRES_DSN = process.env.POSTGRES_DSN ?? "";
const POSTGRES_HOST = process.env.POSTGRES_HOST ?? "postgres";
const POSTGRES_PORT = process.env.POSTGRES_PORT ?? "5432";
const POSTGRES_USER = process.env.POSTGRES_USER ?? "postgres";
const POSTGRES_PASSWORD = process.env.POSTGRES_PASSWORD ?? "";
const POSTGRES_DATABASE = process.env.POSTGRES_DATABASE ?? "discord_iptv_player";
const POSTGRES_SSLMODE = process.env.POSTGRES_SSLMODE ?? "disable";
const POSTGRES_MAX_CONNS = parseInt(process.env.POSTGRES_MAX_CONNS ?? "5", 10);

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

function normalizeControlChannel(value: string): string {
    const channel = value.trim();
    if (!channel) {
        console.error("CONTROL_CHANNEL cannot be empty");
        process.exit(1);
    }
    if (Buffer.byteLength(channel, "utf8") > 63 || !/^[A-Za-z_][A-Za-z0-9_]*$/.test(channel)) {
        console.error(`Invalid CONTROL_CHANNEL: ${value}. Use [A-Za-z_][A-Za-z0-9_]* with at most 63 bytes.`);
        process.exit(1);
    }
    return channel;
}

function quoteIdentifier(identifier: string): string {
    return `"${identifier.replace(/"/g, "\"\"")}"`;
}

function postgresConnectionString(): string {
    if (POSTGRES_DSN.trim() !== "") {
        return POSTGRES_DSN;
    }

    const credentials = POSTGRES_PASSWORD
        ? `${encodeURIComponent(POSTGRES_USER)}:${encodeURIComponent(POSTGRES_PASSWORD)}`
        : encodeURIComponent(POSTGRES_USER);
    return `postgres://${credentials}@${POSTGRES_HOST}:${POSTGRES_PORT}/${encodeURIComponent(POSTGRES_DATABASE)}?sslmode=${encodeURIComponent(POSTGRES_SSLMODE)}`;
}

// --- Types ---
interface ControlMessage {
    command: string;
    title: string;
    url: string;
    voice_channel_id: string;
}

interface ActivePlayback {
    id: number;
    title: string;
    url: string;
    voiceChannelId: string;
    controller: AbortController;
    done: Promise<void>;
}

// --- State ---
const client = new Client();
const streamer = new Streamer(client);
let inVoiceChannel = false;
let commandListener: PoolClient | null = null;
let activePlayback: ActivePlayback | null = null;
let playbackOperation: Promise<void> = Promise.resolve();
let nextPlaybackId = 0;

// Metrics state — periodically flushed to PostgreSQL at key
// `tv_player:state` for the nri-flex monitor. All counters are monotonically
// increasing since process start.
const startedAt = Date.now() / 1000;
let currentTitle = "";
let currentUrl = "";
let currentVoiceChannelId = "";
let streamStartedAt = 0; // epoch seconds while streaming, 0 otherwise
let totalPlays = 0;
let totalStops = 0;
let totalErrors = 0;
let lastCommandAt = 0;
let lastCommand = "";
let lastError = "";

const STATE_KEY = "tv_player:state";
const STATE_TTL_SEC = 30; // collector reads at most every 30s; expire if we crash

// --- Discord ---
client.on("ready", async () => {
    console.log(`[discord] Logged in as ${client.user?.tag}`);
    setIdleStatus();

    // Save bot ID for the remote-control service.
    const botId = client.user?.id;
    if (botId) {
        try {
            await setStorageValue("tv_player_bot_id", botId);
            console.log(`[discord] Bot ID ${botId} saved to PostgreSQL`);
        } catch (err) {
            console.error("[discord] Failed to save bot ID to PostgreSQL:", err);
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
        await enqueuePlaybackOperation(() => startPlayback(title, url, voiceChannelId));
    });
}

function enqueuePlaybackOperation(operation: () => Promise<void>): Promise<void> {
    const next = playbackOperation.then(operation, operation);
    playbackOperation = next.catch(() => undefined);
    return next;
}

function recordStreamError(context: string, err: unknown) {
    newrelic.noticeError(err as Error);
    totalErrors += 1;
    lastError = (err as Error)?.message ?? String(err);
    console.error(context, err);
}

function buildPreparedStream(url: string, signal: AbortSignal) {
    return prepareStream(url, {
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
}

async function startPlayback(title: string, url: string, voiceChannelId: string) {
    try {
        newrelic.addCustomAttribute("videoTitle", title);
        newrelic.addCustomAttribute("videoUrl", url);
        newrelic.addCustomAttribute("voiceChannelId", voiceChannelId);

        if (!client.isReady()) {
            console.warn("[play] Discord client not ready, skipping");
            return;
        }

        const switching = activePlayback !== null;
        console.log(`[play] ${switching ? "Switching to" : "Playing"} "${title}" in channel ${voiceChannelId}`);

        const controller = new AbortController();
        const { signal } = controller;
        const { output } = buildPreparedStream(url, signal);

        if (inVoiceChannel && currentVoiceChannelId !== voiceChannelId) {
            console.log(`[play] Moving voice channel from ${currentVoiceChannelId} to ${voiceChannelId}`);
            await stopStream(false);
            try {
                streamer.leaveVoice();
            } catch {
                // ignore if already disconnected
            }
            inVoiceChannel = false;
        } else if (activePlayback) {
            await stopStream(false);
        }

        if (!inVoiceChannel) {
            console.log(`[play] Joining voice channel ${voiceChannelId}`);
            try {
                await newrelic.startSegment("join-voice-channel", true, async () => {
                    await streamer.joinVoice(GUILD_ID, voiceChannelId);
                    inVoiceChannel = true;
                    console.log("[play] Joined voice channel");
                });
            } catch (err) {
                controller.abort();
                recordStreamError("[play] Failed to join voice channel:", err);
                return;
            }
        }

        totalPlays += 1;
        currentTitle = title;
        currentUrl = url;
        currentVoiceChannelId = voiceChannelId;
        streamStartedAt = Date.now() / 1000;
        setWatchingStatus(title);
        void writeState();

        const id = ++nextPlaybackId;
        const playback: ActivePlayback = {
            id,
            title,
            url,
            voiceChannelId,
            controller,
            done: Promise.resolve(),
        };
        activePlayback = playback;

        playback.done = newrelic.startSegment("start-streaming", true, async () => {
            await playStream(output, streamer, { type: "go-live" }, signal);
        });

        void playback.done
            .then(() => {
                if (signal.aborted) {
                    console.log(`[play] Stream switched away from "${title}"`);
                    return;
                }
                console.log(`[play] Stream ended for "${title}"`);
            })
            .catch((err) => {
                if (signal.aborted) {
                    console.log(`[play] Stream switched away from "${title}"`);
                    return;
                }
                recordStreamError(`[play] Stream error for "${title}":`, err);
            })
            .finally(() => {
                if (activePlayback?.id !== id) return;
                activePlayback = null;
                currentTitle = "";
                currentUrl = "";
                streamStartedAt = 0;
                setIdleStatus();
                void writeState();
            });
    } catch (err) {
        recordStreamError(`[play] Unexpected error for "${title}":`, err);
    }
}

async function stopStream(updateStatus = true) {
    return newrelic.startWebTransaction("handle-stop-stream", async () => {
        const playback = activePlayback;
        activePlayback = null;

        if (playback) {
            playback.controller.abort();
            await playback.done.catch(() => undefined);
        }

        try {
            streamer.stopStream();
        } catch {
            // ignore if no active stream
        }

        if (updateStatus) {
            setIdleStatus();
        }
        await sleep(50);
    });
}

async function handleStop() {
    return newrelic.startWebTransaction("handle-stop", async () => {
        await enqueuePlaybackOperation(async () => {
            console.log("[stop] Stopping playback");
            totalStops += 1;
            await stopStream();

            try {
                streamer.leaveVoice();
            } catch {
                // ignore
            }
            inVoiceChannel = false;
            currentTitle = "";
            currentUrl = "";
            currentVoiceChannelId = "";
            streamStartedAt = 0;
            void writeState();

            console.log("[stop] Playback stopped");
        });
    });
}

// --- PostgreSQL command bus and storage ---
const storagePool = new Pool({
    connectionString: postgresConnectionString(),
    max: POSTGRES_MAX_CONNS,
});

storagePool.on("error", (err) => {
    console.error("[postgres] Pool error:", err);
});

async function startCommandListener(): Promise<void> {
    const listener = await storagePool.connect();
    commandListener = listener;

    listener.on("notification", (notification) => {
        if (notification.channel !== CONTROL_CHANNEL) return;
        void handleControlMessage(notification.payload ?? "");
    });
    listener.on("error", (err) => {
        console.error("[postgres] Command listener error:", err);
        void shutdown("postgres-listener-error", 1);
    });

    try {
        await listener.query(`LISTEN ${quoteIdentifier(CONTROL_CHANNEL)}`);
        console.log(`[postgres] Listening for commands on channel: ${CONTROL_CHANNEL}`);
    } catch (err) {
        commandListener = null;
        listener.release();
        throw err;
    }
}

async function handleControlMessage(raw: string): Promise<void> {
    await newrelic.startBackgroundTransaction("handle-message", "PostgreSQL", async () => {
        let msg: ControlMessage;
        try {
            msg = JSON.parse(raw);
        } catch {
            console.error("[postgres] Failed to parse message:", raw.substring(0, 100));
            return;
        }

        if (!msg.command) {
            console.error("[postgres] Message missing command field");
            return;
        }

        lastCommand = msg.command;
        lastCommandAt = Date.now() / 1000;
        void writeState();

        newrelic.addCustomAttribute("command", msg.command);
        if (msg.title) newrelic.addCustomAttribute("title", msg.title);
        if (msg.url) newrelic.addCustomAttribute("url", msg.url);

        console.log(`[postgres] Received command: ${msg.command}`, {
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

                default:
                    console.warn(`[postgres] Unknown command: ${msg.command}`);
            }
        } catch (err) {
            newrelic.noticeError(err as Error);
            console.error("[postgres] Error handling message:", err);
        }
    });
}

// --- Shutdown ---
let shuttingDown = false;

async function shutdown(signal: string, exitCode = 0) {
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
        commandListener?.release();
        commandListener = null;
    } catch {
        // ignore
    }

    try {
        await storagePool.end();
    } catch {
        // ignore
    }

    console.log("[shutdown] Done");
    process.exit(exitCode);
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
console.log(`PostgreSQL command channel: ${CONTROL_CHANNEL}`);
console.log(`Storage: PostgreSQL`);
console.log("New Relic monitoring: Enabled");

async function initializeStorage(): Promise<void> {
    const maxAttempts = 3;
    let lastError: unknown;
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        try {
            await storagePool.query(`
                CREATE TABLE IF NOT EXISTS key_values (
                    key text PRIMARY KEY,
                    value text NOT NULL,
                    expires_at timestamptz,
                    updated_at timestamptz NOT NULL DEFAULT now()
                )
            `);
            await storagePool.query(`
                CREATE INDEX IF NOT EXISTS key_values_expires_at_idx
                    ON key_values (expires_at) WHERE expires_at IS NOT NULL
            `);
            return;
        } catch (err) {
            lastError = err;
            console.error(`[storage] PostgreSQL initialization attempt ${attempt}/${maxAttempts} failed:`, err);
            if (attempt < maxAttempts) {
                await sleep(attempt * 2000);
            }
        }
    }

    throw lastError;
}

async function setStorageValue(key: string, value: string, ttlSeconds?: number): Promise<void> {
    const expiresAt = ttlSeconds ? new Date(Date.now() + ttlSeconds * 1000) : null;
    await storagePool.query(`
        INSERT INTO key_values (key, value, expires_at, updated_at)
        VALUES ($1, $2, $3, now())
        ON CONFLICT (key) DO UPDATE SET
            value = EXCLUDED.value,
            expires_at = EXCLUDED.expires_at,
            updated_at = EXCLUDED.updated_at
    `, [key, value, expiresAt]);
}

// Periodic state writer for the nri-flex collector. Writes a JSON snapshot at
// `tv_player:state` with TTL so a crashed process doesn't leave stale data.
async function writeState(): Promise<void> {
    try {
        const state = {
            timestamp: Date.now() / 1000,
            start_time: startedAt,
            uptime_sec: Math.max(0, Date.now() / 1000 - startedAt),
            bot_ready: client.isReady(),
            bot_user_id: client.user?.id ?? "",
            bot_user_tag: client.user?.tag ?? "",
            in_voice_channel: inVoiceChannel,
            voice_channel_id: currentVoiceChannelId,
            current_title: currentTitle,
            current_url: currentUrl.substring(0, 256),
            stream_active: streamStartedAt > 0,
            stream_started_at: streamStartedAt,
            stream_uptime_sec: streamStartedAt > 0 ? Math.max(0, Date.now() / 1000 - streamStartedAt) : 0,
            stream_width: STREAM_WIDTH,
            stream_height: STREAM_HEIGHT,
            stream_fps: STREAM_FPS,
            stream_codec: STREAM_VIDEO_CODEC,
            stream_bitrate_kbps: STREAM_BITRATE_KBPS,
            stream_hw_accel: STREAM_HW_ACCEL,
            total_plays: totalPlays,
            total_stops: totalStops,
            total_errors: totalErrors,
            last_command: lastCommand,
            last_command_at: lastCommandAt,
            last_error: lastError.substring(0, 256),
        };
        await setStorageValue(STATE_KEY, JSON.stringify(state), STATE_TTL_SEC);
    } catch (err) {
        // Never crash the bot just because metrics write failed.
        console.error("[metrics] Failed to write state to PostgreSQL:", err);
    }
}

const stateInterval = setInterval(() => {
    void writeState();
}, 5000);
stateInterval.unref?.();

try {
    await initializeStorage();
    await startCommandListener();
    await client.login(TOKEN);
} catch (err) {
    console.error("[startup] Failed to start tv-player2:", err);
    process.exit(1);
}

// --- Util ---
function sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
