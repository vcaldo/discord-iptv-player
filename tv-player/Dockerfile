FROM node:22.11-bookworm-slim

WORKDIR /app

# Install system dependencies
RUN apt update && apt install -y ffmpeg procps

# Set proper signal handling for the container
STOPSIGNAL SIGTERM

# Install pnpm
RUN npm install pnpm -g

# Copy package files
COPY package.json ./
COPY package-lock.json ./

# Install dependencies
RUN pnpm install

# Copy source files
COPY src/ ./src/
COPY tsconfig.json ./

# Build the application
RUN pnpm run build

# Run the application directly with Node instead of through pnpm
# to ensure signals are properly forwarded to the Node process
CMD ["node", "--unhandled-rejections=strict", "./dist/index.js"]