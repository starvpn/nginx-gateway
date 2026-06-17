#!/bin/bash
set -euo pipefail

# init OpenResty-compatible nginx config dir
if [ "$(ls -A /etc/nginx 2>/dev/null)" = "" ]; then
    echo "Initializing OpenResty/Nginx config dir"
    cp -rp /etc/nginx.orig/* /etc/nginx/
    echo "Initialized OpenResty/Nginx config dir"
fi

mkdir -p /etc/nginx/conf.d \
    /etc/nginx/sites-available \
    /etc/nginx/sites-enabled \
    /etc/nginx/streams-available \
    /etc/nginx/streams-enabled \
    /var/log/nginx \
    /var/cache/nginx

# Start OpenResty through the nginx-compatible symlink. If it is already
# running, keep the script idempotent for supervisor restarts.
if nginx -t; then
    nginx || echo "OpenResty/Nginx is already running or could not be started twice"
else
    echo "OpenResty/Nginx config test failed" >&2
    exit 1
fi
