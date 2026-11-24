# reservoir-lite

[English](README.md) | [简体中文](README_CN.md)

[![CI & Build](https://github.com/QuietSugar/reservoir-lite/actions/workflows/ci.yml/badge.svg)](https://github.com/QuietSugar/reservoir-lite/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

一个专为**静态资源加速**定制的轻量级、零运行时依赖的 MITM（中间人解密）HTTP(S) 缓存正向代理。

> **开源致谢**：本项目基于 Frederik P. 开源的 [F0903/reservoir](https://github.com/F0903/reservoir) 衍生裁剪，遵循 Apache 2.0 许可证。关于上游版本跟踪、缺陷同步与架构差异，详见 [UPSTREAM_SYNC.md](UPSTREAM_SYNC.md)。

---

## 🌟 核心特性

- **纯 Go 构建，零运行时依赖**：
  彻底剥离了上游庞大的 SvelteKit Web 控制台与 SQLite 用户认证数据库，无需 Node.js/pnpm 构建链，无需 CGO，开箱即用。
- **静态资源白名单定向缓存**：
  仅对静态资源扩展名（如 `.js`、`.css`、`.png`、`.jpg`、`.svg`、`.woff2`、`.json` 等）进行去重与本地缓存；所有非静态接口（如动态 API、未命中白名单的文件）直接穿透代理，零缓存开销。
- **HTTPS IP 直连证书信任（IP SAN 支持）**：
  动态签发的 TLS 证书中自动注入了目标 IP 的 Subject Alternative Name (SAN)，使客户端可以通过 `https://192.168.x.x` 直连 IP 且能被操作系统根证书正常信任。
- **更平滑的上游代理通道**：
  默认跳过上游证书验证（`InsecureSkipVerify: true`），内网自签名服务无缝代理；关闭自动压缩并剥离冲突的响应头，彻底避免 gzip/brotli 解压冲突。
- **请求并发合并（Request Coalescing）**：
  当多个客户端并发请求同一个尚未缓存的静态文件时，代理内部会自动将其合并，仅向上游发送单次真实请求，大幅降低源站负载。
- **已加固的高并发稳定性**：
  已合入上游针对文件句柄泄露（FD Leak）以及并发读写 `LastAccess` 数据竞争（Data Race）的修复，满足常驻高并发代理服务的要求。

---

## 📋 运行要求

- **Go 1.24** 或更高版本
- **OpenSSL**（用于生成本地 CA 根证书及私钥）

---

## 🚀 快速开始

### 1. 生成本地 CA 根证书与私钥

代理需要使用本地自签的证书颁发机构（CA）动态签发目标站点的中间人证书。

```bash
mkdir -p ssl
openssl genrsa -out ssl/ca.key 2048
openssl req -x509 -new -nodes -key ssl/ca.key -sha256 -days 3650 -out ssl/ca.crt -subj "//CN=reservoir"
```

> ⚠️ **安全红线**：切勿将 `ssl/ca.key` 提交至任何公共代码仓库！持有 CA 私钥可对信任该 CA 的机器发起中间人攻击。

### 2. 在客户端信任该 CA 根证书

- **Linux**:
  ```bash
  sudo cp ssl/ca.crt /usr/local/share/ca-certificates/reservoir.crt
  sudo update-ca-certificates
  ```
- **Windows**:
  双击 `ssl/ca.crt`，点击“安装证书”，选择安装至“受信任的根证书颁发机构”。
- **Docker / 容器**:
  在容器初始化脚本中将 `ssl/ca.crt` 挂载并加入容器内的 CA 信任列表。

### 3. 编译与运行

```bash
# 编译二进制
make build
# 或者直接：go build -o reservoir main.go

# 启动代理（默认监听 0.0.0.0:9999）
./reservoir --listen 0.0.0.0:9999 --ca-cert ssl/ca.crt --ca-key ssl/ca.key
```

也可以直接通过 make 快速运行：
```bash
make run
```

---

## ⚙️ 配置参数

配置可通过命令行参数或直接编辑 `var/config.json` 文件指定：

| 参数标志 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `--listen` | `0.0.0.0:9999` | 代理监听的地址与端口 |
| `--ca-cert` | `ssl/ca.crt` | 本地 CA 证书路径（PEM 格式） |
| `--ca-key` | `ssl/ca.key` | 本地 CA 私钥路径（PEM 格式） |
| `--cache-dir` | `var/cache/` | 静态资源缓存落盘目录 |
| `--cache-max-age` | `720h` (30天) | 默认静态缓存有效期（如 `720h`, `24h`, `30m`） |
| `--max-cache-size` | `10G` | 最大磁盘缓存占用上限（如 `10G`, `500M`） |
| `--max-mem-cache-size` | `100M` | 内存热缓存上限（如 `100M`, `500M`；设为 `0` 代表仅使用磁盘） |
| `--no-api` | `false` | 关闭内嵌管理 API/控制台 |
| `--log-level` | `INFO` | 日志级别（`DEBUG`, `INFO`, `WARN`, `ERROR`） |
| `--log-to-stdout` | `true` | 是否输出日志到控制台标准输出 |

---

## 🧹 缓存主动清理与管理 (Purge)

当源站更新静态资源时，你可以通过以下多种方式随时主动清理缓存：

### 1. 浏览器控制台界面
直接在浏览器访问 `http://<代理IP>:9999/__purge`，即可打开轻量 Web 管理面板：
- **按 URL 清理**：输入目标资源的完整 URL（如 `https://121.29.88.190:39092/static/js/chunk.js`），实时清除对应缓存。
- **一键清空全部**：点击危险操作按钮一键清除所有落盘缓存并重置占用统计。

### 2. HTTP GET 快捷地址清理 (支持浏览器与脚本)
```bash
# 清空全部缓存
curl http://127.0.0.1:9999/__purge?all=1

# 清理指定 URL 的缓存
curl "http://127.0.0.1:9999/__purge?url=https://121.29.88.190:39092/static/js/app.js"
```

### 3. 标准 HTTP PURGE 请求方法 (走代理)
客户端若已配置代理，可直接发送 `PURGE` 请求：
```bash
curl -X PURGE -x http://127.0.0.1:9999 https://121.29.88.190:39092/static/js/app.js
```

---

## 🔍 故障排查与运行埋点 (Diagnostics & Observability)

程序内置了多维度的运行埋点与诊断机制，发生问题时可通过以下 4 种途径进行快速排查：

### 1. 客户端网络响应头 (F12 / curl)
所有经由代理的响应均会注入诊断标头：
- `X-Cache`: `HIT`（命中缓存）或 `MISS`（回源上游）。
- `X-Cache-Tier`: `RAM`（内存直出）、`DISK`（磁盘读取）或 `UPSTREAM`（网络回源）。
- `Cache-Status`: 标准 RFC 9211 缓存诊断头（如 `reservoir; hit; ttl=2591999` 或 `reservoir; miss; fwd-status=200; stored`）。
- `Age`: 资源已在本地缓存中存活的秒数。
- `Via`: `HTTP/1.1 reservoir`。

### 2. Web 控制台实时排查面板
浏览器访问 `http://<代理IP>:9999/`，在「运行状态与排查埋点」区域可直观看到：
- **网络回源次数** (Upstream Requests)
- **状态码分布** (2xx 成功、4xx 客户端错误、5xx 服务端错误)
- **缓存异常数** (Cache Errors)
- **运行时活跃协程** (Goroutines，排查协程泄露/卡死)

### 3. JSON 监控埋点接口 (`/__stats`)
支持运维监控探针或 Prometheus 采集：
```bash
curl http://127.0.0.1:9999/__stats
```

### 4. 结构化日志排查 (`slog`)
可启用 `--log-level DEBUG` 输出毫秒级链路明细，支持写入指定文件：
```bash
./reservoir --log-level DEBUG --log-file var/proxy.log --log-to-stdout
```

---

## 🧪 自动化测试

```bash
# 运行全部单元测试（含并发竞态检测）
go test -v -race ./...
```

---

## 📄 开源许可证

本项目基于 [Apache License 2.0](LICENSE) 开源。
