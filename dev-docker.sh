#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

IMAGE_NAME="${IMAGE_NAME:-nginx-ui:dev}"
CONTAINER_NAME="${CONTAINER_NAME:-nginx-ui-dev}"
HTTP_PORT="${HTTP_PORT:-18080}"
HTTPS_PORT="${HTTPS_PORT:-18443}"
TZ="${TZ:-Asia/Shanghai}"
OPENRESTY_IMAGE="${OPENRESTY_IMAGE:-openresty/openresty:bookworm}"
LUA_RESTY_WAF_REF="${LUA_RESTY_WAF_REF:-5e701b6}"
GO_IMAGE="${GO_IMAGE:-golang:1.26.2-bookworm}"
NODE_IMAGE="${NODE_IMAGE:-node:current-bookworm}"
PNPM_VERSION="${PNPM_VERSION:-11.1.1}"
S6_OVERLAY_VERSION="${S6_OVERLAY_VERSION:-3.2.1.0}"
TARGETOS="${TARGETOS:-linux}"
TARGETARCH="${TARGETARCH:-${GOARCH:-}}"
TARGETVARIANT="${TARGETVARIANT:-}"
GOARM="${GOARM:-}"
CGO_ENABLED="${CGO_ENABLED:-1}"
DETACH="${DETACH:-true}"
MOUNT_DOCKER_SOCK="${MOUNT_DOCKER_SOCK:-true}"
NO_CACHE="${NO_CACHE:-false}"
PULL_BASE="${PULL_BASE:-false}"
SKIP_INSTALLATION="${SKIP_INSTALLATION:-true}"
PREDEFINED_USER_NAME="${PREDEFINED_USER_NAME:-admin}"
PREDEFINED_USER_PASSWORD="${PREDEFINED_USER_PASSWORD:-nginx-ui-dev}"

DATA_DIR="${DATA_DIR:-$ROOT_DIR/tmp/docker-dev}"
BUILD_WORK_DIR="${BUILD_WORK_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/nginx-gateway-dev}"
BUILD_CONTEXT="${BUILD_CONTEXT:-$BUILD_WORK_DIR/build-context}"
NGINX_DATA_DIR="${NGINX_DATA_DIR:-$DATA_DIR/nginx}"
NGINX_UI_DATA_DIR="${NGINX_UI_DATA_DIR:-$DATA_DIR/nginx-ui}"

usage() {
    cat <<'USAGE'
Usage: ./dev-docker.sh [extra docker run args...]

Copies the current working tree to a temporary Docker build context, builds the
frontend, generated Go files, backend binary, and runtime image inside Docker,
and then restarts a local development container.

Common overrides:
  HTTP_PORT=8081 ./dev-docker.sh
  HTTPS_PORT=8444 ./dev-docker.sh
  IMAGE_NAME=nginx-ui:test CONTAINER_NAME=nginx-ui-test ./dev-docker.sh
  GO_IMAGE=golang:1.26.2-bookworm NODE_IMAGE=node:current-bookworm ./dev-docker.sh
  OPENRESTY_IMAGE=openresty/openresty:bookworm ./dev-docker.sh
  LUA_RESTY_WAF_REF=5e701b6 ./dev-docker.sh
  PREDEFINED_USER_NAME=admin PREDEFINED_USER_PASSWORD=change-me ./dev-docker.sh
  SKIP_INSTALLATION=false ./dev-docker.sh  # keep first-run install wizard
  NO_CACHE=true ./dev-docker.sh
  DETACH=false ./dev-docker.sh
  ./dev-docker.sh -v /var/www:/var/www
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi

EXTRA_RUN_ARGS=("$@")

log() {
    printf '[dev-docker] %s\n' "$*"
}

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'Missing required command: %s\n' "$1" >&2
        exit 1
    fi
}

detect_target_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            printf 'amd64'
            ;;
        aarch64|arm64)
            printf 'arm64'
            ;;
        armv7*|armv7l)
            printf 'arm'
            ;;
        armv6*|armv6l)
            printf 'arm'
            ;;
        armv5*|armv5l)
            printf 'arm'
            ;;
        riscv64)
            printf 'riscv64'
            ;;
        *)
            printf 'Unsupported architecture: %s\n' "$(uname -m)" >&2
            exit 1
            ;;
    esac
}

detect_goarm() {
    case "$(uname -m)" in
        armv7*|armv7l)
            printf '7'
            ;;
        armv6*|armv6l)
            printf '6'
            ;;
        armv5*|armv5l)
            printf '5'
            ;;
        *)
            printf '7'
            ;;
    esac
}

normalize_bool() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|on)
            printf 'true'
            ;;
        *)
            printf 'false'
            ;;
    esac
}

TARGETARCH="${TARGETARCH:-$(detect_target_arch)}"

