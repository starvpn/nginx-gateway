FROM mcr.microsoft.com/devcontainers/base:noble

# Install OpenResty (nginx-compatible CLI + Lua runtime) and Go for development.
# Keep /etc/nginx as the working config directory so the rest of the project,
# mounted volumes, tests, and settings remain compatible with normal Nginx.
RUN apt-get update && \
  apt-get install -y --no-install-recommends curl ca-certificates jq cloc gnupg2 lsb-release ubuntu-keyring inotify-tools file && \
  \
  # Install OpenResty packages from the official OpenResty repository.
  curl -fsSL https://openresty.org/package/pubkey.gpg | \
  gpg --dearmor -o /usr/share/keyrings/openresty-archive-keyring.gpg && \
  echo "deb [signed-by=/usr/share/keyrings/openresty-archive-keyring.gpg] https://openresty.org/package/ubuntu $(lsb_release -cs) main" > \
  /etc/apt/sources.list.d/openresty.list && \
  apt-get update && \
  apt-get install -y --no-install-recommends openresty && \
  \
  # Provide nginx-compatible user, command, config, log, and html paths.
  if ! getent group nginx >/dev/null; then groupadd --system nginx; fi && \
  if ! id nginx >/dev/null 2>&1; then useradd --system --no-create-home --gid nginx --shell /usr/sbin/nologin nginx; fi && \
  ln -sf /usr/local/openresty/nginx/sbin/nginx /usr/local/bin/nginx && \
  mkdir -p /etc/nginx /etc/nginx.orig /var/cache/nginx /var/log/nginx /usr/share/nginx && \
  cp -a /usr/local/openresty/nginx/conf/. /etc/nginx.orig/ && \
  mkdir -p /etc/nginx.orig/conf.d /etc/nginx.orig/sites-available /etc/nginx.orig/sites-enabled \
    /etc/nginx.orig/streams-available /etc/nginx.orig/streams-enabled && \
  if [ -d /usr/local/openresty/nginx/html ] && [ ! -e /usr/share/nginx/html ]; then \
    ln -s /usr/local/openresty/nginx/html /usr/share/nginx/html; \
  fi && \
  rm -rf /usr/local/openresty/nginx/conf && \
  ln -s /etc/nginx /usr/local/openresty/nginx/conf && \
  \
  # Automatically retrieve the latest stable Go version and install it.
  GO_VERSION=$(curl -sSL "https://golang.org/dl/?mode=json" | \
  jq -r 'map(select(.stable)) | .[0].version' | sed 's/^go//') && \
  ARCH=$(dpkg --print-architecture) && \
  if [ "$ARCH" = "arm64" ]; then \
  GO_ARCH=linux-arm64; \
  else \
  GO_ARCH=linux-amd64; \
  fi && \
  echo "Installing Go version: ${GO_VERSION} for architecture: ${GO_ARCH}" && \
  curl -sSL "https://golang.org/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -o go.tar.gz && \
  rm -rf /usr/local/go && \
  tar -C /usr/local -xzf go.tar.gz && \
  rm go.tar.gz && \
  rm -rf /var/lib/apt/lists/*

# Set PATH to include Go installation, default go install binary location, and OpenResty binaries.
ENV PATH="/usr/local/go/bin:/root/go/bin:/usr/local/openresty/bin:/usr/local/openresty/nginx/sbin:${PATH}"

ENV NGINX_UI_WORKING_DIR=/var/run/
ENV NGINX_UI_NGINX_SBIN_PATH=/usr/local/bin/nginx
ENV NGINX_UI_NGINX_CONFIG_DIR=/etc/nginx
ENV NGINX_UI_NGINX_CONFIG_PATH=/etc/nginx/nginx.conf
ENV NGINX_UI_NGINX_PID_PATH=/var/run/nginx.pid
ENV NGINX_UI_NGINX_ACCESS_LOG_PATH=/var/log/nginx/access.log
ENV NGINX_UI_NGINX_ERROR_LOG_PATH=/var/log/nginx/error.log

# set zsh as default shell
RUN chsh -s $(which zsh)
