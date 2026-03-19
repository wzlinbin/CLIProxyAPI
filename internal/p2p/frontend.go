package p2p

const frontendHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>P2P Shared API Pool</title>
  <style>
    :root {
      --bg: #08141b;
      --panel: rgba(10, 27, 37, 0.86);
      --panel-strong: rgba(15, 38, 51, 0.94);
      --line: rgba(126, 191, 178, 0.24);
      --text: #ecf5f2;
      --muted: #98b5ae;
      --accent: #82d9c8;
      --warn: #f5af68;
      --danger: #ff7c6d;
      --shadow: 0 22px 60px rgba(0, 0, 0, 0.28);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Segoe UI", "Helvetica Neue", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(130, 217, 200, 0.22), transparent 32%),
        radial-gradient(circle at top right, rgba(255, 124, 109, 0.16), transparent 28%),
        linear-gradient(160deg, #051018 0%, #0a1c26 48%, #102732 100%);
      color: var(--text);
      min-height: 100vh;
    }
    .shell {
      width: min(1160px, calc(100% - 32px));
      margin: 0 auto;
      padding: 28px 0 48px;
    }
    .hero {
      display: grid;
      gap: 20px;
      grid-template-columns: 1.35fr 0.95fr;
      margin-bottom: 22px;
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 24px;
      padding: 24px;
      box-shadow: var(--shadow);
      backdrop-filter: blur(18px);
    }
    h1, h2, h3, p { margin: 0; }
    .eyebrow {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 14px;
      padding: 6px 11px;
      border-radius: 999px;
      border: 1px solid rgba(130, 217, 200, 0.35);
      color: var(--accent);
      font-size: 12px;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    .hero h1 {
      font-size: clamp(32px, 5vw, 52px);
      line-height: 0.95;
      margin-bottom: 14px;
      max-width: 10ch;
    }
    .hero p {
      color: var(--muted);
      line-height: 1.7;
      max-width: 62ch;
    }
    .quick-grid, .stats-grid {
      display: grid;
      gap: 14px;
    }
    .quick-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      margin-top: 18px;
    }
    .stat {
      padding: 18px;
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
    }
    .stat .value {
      font-size: 28px;
      font-weight: 700;
      margin-bottom: 6px;
    }
    .stat .label {
      font-size: 13px;
      color: var(--muted);
    }
    .hero-note {
      display: flex;
      flex-direction: column;
      gap: 14px;
      justify-content: space-between;
      background: linear-gradient(180deg, rgba(18, 44, 58, 0.96), rgba(10, 26, 34, 0.94));
    }
    .hero-note code {
      display: block;
      margin-top: 14px;
      padding: 14px 16px;
      border-radius: 16px;
      background: rgba(4, 12, 16, 0.72);
      border: 1px solid rgba(130, 217, 200, 0.22);
      color: var(--accent);
      overflow-x: auto;
    }
    .layout {
      display: grid;
      gap: 22px;
      grid-template-columns: 1.1fr 0.9fr;
      align-items: start;
    }
    .stack {
      display: grid;
      gap: 22px;
    }
    form {
      display: grid;
      gap: 16px;
      margin-top: 18px;
    }
    .row {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    label {
      display: block;
      margin-bottom: 7px;
      color: var(--muted);
      font-size: 13px;
      font-weight: 600;
      letter-spacing: 0.01em;
    }
    input, select {
      width: 100%;
      border: 1px solid rgba(255, 255, 255, 0.08);
      background: var(--panel-strong);
      color: var(--text);
      padding: 13px 14px;
      border-radius: 14px;
      font-size: 14px;
      outline: none;
    }
    input:focus, select:focus {
      border-color: rgba(130, 217, 200, 0.6);
      box-shadow: 0 0 0 4px rgba(130, 217, 200, 0.1);
    }
    .actions {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      align-items: center;
    }
    button {
      border: 0;
      border-radius: 999px;
      padding: 12px 18px;
      cursor: pointer;
      font-size: 14px;
      font-weight: 700;
      transition: transform 0.18s ease, box-shadow 0.18s ease, opacity 0.18s ease;
    }
    button:hover { transform: translateY(-1px); }
    .primary {
      background: linear-gradient(135deg, #82d9c8, #b3ffcc);
      color: #06212b;
      box-shadow: 0 10px 30px rgba(130, 217, 200, 0.22);
    }
    .secondary {
      background: rgba(255, 255, 255, 0.08);
      color: var(--text);
      border: 1px solid rgba(255, 255, 255, 0.08);
    }
    .notice {
      margin-top: 16px;
      border-radius: 16px;
      padding: 14px 16px;
      font-size: 14px;
      line-height: 1.6;
      display: none;
    }
    .notice.success {
      display: block;
      background: rgba(130, 217, 200, 0.12);
      border: 1px solid rgba(130, 217, 200, 0.3);
      color: #d8fff4;
    }
    .notice.error {
      display: block;
      background: rgba(255, 124, 109, 0.12);
      border: 1px solid rgba(255, 124, 109, 0.3);
      color: #ffd8d4;
    }
    .key-box {
      margin-top: 16px;
      border-radius: 18px;
      padding: 16px;
      background: rgba(4, 12, 16, 0.72);
      border: 1px dashed rgba(130, 217, 200, 0.35);
    }
    .key-box code {
      display: block;
      margin-top: 10px;
      color: var(--accent);
      font-size: 15px;
      word-break: break-all;
    }
    .stats-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      margin-top: 18px;
    }
    .warning {
      margin-top: 16px;
      border-radius: 16px;
      padding: 14px 16px;
      background: rgba(245, 175, 104, 0.1);
      border: 1px solid rgba(245, 175, 104, 0.3);
      color: #ffddb5;
      display: none;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      margin-top: 18px;
      overflow: hidden;
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.02);
    }
    th, td {
      padding: 14px 12px;
      text-align: left;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
      font-size: 13px;
    }
    th {
      color: var(--muted);
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      padding: 5px 10px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
    }
    .badge.verified { background: rgba(130, 217, 200, 0.14); color: var(--accent); }
    .badge.pending { background: rgba(245, 175, 104, 0.14); color: #ffcf96; }
    .badge.failed { background: rgba(255, 124, 109, 0.14); color: #ffb7ae; }
    .tips {
      display: grid;
      gap: 12px;
      margin-top: 18px;
      color: var(--muted);
      font-size: 14px;
      line-height: 1.7;
    }
    .empty {
      color: var(--muted);
      text-align: center;
      padding: 18px 12px;
    }
    @media (max-width: 960px) {
      .hero, .layout { grid-template-columns: 1fr; }
    }
    @media (max-width: 680px) {
      .row, .quick-grid, .stats-grid { grid-template-columns: 1fr; }
      .shell { width: min(100% - 20px, 1160px); }
      .panel { padding: 20px; border-radius: 20px; }
      button { width: 100%; }
      .actions { display: grid; }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="panel">
        <div class="eyebrow">P2P Shared Pool</div>
        <h1>Share capacity. Earn usage. Stay within ratio.</h1>
        <p>
          Register an API credential you control, let the platform verify it, and receive a
          dedicated <code>sk-p2p-*</code> key for the shared request pool. Shared requests go through
          <code>/p2p/v1</code> so they stay isolated from the main proxy traffic.
        </p>
        <div class="quick-grid">
          <div class="stat"><div class="value" id="ovUsers">-</div><div class="label">registered users</div></div>
          <div class="stat"><div class="value" id="ovProviders">-</div><div class="label">verified providers</div></div>
          <div class="stat"><div class="value" id="ovModels">-</div><div class="label">shared models</div></div>
          <div class="stat"><div class="value" id="ovTokens">-</div><div class="label">tokens today</div></div>
        </div>
      </div>
      <div class="panel hero-note">
        <div>
          <div class="eyebrow">Client Base URL</div>
          <h2>Use the shared namespace for your P2P key.</h2>
          <code id="baseUrlBox">/p2p/v1</code>
        </div>
        <div class="tips">
          <div>1. Register a provider and wait for verification.</div>
          <div>2. Use the issued shared key only against the shared namespace.</div>
          <div>3. The hourly guard suspends accounts whose total consumed tokens exceed contributed tokens by 1.2x.</div>
        </div>
      </div>
    </section>

    <section class="layout">
      <div class="stack">
        <div class="panel">
          <div class="eyebrow">Register Provider</div>
          <h2>Add a credential to the pool</h2>
          <form id="registerForm">
            <div class="row">
              <div>
                <label for="email">Email</label>
                <input id="email" type="email" placeholder="optional@example.com">
              </div>
              <div>
                <label for="providerType">Provider type</label>
                <select id="providerType" required>
                  <option value="openai">OpenAI compatible</option>
                  <option value="claude">Claude</option>
                  <option value="gemini">Gemini</option>
                  <option value="codex">Codex</option>
                  <option value="qwen">Qwen</option>
                </select>
              </div>
            </div>
            <div class="row">
              <div>
                <label for="name">Provider label</label>
                <input id="name" type="text" placeholder="Office OpenAI key" required>
              </div>
              <div>
                <label for="baseUrl">Base URL</label>
                <input id="baseUrl" type="text" placeholder="optional custom base URL">
              </div>
            </div>
            <div class="row">
              <div>
                <label for="apiKey">Provider API key</label>
                <input id="apiKey" type="text" placeholder="sk-..." required>
              </div>
              <div>
                <label for="dailyLimit">Daily token limit</label>
                <input id="dailyLimit" type="number" value="1000000" min="0">
              </div>
            </div>
            <div>
              <label for="models">Known models</label>
              <input id="models" type="text" placeholder="optional, comma separated">
            </div>
            <div class="actions">
              <button class="primary" type="submit">Register and verify</button>
              <button class="secondary" type="button" onclick="refreshOverview()">Refresh platform stats</button>
            </div>
          </form>
          <div id="registerNotice" class="notice"></div>
          <div id="issuedKey" class="key-box" style="display:none">
            <strong>Your shared pool key</strong>
            <code id="issuedKeyValue"></code>
            <div class="actions" style="margin-top:12px">
              <button class="secondary" type="button" onclick="copyIssuedKey()">Copy key</button>
              <button class="secondary" type="button" onclick="prefillDashboard()">Load my dashboard</button>
            </div>
          </div>
        </div>

        <div class="panel">
          <div class="eyebrow">Personal Dashboard</div>
          <h2>Inspect your contribution and usage</h2>
          <div class="row" style="margin-top:18px">
            <div style="grid-column:1 / -1">
              <label for="queryKey">Shared pool API key</label>
              <input id="queryKey" type="text" placeholder="sk-p2p-...">
            </div>
          </div>
          <div class="actions" style="margin-top:16px">
            <button class="primary" type="button" onclick="loadDashboard()">Load dashboard</button>
            <button class="secondary" type="button" onclick="loadSharedModels()">List shared models</button>
          </div>
          <div id="dashboardNotice" class="notice"></div>
          <div id="ratioWarning" class="warning">Current usage ratio is above 1.2x. The hourly guard may suspend this account if it stays over the limit.</div>
          <div class="stats-grid">
            <div class="stat"><div class="value" id="statContributed">-</div><div class="label">contributed tokens</div></div>
            <div class="stat"><div class="value" id="statConsumed">-</div><div class="label">consumed tokens</div></div>
            <div class="stat"><div class="value" id="statRatio">-</div><div class="label">total ratio</div></div>
            <div class="stat"><div class="value" id="statActiveProviders">-</div><div class="label">verified providers</div></div>
          </div>
          <table>
            <thead>
              <tr>
                <th>provider</th>
                <th>type</th>
                <th>status</th>
                <th>daily limit</th>
              </tr>
            </thead>
            <tbody id="providersTable">
              <tr><td colspan="4" class="empty">Load your dashboard to see provider status.</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="stack">
        <div class="panel">
          <div class="eyebrow">Shared Models</div>
          <h2>What the pool can serve right now</h2>
          <div class="tips">
            <div>Call <code>GET /p2p/v1/models</code> with your shared key for an OpenAI-style listing.</div>
            <div>Native Gemini-compatible traffic is also available at <code>/p2p/v1beta</code>.</div>
          </div>
          <table>
            <thead>
              <tr>
                <th>model</th>
              </tr>
            </thead>
            <tbody id="modelsTable">
              <tr><td class="empty">No model snapshot loaded yet.</td></tr>
            </tbody>
          </table>
        </div>
        <div class="panel">
          <div class="eyebrow">Notes</div>
          <h2>How the shared pool is enforced</h2>
          <div class="tips">
            <div>Shared traffic is isolated from the main proxy by a dedicated request pool.</div>
            <div>Usage accounting records the consumer key, the serving provider owner, and token totals for each request.</div>
            <div>If a provider fails verification, it stays out of the runtime pool until the next successful sync.</div>
            <div>Accounts are checked every hour against the configured contribution ratio.</div>
          </div>
        </div>
      </div>
    </section>
  </div>

  <script>
    let currentIssuedKey = "";

    function setNotice(id, kind, text) {
      const el = document.getElementById(id);
      el.className = "notice " + kind;
      el.textContent = text;
    }

    function clearNotice(id) {
      const el = document.getElementById(id);
      el.className = "notice";
      el.textContent = "";
    }

    function formatCompact(value) {
      const num = Number(value || 0);
      if (!Number.isFinite(num)) return "-";
      if (num >= 1000000) return (num / 1000000).toFixed(1) + "M";
      if (num >= 1000) return (num / 1000).toFixed(1) + "K";
      return String(num);
    }

    function formatRatio(value) {
      const num = Number(value || 0);
      if (!Number.isFinite(num)) return "-";
      return num.toFixed(2) + "x";
    }

    async function refreshOverview() {
      try {
        const res = await fetch("/p2p/overview");
        const data = await res.json();
        if (!res.ok || data.error) {
          throw new Error(data.error || "Failed to load overview");
        }
        document.getElementById("ovUsers").textContent = formatCompact(data.total_users);
        document.getElementById("ovProviders").textContent = formatCompact(data.verified_providers);
        document.getElementById("ovModels").textContent = formatCompact(data.available_models);
        document.getElementById("ovTokens").textContent = formatCompact(data.today_tokens);
      } catch (err) {
        setNotice("registerNotice", "error", err.message);
      }
    }

    async function loadSharedModels() {
      try {
        const res = await fetch("/p2p/models");
        const data = await res.json();
        if (!res.ok || data.error) {
          throw new Error(data.error || "Failed to load models");
        }
        const tbody = document.getElementById("modelsTable");
        if (!Array.isArray(data.models) || data.models.length === 0) {
          tbody.innerHTML = '<tr><td class="empty">No verified models are currently available.</td></tr>';
          return;
        }
        tbody.innerHTML = data.models.map(model => '<tr><td>' + model + '</td></tr>').join("");
      } catch (err) {
        setNotice("dashboardNotice", "error", err.message);
      }
    }

    async function loadDashboard() {
      clearNotice("dashboardNotice");
      const apiKey = document.getElementById("queryKey").value.trim();
      if (!apiKey) {
        setNotice("dashboardNotice", "error", "Provide a shared pool API key first.");
        return;
      }

      try {
        const res = await fetch("/p2p/info?api_key=" + encodeURIComponent(apiKey));
        const data = await res.json();
        if (!res.ok || data.error) {
          throw new Error(data.error || "Failed to load dashboard");
        }

        document.getElementById("statContributed").textContent = formatCompact(data.stats.total_contributed);
        document.getElementById("statConsumed").textContent = formatCompact(data.stats.total_consumed);
        document.getElementById("statRatio").textContent = formatRatio(data.stats.ratio);
        document.getElementById("statActiveProviders").textContent = formatCompact(data.stats.active_provider_count);
        document.getElementById("ratioWarning").style.display = data.stats.ratio > 1.2 ? "block" : "none";

        const rows = (data.providers || []).map(provider => {
          const badge = "badge " + provider.status;
          return "<tr>" +
            "<td>" + provider.name + "</td>" +
            "<td>" + provider.provider_type + "</td>" +
            "<td><span class='" + badge + "'>" + provider.status + "</span></td>" +
            "<td>" + formatCompact(provider.daily_token_limit) + "</td>" +
          "</tr>";
        });
        document.getElementById("providersTable").innerHTML = rows.length > 0
          ? rows.join("")
          : '<tr><td colspan="4" class="empty">No provider records found for this key.</td></tr>';
      } catch (err) {
        setNotice("dashboardNotice", "error", err.message);
      }
    }

    async function registerProvider(event) {
      event.preventDefault();
      clearNotice("registerNotice");
      const payload = {
        email: document.getElementById("email").value.trim(),
        provider_type: document.getElementById("providerType").value,
        name: document.getElementById("name").value.trim(),
        base_url: document.getElementById("baseUrl").value.trim(),
        api_key: document.getElementById("apiKey").value.trim(),
        models: document.getElementById("models").value
          .split(",")
          .map(item => item.trim())
          .filter(Boolean),
        daily_token_limit: Number(document.getElementById("dailyLimit").value || 0)
      };

      try {
        const res = await fetch("/p2p/register", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (!res.ok || !data.success) {
          throw new Error(data.error || data.message || "Registration failed");
        }

        currentIssuedKey = data.api_key || "";
        document.getElementById("issuedKeyValue").textContent = currentIssuedKey;
        document.getElementById("issuedKey").style.display = currentIssuedKey ? "block" : "none";
        setNotice("registerNotice", "success", data.message || "Provider registered and queued for verification.");
        document.getElementById("registerForm").reset();
        document.getElementById("dailyLimit").value = "1000000";
        refreshOverview();
      } catch (err) {
        setNotice("registerNotice", "error", err.message);
      }
    }

    function copyIssuedKey() {
      if (!currentIssuedKey) return;
      navigator.clipboard.writeText(currentIssuedKey).then(() => {
        setNotice("registerNotice", "success", "Shared pool key copied to clipboard.");
      });
    }

    function prefillDashboard() {
      if (!currentIssuedKey) return;
      document.getElementById("queryKey").value = currentIssuedKey;
      loadDashboard();
    }

    document.getElementById("registerForm").addEventListener("submit", registerProvider);
    document.getElementById("baseUrlBox").textContent = window.location.origin + "/p2p/v1";
    refreshOverview();
    loadSharedModels();
  </script>
</body>
</html>
`
