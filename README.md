# reservoir-lite

[English](README.md) | [简体中文](README_CN.md)

[![CI & Build](https://github.com/QuietSugar/reservoir-lite/actions/workflows/ci.yml/badge.svg)](https://github.com/QuietSugar/reservoir-lite/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A lightweight, zero-dependency caching MITM (Man-in-the-Middle) HTTP(S) forward proxy tailored for static asset acceleration.

> **Attribution**: This project is a specialized lightweight fork of [F0903/reservoir](https://github.com/F0903/reservoir) by Frederik P., licensed under the Apache License 2.0. Repository: [QuietSugar/reservoir-lite](https://github.com/QuietSugar/reservoir-lite). See [UPSTREAM_SYNC.md](UPSTREAM_SYNC.md) for synchronization tracking and divergence details.

---

## Key Features

- **Pure Go & Zero Runtime Dependencies**: Web dashboard and SQLite user database have been stripped out. No Node.js/pnpm build chain or CGO required.
- **Static Asset Whitelist Caching**: Automatically filters and caches static assets (`.js`, `.css`, `.png`, `.jpg`, `.svg`, `.woff2`, `.json`, etc.) while directly bypassing and proxying dynamic requests without caching overhead.
- **HTTPS IP SAN Support**: Generates on-the-fly TLS certificates with IP Subject Alternative Names (SAN), allowing clients to directly trust HTTPS connections to IP addresses.
- **Transparent Upstream Proxying**: Supports upstream self-signed/internal certificates (`InsecureSkipVerify`), avoids gzip/brotli decompression conflicts, and handles hop-by-hop headers transparently.
- **Request Coalescing**: Deduplicates concurrent in-flight requests for the same cacheable resource to reduce upstream load.

---

## Requirements

- **Go 1.24** or newer
- **OpenSSL** (for generating the local CA certificate and private key)

---

## Quick Start

### 1. Generate Local CA Certificate & Key

The proxy uses a local Certificate Authority to dynamically sign certificates for intercepted HTTPS requests.

```bash
mkdir -p ssl
openssl genrsa -out ssl/ca.key 2048
openssl req -x509 -new -nodes -key ssl/ca.key -sha256 -days 3650 -out ssl/ca.crt -subj "//CN=reservoir"
```

> ⚠️ **Security Note**: Never commit `ssl/ca.key` to Git. Keep your CA private key secure.

### 2. Trust the CA Certificate on Clients

- **Linux**:
  ```bash
  sudo cp ssl/ca.crt /usr/local/share/ca-certificates/reservoir.crt
  sudo update-ca-certificates
  ```
- **Windows**: Double-click `ssl/ca.crt` and install it into "Trusted Root Certification Authorities".
- **Docker / Containers**: Mount `ssl/ca.crt` and update CA certificates in container bootstrap scripts.

### 3. Build & Run

```bash
# Build binary
go build -o reservoir main.go

# Run proxy (defaults to 0.0.0.0:9999)
./reservoir --listen 0.0.0.0:9999 --ca-cert ssl/ca.crt --ca-key ssl/ca.key
```

Or run directly via Makefile:
```bash
make run
```

---

## Configuration

Settings can be specified via command-line flags or in `var/config.json`:

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--listen` | `0.0.0.0:9999` | Proxy listen address and port |
| `--ca-cert` | `ssl/ca.crt` | Path to CA certificate (PEM) |
| `--ca-key` | `ssl/ca.key` | Path to CA private key (PEM) |
| `--cache-dir` | `var/cache/` | Directory where cached assets are stored |
| `--cache-max-age` | `720h` (30 days) | Default cache expiration max age (e.g. `720h`, `24h`, `30m`) |
| `--max-cache-size` | `10G` | Maximum cache size on disk (e.g. `10G`, `500M`) |
| `--no-api` | `false` | Disable embedded API and web dashboard |
| `--log-level` | `INFO` | Logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `--log-to-stdout` | `true` | Log to standard output |

---

## 🧹 Cache Purge & Invalidation

When upstream website assets are updated, cache can be invalidated using any of the following methods:

### 1. Interactive Web Interface
Navigate to `http://<proxy-ip>:9999/__purge` in your browser:
- **Purge by URL**: Input target URL to clear its specific cache.
- **Purge All**: Click one button to clear all disk cache and reset byte usage.

### 2. HTTP GET Endpoint
```bash
# Invalidate all cached assets
curl http://127.0.0.1:9999/__purge?all=1

# Invalidate specific URL
curl "http://127.0.0.1:9999/__purge?url=https://121.29.88.190:39092/static/js/app.js"
```

### 3. HTTP PURGE Method (via Proxy)
```bash
curl -X PURGE -x http://127.0.0.1:9999 https://121.29.88.190:39092/static/js/app.js
```

---

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
