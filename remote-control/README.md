# Discord IPTV Player - Remote Control

The Remote Control component is a Discord bot written in Go that provides a user-friendly interface to control the Discord IPTV Player system through slash commands.

## Features

- Discord slash command interface for controlling IPTV playback
- M3U playlist support with automatic parsing
- Channel search functionality with formatted results
- PostgreSQL `LISTEN`/`NOTIFY` messaging for communication with the TV Player service
- New Relic instrumentation for performance monitoring
- Graceful shutdown handling
- Docker support for containerized deployment

## Architecture

The Remote Control component follows a modular architecture:

```
┌───────────────┐      ┌───────────────┐      ┌──────────────┐
│               │      │               │      │              │
│  Discord API  │◄────►│  Remote       │◄────►│  PostgreSQL  │
│               │      │  Control Bot  │      │              │
└───────────────┘      └───────┬───────┘      └──────┬───────┘
                               │                     │
                               │                     │ LISTEN/NOTIFY
                               ▼                     ▼
                       ┌───────────────┐      ┌──────────────┐
                       │               │      │              │
                       │  M3U Playlist │      │  TV Player   │
                       │  Parser       │      │  Service     │
                       │               │      │              │
                       └───────────────┘      └──────────────┘
```

## Prerequisites

- Go 1.26 or higher
- PostgreSQL server
- Discord bot token with proper permissions
- M3U playlist URL for IPTV channels

## Installation

### From Source

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/discord-iptv-player.git
   cd discord-iptv-player/remote-control
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o remote-control ./cmd/remote-control
   ```

### Using Docker

```bash
docker build -t discord-iptv-remote-control .
```

## Configuration

The Remote Control component uses environment variables for configuration. For your convenience, you can use the sample env.example file as a template:

1. Copy the env.example file to create your own .env file:
   ```bash
   cp env.example .env
   ```

2. Edit the .env file and fill in your specific values:
   ```env
   # Discord Bot Configuration
   DISCORD_BOT_TOKEN=your_discord_bot_token
   DISCORD_GUILD_ID=your_discord_guild_id
   DISCORD_VIDEO_CHANNEL_ID=your_discord_video_channel_id

   # User Access Control
   # Comma-separated list of Discord user IDs that are blacklisted from using certain commands
   BLACKLISTED_USERS=123456789012345678,987654321098765432

   # PostgreSQL command bus
   CONTROL_CHANNEL=iptv

   # PostgreSQL storage
   POSTGRES_HOST=localhost
   POSTGRES_PORT=5432
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=postgres
   POSTGRES_DATABASE=discord_iptv_player
   POSTGRES_SSLMODE=disable

   # Playlist Configuration
   PLAYLIST_URL=https://example.com/playlist.m3u
   PLAYLIST_NAME=iptv
   PLAYLIST_MAX_AGE_DAYS=10

   # New Relic Monitoring (optional)
   NEW_RELIC_APP_NAME=Discord IPTV Player - Remote Control
   NEW_RELIC_LICENSE_KEY=your_new_relic_license_key
   ```

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| DISCORD_BOT_TOKEN | Your Discord bot token | Yes | - |
| DISCORD_GUILD_ID | ID of the Discord server | Yes | - |
| DISCORD_VIDEO_CHANNEL_ID | ID of the Discord voice channel | Yes | - |
| BLACKLISTED_USERS | Comma-separated list of Discord user IDs to blacklist from using restricted commands (tv, yt, stop, restart, playlist) | No | - |
| CONTROL_CHANNEL | PostgreSQL `LISTEN`/`NOTIFY` channel for TV Player commands; must match tv-player2 | No | iptv |
| POSTGRES_DSN | PostgreSQL connection string overriding individual connection fields | No | - |
| POSTGRES_HOST | PostgreSQL server hostname | No | localhost |
| POSTGRES_PORT | PostgreSQL server port | No | 5432 |
| POSTGRES_USER | PostgreSQL user | No | postgres |
| POSTGRES_PASSWORD | PostgreSQL password | No | - |
| POSTGRES_DATABASE | PostgreSQL database name | No | discord_iptv_player |
| POSTGRES_SSLMODE | PostgreSQL SSL mode | No | disable |
| PLAYLIST_URL | URL to M3U playlist | Yes | - |
| PLAYLIST_NAME | Name identifier for the playlist | No | iptv |
| PLAYLIST_MAX_AGE_DAYS | Maximum age of cached playlist | No | 10 |
| NEW_RELIC_APP_NAME | Name for New Relic monitoring | No | Discord IPTV Player - Remote Control |
| NEW_RELIC_LICENSE_KEY | New Relic license key | No | - |

## Usage

### Running the Bot

```bash
# From source
./remote-control

