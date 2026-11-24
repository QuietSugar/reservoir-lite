package proxy

const dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>reservoir-lite 缓存管理控制台</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background: #f4f6f9;
      color: #1a202c;
      padding: 30px 16px;
      line-height: 1.6;
    }
    .container {
      max-width: 760px;
      margin: 0 auto;
    }
    .header {
      background: white;
      border-radius: 12px;
      padding: 24px;
      margin-bottom: 20px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.06);
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;
    }
    .header h1 {
      font-size: 22px;
      font-weight: 700;
      color: #2b6cb0;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .status-badge {
      background: #e6fffa;
      color: #234e52;
      border: 1px solid #b2f5ea;
      padding: 4px 12px;
      border-radius: 20px;
      font-size: 13px;
      font-weight: 600;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .status-dot {
      width: 8px;
      height: 8px;
      background: #38b2ac;
      border-radius: 50%;
      animation: pulse 2s infinite;
    }
    @keyframes pulse {
      0% { transform: scale(0.95); opacity: 0.8; }
      50% { transform: scale(1.2); opacity: 1; }
      100% { transform: scale(0.95); opacity: 0.8; }
    }
    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 16px;
      margin-bottom: 20px;
    }
    .card {
      background: white;
      border-radius: 12px;
      padding: 20px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.06);
      margin-bottom: 20px;
    }
    .stat-card {
      background: white;
      border-radius: 12px;
      padding: 18px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.06);
    }
    .stat-title {
      font-size: 13px;
      color: #718096;
      font-weight: 600;
      margin-bottom: 6px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .stat-value {
      font-size: 24px;
      font-weight: 700;
      color: #2d3748;
      margin-bottom: 8px;
    }
    .stat-meta {
      font-size: 12px;
      color: #a0aec0;
    }
    .progress-bar-bg {
      background: #edf2f7;
      border-radius: 6px;
      height: 8px;
      overflow: hidden;
      margin-top: 8px;
    }
    .progress-bar-fill {
      background: #4299e1;
      height: 100%;
      width: 0%;
      transition: width 0.4s ease;
    }
    .progress-bar-fill.warning {
      background: #ed8936;
    }
    h2 {
      font-size: 17px;
      font-weight: 600;
      color: #2d3748;
      margin-bottom: 14px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .btn-group {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }
    button {
      padding: 12px 20px;
      border: none;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      transition: all 0.2s ease;
    }
    button:hover {
      opacity: 0.9;
      transform: translateY(-1px);
    }
    button:active {
      transform: translateY(0);
    }
    .btn-danger {
      background: #e53e3e;
      color: white;
      box-shadow: 0 2px 6px rgba(229, 62, 62, 0.3);
    }
    .btn-warning {
      background: #dd6b20;
      color: white;
      box-shadow: 0 2px 6px rgba(221, 107, 32, 0.3);
    }
    .btn-primary {
      background: #3182ce;
      color: white;
      box-shadow: 0 2px 6px rgba(49, 130, 206, 0.3);
    }
    .btn-secondary {
      background: #e2e8f0;
      color: #4a5568;
    }
    .input-row {
      display: flex;
      gap: 10px;
      margin-top: 10px;
    }
    input[type=text] {
      flex: 1;
      padding: 12px 14px;
      border: 1px solid #cbd5e0;
      border-radius: 8px;
      font-size: 14px;
      outline: none;
      transition: border 0.2s;
    }
    input[type=text]:focus {
      border-color: #3182ce;
      box-shadow: 0 0 0 3px rgba(66, 153, 225, 0.2);
    }
    .alert-box {
      display: none;
      margin-top: 16px;
      padding: 14px 18px;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 500;
      line-height: 1.5;
      animation: fadeIn 0.3s;
    }
    @keyframes fadeIn {
      from { opacity: 0; transform: translateY(-4px); }
      to { opacity: 1; transform: translateY(0); }
    }
    .alert-success {
      background: #c6f6d5;
      color: #22543d;
      border: 1px solid #9ae6b4;
    }
    .alert-error {
      background: #fed7d7;
      color: #742a2a;
      border: 1px solid #feb2b2;
    }
    .tips {
      font-size: 13px;
      color: #718096;
      margin-top: 8px;
    }
    .quick-links {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      margin-top: 8px;
    }
    .tag {
      background: #edf2f7;
      color: #4a5568;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 12px;
      cursor: pointer;
      user-select: none;
    }
    .tag:hover {
      background: #e2e8f0;
      color: #2b6cb0;
    }
  </style>
</head>
<body>
  <div class="container">
    <!-- 顶部状态栏 -->
    <div class="header">
      <h1>🌊 reservoir-lite 控制台</h1>
      <div class="status-badge">
        <span class="status-dot"></span>
        <span>运行中 :9999</span>
      </div>
    </div>

    <!-- 实时容量与命中指标卡片 -->
    <div class="stats-grid">
      <!-- 内存缓存卡片 -->
      <div class="stat-card">
        <div class="stat-title">🧠 内存缓存 (L1 RAM)</div>
        <div class="stat-value" id="memUsageText">-- MB</div>
        <div class="stat-meta" id="memLimitText">上限: -- MB</div>
        <div class="progress-bar-bg">
          <div class="progress-bar-fill" id="memProgressBar"></div>
        </div>
        <div class="stat-meta" style="margin-top: 6px;" id="memHitsText">RAM 命中: -- 次</div>
      </div>

      <!-- 磁盘缓存卡片 -->
      <div class="stat-card">
        <div class="stat-title">💾 磁盘缓存 (L2 Disk)</div>
        <div class="stat-value" id="diskUsageText">-- MB</div>
        <div class="stat-meta" id="diskEntriesText">已存文件: -- 个</div>
        <div class="progress-bar-bg">
          <div class="progress-bar-fill" id="diskProgressBar" style="background: #48bb78;"></div>
        </div>
        <div class="stat-meta" style="margin-top: 6px;" id="totalHitsText">总命中: -- 次</div>
      </div>
    </div>

    <!-- 一键清理操作区（无需输入任何命令） -->
    <div class="card">
      <h2>⚡ 一键清理缓存</h2>
      <p style="color: #718096; font-size: 14px; margin-bottom: 16px;">
        点击以下按钮即可直接清空缓存，操作完成后页面将自动刷新实时数据：
      </p>
      <div class="btn-group">
        <button class="btn-danger" onclick="triggerPurgeAll()">
          🗑️ 一键清空全部缓存 (内存 + 磁盘)
        </button>
        <button class="btn-warning" onclick="triggerPurgeMemOnly()">
          ⚡ 仅清空内存缓存 (释放 RAM)
        </button>
        <button class="btn-secondary" onclick="loadStats()">
          🔄 刷新统计数据
        </button>
      </div>

      <!-- 操作结果提示框 -->
      <div id="actionAlert" class="alert-box"></div>
    </div>

    <!-- 按 URL 单独清理 -->
    <div class="card">
      <h2>🎯 按指定 URL 精确清理</h2>
      <p style="color: #718096; font-size: 14px;">
        输入或粘贴目标静态资源的完整地址，点击按钮即可从缓存中删除该单个资源：
      </p>
      <div class="input-row">
        <input type="text" id="targetUrlInput" placeholder="https://121.29.88.190:39092/static/js/chunk-vendors.d3883a48.js">
        <button class="btn-primary" onclick="triggerPurgeUrl()">清理此资源</button>
      </div>
      <div class="tips">快捷填入示例：</div>
      <div class="quick-links">
        <span class="tag" onclick="fillUrl('https://121.29.88.190:39092/static/js/chunk-vendors.d3883a48.js')">chunk-vendors.js (5MB)</span>
        <span class="tag" onclick="fillUrl('https://121.29.88.190:39092/static/img/0924.405d629e.mp4')">0924.mp4 (34.3MB)</span>
        <span class="tag" onclick="fillUrl('https://121.29.88.190:39092/static/fonts/element-icons.535825a5.ttf')">element-icons.ttf</span>
      </div>
    </div>
  </div>

  <script>
    function formatBytes(bytes) {
      if (bytes === 0) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
    }

    async function loadStats() {
      try {
        const res = await fetch('/__stats');
        if (!res.ok) return;
        const data = await res.json();

        // 内存信息
        const memUsedMB = (data.mem_used_bytes / (1024 * 1024)).toFixed(1);
        const memMaxMB = (data.mem_max_bytes / (1024 * 1024)).toFixed(1);
        document.getElementById('memUsageText').textContent = memUsedMB + ' MB';
        
        if (data.mem_enabled) {
          document.getElementById('memLimitText').textContent = '上限: ' + memMaxMB + ' MB';
          const pct = Math.min(100, Math.round((data.mem_used_bytes / data.mem_max_bytes) * 100)) || 0;
          const bar = document.getElementById('memProgressBar');
          bar.style.width = pct + '%';
          if (pct > 85) bar.className = 'progress-bar-fill warning';
          else bar.className = 'progress-bar-fill';
        } else {
          document.getElementById('memLimitText').textContent = '未启用 (上限 0M)';
          document.getElementById('memProgressBar').style.width = '0%';
        }
        document.getElementById('memHitsText').textContent = 'RAM 命中: ' + (data.mem_cache_hits || 0) + ' 次';

        // 磁盘信息
        const diskUsedMB = (data.disk_used_bytes / (1024 * 1024)).toFixed(1);
        const diskMaxGB = (data.disk_max_bytes / (1024 * 1024 * 1024)).toFixed(1);
        document.getElementById('diskUsageText').textContent = diskUsedMB + ' MB';
        document.getElementById('diskEntriesText').textContent = '已存文件: ' + (data.entries_count || 0) + ' 个 (上限 ' + diskMaxGB + ' GB)';
        const diskPct = Math.min(100, Math.round((data.disk_used_bytes / data.disk_max_bytes) * 100)) || 0;
        document.getElementById('diskProgressBar').style.width = diskPct + '%';
        document.getElementById('totalHitsText').textContent = '总命中: ' + (data.cache_hits || 0) + ' 次';
      } catch (e) {
        console.error('加载统计失败', e);
      }
    }

    function showAlert(msg, isSuccess) {
      const box = document.getElementById('actionAlert');
      box.style.display = 'block';
      box.className = 'alert-box ' + (isSuccess ? 'alert-success' : 'alert-error');
      box.textContent = (isSuccess ? '✅ ' : '❌ ') + msg;
    }

    async function triggerPurgeAll() {
      if (!confirm('确定要清空全部静态资源缓存吗？（包括内存与磁盘文件）')) {
        return;
      }
      showAlert('正在清空全部缓存...', true);
      try {
        const res = await fetch('/__purge?all=1');
        const data = await res.json();
        if (res.ok && data.status === 'ok') {
          showAlert(data.message || ('成功清空全部 ' + (data.purged_count || 0) + ' 个缓存资源'), true);
        } else {
          showAlert(data.message || '清理失败', false);
        }
      } catch (err) {
        showAlert('网络请求异常: ' + err.message, false);
      }
      loadStats();
    }

    async function triggerPurgeMemOnly() {
      showAlert('正在释放内存缓存...', true);
      try {
        const res = await fetch('/__purge?mem=1');
        const data = await res.json();
        if (res.ok && data.status === 'ok') {
          showAlert(data.message || '成功释放内存缓存', true);
        } else {
          showAlert(data.message || '释放内存失败', false);
        }
      } catch (err) {
        showAlert('网络请求异常: ' + err.message, false);
      }
      loadStats();
    }

    async function triggerPurgeUrl() {
      const url = document.getElementById('targetUrlInput').value.trim();
      if (!url) {
        alert('请输入需要清理的目标 URL');
        return;
      }
      showAlert('正在清理目标资源: ' + url + ' ...', true);
      try {
        const res = await fetch('/__purge?url=' + encodeURIComponent(url));
        const data = await res.json();
        if (res.ok && data.status === 'ok') {
          showAlert(data.message || ('已成功清除资源: ' + url), true);
        } else {
          showAlert(data.message || ('未找到该资源或清理失败: ' + url), false);
        }
      } catch (err) {
        showAlert('请求异常: ' + err.message, false);
      }
      loadStats();
    }

    function fillUrl(url) {
      document.getElementById('targetUrlInput').value = url;
    }

    // 页面加载完成后立即拉取统计
    window.addEventListener('DOMContentLoaded', loadStats);
  </script>
</body>
</html>
`
