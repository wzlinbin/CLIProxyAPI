package p2p

const frontendHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>P2P API 共享平台</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); color: #eee; min-height: 100vh; }
        .container { max-width: 1000px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; padding: 40px 0; }
        .header h1 { font-size: 2.2em; margin-bottom: 10px; background: linear-gradient(90deg, #4ecca3, #3db892); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
        .header p { color: #888; }
        .card { background: rgba(22, 33, 62, 0.9); border-radius: 12px; padding: 25px; margin-bottom: 20px; border: 1px solid rgba(78, 204, 163, 0.2); }
        .card h2 { color: #4ecca3; margin-bottom: 20px; font-size: 1.3em; }
        .form-group { margin-bottom: 18px; }
        .form-group label { display: block; margin-bottom: 6px; color: #aaa; font-size: 14px; }
        .form-group input, .form-group select { width: 100%; padding: 12px; border: 1px solid #333; border-radius: 6px; background: #1a1a2e; color: #fff; font-size: 14px; }
        .form-group input:focus, .form-group select:focus { outline: none; border-color: #4ecca3; }
        .form-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; }
        .btn { padding: 12px 24px; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; font-weight: 600; transition: all 0.3s; }
        .btn-primary { background: linear-gradient(90deg, #4ecca3, #3db892); color: #1a1a2e; }
        .btn-primary:hover { transform: translateY(-2px); box-shadow: 0 5px 15px rgba(78, 204, 163, 0.3); }
        .btn-secondary { background: #333; color: #fff; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; margin-bottom: 20px; }
        .stat-item { background: rgba(26, 26, 46, 0.6); padding: 18px; border-radius: 8px; text-align: center; }
        .stat-item .value { font-size: 1.8em; font-weight: bold; color: #4ecca3; }
        .stat-item .label { color: #888; font-size: 12px; margin-top: 4px; }
        .warning { background: rgba(233, 69, 96, 0.15); border: 1px solid #e94560; color: #e94560; padding: 12px; border-radius: 6px; margin-bottom: 15px; }
        .success { background: rgba(78, 204, 163, 0.15); border: 1px solid #4ecca3; color: #4ecca3; padding: 12px; border-radius: 6px; margin-bottom: 15px; }
        .key-display { background: #1a1a2e; padding: 12px; border-radius: 6px; font-family: monospace; word-break: break-all; margin: 12px 0; border: 1px dashed #4ecca3; font-size: 14px; }
        .copy-btn { background: none; border: 1px solid #4ecca3; color: #4ecca3; padding: 4px 10px; border-radius: 4px; cursor: pointer; margin-left: 8px; font-size: 12px; }
        .copy-btn:hover { background: rgba(78, 204, 163, 0.1); }
        table { width: 100%; border-collapse: collapse; margin-top: 12px; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #333; font-size: 13px; }
        th { color: #4ecca3; }
        .status-badge { padding: 3px 10px; border-radius: 12px; font-size: 11px; }
        .status-verified { background: rgba(78, 204, 163, 0.2); color: #4ecca3; }
        .status-pending { background: rgba(240, 165, 0, 0.2); color: #f0a500; }
        .status-failed { background: rgba(233, 69, 96, 0.2); color: #e94560; }
        .hidden { display: none; }
        .info-box { background: rgba(78, 204, 163, 0.1); padding: 15px; border-radius: 6px; margin-top: 20px; }
        .info-box h3 { color: #4ecca3; margin-bottom: 10px; font-size: 14px; }
        .info-box p { color: #aaa; font-size: 13px; line-height: 1.6; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔄 P2P API 共享平台</h1>
            <p>贡献您的 API Key，获取系统访问权限</p>
        </div>
        
        <div id="registerPanel" class="card">
            <h2>📝 注册新账户</h2>
            <form id="registerForm">
                <div class="form-row">
                    <div class="form-group">
                        <label>邮箱 (可选)</label>
                        <input type="email" id="email" placeholder="your@email.com">
                    </div>
                    <div class="form-group">
                        <label>提供商类型 *</label>
                        <select id="providerType" required>
                            <option value="openai">OpenAI</option>
                            <option value="claude">Claude</option>
                            <option value="gemini">Gemini</option>
                            <option value="codex">Codex</option>
                            <option value="qwen">Qwen</option>
                        </select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>配置名称 *</label>
                        <input type="text" id="name" placeholder="我的 OpenAI Key" required>
                    </div>
                    <div class="form-group">
                        <label>Base URL (可选)</label>
                        <input type="text" id="baseUrl" placeholder="留空使用默认">
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label>API Key *</label>
                        <input type="text" id="apiKey" placeholder="sk-..." required>
                    </div>
                    <div class="form-group">
                        <label>每日 Token 限额</label>
                        <input type="number" id="dailyLimit" placeholder="1000000" value="1000000">
                    </div>
                </div>
                <div class="form-group">
                    <label>支持的模型 (留空自动检测)</label>
                    <input type="text" id="models" placeholder="gpt-4o, gpt-4o-mini (逗号分隔)">
                </div>
                <button type="submit" class="btn btn-primary">注册并验证</button>
            </form>
        </div>
        
        <div id="successPanel" class="card hidden">
            <h2>✅ 注册成功！</h2>
            <div class="success">您的配置正在验证中，验证通过后即可使用。</div>
            <p><strong>您的系统 API Key：</strong></p>
            <div class="key-display">
                <span id="userApiKey"></span>
                <button class="copy-btn" onclick="copyKey()">复制</button>
            </div>
            <p style="color: #888; font-size: 13px;">⚠️ 请保存好您的 Key，它只会显示一次。</p>
            <button class="btn btn-secondary" onclick="showDashboard()" style="margin-top: 12px;">查看仪表盘</button>
        </div>
        
        <div id="dashboardPanel" class="card hidden">
            <h2>📊 我的仪表盘</h2>
            <div class="form-group">
                <label>输入您的 API Key 查看详情</label>
                <div style="display: flex; gap: 10px;">
                    <input type="text" id="queryKey" placeholder="sk-p2p-..." style="flex: 1;">
                    <button class="btn btn-primary" onclick="loadDashboard()">查询</button>
                </div>
            </div>
            
            <div id="statsContainer" class="hidden">
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="value" id="statContributed">-</div>
                        <div class="label">贡献 Tokens</div>
                    </div>
                    <div class="stat-item">
                        <div class="value" id="statConsumed">-</div>
                        <div class="label">消耗 Tokens</div>
                    </div>
                    <div class="stat-item">
                        <div class="value" id="statRatio">-</div>
                        <div class="label">使用比例</div>
                    </div>
                    <div class="stat-item">
                        <div class="value" id="statProviders">-</div>
                        <div class="label">活跃提供商</div>
                    </div>
                </div>
                
                <div id="ratioWarning" class="warning hidden">
                    ⚠️ 您的使用量已超过贡献量的 1.2 倍，账户可能被暂停！
                </div>
                
                <h3 style="margin: 20px 0 10px; color: #4ecca3; font-size: 14px;">我的提供商</h3>
                <table>
                    <thead>
                        <tr><th>名称</th><th>类型</th><th>状态</th><th>每日限额</th></tr>
                    </thead>
                    <tbody id="providersTable"></tbody>
                </table>
            </div>
        </div>
        
        <div class="info-box">
            <h3>📖 使用说明</h3>
            <p>1. 注册：提交您拥有的 API Key 和配置信息<br>
            2. 验证：系统会自动验证您的配置有效性<br>
            3. 获取 Key：验证通过后，您将获得一个系统 Key<br>
            4. 使用：使用系统 Key 访问所有可用模型<br>
            5. 公平机制：如果您的使用量 > 贡献量 × 1.2，Key 将自动失效</p>
        </div>
    </div>
    
    <script>
        let currentKey = '';
        
        document.getElementById('registerForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const data = {
                email: document.getElementById('email').value,
                provider_type: document.getElementById('providerType').value,
                name: document.getElementById('name').value,
                base_url: document.getElementById('baseUrl').value,
                api_key: document.getElementById('apiKey').value,
                models: document.getElementById('models').value.split(',').map(s => s.trim()).filter(s => s),
                daily_token_limit: parseInt(document.getElementById('dailyLimit').value) || 1000000
            };
            
            try {
                const res = await fetch('/p2p/register', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                const result = await res.json();
                
                if (result.success) {
                    currentKey = result.api_key;
                    document.getElementById('userApiKey').textContent = result.api_key;
                    document.getElementById('registerPanel').classList.add('hidden');
                    document.getElementById('successPanel').classList.remove('hidden');
                } else {
                    alert('注册失败: ' + (result.error || '未知错误'));
                }
            } catch (err) {
                alert('请求失败: ' + err.message);
            }
        });
        
        function copyKey() {
            navigator.clipboard.writeText(currentKey).then(() => alert('已复制到剪贴板'));
        }
        
        function showDashboard() {
            document.getElementById('successPanel').classList.add('hidden');
            document.getElementById('dashboardPanel').classList.remove('hidden');
            document.getElementById('queryKey').value = currentKey;
            loadDashboard();
        }
        
        async function loadDashboard() {
            const apiKey = document.getElementById('queryKey').value;
            if (!apiKey) { alert('请输入 API Key'); return; }
            
            try {
                const res = await fetch('/p2p/info?api_key=' + encodeURIComponent(apiKey));
                const result = await res.json();
                
                if (result.error) { alert('查询失败: ' + result.error); return; }
                
                document.getElementById('statsContainer').classList.remove('hidden');
                
                document.getElementById('statContributed').textContent = formatNumber(result.stats.total_contributed);
                document.getElementById('statConsumed').textContent = formatNumber(result.stats.total_consumed);
                document.getElementById('statRatio').textContent = result.stats.ratio.toFixed(2) + 'x';
                document.getElementById('statProviders').textContent = result.stats.active_provider_count;
                
                document.getElementById('ratioWarning').classList.toggle('hidden', result.stats.ratio <= 1.2);
                
                const tbody = document.getElementById('providersTable');
                tbody.innerHTML = (result.providers || []).map(p => 
                    '<tr><td>' + p.name + '</td><td>' + p.provider_type + '</td>' +
                    '<td><span class="status-badge status-' + p.status + '">' + p.status + '</span></td>' +
                    '<td>' + formatNumber(p.daily_token_limit) + '</td></tr>'
                ).join('') || '<tr><td colspan="4" style="text-align:center;color:#888">暂无提供商</td></tr>';
            } catch (err) {
                alert('请求失败: ' + err.message);
            }
        }
        
        function formatNumber(num) {
            if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
            if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
            return num;
        }
    </script>
</body>
</html>