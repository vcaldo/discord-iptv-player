# Discord IPTV Player

A microservice-based system that enables streaming video content (IPTV, YouTube, or other sources) to Discord voice channels.

## System Architecture

The project consists of the following components:

- **TV Player**: Core service that handles video streaming to Discord voice channels
- **Remote Control**: Discord bot that provides commands for controlling playback
- **Redis**: Message broker for communication between components
- **New Relic Monitoring**: Optional infrastructure for performance monitoring

```
┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │
│  Remote Control │◄────┤   Discord API   │
│  (Discord Bot)  │     │                 │
│                 │     └─────────────────┘
└───────┬─────────┘
        │
        │ Redis PubSub
        ▼
┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │
│    TV Player    │────►│   Discord API   │
│   (Streaming)   │     │                 │
│                 │     └─────────────────┘
└─────────────────┘
```

## Features

- Stream video content from various sources to Discord voice channels
- Control playback via Discord commands
- Support for IPTV playlists (M3U format)
- YouTube video playback support
- Hardware acceleration options for video encoding
- Configurable stream quality settings
- Robust error handling and recovery
- New Relic performance monitoring
- Docker-based deployment

## Prerequisites

- Docker and Docker Compose (for containerized deployment)
- Node.js 16+ and npm/bun (for local development)
- Go 1.19+ (for local development of remote-control)
- Redis server (or use the included Docker container)
- Discord bot token and permissions
- Discord selfbot token (for streaming capability)

## Quick Start (Docker)

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/discord-iptv-player.git
   cd discord-iptv-player
   ```

2. Configure environment:
   - Copy `.env.example` to `.env` and fill in the required variables
   - Optional: Add IPTV channels to `remote-control/playlist.m3u`

3. Build and start with Docker Compose:
   ```bash
   docker-compose up -d
   ```

4. Check the logs to ensure everything is running:
   ```bash
   docker-compose logs -f
   ```

## Manual Setup

### TV Player Component

1. Navigate to the tv-player directory:
   ```bash
   cd tv-player
   ```

2. Install dependencies:
   ```bash
   npm install
   # or using bun
   bun install
   ```

3. Build the application:
   ```bash
   npm run build
   ```

4. Configure the environment:
   - Copy `.env.example` to `.env` for development
   - Or copy `.env.production.example` to `.env.production` for production use
   - Fill in the required variables

5. Start the application:
   ```bash
   # Development mode
   npm run start

   # Production mode
   NODE_ENV=production npm run start
   ```

### Remote Control Component

1. Navigate to the remote-control directory:
   ```bash
   cd remote-control
   ```

2. Build the application:
   ```bash
   go build -o remote-control ./cmd/remote-control
   ```

3. Configure the environment:
   - Create a `.env` file with the required variables
   - Optional: Add IPTV channels to `playlist.m3u`

4. Start the application:
   ```bash
   ./remote-control
   ```

## Configuration

### TV Player Configuration

#### Discord Configuration
- `TOKEN`: Your Discord token (required)
- `GUILD_ID`: ID of the Discord server/guild (required)
- `VIDEO_CHANNEL_ID`: ID of the voice channel for streaming (required)

#### Stream Configuration
- `STREAM_RESPECT_VIDEO_PARAMS`: Whether to respect source video parameters (default: false)
- `STREAM_WIDTH`: Stream width in pixels (default: 1920)
- `STREAM_HEIGHT`: Stream height in pixels (default: 1080)
- `STREAM_FPS`: Stream frames per second (default: 60)
- `STREAM_BITRATE_KBPS`: Streaming bitrate in Kbps (default: 1000)
- `STREAM_MAX_BITRATE_KBPS`: Maximum bitrate in Kbps (default: 4200)
- `STREAM_HARDWARE_ACCELERATION`: Use hardware acceleration (default: false)
- `STREAM_VIDEO_CODEC`: Video codec to use - VP8, VP9, H264, or H265 (default: VP8)

#### Redis Configuration
- `REDIS_HOST`: Redis server hostname (required)
- `REDIS_PORT`: Redis server port (required)
- `REDIS_PASSWORD`: Redis password (optional)
- `REDIS_PUB_SUB_CHANNEL`: Redis pub/sub channel for control messages (default: "iptv")

### Remote Control Configuration

- `DISCORD_TOKEN`: Discord bot token
- `DISCORD_GUILD_ID`: Target Discord guild ID
- `REDIS_ADDR`: Redis server address (host:port)
- `REDIS_PASSWORD`: Redis password (optional)
- `REDIS_PUB_SUB_CHANNEL`: Redis pub/sub channel for control messages
- `PLAYLIST_PATH`: Path to M3U playlist file

## Usage

### Discord Commands

The Remote Control bot provides the following commands:

- `/tv <id>` - Start streaming a video by ID (from the playlist)
- `/stop` - Stop the current video playback
- `/search <term>` - Search channels in the playlist

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.