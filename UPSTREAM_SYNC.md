# Upstream Synchronization Tracker

This project is a tailored, lightweight fork of [F0903/reservoir](https://github.com/F0903/reservoir) (licensed under Apache-2.0).
This document tracks synchronization baselines, cherry-picked upstream commits, and architectural divergence decisions.

---

## 1. Repositories & Baseline

* **This Repository**: `https://github.com/QuietSugar/reservoir-lite.git`
* **Upstream Repository**: `https://github.com/F0903/reservoir.git`
* **Base Commit**: `0aa736c523b283b28b173a9d81b9ea17ac1517c0` (2025-12-05)

---

## 2. Divergence Strategy & Scope

| Component | Status in this Fork | Reason |
| :--- | :--- | :--- |
| **Web Dashboard (Frontend)** | ❌ Completely Removed | We require zero-dependency operation without Node.js or embedded assets. |
| **User Database & Auth (SQLite)** | ❌ Completely Removed | Standalone, stateless forward caching proxy; no authentication/user state required. |
| **Static Asset Whitelist** | ✨ Added in Fork | Only whitelisted static assets (`.js`, `.css`, images, fonts, `.json`) are deduplicated and cached; non-static requests bypass cache immediately. |
| **IP-based HTTPS Certificates** | ✨ Added in Fork | Dynamic CA certificate generation injects IP SANs so clients can trust direct IP HTTPS endpoints. |
| **Upstream TLS / Transport** | ✨ Added in Fork | `InsecureSkipVerify: true`, disabled compression, and stripped conflict encoding headers to avoid decompression issues. |

---

## 3. Upstream Cherry-pick & Sync Log

| Date | Upstream Commit | Description | Status | Notes |
| :--- | :--- | :--- | :--- | :--- |
| 2025-12-05 | `0aa736c` | Base fork commit | ✅ Merged | Fork origin |
| 2026-11-24 | `aa7ccf8` | Fix file handle leak and race conditions | ✅ Merged | Resolved FD leak in `cache/file_cache.go` and race in singleflight entry handles |
| Planned | `1f3bd60` | Stream downstream while caching simultaneously | ⏳ Evaluation | Useful for large asset first-load latency |
| - | SvelteKit 3 / Node 26 | Dashboard frontend modernization | 🚫 Skipped | Frontend stripped in this fork |
| - | SQLite transactions | Database layer refactor | 🚫 Skipped | Database stripped in this fork |

---

## 4. How to Sync Upstream Changes

1. Fetch latest commits from upstream:
   ```bash
   git fetch upstream
   ```
2. Inspect changes in core packages (`proxy/`, `cache/`):
   ```bash
   git log --oneline main..upstream/main -- proxy/ cache/
   ```
3. Cherry-pick or adapt individual commits:
   ```bash
   git cherry-pick <commit-hash>
   ```
4. Verify tests and build:
   ```bash
   go test ./... && go build -o bin/reservoir main.go
   ```
5. Update this file with the synchronized commit hash and description.