if [[ "$TARGETARCH" == "arm" ]]; then
    GOARM="${GOARM:-$(detect_goarm)}"
    TARGETVARIANT="${TARGETVARIANT:-v$GOARM}"
fi

PLATFORM="${PLATFORM:-$TARGETOS/$TARGETARCH${TARGETVARIANT:+/$TARGETVARIANT}}"

if [[ "$OPENRESTY_IMAGE" == "openresty/openresty:bookworm" && "$TARGETARCH" != "amd64" && "$TARGETARCH" != "arm64" ]]; then
    printf 'The default OPENRESTY_IMAGE=%s is published for amd64/arm64 only; set OPENRESTY_IMAGE to a compatible image for %s.\n' "$OPENRESTY_IMAGE" "$PLATFORM" >&2
    exit 1
fi

require_command docker
require_command rsync

log "Preparing Docker build context: $BUILD_CONTEXT"
rm -rf "$BUILD_CONTEXT"
mkdir -p "$BUILD_CONTEXT" "$NGINX_DATA_DIR" "$NGINX_UI_DATA_DIR"
rsync -a --delete \
    --exclude '/tmp/' \
    --exclude '/node_modules/' \
    --exclude '/app/node_modules/' \
    --exclude '/app/dist/' \
    --exclude '/docs/.vitepress/dist/' \
    --exclude '/.go/' \
    "$ROOT_DIR/" "$BUILD_CONTEXT/"

cat > "$BUILD_CONTEXT/.dockerignore" <<'DOCKERIGNORE'
tmp
node_modules
app/node_modules
app/dist
docs/.vitepress/dist
.devcontainer
.idea
.vscode
.go
.pnpm-store
DOCKERIGNORE

cat > "$BUILD_CONTEXT/Dockerfile.dev" <<'DOCKERFILE'
# syntax=docker/dockerfile:1.7
ARG GO_IMAGE=golang:1.26.2-bookworm
ARG NODE_IMAGE=node:current-bookworm
ARG OPENRESTY_IMAGE=openresty/openresty:bookworm
ARG LUA_RESTY_WAF_REF=5e701b6

FROM ${OPENRESTY_IMAGE} AS lua-resty-waf-builder
ARG LUA_RESTY_WAF_REF
RUN apt-get update -y \
    && apt-get install -y --no-install-recommends ca-certificates git make gcc g++ libc6-dev luarocks libpcre3-dev openresty-opm openresty-resty \
    && rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    git clone --recurse-submodules https://github.com/p0pr0ck5/lua-resty-waf.git /tmp/lua-resty-waf; \
    cd /tmp/lua-resty-waf; \
    git checkout "${LUA_RESTY_WAF_REF}"; \
    git submodule update --init --recursive; \
    sed -i 's/sqlparse_map.py fingerprints/sqlparse_map.py fingerprints.txt/' libinjection/src/Makefile; \
    sed -i 's/CFLAGS = -msse2 -msse3 -msse4.1 -O3/CFLAGS = -O3/' lua-aho-corasick/Makefile; \
    sed -i 's/ROCK_DEPS  = "lrexlib-pcre 2.7.2-1" busted luafilesystem/ROCK_DEPS  = "lrexlib-pcre 2.7.2-1"/' Makefile; \
    sed -i 's/ -Werror//g' src/Makefile; \
    sed -i 's/"pattern" : "\\\\import\\\\b"/"pattern" : "\\\\bimport\\\\b"/' rules/42000_xss.json; \
    touch libinjection/src/fingerprints.txt libinjection/src/sqlparse_data.json libinjection/src/libinjection_sqli_data.h; \
    make; \
    make install OPENRESTY_PREFIX=/usr/local/openresty; \
    rm -rf /tmp/lua-resty-waf /root/.cache/luarocks

FROM ${NODE_IMAGE} AS frontend
ARG PNPM_VERSION=11.1.1
WORKDIR /src
RUN npm install -g pnpm@${PNPM_VERSION}
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY app/package.json ./app/package.json
COPY docs/package.json ./docs/package.json
RUN --mount=type=cache,id=nginx-ui-pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm config set store-dir /root/.local/share/pnpm/store && \
    pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM ${GO_IMAGE} AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=
