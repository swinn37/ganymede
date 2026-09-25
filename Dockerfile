ARG TWITCHDOWNLOADER_VERSION="1.56.5"
ARG YT_DLP_VERSION="2026.08.19"
ARG FFMPEG_VERSION="9.0"
ARG FFMPEG_RELEASE="autobuild-2026-09-24-14-14"
ARG FFMPEG_BUILD="n9.0.2-3-ga5923073bf"
ARG FFMPEG_SHA256_LINUX64="ec732ce7498079da2468f0a36db52014b91d89f83279bce778f2332a068cbbd6"
ARG FFMPEG_SHA256_LINUXARM64="bed1fbf7ebaf97b4afda0c40b629327a9d2f8b1d7580f0704f0c7181d4b5cc25"

#
# API Build
#
FROM golang:1.27-bookworm AS build-api
ARG GIT_SHA
ARG GIT_TAG
ENV GIT_SHA=$GIT_SHA
ENV GIT_TAG=$GIT_TAG
RUN echo "GIT_SHA=$GIT_SHA"
RUN echo "GIT_TAG=$GIT_TAG"
RUN apt update && apt install -y make git
WORKDIR /app
COPY . .
RUN make build_server build_worker

#
# Build yt-dlp
#
FROM python:3.14-bookworm AS build-yt-dlp
ARG YT_DLP_VERSION

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential libffi-dev libssl-dev python3-dev zip pandoc \
    && rm -rf /var/lib/apt/lists/*

RUN pip install requests --break-system-packages
# Clone yt-dlp repository
RUN git clone --depth 1 --branch ${YT_DLP_VERSION} https://github.com/yt-dlp/yt-dlp.git /app/yt-dlp
# Copy patch for Twitch Ganymede 
#COPY ganymede_twitch_yt_dlp_git.patch /tmp/ganymede_twitch_yt_dlp_git.patch
WORKDIR /app/yt-dlp
#RUN git apply /tmp/ganymede_twitch_yt_dlp_git.patch
# Build
RUN make

#
# API Tools
#
FROM debian:bookworm-slim AS tools

ARG YT_DLP_VERSION

WORKDIR /tmp
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip git ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# Download TwitchDownloader for the correct platform
ARG TWITCHDOWNLOADER_VERSION
ENV TWITCHDOWNLOADER_URL=https://github.com/lay295/TwitchDownloader/releases/download/${TWITCHDOWNLOADER_VERSION}/TwitchDownloaderCLI-${TWITCHDOWNLOADER_VERSION}-Linux-x64.zip


RUN if [ "$(uname -m)" = "aarch64" ]; then \
    TWITCHDOWNLOADER_URL=https://github.com/lay295/TwitchDownloader/releases/download/${TWITCHDOWNLOADER_VERSION}/TwitchDownloaderCLI-${TWITCHDOWNLOADER_VERSION}-LinuxArm64.zip; \
    fi && \
    echo "Download URL: $TWITCHDOWNLOADER_URL" && \
    curl -L $TWITCHDOWNLOADER_URL -o twitchdownloader.zip && \
    unzip twitchdownloader.zip && \
    rm twitchdownloader.zip

# Install yt-dlp
COPY --from=build-yt-dlp /app/yt-dlp/yt-dlp /usr/local/bin/yt-dlp

#
# FFmpeg (static build, latest stable - Debian's ffmpeg is very old)
# Uses BtbN static builds (linked from ffmpeg.org) to get the latest release.
# Tracks the latest point release of the major version in FFMPEG_VERSION.
#
FROM debian:bookworm-slim AS ffmpeg
ARG FFMPEG_VERSION
ARG FFMPEG_RELEASE
ARG FFMPEG_BUILD
ARG FFMPEG_SHA256_LINUX64
ARG FFMPEG_SHA256_LINUXARM64

WORKDIR /tmp
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl xz-utils ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    ARCH="$(uname -m)"; \
    case "$ARCH" in \
      x86_64) FFMPEG_ARCH="linux64"; EXPECTED_HASH="${FFMPEG_SHA256_LINUX64}" ;; \
      aarch64) FFMPEG_ARCH="linuxarm64"; EXPECTED_HASH="${FFMPEG_SHA256_LINUXARM64}" ;; \
      *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;; \
    esac; \
    FFMPEG_TAR="ffmpeg-${FFMPEG_BUILD}-${FFMPEG_ARCH}-gpl-${FFMPEG_VERSION}.tar.xz"; \
    echo "Downloading ${FFMPEG_TAR} (FFmpeg ${FFMPEG_VERSION}, ${ARCH}) from ${FFMPEG_RELEASE}"; \
    curl -fSL "https://github.com/BtbN/FFmpeg-Builds/releases/download/${FFMPEG_RELEASE}/${FFMPEG_TAR}" -o ffmpeg.tar.xz; \
    if [ -z "$EXPECTED_HASH" ]; then echo "checksum entry not found for ${FFMPEG_TAR}" >&2; exit 1; fi; \
    echo "${EXPECTED_HASH}  ffmpeg.tar.xz" | sha256sum -c -; \
    mkdir -p /tmp/ffmpeg-extract; \
    tar -xJf ffmpeg.tar.xz -C /tmp/ffmpeg-extract --strip-components=1; \
    cp /tmp/ffmpeg-extract/bin/ffmpeg /usr/local/bin/ffmpeg; \
    cp /tmp/ffmpeg-extract/bin/ffprobe /usr/local/bin/ffprobe; \
    chmod +x /usr/local/bin/ffmpeg /usr/local/bin/ffprobe; \
    rm -rf /tmp/ffmpeg.tar.xz /tmp/ffmpeg-extract; \
    ffmpeg -version; \
    ffprobe -version

#
# Frontend base
#
FROM node:26-alpine AS base-frontend

# Install dependencies only when needed
FROM node:26-alpine AS deps

RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json* ./
RUN \
    if [ -f yarn.lock ]; then yarn --frozen-lockfile; \
    elif [ -f package-lock.json ]; then npm ci --force; \
    elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm i --frozen-lockfile; \
    else echo "Lockfile not found." && exit 1; \
    fi

#
# Frontend build
#
FROM node:26-alpine AS build-frontend

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY frontend/. .

ENV NEXT_TELEMETRY_DISABLED=1

RUN \
    if [ -f yarn.lock ]; then yarn run build; \
    elif [ -f package-lock.json ]; then npm run build; \
    elif [ -f pnpm-lock.yaml ]; then corepack enable pnpm && pnpm run build; \
    else echo "Lockfile not found." && exit 1; \
    fi

#
# Tests stage. Includes dependencies required for tests
#
FROM golang:1.27-bookworm AS tests

RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-pip make git libicu72 libfontconfig1

# Copy ffmpeg/ffprobe (latest static build)
COPY --from=ffmpeg /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /usr/local/bin/ffprobe /usr/local/bin/ffprobe

# Setup fonts (if present)
RUN if [ -d /usr/share/fonts ]; then chmod 644 /usr/share/fonts/* && chmod -R a+rX /usr/share/fonts; fi

# Copy TwitchDownloaderCLI
COPY --from=tools /tmp/TwitchDownloaderCLI /usr/local/bin/
RUN chmod +x /usr/local/bin/TwitchDownloaderCLI

# Copy and install yt-dlp
COPY --from=tools /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp

# Production stage (Debian 13: fewer unpatched OS vulnerabilities than Debian 12)
FROM debian:trixie-slim

WORKDIR /opt/app

# Install dependencies, upgrading the base image's packages to their latest security fixes.
# Processes drop privileges with setpriv (util-linux, always installed) rather than gosu.
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends \
    python3 fontconfig tzdata procps supervisor \
    fonts-noto-core fonts-noto-cjk fonts-noto-extra fonts-inter \
    curl ca-certificates libicu76 \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf python3 /usr/bin/python

# Node.js for the frontend: only the runtime, without npm and its bundled dependencies
COPY --from=node:22-trixie-slim /usr/local/bin/node /usr/local/bin/node
RUN node --version

# Setup user
RUN useradd -u 911 -d /data abc && usermod -a -G users abc

# Install yt-dlp
COPY --from=build-yt-dlp /app/yt-dlp/yt-dlp /usr/local/bin/yt-dlp

# Install ffmpeg/ffprobe (latest static build)
COPY --from=ffmpeg /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /usr/local/bin/ffprobe /usr/local/bin/ffprobe

# Setup fonts
RUN chmod 644 /usr/share/fonts/* && chmod -R a+rX /usr/share/fonts

# Copy TwitchDownloaderCLI
COPY --from=tools /tmp/TwitchDownloaderCLI /usr/local/bin/
RUN chmod +x /usr/local/bin/TwitchDownloaderCLI

# Copy api and worker builds
COPY --from=build-api /app/ganymede-api .
COPY --from=build-api /app/ganymede-worker .

# Setup frontend
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
RUN groupadd --system --gid 1001 nodejs
RUN useradd --system --uid 1001 --no-create-home nextjs

COPY --from=build-frontend /app/public ./public

RUN mkdir .next
RUN chown nextjs:nodejs .next

COPY --from=build-frontend --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build-frontend --chown=nextjs:nodejs /app/.next/static ./.next/static
ENV HOSTNAME="0.0.0.0" 


# Setup entrypoint
COPY entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh
COPY bin/supervisord-exit-on-fatal.sh /usr/local/bin/supervisord-exit-on-fatal.sh
RUN chmod +x /usr/local/bin/supervisord-exit-on-fatal.sh
COPY supervisord.conf /opt/app/supervisord.conf

EXPOSE 4000
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
