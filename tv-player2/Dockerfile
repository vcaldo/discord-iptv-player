FROM node:24-bookworm-slim

WORKDIR /app

RUN apt update && apt install -y ffmpeg procps python3 make g++ && rm -rf /var/lib/apt/lists/*

STOPSIGNAL SIGTERM

RUN npm install pnpm -g

COPY package.json ./

RUN pnpm install

COPY newrelic.js ./
COPY src/ ./src/
COPY tsconfig.json ./

RUN pnpm run build

CMD ["node", "--unhandled-rejections=strict", "./dist/index.js"]
