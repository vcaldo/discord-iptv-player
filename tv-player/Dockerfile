FROM node:22.11-bookworm-slim

WORKDIR /app

RUN npm install pnpm -g

COPY package.json ./

COPY package-lock.json ./

RUN pnpm install

COPY src/ ./src/

COPY tsconfig.json ./

RUN pnpm run build

RUN apt update && apt install -y ffmpeg

CMD ["pnpm", "run", "start"]