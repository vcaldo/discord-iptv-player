import dotenv from "dotenv";
import { z } from "zod";
import path from "path";
import fs from "fs";

// Load environment variables based on NODE_ENV
const NODE_ENV = process.env.NODE_ENV || "development";
console.log(`Running in ${NODE_ENV} environment`);

// Try to load environment-specific .env file first, then fall back to default .env
const envFiles = [
  `.env.${NODE_ENV}`,
  `.env`
];

for (const file of envFiles) {
  const filePath = path.resolve(process.cwd(), file);
  if (fs.existsSync(filePath)) {
    console.log(`Loading environment from ${file}`);
    dotenv.config({ path: filePath });
    break;
  }
}

// Define schema for Discord configuration
const DiscordSchema = z.object({
  token: z.string({
    required_error: "Discord TOKEN is required",
    invalid_type_error: "Discord TOKEN must be a string"
  }).min(1, "Discord TOKEN cannot be empty"),

  guildId: z.string({
    required_error: "GUILD_ID is required",
    invalid_type_error: "GUILD_ID must be a string"
  }).min(1, "GUILD_ID cannot be empty"),
});

// Define schema for stream configuration
const StreamSchema = z.object({
  respect_video_params: z.preprocess(
    (val) => val === "true" || val === "1" || val === "yes" || val === "on",
    z.boolean()
  ).default(false),

  width: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive().default(1920)
  ),

  height: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive().default(1080)
  ),

  fps: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive().default(60)
  ),

  bitrateKbps: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive().default(1000)
  ),

  maxBitrateKbps: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive().default(4200)
  ),

  hardwareAcceleratedDecoding: z.preprocess(
    (val) => val === "true" || val === "1" || val === "yes" || val === "on",
    z.boolean()
  ).default(false),

  videoCodec: z.enum(["VP8", "VP9", "H264", "H265"], {
    invalid_type_error: "Video codec must be one of: VP8, VP9, H264, H265"
  }).default("VP8")
});

// Define schema for Redis configuration
const RedisSchema = z.object({
  host: z.string({
    required_error: "REDIS_HOST is required",
    invalid_type_error: "REDIS_HOST must be a string"
  }).min(1, "REDIS_HOST cannot be empty"),

  port: z.preprocess(
    (val) => parseInt(String(val), 10),
    z.number().int().positive({
      message: "REDIS_PORT must be a positive integer"
    })
  ),

  password: z.string({
    invalid_type_error: "REDIS_PASSWORD must be a string"
  }).optional().default(""), // Make password optional with empty default

  pubSubChannel: z.string().default("iptv")
});

// Combine all schemas into one config schema
const ConfigSchema = z.object({
  discord: DiscordSchema,
  stream: StreamSchema,
  redis: RedisSchema,
  env: z.enum(["development", "production", "test"]).default("development")
});

// Type for our validated config
export type AppConfig = z.infer<typeof ConfigSchema>;

// Function to validate environment variables against schema
function validateConfig(): AppConfig {
  try {
    // Extract environment variables for validation
    const config = {
      discord: {
        token: process.env.TOKEN,
        guildId: process.env.GUILD_ID,
      },
      stream: {
        respect_video_params: process.env.STREAM_RESPECT_VIDEO_PARAMS,
        width: process.env.STREAM_WIDTH,
        height: process.env.STREAM_HEIGHT,
        fps: process.env.STREAM_FPS,
        bitrateKbps: process.env.STREAM_BITRATE_KBPS,
        maxBitrateKbps: process.env.STREAM_MAX_BITRATE_KBPS,
        hardwareAcceleratedDecoding: process.env.STREAM_HARDWARE_ACCELERATION,
        videoCodec: process.env.STREAM_VIDEO_CODEC
      },
      redis: {
        host: process.env.REDIS_HOST,
        port: process.env.REDIS_PORT,
        password: process.env.REDIS_PASSWORD || "", // Default to empty string if not provided
        pubSubChannel: process.env.REDIS_PUB_SUB_CHANNEL
      },
      env: process.env.NODE_ENV as "development" | "production" | "test" | undefined
    };

    // Validate and return the config
    const validatedConfig = ConfigSchema.parse(config);

    // Log validation success
    console.log("Configuration validated successfully");
    return validatedConfig;
  } catch (error) {
    if (error instanceof z.ZodError) {
      // Format and log validation errors
      console.error("Configuration validation failed:");

      error.errors.forEach((err) => {
        console.error(`- ${err.path.join('.')}: ${err.message}`);
      });

      // Exit process with error in production, allow to continue in development with warning
      if (NODE_ENV === "production") {
        console.error("Exiting due to invalid configuration in production environment");
        process.exit(1);
      } else {
        // In development, try to create a safe fallback configuration
        console.warn("Attempting to create fallback configuration in development environment");
        try {
          return createFallbackConfig(error, config);
        } catch (fallbackError) {
          console.error("Failed to create fallback configuration:", fallbackError);
          console.warn("Continuing with potentially invalid configuration in development environment");
        }
      }
    } else {
      console.error("Unexpected error during configuration validation:", error);
    }

    throw error;
  }
}

