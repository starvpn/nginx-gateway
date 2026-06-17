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
RUN apt-get update -y \
    && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
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
    esac \
    && wget -O /tmp/s6-overlay-noarch.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz \
    && tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz \
    && wget -O /tmp/s6-overlay-${S6_ARCH}.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz \
    && tar -C / -Jxpf /tmp/s6-overlay-${S6_ARCH}.tar.xz \
    && rm -f /tmp/s6-overlay-noarch.tar.xz /tmp/s6-overlay-${S6_ARCH}.tar.xz

COPY --from=lua-resty-waf-builder /usr/local/openresty/site/ /usr/local/openresty/site/
COPY --from=lua-resty-waf-builder /usr/local/lib/lua/5.1/rex_pcre.so /usr/local/openresty/site/lualib/rex_pcre.so
COPY resources/docker/waf /usr/local/share/nginx-ui/waf
RUN chmod -R a+rX /usr/local/share/nginx-ui/waf

COPY resources/docker/nginx.run /etc/s6-overlay/s6-rc.d/nginx/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx/type \
    && touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx

COPY resources/docker/init-config.up /etc/s6-overlay/s6-rc.d/init-config/up
COPY resources/docker/init-config.sh /etc/s6-overlay/s6-rc.d/init-config/init-config.sh
RUN chmod +x /etc/s6-overlay/s6-rc.d/init-config/init-config.sh \
    && echo 'oneshot' > /etc/s6-overlay/s6-rc.d/init-config/type \
    && touch /etc/s6-overlay/s6-rc.d/user/contents.d/init-config \
    && mkdir -p /etc/s6-overlay/s6-rc.d/nginx/dependencies.d \
    && touch /etc/s6-overlay/s6-rc.d/nginx/dependencies.d/init-config

COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type \
    && touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui \
    && mkdir -p /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d \
    && touch /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d/init-config

COPY resources/docker/nginx.conf /usr/local/etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /usr/local/etc/nginx/conf.d/nginx-ui.conf
COPY resources/docker/nginx-ui.conf.known-hashes /usr/local/share/nginx-ui/nginx-ui.conf.known-hashes
COPY --from=builder /out/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf \
    && rm -f /usr/local/etc/nginx/conf.d/default.conf

RUN rm -f /var/log/nginx/access.log \
    && touch /var/log/nginx/access.log \
    && rm -f /var/log/nginx/error.log \
    && touch /var/log/nginx/error.log

ENTRYPOINT ["/init"]
