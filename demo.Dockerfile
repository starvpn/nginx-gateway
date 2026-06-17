# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
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

FROM ${OPENRESTY_IMAGE}
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG S6_OVERLAY_VERSION=3.2.1.0
WORKDIR /app
EXPOSE 80

ENV DEBIAN_FRONTEND=noninteractive
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
        /etc/nginx/conf.d \
        /etc/nginx/sites-available \
        /etc/nginx/sites-enabled \
        /etc/nginx/streams-available \
        /etc/nginx/streams-enabled \
        /var/cache/nginx \
        /var/log/nginx \
        /usr/share/nginx; \
    if [ -d /usr/local/openresty/nginx/conf ]; then \
        cp -a /usr/local/openresty/nginx/conf/. /etc/nginx/; \
    fi; \
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

# init config
COPY resources/docker/init-config.up /etc/s6-overlay/s6-rc.d/init-config/up
COPY resources/docker/init-config.sh /etc/s6-overlay/s6-rc.d/init-config/init-config.sh

RUN chmod +x /etc/s6-overlay/s6-rc.d/init-config/init-config.sh && \
    echo 'oneshot' > /etc/s6-overlay/s6-rc.d/init-config/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/init-config && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx/dependencies.d/init-config

# copy demo config
COPY resources/demo/ojbk.me /etc/nginx/sites-available/ojbk.me
COPY ["resources/demo/Prime Sponsor", "/etc/nginx/sites-available/Prime Sponsor"]
RUN ln -s /etc/nginx/sites-available/ojbk.me /etc/nginx/sites-enabled/ojbk.me
RUN ln -s "/etc/nginx/sites-available/Prime Sponsor" \
          "/etc/nginx/sites-enabled/Prime Sponsor"
COPY resources/demo/app.ini /etc/nginx-ui/app.ini
COPY resources/demo/demo.db /etc/nginx-ui/database.db

# register nginx-ui service
COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx-ui/dependencies.d/init-config

# copy OpenResty/Nginx-compatible config
COPY resources/docker/nginx.conf /etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf
COPY resources/demo/stub_status_nginx-ui.conf /etc/nginx/conf.d/stub_status_nginx-ui.conf

# copy nginx-ui executable binary
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf

# recreate access.log and error.log
RUN rm -f /var/log/nginx/access.log && \
    touch /var/log/nginx/access.log && \
    rm -f /var/log/nginx/error.log && \
    touch /var/log/nginx/error.log

ENTRYPOINT ["/init"]