# Using Docker
docker run --env-file .env discord-iptv-remote-control
```

### Available Discord Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/tv [channel]` | Play a specific channel by number | `/tv 1` |
| `/stop` | Stop the currently playing channel | `/stop` |
| `/search [name]` | Search for channels containing the given text | `/search news` |

### User Access Control

The bot supports blacklisting specific users from using certain commands. This is useful for restricting access to control commands in larger servers.

#### Blacklisted Commands

The following commands can be restricted through the blacklist:
- `/tv` - Play TV channels
- `/yt` - Play YouTube videos  
- `/stop` - Stop playback
- `/restart` - Restart the bot
- `/playlist` - Switch playlists

Commands like `/search`, `/categories`, `/list`, `/catalog`, and `/csv` are not restricted as they are read-only operations.

#### Configuration

To blacklist users, add their Discord user IDs to the `BLACKLISTED_USERS` environment variable as a comma-separated list:

```env
BLACKLISTED_USERS=123456789012345678,987654321098765432
```

#### Finding Discord User IDs

1. Enable Developer Mode in Discord (User Settings > App Settings > Advanced > Developer Mode)
2. Right-click on a user's name or profile picture
3. Select "Copy User ID"

When a blacklisted user attempts to use a restricted command, they will receive a message: "❌ You are not authorized to use this command."

## Discord Bot Setup

1. Create a new Discord application at the [Discord Developer Portal](https://discord.com/developers/applications)
2. Add a bot to your application and copy the bot token
3. Enable the following Privileged Gateway Intents:
   - Presence Intent
   - Server Members Intent
   - Message Content Intent
4. Generate an OAuth2 URL with the following scopes:
   - `bot`
   - `applications.commands`
5. Add the following bot permissions:
   - Send Messages
   - Use Slash Commands
6. Invite the bot to your server using the generated URL

## M3U Playlist Format

The Remote Control component supports standard M3U playlist format. Example:

```
#EXTM3U
#EXTINF:-1 tvg-id="1" tvg-name="CNN" tvg-logo="https://example.com/cnn.png" group-title="News", CNN
https://example.com/cnn.m3u8
#EXTINF:-1 tvg-id="2" tvg-name="BBC World" tvg-logo="https://example.com/bbc.png" group-title="News", BBC World
https://example.com/bbc.m3u8
```

## Development

### Project Structure

```
remote-control/
├── cmd/
│   └── remote-control/      # Main application entry point
│       └── main.go
├── internal/
│   ├── config/              # Configuration handling
│   ├── discord/             # Discord bot implementation
│   ├── m3u/                 # M3U playlist parsing
│   ├── models/              # Data models
│   └── storage/             # PostgreSQL storage and command bus
├── Dockerfile               # Docker build configuration
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── playlist.m3u             # Example playlist (local fallback)
└── README.md                # This documentation
```

### Building from Source

```bash
# Build for the current platform
go build -o remote-control ./cmd/remote-control

# Build for a specific platform (e.g., Linux)
GOOS=linux GOARCH=amd64 go build -o remote-control ./cmd/remote-control
```

## Troubleshooting

### Common Issues

1. **Bot doesn't respond to commands**
   - Verify the bot token is correct
   - Ensure the bot has the necessary permissions
   - Check if slash commands are registered (may take up to an hour to propagate)

2. **Playlist doesn't load**
   - Verify the playlist URL is accessible
   - Check the M3U format is correct
   - Inspect logs for parsing errors

3. **Cannot connect to PostgreSQL**
   - Verify PostgreSQL is running
   - Check PostgreSQL connection details or `POSTGRES_DSN`
   - Ensure network connectivity between the bot and PostgreSQL

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
