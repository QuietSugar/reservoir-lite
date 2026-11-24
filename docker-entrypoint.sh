#!/bin/sh
set -e

# Ensure directories exist
mkdir -p /app/ssl /app/var/cache

# Generate self-signed CA if not already present
if [ ! -f /app/ssl/ca.crt ] || [ ! -f /app/ssl/ca.key ]; then
    echo "==> [Reservoir] No CA certificate found in /app/ssl."
    echo "==> [Reservoir] Generating self-signed root CA certificate & private key (valid for 10 years)..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout /app/ssl/ca.key \
        -out /app/ssl/ca.crt \
        -days 3650 \
        -subj "/CN=Reservoir Root CA/O=Reservoir/C=CN" \
        -addext "basicConstraints=critical,CA:TRUE"
    chmod 600 /app/ssl/ca.key
    chmod 644 /app/ssl/ca.crt
    echo "==> [Reservoir] Root CA generated successfully at /app/ssl/ca.crt"
    echo "==> [Reservoir] You can export this ca.crt to your client / browser / system trust store."
else
    echo "==> [Reservoir] Using existing CA certificate from /app/ssl/ca.crt"
fi

# If first arg is a flag or no args provided, run reservoir
if [ $# -eq 0 ] || [ "${1#-}" != "$1" ]; then
    EXTRA_ARGS=""

    # Pass optional environment variables if not already specified in args
    if [ -n "$MAX_MEM_CACHE_SIZE" ]; then
        EXTRA_ARGS="$EXTRA_ARGS --max-mem-cache-size $MAX_MEM_CACHE_SIZE"
    fi
    if [ -n "$MAX_CACHE_SIZE" ]; then
        EXTRA_ARGS="$EXTRA_ARGS --max-cache-size $MAX_CACHE_SIZE"
    fi
    if [ -n "$CACHE_MAX_AGE" ]; then
        EXTRA_ARGS="$EXTRA_ARGS --cache-max-age $CACHE_MAX_AGE"
    fi
    if [ -n "$LOG_LEVEL" ]; then
        EXTRA_ARGS="$EXTRA_ARGS --log-level $LOG_LEVEL"
    fi

    exec /app/reservoir "$@" $EXTRA_ARGS
fi

exec "$@"
