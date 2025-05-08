# TV Player

A Discord-based IPTV player component that allows streaming video content to Discord voice channels.

## Installation Steps

1. Use [bun](https://bun.sh) or any other package manager to install all the dependencies:
```
npm install
```

2. Build:
```
npm run build
```

3. Configure the application:
   - Copy `.env.example` to `.env` for development
   - Or copy `.env.production.example` to `.env.production` for production use

## Configuration

The application uses a robust configuration system with environment-specific settings and validation:

- **Environment Detection**: The app automatically detects your environment based on `NODE_ENV` (defaults to "development")
- **Environment-Specific Config**: It first looks for `.env.{environment}` files before falling back to `.env`
- **Validation**: All configuration values are validated on startup to catch misconfiguration early

### Configuration Options

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
- `STREAM_VIDEO_CODEC`: Video codec to use - VP8, VP9, or H264 (default: VP8)

#### Redis Configuration
- `REDIS_HOST`: Redis server hostname (required)
- `REDIS_PORT`: Redis server port (required)
- `REDIS_PASSWORD`: Redis password (required)
- `REDIS_PUB_SUB_CHANNEL`: Redis pub/sub channel for control messages (default: "iptv")

## Usage

Start in development mode:
```
npm run start
```

Start in production mode:
```
NODE_ENV=production npm run start
```

## Environment-Specific Configurations

The application supports different configurations based on your environment:

- **Development**: Higher quality defaults for local testing
- **Production**: More conservative settings optimized for stability
- **Test**: Configured for automated testing

You can create environment-specific files like `.env.production` or `.env.test` to customize settings per environment.