// Create a fallback configuration when validation fails in development
function createFallbackConfig(error: z.ZodError, originalConfig: any): AppConfig {
  // Start with default configuration
  const fallbackConfig: any = {
    discord: {
      token: originalConfig.discord.token || "dummy-token-for-dev",
      guildId: originalConfig.discord.guildId || "dummy-guild-for-dev",
    },
    stream: {
      respect_video_params: false,
      width: 1280,
      height: 720,
      fps: 30,
      bitrateKbps: 800,
      maxBitrateKbps: 3000,
      hardwareAcceleratedDecoding: false,
      videoCodec: "VP8"
    },
    redis: {
      host: originalConfig.redis.host || "localhost",
      port: originalConfig.redis.port || 6379,
      password: "", // Default to empty string
      pubSubChannel: originalConfig.redis.pubSubChannel || "iptv"
    },
    env: "development"
  };

  // Apply any valid values from the original config
  error.errors.forEach(err => {
    const path = err.path;
    if (path.length === 0) return;

    // Skip the problematic fields, keep the rest
    let current = originalConfig;
    let fallback = fallbackConfig;

    // Navigate to the parent object of the problematic field
    for (let i = 0; i < path.length - 1; i++) {
      const key = path[i];
      current = current[key];
      fallback = fallback[key];

      if (!current) break;
    }

    // Copy over all sibling values that weren't in the error list
    if (current) {
      const lastKey = path[path.length - 1];
      Object.keys(current).forEach(key => {
        if (key !== lastKey && current[key] !== undefined) {
          fallback[key] = current[key];
        }
      });
    }
  });

  console.log("Created fallback configuration for development");
  return fallbackConfig as AppConfig;
}

// Try to validate the config, with graceful fallback in development
let validatedConfig;
try {
  validatedConfig = validateConfig();
} catch (error) {
  if (NODE_ENV !== "production") {
    console.warn("Using default fallback configuration due to validation errors");
    // Ultimate fallback if everything else fails in development
    validatedConfig = {
      discord: {
        token: process.env.TOKEN || "dummy-token",
        guildId: process.env.GUILD_ID || "dummy-guild",
      },
      stream: {
        respect_video_params: false,
        width: 1280,
        height: 720,
        fps: 30,
        bitrateKbps: 800,
        maxBitrateKbps: 3000,
        hardwareAcceleratedDecoding: false,
        videoCodec: "VP8"
      },
      redis: {
        host: process.env.REDIS_HOST || "localhost",
        port: parseInt(process.env.REDIS_PORT || "6379", 10),
        password: "",
        pubSubChannel: process.env.REDIS_PUB_SUB_CHANNEL || "iptv"
      },
      env: "development"
    };
  } else {
    throw error; // In production, we don't want to continue with invalid config
  }
}

// Create a flattened structure for backward compatibility
const config = {
  // Discord options
  token: validatedConfig.discord.token,
  guildId: validatedConfig.discord.guildId,

  // Stream options
  respect_video_params: validatedConfig.stream.respect_video_params,
  width: validatedConfig.stream.width,
  height: validatedConfig.stream.height,
  fps: validatedConfig.stream.fps,
  bitrateKbps: validatedConfig.stream.bitrateKbps,
  maxBitrateKbps: validatedConfig.stream.maxBitrateKbps,
  hardwareAcceleratedDecoding: validatedConfig.stream.hardwareAcceleratedDecoding,
  videoCodec: validatedConfig.stream.videoCodec,

  // Redis options
  redisHost: validatedConfig.redis.host,
  redisPort: validatedConfig.redis.port,
  redisPassword: validatedConfig.redis.password,
  redisPubSubChannel: validatedConfig.redis.pubSubChannel,

  // Helper function to get environment
  isProduction: () => validatedConfig.env === "production",
  isDevelopment: () => validatedConfig.env === "development",
  isTest: () => validatedConfig.env === "test"
};

// Export both the raw validated config and the flattened version
export { validatedConfig };
export default config;