ARG GOARM=
ARG CGO_ENABLED=1
ARG LD_FLAGS=
ARG BUILD_TIME=
WORKDIR /src
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends ca-certificates git && \
    rm -rf /var/lib/apt/lists/*
COPY . .
COPY --from=frontend /src/app/dist ./app/dist
RUN --mount=type=cache,id=nginx-ui-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=nginx-ui-go-build,target=/root/.cache/go-build \
    go generate
RUN --mount=type=cache,id=nginx-ui-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=nginx-ui-go-build,target=/root/.cache/go-build \
    set -eux; \
    mkdir -p /out; \
    build_time="${BUILD_TIME:-$(date +%s)}"; \
    export CGO_ENABLED GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    if [ -n "${GOARM}" ]; then export GOARM; fi; \
    go build -trimpath -tags=jsoniter \
        -ldflags "${LD_FLAGS} -X github.com/0xJacky/Nginx-UI/settings.buildTime=${build_time}" \
        -o /out/nginx-ui -v main.go; \
    chmod +x /out/nginx-ui

FROM ${OPENRESTY_IMAGE}
ARG TARGETARCH
ARG TARGETVARIANT
ARG S6_OVERLAY_VERSION=3.2.1.0
EXPOSE 80 443

ENV DEBIAN_FRONTEND=noninteractive
ENV NGINX_UI_OFFICIAL_DOCKER=true
ENV NGINX_UI_WORKING_DIR=/var/run/
ENV NGINX_UI_NGINX_SBIN_PATH=/usr/local/bin/nginx
ENV NGINX_UI_NGINX_CONFIG_DIR=/etc/nginx
ENV NGINX_UI_NGINX_CONFIG_PATH=/etc/nginx/nginx.conf
ENV NGINX_UI_NGINX_PID_PATH=/var/run/nginx.pid
ENV NGINX_UI_NGINX_ACCESS_LOG_PATH=/var/log/nginx/access.log
ENV NGINX_UI_NGINX_ERROR_LOG_PATH=/var/log/nginx/error.log

RUN apt-get update -y \
    && apt-get install -y --no-install-recommends wget xz-utils logrotate libpcre3 \
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    if ! getent group nginx >/dev/null; then groupadd --system nginx; fi; \
    if ! id nginx >/dev/null 2>&1; then useradd --system --no-create-home --gid nginx --shell /usr/sbin/nologin nginx; fi; \
    if [ -x /usr/local/openresty/nginx/sbin/nginx ]; then \
        ln -sf /usr/local/openresty/nginx/sbin/nginx /usr/local/bin/nginx; \
    elif command -v openresty >/dev/null 2>&1; then \
        ln -sf "$(command -v openresty)" /usr/local/bin/nginx; \
    fi; \
    mkdir -p /usr/local/etc/nginx \
        /etc/nginx \
        /var/cache/nginx \
        /var/log/nginx \
        /usr/local/share/nginx-ui \
        /usr/share/nginx; \
    if [ -d /usr/local/openresty/nginx/conf ]; then \
        cp -a /usr/local/openresty/nginx/conf/. /usr/local/etc/nginx/; \
    fi; \
    mkdir -p /usr/local/etc/nginx/conf.d \
        /usr/local/etc/nginx/sites-available \
        /usr/local/etc/nginx/sites-enabled \
        /usr/local/etc/nginx/streams-available \
        /usr/local/etc/nginx/streams-enabled; \
    if [ -d /usr/local/openresty/nginx/html ] && [ ! -e /usr/share/nginx/html ]; then \
        ln -s /usr/local/openresty/nginx/html /usr/share/nginx/html; \
    fi; \
    rm -rf /usr/local/openresty/nginx/conf; \
    mkdir -p /usr/local/openresty/nginx; \
    ln -s /etc/nginx /usr/local/openresty/nginx/conf

RUN case "${TARGETARCH}/${TARGETVARIANT}" in \
        "amd64/"*) S6_ARCH="x86_64" ;; \
        "arm64/"*) S6_ARCH="aarch64" ;; \
        "arm/v7"*) S6_ARCH="arm" ;; \
        "arm/v6"*) S6_ARCH="arm" ;; \
        "arm/v5"*) S6_ARCH="arm" ;; \
        "riscv64/"*) S6_ARCH="riscv64" ;; \
        *) echo "Unsupported arch: ${TARGETARCH}/${TARGETVARIANT}" && exit 1 ;; \
    esac && \
    wget -O /tmp/s6-overlay-noarch.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
    wget -O /tmp/s6-overlay-${S6_ARCH}.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-${S6_ARCH}.tar.xz && \
    rm -f /tmp/s6-overlay-noarch.tar.xz /tmp/s6-overlay-${S6_ARCH}.tar.xz

# copy lua-resty-waf runtime libraries and hooks
COPY --from=lua-resty-waf-builder /usr/local/openresty/site/ /usr/local/openresty/site/
COPY --from=lua-resty-waf-builder /usr/local/lib/lua/5.1/rex_pcre.so /usr/local/openresty/site/lualib/rex_pcre.so
COPY resources/docker/waf /usr/local/share/nginx-ui/waf
RUN chmod -R a+rX /usr/local/share/nginx-ui/waf

# register nginx-compatible OpenResty service
COPY resources/docker/nginx.run /etc/s6-overlay/s6-rc.d/nginx/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx

COPY resources/docker/init-config.up /etc/s6-overlay/s6-rc.d/init-config/up
COPY resources/docker/init-config.sh /etc/s6-overlay/s6-rc.d/init-config/init-config.sh

RUN chmod +x /etc/s6-overlay/s6-rc.d/init-config/init-config.sh && \
    echo 'oneshot' > /etc/s6-overlay/s6-rc.d/init-config/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/init-config && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx/dependencies.d/init-config

COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d/init-config

COPY resources/docker/nginx.conf /usr/local/etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /usr/local/etc/nginx/conf.d/nginx-ui.conf
COPY resources/docker/nginx-ui.conf.known-hashes /usr/local/share/nginx-ui/nginx-ui.conf.known-hashes
COPY --from=builder /out/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf  \
    && rm -f /usr/local/etc/nginx/conf.d/default.conf

RUN rm -f /var/log/nginx/access.log && \
    touch /var/log/nginx/access.log && \
    rm -f /var/log/nginx/error.log && \
    touch /var/log/nginx/error.log

ENTRYPOINT ["/init"]
DOCKERFILE

log "Building Docker image: $IMAGE_NAME"
build_args=(
    build
    -f "$BUILD_CONTEXT/Dockerfile.dev"
    --platform "$PLATFORM"
    --build-arg "GO_IMAGE=$GO_IMAGE"
    --build-arg "NODE_IMAGE=$NODE_IMAGE"
    --build-arg "PNPM_VERSION=$PNPM_VERSION"
    --build-arg "OPENRESTY_IMAGE=$OPENRESTY_IMAGE"
    --build-arg "LUA_RESTY_WAF_REF=$LUA_RESTY_WAF_REF"
    --build-arg "S6_OVERLAY_VERSION=$S6_OVERLAY_VERSION"
    --build-arg "TARGETOS=$TARGETOS"
    --build-arg "TARGETARCH=$TARGETARCH"
    --build-arg "TARGETVARIANT=$TARGETVARIANT"
    --build-arg "GOARM=$GOARM"
    --build-arg "CGO_ENABLED=$CGO_ENABLED"
    --build-arg "LD_FLAGS=${LD_FLAGS:-}"
    --build-arg "BUILD_TIME=${BUILD_TIME:-$(date +%s)}"
    -t "$IMAGE_NAME"
)

if [[ "$(normalize_bool "$NO_CACHE")" == "true" ]]; then
    build_args+=(--no-cache)
fi

if [[ "$(normalize_bool "$PULL_BASE")" == "true" ]]; then
    build_args+=(--pull)
fi

build_args+=("$BUILD_CONTEXT")
docker "${build_args[@]}"

if [[ -n "$(docker ps -aq -f "name=^/${CONTAINER_NAME}$")" ]]; then
    log "Removing previous container: $CONTAINER_NAME"
    docker rm -f "$CONTAINER_NAME" >/dev/null
fi

log "Starting container: $CONTAINER_NAME"
run_args=(
    run
    -d
    --name "$CONTAINER_NAME"
    --restart unless-stopped
    -e "TZ=$TZ"
    -e "NGINX_UI_NODE_SKIP_INSTALLATION=$SKIP_INSTALLATION"
    -e "NGINX_UI_PREDEFINED_USER_NAME=$PREDEFINED_USER_NAME"
    -e "NGINX_UI_PREDEFINED_USER_PASSWORD=$PREDEFINED_USER_PASSWORD"
    -v "$NGINX_DATA_DIR:/etc/nginx"
    -v "$NGINX_UI_DATA_DIR:/etc/nginx-ui"
    -p "$HTTP_PORT:80"
    -p "$HTTPS_PORT:443"
)

if [[ "$(normalize_bool "$MOUNT_DOCKER_SOCK")" == "true" && -S /var/run/docker.sock ]]; then
    run_args+=(-v /var/run/docker.sock:/var/run/docker.sock)
fi

run_args+=("${EXTRA_RUN_ARGS[@]}" "$IMAGE_NAME")
docker "${run_args[@]}" >/dev/null

log "Nginx UI is starting at http://localhost:$HTTP_PORT"
log "Persistent data: $DATA_DIR"
if [[ "$(normalize_bool "$SKIP_INSTALLATION")" == "true" ]]; then
    log "Development login: $PREDEFINED_USER_NAME / $PREDEFINED_USER_PASSWORD (only seeds an empty database)"
fi

if [[ "$(normalize_bool "$DETACH")" != "true" ]]; then
    log "Following logs. Press Ctrl+C to stop following; the container keeps running."
    docker logs -f "$CONTAINER_NAME"
fi
