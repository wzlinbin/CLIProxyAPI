
      (function () {
        var ROOT_ID = "ops-billing-shell";
        var MENU_ID = "ops-billing-menu-entry";
        var STORAGE_KEY = "opsBilling.managementKey";
        var COST_POOL_KEY = "opsBilling.costPool";
        var DATE_START_KEY = "cli-proxy-usage-date-start";
        var DATE_END_KEY = "cli-proxy-usage-date-end";
        var PALETTE = ["#3f3424","#8d6f3f","#1b8a5d","#b47c2d","#7f5f78","#4e86a6","#c05f47","#7c7a43","#6f5a45"];
        var originalFetch = window.fetch ? window.fetch.bind(window) : null;
        var booted = false;

        var state = {
          usage: null,
          config: null,
          providerMap: {},
          pricing: [],
          pricingMap: {},
          pricingError: "",
          savingPricing: false,
          costMode: "pool",
          displayCostTotal: 0,
          loading: false,
          unauthorized: false,
          error: "",
          managementKey: localStorage.getItem(STORAGE_KEY) || "",
          costPool: Number(localStorage.getItem(COST_POOL_KEY) || "100"),
          loadedRangeSignature: ""
        };

        function escapeHtml(v) { return String(v == null ? "" : v).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;"); }
        function formatNumber(v) { return new Intl.NumberFormat("zh-CN").format(Number.isFinite(v) ? v : 0); }
        function formatCurrency(v) { return new Intl.NumberFormat("zh-CN", { style: "currency", currency: "CNY", minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number.isFinite(v) ? v : 0); }
        function formatCompactTokens(v) { var n = Number(v) || 0; if (n >= 1e6) return (n / 1e6).toFixed(2) + "M"; if (n >= 1e3) return (n / 1e3).toFixed(1) + "K"; return String(n); }
        function formatPercent(n, d) { if (!d) return "0.00%"; return ((n / d) * 100).toFixed(2) + "%"; }
        function formatLocalDate(d) { var year = d.getFullYear(), month = String(d.getMonth() + 1).padStart(2, "0"), day = String(d.getDate()).padStart(2, "0"); return year + "-" + month + "-" + day; }
        function todayStr() { return formatLocalDate(new Date()); }
        function daysAgo(n) { var d = new Date(); d.setDate(d.getDate() - n); return formatLocalDate(d); }
        function getSavedRangeStart() { return localStorage.getItem(DATE_START_KEY) || ""; }
        function getSavedRangeEnd() { return localStorage.getItem(DATE_END_KEY) || ""; }
        function getSavedRangeSignature() { return getSavedRangeStart() + "|" + getSavedRangeEnd(); }
        function saveRange(start, end) {
          if (start) localStorage.setItem(DATE_START_KEY, start); else localStorage.removeItem(DATE_START_KEY);
          if (end) localStorage.setItem(DATE_END_KEY, end); else localStorage.removeItem(DATE_END_KEY);
        }
        function guessQuickRange(start, end) {
          var today = todayStr();
          if (!start && !end) return "all";
          if (start === today && end === today) return "today";
          if (end === today) {
            if (start === daysAgo(7)) return "7d";
            if (start === daysAgo(30)) return "30d";
            if (start === daysAgo(90)) return "90d";
          }
          return "";
        }
        function describeRange(start, end) {
          if (!start && !end) return "全部时间";
          if (start && end) return start === end ? start : start + " 至 " + end;
          if (start) return start + " 起";
          return "截至 " + end;
        }
        function looksLikeSecret(v) { var t = String(v || "").trim(); if (!t) return false; if (/^(sk-|nvapi-|LB-|WZ-|XX-|AI-)/i.test(t)) return true; return t.length >= 24 && !t.includes(" ") && !t.includes("@"); }
        function maskSensitive(v) { var t = String(v || "").trim(); if (!t) return "-"; if (t.includes("@")) { var p = t.split("@"), name = p[0] || "", domain = p[1] || "", masked = name.length <= 4 ? (name.slice(0, 1) + "***") : (name.slice(0, 2) + "***" + name.slice(-2)); return masked + "@" + domain; } if (looksLikeSecret(t)) { if (t.length <= 10) return t.slice(0, 2) + "***"; return t.slice(0, 6) + "..." + t.slice(-4); } return t; }
        function friendlySourceLabel(source) { var raw = String(source || "").trim(); if (!raw) return { key: "unlabeled-source", label: "未标记来源", raw: "-" }; if (looksLikeSecret(raw)) { var masked = maskSensitive(raw); return { key: "api-key:" + masked, label: "API Key · " + masked, raw: raw }; } return { key: raw, label: raw, raw: raw }; }
        function providerToneClass(name) { var v = String(name || "").toLowerCase(); if (v.includes("minimax")) return "minimax"; if (v.includes("nvidia")) return "nvidia"; if (v.includes("bailian")) return "bailian"; if (v.includes("claude")) return "claude"; if (v.includes("codex")) return "codex"; if (v.includes("gemini")) return "gemini"; if (v.includes("vertex")) return "vertex"; return "default"; }
        function getFirst(obj, keys) { for (var i = 0; i < keys.length; i++) { if (obj && obj[keys[i]] != null) return obj[keys[i]]; } return undefined; }

        function buildProviderMap(config) {
          var map = {};
          if (!config || typeof config !== "object") return map;
          var openaiCompat = getFirst(config, ["openai-compatibility", "openaiCompatibility"]) || [];
          openaiCompat.forEach(function (provider) {
            var providerName = String(getFirst(provider, ["name"]) || "OpenAI Compatible").trim();
            var entries = getFirst(provider, ["api-key-entries", "apiKeyEntries"]) || [];
            entries.forEach(function (entry) { var apiKey = String(getFirst(entry, ["api-key", "apiKey"]) || "").trim(); if (apiKey) map[apiKey] = providerName; });
          });
          [{ keys: ["gemini-api-key", "geminiApiKey"], name: "Gemini" }, { keys: ["claude-api-key", "claudeApiKey"], name: "Claude" }, { keys: ["codex-api-key", "codexApiKey"], name: "Codex" }, { keys: ["vertex-api-key", "vertexApiKey"], name: "Vertex" }].forEach(function (entry) {
            var list = getFirst(config, entry.keys) || [];
            list.forEach(function (item) { var apiKey = String(getFirst(item, ["api-key", "apiKey"]) || "").trim(); if (apiKey) map[apiKey] = entry.name; });
          });
          return map;
        }

        function resolveProviderName(apiKey, source) {
          var sourceText = String(source || "").trim(), keyText = String(apiKey || "").trim();
          if (sourceText && !looksLikeSecret(sourceText)) return sourceText;
          if (sourceText && state.providerMap[sourceText]) return state.providerMap[sourceText];
          if (keyText && state.providerMap[keyText]) return state.providerMap[keyText];
          return friendlySourceLabel(sourceText || keyText).label;
        }

        function normalisePricingRows(rows) {
          var indexed = {};
          (Array.isArray(rows) ? rows : []).forEach(function (item) {
            var modelName = String(getFirst(item, ["model_name", "modelName"]) || "").trim();
            if (!modelName) return;
            indexed[modelName.toLowerCase()] = {
              modelName: modelName,
              inputPricePerM: Math.max(0, Number(getFirst(item, ["input_price_per_m_tokens", "inputPricePerM"]) || 0) || 0),
              outputPricePerM: Math.max(0, Number(getFirst(item, ["output_price_per_m_tokens", "outputPricePerM"]) || 0) || 0),
              reasoningPricePerM: Math.max(0, Number(getFirst(item, ["reasoning_price_per_m_tokens", "reasoningPricePerM"]) || 0) || 0),
              cachedPricePerM: Math.max(0, Number(getFirst(item, ["cached_price_per_m_tokens", "cachedPricePerM"]) || 0) || 0)
            };
          });
          return Object.keys(indexed).sort().map(function (key) { return indexed[key]; });
        }

        function buildPricingMap(rows) {
          var map = {};
          normalisePricingRows(rows).forEach(function (item) { map[item.modelName.toLowerCase()] = item; });
          return map;
        }

        function syncPricingState(rows) {
          state.pricing = normalisePricingRows(rows);
          state.pricingMap = buildPricingMap(state.pricing);
        }

        function syncPricingInputs() {
          var shell = document.getElementById(ROOT_ID);
          if (!shell) return;
          var rows = Array.from(shell.querySelectorAll('[data-obs-price-row]')).map(function (row) {
            var value = function (field) {
              var input = row.querySelector('[data-obs-price-field="' + field + '"]');
              return input ? input.value : '';
            };
            return {
              modelName: String(value('modelName') || '').trim(),
              inputPricePerM: Math.max(0, Number(value('inputPricePerM') || 0) || 0),
              outputPricePerM: Math.max(0, Number(value('outputPricePerM') || 0) || 0),
              reasoningPricePerM: Math.max(0, Number(value('reasoningPricePerM') || 0) || 0),
              cachedPricePerM: Math.max(0, Number(value('cachedPricePerM') || 0) || 0)
            };
          });
          syncPricingState(rows);
        }

        function getModelPricing(modelName) {
          var key = String(modelName || '').trim().toLowerCase();
          return key ? state.pricingMap[key] || null : null;
        }

        function computeRequestCost(modelName, detail) {
          var pricing = getModelPricing(modelName);
          if (!pricing) return { configured: false, cost: 0 };
          var tokens = detail && detail.tokens ? detail.tokens : {};
          var inputTokens = Number(tokens.input_tokens || tokens.inputTokens || 0) || 0;
          var outputTokens = Number(tokens.output_tokens || tokens.outputTokens || 0) || 0;
          var reasoningTokens = Number(tokens.reasoning_tokens || tokens.reasoningTokens || 0) || 0;
          var cachedTokens = Number(tokens.cached_tokens || tokens.cachedTokens || 0) || 0;
          var uncachedInputTokens = Math.max(0, inputTokens - cachedTokens);
          var cost = 0;
          cost += uncachedInputTokens / 1000000 * pricing.inputPricePerM;
          cost += outputTokens / 1000000 * pricing.outputPricePerM;
          cost += reasoningTokens / 1000000 * pricing.reasoningPricePerM;
          cost += cachedTokens / 1000000 * pricing.cachedPricePerM;
          return { configured: true, cost: cost };
        }

        function renderPricingEditor(models) {
          var rows = state.pricing.length ? state.pricing : [{ modelName: '', inputPricePerM: 0, outputPricePerM: 0, reasoningPricePerM: 0, cachedPricePerM: 0 }];
          var suggestions = Array.from(new Set((Array.isArray(models) ? models : []).map(function (item) { return String(item.modelName || item || '').trim(); }).filter(Boolean))).sort();
          var statusText = state.savingPricing ? 'Saving pricing...' : (state.pricingError ? state.pricingError : (state.pricing.length ? ('Saved models: ' + state.pricing.length) : 'No persisted model pricing yet.'));
          var rowsHtml = rows.map(function (row, index) {
            return '<tr data-obs-price-row="1">'
              + '<td><input list="obs-modelSuggestions" data-obs-price-field="modelName" data-obs-price-index="' + index + '" style="' + inputStyle() + 'width:100%;min-width:180px;" value="' + escapeHtml(row.modelName || '') + '" placeholder="model name" /></td>'
              + '<td><input data-obs-price-field="inputPricePerM" data-obs-price-index="' + index + '" style="' + inputStyle() + 'width:120px;" type="number" min="0" step="0.0001" value="' + escapeHtml(row.inputPricePerM || 0) + '" /></td>'
              + '<td><input data-obs-price-field="cachedPricePerM" data-obs-price-index="' + index + '" style="' + inputStyle() + 'width:120px;" type="number" min="0" step="0.0001" value="' + escapeHtml(row.cachedPricePerM || 0) + '" /></td>'
              + '<td><input data-obs-price-field="outputPricePerM" data-obs-price-index="' + index + '" style="' + inputStyle() + 'width:120px;" type="number" min="0" step="0.0001" value="' + escapeHtml(row.outputPricePerM || 0) + '" /></td>'
              + '<td><input data-obs-price-field="reasoningPricePerM" data-obs-price-index="' + index + '" style="' + inputStyle() + 'width:120px;" type="number" min="0" step="0.0001" value="' + escapeHtml(row.reasoningPricePerM || 0) + '" /></td>'
              + '<td><button class="obs-btn secondary" data-obs-action="remove-price-row" data-obs-price-index="' + index + '">Remove</button></td>'
              + '</tr>';
          }).join('');
          var optionsHtml = suggestions.map(function (item) { return '<option value="' + escapeHtml(item) + '"></option>'; }).join('');
          return '<div class="obs-meta">Prices are stored in SQLite and interpreted as CNY per 1M tokens. Cached input is billed separately from uncached input.</div>'
            + '<div class="obs-actions" style="justify-content:flex-start;margin:12px 0;gap:8px;flex-wrap:wrap;">'
            + '<button class="obs-btn secondary" data-obs-action="add-price-row">Add Price Row</button>'
            + '<button class="obs-btn primary" data-obs-action="save-pricing">' + (state.savingPricing ? 'Saving...' : 'Save Pricing') + '</button>'
            + '<span class="obs-meta">' + escapeHtml(statusText) + '</span>'
            + '</div>'
            + '<table class="obs-table"><thead><tr><th>Model</th><th>Input / 1M</th><th>Cached Input / 1M</th><th>Output / 1M</th><th>Reasoning / 1M</th><th>Action</th></tr></thead><tbody>' + rowsHtml + '</tbody></table>'
            + '<datalist id="obs-modelSuggestions">' + optionsHtml + '</datalist>';
        }

        async function savePricing() {
          if (!originalFetch) return;
          syncPricingInputs();
          state.savingPricing = true;
          state.pricingError = '';
          renderData();
          try {
            var headers = { 'Content-Type': 'application/json' };
            if (state.managementKey) headers.Authorization = /^Bearer\s+/i.test(state.managementKey) ? state.managementKey : 'Bearer ' + state.managementKey;
            var response = await originalFetch('/v0/management/billing-prices', {
              method: 'PUT',
              headers: headers,
              credentials: 'same-origin',
              body: JSON.stringify({ prices: state.pricing.map(function (item) {
                return {
                  model_name: item.modelName,
                  input_price_per_m_tokens: item.inputPricePerM,
                  output_price_per_m_tokens: item.outputPricePerM,
                  reasoning_price_per_m_tokens: item.reasoningPricePerM,
                  cached_price_per_m_tokens: item.cachedPricePerM
                };
              }) })
            });
            if (response.status === 401) throw new Error('Management API unauthorized');
            if (!response.ok) throw new Error('HTTP ' + response.status);
            var payload = await response.json();
            syncPricingState(payload && payload.prices ? payload.prices : state.pricing);
          } catch (error) {
            state.pricingError = error && error.message ? error.message : 'Failed to save pricing';
          } finally {
            state.savingPricing = false;
            renderData();
          }
        }

        var shellOpen = false;

        function ensureShell() {
          var shell = document.getElementById(ROOT_ID);
          if (!shell) { shell = document.createElement("div"); shell.id = ROOT_ID; document.body.appendChild(shell); }
          return shell;
        }

        function openShell() {
          shellOpen = true;
          document.body.classList.add("ops-billing-route");
          var entry = document.getElementById(MENU_ID);
          if (entry) entry.classList.add("obs-menu-active");
          renderPage();
          if (!state.usage || state.loadedRangeSignature !== getSavedRangeSignature()) loadUsage();
        }

        function closeShell() {
          shellOpen = false;
          document.body.classList.remove("ops-billing-route");
          var entry = document.getElementById(MENU_ID);
          if (entry) entry.classList.remove("obs-menu-active");
          var shell = document.getElementById(ROOT_ID);
          if (shell) shell.innerHTML = "";
        }

        function isUsageAnchorNode(node) {
          if (!node || node.nodeType !== 1) return false;
          var href = node.getAttribute && node.getAttribute("href");
          if (href && /#\/usage(?:$|[?#])/.test(href)) return true;
          var onclick = node.getAttribute && node.getAttribute("onclick");
          if (onclick && onclick.indexOf("/usage") >= 0) return true;
          var text = String(node.textContent || "").trim();
          return text === "浣跨敤缁熻" || text === "使用统计";
        }

        function findUsageMenuAnchor() {
          var direct = document.querySelector('a[href="#/usage"], a[href$="#/usage"], a[href*="#/usage?"], a[href*="#/usage/"]');
          if (direct) return direct;
          var nodes = Array.from(document.querySelectorAll("button, a, [role='button'], div"));
          return nodes.find(isUsageAnchorNode);
        }

        function syncHashRoute() {
          var hash = String(window.location.hash || "");
          if (hash.indexOf("#/ops-billing") === 0) {
            if (!shellOpen) openShell();
          } else if (shellOpen) {
            closeShell();
          }
        }

        function injectMenuEntry() {
          if (document.getElementById(MENU_ID)) return;
          var anchor = findUsageMenuAnchor();
          if (!anchor) return;
          var row = anchor.closest("button, a, [role='button'], div");
          if (!row) return;
          var clone = row.cloneNode(true);
          clone.id = MENU_ID;
          clone.setAttribute("role", "button");
          clone.setAttribute("href", "#/ops-billing");
          clone.setAttribute("data-obs-route", "ops-billing");
          clone.style.cursor = "pointer";
          Array.from(clone.querySelectorAll("a[href]")).forEach(function (a) {
            a.setAttribute("href", "#/ops-billing");
            a.setAttribute("data-obs-route", "ops-billing");
          });
          var textNode = Array.from(clone.querySelectorAll("*")).find(function (node) { return (node.textContent || "").trim() === "浣跨敤缁熻" && node.children.length === 0; });
          if (!textNode) textNode = Array.from(clone.querySelectorAll("*")).find(function (node) { return (node.textContent || "").trim() === "使用统计" && node.children.length === 0; });
          if (textNode) textNode.textContent = "营运计费"; else clone.textContent = "营运计费";
          row.parentNode.insertBefore(clone, row.nextSibling);
          if (shellOpen) clone.classList.add("obs-menu-active");
        }

        /* capture-phase: menu click opens shell; sidebar click closes shell */
        document.addEventListener("click", function (event) {
          var el = event.target;
          while (el && el !== document) {
            if (el.id === MENU_ID) {
              event.preventDefault();
              event.stopPropagation();
              event.stopImmediatePropagation();
              if (shellOpen) {
                window.location.hash = "#/usage";
              } else {
                window.location.hash = "#/ops-billing";
              }
              return;
            }
            el = el.parentElement;
          }
          if (shellOpen) {
            var shell = document.getElementById(ROOT_ID);
            if (shell && !shell.contains(event.target)) {
              window.location.hash = "#/usage";
              /* DON'T preventDefault — let sidebar click propagate to React */
            }
          }
        }, true);

        function captureAuth(init, input) {
          try {
            var headers = new Headers((init && init.headers) || (input && input.headers) || undefined);
            var auth = headers.get("Authorization");
            if (auth && /^Bearer\s+/i.test(auth)) { state.managementKey = auth.replace(/^Bearer\s+/i, "").trim(); localStorage.setItem(STORAGE_KEY, state.managementKey); }
          } catch (e) {}
        }

        function wireFetchInterceptor() {
          if (!originalFetch || window.__opsBillingFetchPatched) return;
          window.__opsBillingFetchPatched = true;
          window.fetch = function (input, init) {
            captureAuth(init, input);
            return originalFetch(input, init);
          };
        }

        /* ── rendering helpers (ported from ops-billing.html) ── */
        function currentMetricTotal(totalTokens) {
          return state.costMode === 'pricing' ? Number(state.displayCostTotal || 0) : Number(totalTokens || 0);
        }

        function rowMetricValue(row, tokenKey) {
          return state.costMode === 'pricing' ? Number(row.totalCost || 0) : Number(row[tokenKey] || 0);
        }

        function allocatedCostForRow(row, totalTokens, tokenKey) {
          if (state.costMode === 'pricing') return Number(row.totalCost || 0);
          var tokens = Number(row[tokenKey] || 0);
          return totalTokens > 0 ? Number(state.displayCostTotal || 0) * (tokens / totalTokens) : 0;
        }

        function renderDonut(rows, totalTokens) {
          var totalMetric = currentMetricTotal(totalTokens);
          if (!rows.length || totalMetric <= 0) return '<div class="obs-chart-empty">No data available for donut view.</div>';
          var topRows = rows.slice(0, 8).map(function (row, i) { return { label: row.apiKeyLabel || row.apiKey, value: rowMetricValue(row, 'totalTokens'), color: PALETTE[i % PALETTE.length] }; });
          if (rows.length > 8) { var ot = rows.slice(8).reduce(function (s, r) { return s + rowMetricValue(r, 'totalTokens'); }, 0); if (ot > 0) topRows.push({ label: state.costMode === 'pricing' ? 'Other priced keys' : 'Other Keys', value: ot, color: '#cabda8' }); }
          var cur = 0, gradient = topRows.map(function (r) { var next = cur + r.value / totalMetric * 100; var seg = r.color + ' ' + cur.toFixed(2) + '% ' + next.toFixed(2) + '%'; cur = next; return seg; }).join(', ');
          var legend = topRows.map(function (r) { return '<div class="obs-legend-item"><span class="obs-legend-dot" style="background:' + r.color + ';"></span><span class="obs-legend-label" title="' + escapeHtml(r.label) + '">' + escapeHtml(r.label) + '</span><span class="obs-legend-value">' + formatPercent(r.value, totalMetric) + '</span></div>'; }).join('');
          return '<div class="obs-donut-layout"><div class="obs-donut-shell" style="background:conic-gradient(' + gradient + ');"><div class="obs-donut-hole"><strong>' + rows.length + '</strong><span>' + (state.costMode === 'pricing' ? 'priced keys' : 'active keys') + '</span></div></div><div class="obs-legend-list">' + legend + '</div></div>';
        }

        function renderBarRows(rows, totalTokens, labelKey, tokenKey, subtitleBuilder) {
          var totalMetric = currentMetricTotal(totalTokens);
          if (!rows.length || totalMetric <= 0) return '<div class="obs-chart-empty">No data available for bar view.</div>';
          return '<div class="obs-bar-list">' + rows.map(function (row, i) {
            var tokens = Number(row[tokenKey] || 0), metricValue = rowMetricValue(row, tokenKey), share = totalMetric > 0 ? metricValue / totalMetric : 0, cost = allocatedCostForRow(row, totalTokens, tokenKey), color = PALETTE[i % PALETTE.length];
            var subtitle = subtitleBuilder ? subtitleBuilder(row, tokens, totalTokens, cost) : (state.costMode === 'pricing' ? (formatCurrency(cost) + ' / ' + formatPercent(metricValue, totalMetric)) : (formatCompactTokens(tokens) + ' Tokens / ' + formatPercent(tokens, totalTokens)));
            return '<div class="obs-bar-row"><div class="obs-bar-key"><strong title="' + escapeHtml(row[labelKey]) + '">' + escapeHtml(row[labelKey]) + '</strong><span>' + escapeHtml(subtitle) + '</span></div><div class="obs-bar-track"><div class="obs-bar-fill" style="width:' + Math.max(2, share * 100) + '%;background:' + color + ';"></div></div><div class="obs-bar-cost">' + formatCurrency(cost) + '</div></div>';
          }).join('') + '</div>';
        }

        function renderProviderStructure(rows, totalTokens) {
          var totalMetric = currentMetricTotal(totalTokens);
          if (!rows.length || totalMetric <= 0) return '<div class="obs-chart-empty">No provider cost structure available.</div>';
          return '<div class="obs-provider-list">' + rows.map(function (row) {
            var metricValue = state.costMode === 'pricing' ? Number(row.totalCost || 0) : Number(row.tokens || 0), share = totalMetric > 0 ? metricValue / totalMetric : 0, cost = state.costMode === 'pricing' ? Number(row.totalCost || 0) : Number(state.displayCostTotal || 0) * share, tone = providerToneClass(row.sourceLabel);
            return '<div class="obs-provider-item"><div class="obs-provider-head"><div class="obs-provider-left"><span class="obs-provider-badge ' + tone + '">' + escapeHtml(row.sourceLabel) + '</span></div><span class="obs-provider-share">' + formatPercent(metricValue, totalMetric) + '</span></div><div class="obs-provider-meta"><span class="obs-provider-chip">Models: ' + formatNumber(row.modelsSet ? row.modelsSet.size : 0) + '</span><span class="obs-provider-chip">Requests: ' + formatNumber(row.requests) + '</span><span class="obs-provider-chip">Cost: ' + formatCurrency(cost) + '</span></div><div class="obs-bar-track"><div class="obs-bar-fill" style="width:' + Math.max(2, share * 100) + '%;"></div></div></div>';
          }).join('') + '</div>';
        }

        function renderTrend(days, totalTokens) {
          if (!days.length) return '<div class="obs-chart-empty">No recent samples.</div>';
          var values = days.map(function (item) { return state.costMode === 'pricing' ? Number(item.totalCost || 0) : Number(item.tokens || 0); });
          var maxValue = values.reduce(function (m, v) { return Math.max(m, v); }, 0) || 1;
          return '<div class="obs-trend-bars">' + days.map(function (item) {
            var metricValue = state.costMode === 'pricing' ? Number(item.totalCost || 0) : Number(item.tokens || 0), ratio = metricValue / maxValue, cost = state.costMode === 'pricing' ? Number(item.totalCost || 0) : (totalTokens > 0 ? Number(state.displayCostTotal || 0) * (Number(item.tokens || 0) / totalTokens) : 0);
            return '<div class="obs-trend-col"><div class="obs-trend-track"><div class="obs-trend-fill" style="height:' + Math.max(8, ratio * 100) + '%;"></div></div><span class="obs-trend-label">' + escapeHtml(item.day.slice(5)) + '</span><span class="obs-trend-value">' + formatCurrency(cost) + '</span></div>';
          }).join('') + '</div>';
        }

        function renderModelBars(models, totalTokens) {
          if (!models.length) return '<div class="obs-chart-empty">No model breakdown available.</div>';
          return renderBarRows(models.slice(0, 10), totalTokens, 'modelName', 'tokens', function (m, tokens, total, cost) { return state.costMode === 'pricing' ? (formatCurrency(cost) + ' / ' + formatNumber(m.requests) + ' req') : (formatNumber(m.requests) + ' req / ' + formatNumber(m.keySet.size) + ' keys'); });
        }

        function renderRiskWarnings(rows, totalTokens) {
          var totalMetric = currentMetricTotal(totalTokens);
          if (!rows.length || totalMetric <= 0) return '<div class="obs-chart-empty">No key risk analysis available.</div>';
          var risks = rows.map(function (row) {
            var metricValue = rowMetricValue(row, 'totalTokens'), share = totalMetric > 0 ? metricValue / totalMetric : 0, failureRate = row.totalRequests > 0 ? row.failureCount / row.totalRequests : 0, allocatedCost = allocatedCostForRow(row, totalTokens, 'totalTokens');
            var score = 0, reasons = [];
            if (share >= 0.35) { score += 55; reasons.push((state.costMode === 'pricing' ? 'Cost share ' : 'Token share ') + formatPercent(metricValue, totalMetric) + ' is highly concentrated'); }
            else if (share >= 0.2) { score += 30; reasons.push((state.costMode === 'pricing' ? 'Cost share ' : 'Token share ') + formatPercent(metricValue, totalMetric) + ' needs attention'); }
            if (failureRate >= 0.3 && row.totalRequests >= 5) { score += 30; reasons.push('Failure rate ' + (failureRate * 100).toFixed(1) + '% may affect stability'); }
            else if (failureRate >= 0.15 && row.totalRequests >= 5) { score += 15; reasons.push('Failure rate ' + (failureRate * 100).toFixed(1) + '% should be checked'); }
            if (allocatedCost >= Number(state.displayCostTotal || 0) * 0.25 && Number(state.displayCostTotal || 0) > 0) { score += 20; reasons.push('Single key estimated cost reached ' + formatCurrency(allocatedCost)); }
            return { apiKey: row.apiKey, score: score, level: score >= 60 ? 'high' : score >= 30 ? 'medium' : 'low', reasons: reasons };
          }).filter(function (r) { return r.score > 0; }).sort(function (a, b) { return b.score - a.score; }).slice(0, 5);
          if (!risks.length) return '<div class="obs-chart-empty">No obvious high-risk keys at the moment.</div>';
          return '<div class="obs-risk-list">' + risks.map(function (risk) {
            return '<div class="obs-risk-item ' + (risk.level === 'high' ? 'high' : 'medium') + '"><div class="obs-risk-head"><div class="obs-risk-title" title="' + escapeHtml(maskSensitive(risk.apiKey)) + '">' + escapeHtml(maskSensitive(risk.apiKey)) + '</div><div class="obs-risk-score">Risk ' + risk.score + '</div></div><div class="obs-risk-reason">' + escapeHtml(risk.reasons.join('; ')) + '</div></div>';
          }).join('') + '</div>';
        }

        function collectSnapshot(snapshot) {
          var apis = snapshot && snapshot.apis ? snapshot.apis : {};
          var rows = [], recent = [], providerMap = new Map(), dayMap = new Map(), modelMap = new Map();
          var totalEstimatedCost = 0, pricedRequests = 0, unpricedModels = new Set();
          Object.keys(apis).forEach(function (apiKey) {
            var api = apis[apiKey] || {}, models = api.models || {}, successCount = 0, failureCount = 0, requestCount = 0, apiTotalCost = 0;
            Object.keys(models).forEach(function (modelName) {
              var model = models[modelName] || {};
              (model.details || []).forEach(function (detail) {
                var tokens = detail.tokens || {}, totalTokens = Number(tokens.total_tokens || tokens.totalTokens || 0), failed = Boolean(detail.failed), costInfo = computeRequestCost(modelName, detail);
                requestCount += 1; if (failed) failureCount += 1; else successCount += 1;
                if (costInfo.configured) { totalEstimatedCost += costInfo.cost; pricedRequests += 1; apiTotalCost += costInfo.cost; }
                else if (totalTokens > 0) { unpricedModels.add(modelName || '-'); }
                recent.push({ apiKey: apiKey, apiKeyLabel: maskSensitive(apiKey), modelName: modelName, timestamp: detail.timestamp || '', source: detail.source || '-', failed: failed, totalTokens: totalTokens, totalCost: costInfo.cost, priced: costInfo.configured });
                var providerName = resolveProviderName(apiKey, detail.source || '-');
                var provider = providerMap.get(providerName) || { sourceLabel: providerName, tokens: 0, requests: 0, modelsSet: new Set(), totalCost: 0 };
                provider.tokens += totalTokens; provider.requests += 1; provider.totalCost += costInfo.cost; provider.modelsSet.add(modelName || '-'); providerMap.set(providerName, provider);
                var dayKey = (detail.timestamp || '').slice(0, 10) || 'unknown-date', dayEntry = dayMap.get(dayKey) || { day: dayKey, tokens: 0, requests: 0, totalCost: 0 }; dayEntry.tokens += totalTokens; dayEntry.requests += 1; dayEntry.totalCost += costInfo.cost; dayMap.set(dayKey, dayEntry);
                var modelKey = modelName || '-', modelEntry = modelMap.get(modelKey) || { modelName: modelKey, tokens: 0, requests: 0, keySet: new Set(), totalCost: 0 }; modelEntry.tokens += totalTokens; modelEntry.requests += 1; modelEntry.totalCost += costInfo.cost; modelEntry.keySet.add(apiKey); modelMap.set(modelKey, modelEntry);
              });
            });
            rows.push({ apiKey: apiKey, apiKeyLabel: maskSensitive(apiKey), totalRequests: Number(api.total_requests || api.totalRequests || requestCount || 0), totalTokens: Number(api.total_tokens || api.totalTokens || 0), successCount: successCount, failureCount: failureCount, totalCost: apiTotalCost });
          });
          rows.sort(function (a, b) { return (b.totalCost - a.totalCost) || (b.totalTokens - a.totalTokens) || b.totalRequests - a.totalRequests; });
          recent.sort(function (a, b) { return new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(); });
          return {
            rows: rows,
            recent: recent.slice(0, 20),
            providers: Array.from(providerMap.values()).sort(function (a, b) { return (b.totalCost - a.totalCost) || (b.tokens - a.tokens); }),
            models: Array.from(modelMap.values()).sort(function (a, b) { return (b.totalCost - a.totalCost) || (b.tokens - a.tokens); }),
            daily: Array.from(dayMap.values()).sort(function (a, b) { return new Date(a.day).getTime() - new Date(b.day).getTime(); }).slice(-7),
            totalEstimatedCost: totalEstimatedCost,
            pricedRequests: pricedRequests,
            unpricedModels: Array.from(unpricedModels.values()).sort()
          };
        }

        function buildPageHTML() {
          return [
            '<div class="obs-page" data-obs-version="2">',
            '<div class="obs-topbar"><div>',
            '<div class="obs-kicker">Operations / Cost Allocation</div>',
            '<h1>营运计费</h1>',
            '<p class="obs-subtitle">以使用统计数据为基础，把总成本池按 token 占比摊到各个 key、供应商和模型，适合运营对账与费用复盘。</p>',
            '</div><div class="obs-actions">',
            '<button class="obs-btn secondary" data-obs-action="back">返回主控台</button>',
            '<button class="obs-btn primary" data-obs-action="refresh">刷新数据</button>',
            '</div></div>',
            '<div class="obs-grid">',
            '<div class="obs-card" style="grid-column:span 12">',
            '<div class="obs-section-title">分摊参数</div>',
            '<div class="obs-section-subtitle">只影响本页的成本分摊计算，不会修改后端转发和真实扣费逻辑。</div>',
            '<div class="obs-form-grid">',
            '<div class="obs-field"><label>管理密钥</label><input id="obs-managementKey" type="password" placeholder="输入 management key / Bearer 密钥" /></div>',
            '<div class="obs-field"><label>总成本池（元）</label><input id="obs-costPool" type="number" step="0.01" min="0" value="100" /></div>',
            '<div class="obs-field"><label>开始日期</label><input id="obs-dateStart" type="date" /></div>',
            '<div class="obs-field"><label>结束日期</label><input id="obs-dateEnd" type="date" /></div>',
            '<div class="obs-field"><label>快捷范围</label><select id="obs-dateQuick"><option value="">自定义</option><option value="today">今天</option><option value="7d">最近 7 天</option><option value="30d">最近 30 天</option><option value="90d">最近 90 天</option><option value="all">全部时间</option></select></div>',
            '<div class="obs-field"><label>结算周期</label><select id="obs-settlementCycle"><option value="current">当前累计</option><option value="daily">按日观察</option><option value="monthly">按月观察</option></select></div>',
            '</div>',
            '<div class="obs-range-summary"><span class="obs-range-chip" id="obs-rangeSummary">当前范围：全部时间</span><span class="obs-meta" id="obs-rangeHint">切换时间范围后会重新按筛选后的 usage 数据核算本页成本。</span></div>',
            '<div id="obs-statusHint" class="obs-meta">读取数据后会自动按 token 占比计算各项费用结构。</div>',
            '</div>',
            '<div id="obs-authFallback" class="obs-card" style="grid-column:span 12;display:none"><div class="obs-auth-box">当前接口未授权。请输入管理密钥后再刷新。</div></div>',
            '<div class="obs-card" style="grid-column:span 12"><div class="obs-section-title">Model Pricing</div><div class="obs-section-subtitle">Persist model prices in SQLite and use them for actual cost calculation. Missing prices fall back to cost-pool allocation.</div><div id="obs-pricingWrap" class="obs-empty">Loading pricing settings...</div></div>',
            '<div class="obs-card" style="grid-column:span 3"><div class="obs-label">总请求量</div><div id="obs-totalRequests" class="obs-value">-</div><div id="obs-requestsMeta" class="obs-meta">等待计费数据</div></div>',
            '<div class="obs-card" style="grid-column:span 3"><div class="obs-label">总 Tokens</div><div id="obs-totalTokens" class="obs-value">-</div><div class="obs-meta">用于分摊成本池的分母</div></div>',
            '<div class="obs-card" style="grid-column:span 3"><div class="obs-label">成本池</div><div id="obs-costPoolValue" class="obs-value">-</div><div class="obs-meta">所有 key 共同分摊的总成本</div></div>',
            '<div class="obs-card" style="grid-column:span 3"><div class="obs-label">Estimated Cost</div><div id="obs-pricedCostValue" class="obs-value">-</div><div id="obs-pricedCostMeta" class="obs-meta">Computed from persisted model pricing when available</div></div>',
            '<div class="obs-card" style="grid-column:span 3"><div class="obs-label">活跃 Key 数</div><div id="obs-activeKeys" class="obs-value">-</div><div class="obs-meta">按 usage.apis 聚合</div></div>',
            '<div class="obs-card" style="grid-column:span 5"><div class="obs-section-title">Key 成本占比</div><div class="obs-section-subtitle">看所有 key 在成本池里的分摊比例。</div><div id="obs-keyDonutWrap" class="obs-chart-empty">等待生成占比图表。</div></div>',
            '<div class="obs-card" style="grid-column:span 7"><div class="obs-section-title">Key 分摊条形图</div><div class="obs-section-subtitle">展示所有 key 的 token 占比与应承担金额。</div><div id="obs-keyBarsWrap" class="obs-chart-empty">等待生成分摊条形图。</div></div>',
            '<div class="obs-card" style="grid-column:span 6"><div class="obs-section-title">供应商费用结构</div><div class="obs-section-subtitle">供应商名优先取请求事件明细里的来源口径，必要时再用配置映射纠正。</div><div id="obs-sourceBarsWrap" class="obs-chart-empty">等待生成供应商费用结构图。</div></div>',
            '<div class="obs-card" style="grid-column:span 6"><div class="obs-section-title">近 7 日费用趋势</div><div class="obs-section-subtitle">按最近 7 天的请求样本估算每日费用波动。</div><div id="obs-trendWrap" class="obs-chart-empty">等待生成趋势图。</div></div>',
            '<div class="obs-card" style="grid-column:span 6"><div class="obs-section-title">模型费用结构</div><div class="obs-section-subtitle">按模型聚合 token 与成本，看哪些模型是主要费用来源。</div><div id="obs-modelBarsWrap" class="obs-chart-empty">等待生成模型分摊图。</div></div>',
            '<div class="obs-card" style="grid-column:span 6"><div class="obs-section-title">Top key 风险预警</div><div class="obs-section-subtitle">综合看 token 集中度、失败率和承担金额。</div><div id="obs-riskWrap" class="obs-chart-empty">等待风险预警结果。</div></div>',
            '<div class="obs-card" style="grid-column:span 8"><div class="obs-section-title">Key 分摊明细</div><div class="obs-section-subtitle">每个 key 的应承担金额 = 总成本池 × (该 key Tokens / 全部 Tokens)</div><div id="obs-keyTableWrap" class="obs-empty">还没有可用的分摊数据。</div></div>',
            '<div class="obs-card" style="grid-column:span 4"><div class="obs-section-title">账单说明</div><div class="obs-section-subtitle">这块专门按"承担多少钱"来展示。</div><div class="obs-hint">1. 先统计所有 key 的总 Tokens。<br />2. 设总成本池，例如 100 元。<br />3. 每个 key 的承担金额 = 成本池 × 自己 Tokens 占比。<br />4. 如果某个 key 本期 Tokens 为 0，则承担金额为 0。</div></div>',
            '<div class="obs-card" style="grid-column:span 12"><div class="obs-section-title">最近请求成本样本</div><div class="obs-section-subtitle">单笔请求按同样比例口径折算，方便快速抽查高消耗请求。</div><div id="obs-detailTableWrap" class="obs-empty">暂无最近样本。</div></div>',
            '</div></div>'
          ].join("\n");
        }

        function renderData() {
          var shell = document.getElementById(ROOT_ID);
          if (!shell || !shellOpen) return;
          var costPoolInput = document.getElementById("obs-costPool");
          var startInput = document.getElementById("obs-dateStart");
          var endInput = document.getElementById("obs-dateEnd");
          var quickInput = document.getElementById("obs-dateQuick");
          if (costPoolInput) { var cp = Number(costPoolInput.value || 0); state.costPool = Number.isFinite(cp) ? cp : 0; }
          if (startInput && startInput.value !== getSavedRangeStart()) startInput.value = getSavedRangeStart();
          if (endInput && endInput.value !== getSavedRangeEnd()) endInput.value = getSavedRangeEnd();
          if (quickInput) quickInput.value = guessQuickRange(getSavedRangeStart(), getSavedRangeEnd());
          var cpv = document.getElementById("obs-costPoolValue"); if (cpv) cpv.textContent = formatCurrency(state.costPool);
          var rangeSummary = document.getElementById("obs-rangeSummary");
          if (rangeSummary) rangeSummary.textContent = "当前范围：" + describeRange(getSavedRangeStart(), getSavedRangeEnd());
          var statusHint = document.getElementById("obs-statusHint");
          if (statusHint) {
            if (state.loading) statusHint.textContent = '正在读取使用统计并重新计算各项费用结构...';
            else if (state.error) statusHint.innerHTML = '<span style="color:#b5553f">' + escapeHtml(state.error) + '</span>';
            else statusHint.textContent = '按当前时间范围内的总 token 占比摊成本池，适合快速核算每个 key / 供应商 / 模型应承担多少费用。';
          }
          var authFb = document.getElementById("obs-authFallback"); if (authFb) authFb.style.display = state.unauthorized ? 'block' : 'none';
          if (!state.usage) return;
          var totalRequests = Number(state.usage.total_requests || state.usage.totalRequests || 0);
          var successCount = Number(state.usage.success_count || state.usage.successCount || 0);
          var failureCount = Number(state.usage.failure_count || state.usage.failureCount || 0);
          var totalTokens = Number(state.usage.total_tokens || state.usage.totalTokens || 0);
          var data = collectSnapshot(state.usage);
          state.costMode = data.totalEstimatedCost > 0 ? 'pricing' : 'pool';
          state.displayCostTotal = state.costMode === 'pricing' ? data.totalEstimatedCost : state.costPool;
          var $ = function (id) { return document.getElementById(id); };
          $("obs-totalRequests").textContent = formatNumber(totalRequests);
          $("obs-requestsMeta").textContent = '鎴愬姛 ' + formatNumber(successCount) + ' / 澶辫触 ' + formatNumber(failureCount);
          $("obs-totalTokens").textContent = formatCompactTokens(totalTokens);
          $("obs-activeKeys").textContent = formatNumber(data.rows.length);
          $("obs-costPoolValue").textContent = formatCurrency(state.costPool);
          if ($("obs-pricedCostValue")) $("obs-pricedCostValue").textContent = formatCurrency(data.totalEstimatedCost);
          if ($("obs-pricedCostMeta")) $("obs-pricedCostMeta").textContent = data.pricedRequests > 0 ? ("Priced requests: " + formatNumber(data.pricedRequests) + (data.unpricedModels.length ? " / Unpriced models: " + data.unpricedModels.join(", ") : "")) : "No saved model price matched current usage";
          if ($("obs-pricingWrap")) $("obs-pricingWrap").innerHTML = renderPricingEditor(data.models);
          $("obs-keyDonutWrap").innerHTML = renderDonut(data.rows, totalTokens);
          $("obs-keyBarsWrap").innerHTML = renderBarRows(data.rows, totalTokens, "apiKeyLabel", "totalTokens");
          $("obs-sourceBarsWrap").innerHTML = renderProviderStructure(data.providers, totalTokens);
          $("obs-trendWrap").innerHTML = renderTrend(data.daily, totalTokens);
          $("obs-modelBarsWrap").innerHTML = renderModelBars(data.models, totalTokens);
          $("obs-riskWrap").innerHTML = renderRiskWarnings(data.rows, totalTokens);
          if (statusHint && !state.loading && !state.error) {
            statusHint.textContent = state.costMode === 'pricing' ? ('Using persisted model pricing for ' + formatNumber(data.pricedRequests) + ' requests' + (data.unpricedModels.length ? '; fallback to pool for unpriced models: ' + data.unpricedModels.join(', ') : '')) : 'No saved model price matched current usage. Cost figures fall back to total cost pool allocation.';
          }
          if (!data.rows.length || currentMetricTotal(totalTokens) <= 0) { $("obs-keyTableWrap").innerHTML = "<div class=\"obs-chart-empty\">褰撳墠缁熻閲岃繕娌℃湁鍙敤浜庡垎鎽婄殑 key/token 鏁版嵁銆?/div>"; }
          else {
            var metricTotal = currentMetricTotal(totalTokens);
            var tbody = data.rows.map(function (row) { var metricValue = rowMetricValue(row, "totalTokens"), allocatedCost = allocatedCostForRow(row, totalTokens, "totalTokens"), successRate = row.totalRequests > 0 ? ((row.successCount / row.totalRequests) * 100).toFixed(1) + "%" : "-"; return "<tr><td>" + escapeHtml(row.apiKeyLabel || row.apiKey) + "</td><td>" + formatNumber(row.totalRequests) + "</td><td>" + formatCompactTokens(row.totalTokens) + "</td><td>" + formatPercent(metricValue, metricTotal) + "</td><td>" + successRate + "</td><td>" + formatCurrency(allocatedCost) + "</td></tr>"; }).join("");
            $("obs-keyTableWrap").innerHTML = "<table class=\"obs-table\"><thead><tr><th>API Key</th><th>璇锋眰鏁?/th><th>鎬?Tokens</th><th>" + (state.costMode === "pricing" ? "Estimated Cost Share" : "鎴愭湰鍗犳瘮") + "</th><th>鎴愬姛鐜?/th><th>" + (state.costMode === "pricing" ? "Estimated Cost" : "搴旀壙鎷呴噾棰?") + "</th></tr></thead><tbody>" + tbody + "</tbody></table>";
          }
          if (!data.recent.length || (state.costMode === "pool" && totalTokens <= 0)) { $("obs-detailTableWrap").innerHTML = "<div class=\"obs-chart-empty\">鏆傛棤鏈€杩戞牱鏈€?/div>"; }
          else {
            var dbody = data.recent.map(function (item) { var allocatedCost = state.costMode === "pricing" ? Number(item.totalCost || 0) : (totalTokens > 0 ? state.displayCostTotal * (item.totalTokens / totalTokens) : 0); return "<tr><td>" + escapeHtml(item.timestamp || "-") + "</td><td>" + escapeHtml(item.modelName || "-") + "</td><td>" + escapeHtml(item.apiKeyLabel || item.apiKey || "-") + "</td><td>" + escapeHtml(resolveProviderName(item.apiKey, item.source)) + "</td><td><span class=\"obs-badge " + (item.failed ? "failure" : "success") + "\">" + (item.failed ? "澶辫触" : "鎴愬姛") + "</span></td><td>" + formatCompactTokens(item.totalTokens) + "</td><td>" + formatCurrency(allocatedCost) + "</td></tr>"; }).join("");
            $("obs-detailTableWrap").innerHTML = "<table class=\"obs-table\"><thead><tr><th>鏃堕棿</th><th>妯″瀷</th><th>API Key</th><th>鏉ユ簮</th><th>鐘舵€?/th><th>Tokens</th><th>" + (state.costMode === "pricing" ? "Estimated Cost" : "鎶樼畻鎴愭湰") + "</th></tr></thead><tbody>" + dbody + "</tbody></table>";
          }
        }


        function renderPage() {
          var shell = ensureShell();
          if (!shellOpen) return;
          var page = shell.querySelector(".obs-page");
          var needsRebuild = !page || !shell.querySelector("#obs-dateStart") || !shell.querySelector("#obs-dateQuick") || page.getAttribute("data-obs-version") !== "2";
          if (needsRebuild) {
            shell.innerHTML = buildPageHTML();
            var keyInput = document.getElementById("obs-managementKey");
            var cpInput = document.getElementById("obs-costPool");
            var startInput = document.getElementById("obs-dateStart");
            var endInput = document.getElementById("obs-dateEnd");
            var quickInput = document.getElementById("obs-dateQuick");
            if (keyInput) keyInput.value = state.managementKey;
            if (cpInput) cpInput.value = String(state.costPool);
            if (cpInput) cpInput.addEventListener("input", renderData);
            if (startInput) startInput.value = getSavedRangeStart();
            if (endInput) endInput.value = getSavedRangeEnd();
            if (quickInput) quickInput.value = guessQuickRange(getSavedRangeStart(), getSavedRangeEnd());
            function applyRangeAndReload() {
              var startValue = startInput ? String(startInput.value || "").trim() : "";
              var endValue = endInput ? String(endInput.value || "").trim() : "";
              saveRange(startValue, endValue);
              if (quickInput) quickInput.value = guessQuickRange(startValue, endValue);
              renderData();
              loadUsage();
            }
            if (startInput) startInput.addEventListener("change", function () { if (quickInput) quickInput.value = ""; applyRangeAndReload(); });
            if (endInput) endInput.addEventListener("change", function () { if (quickInput) quickInput.value = ""; applyRangeAndReload(); });
            if (quickInput) quickInput.addEventListener("change", function () {
              var v = quickInput.value;
              if (v === "all") {
                if (startInput) startInput.value = "";
                if (endInput) endInput.value = "";
              } else if (v === "today") {
                if (startInput) startInput.value = todayStr();
                if (endInput) endInput.value = todayStr();
              } else if (v === "7d") {
                if (startInput) startInput.value = daysAgo(7);
                if (endInput) endInput.value = todayStr();
              } else if (v === "30d") {
                if (startInput) startInput.value = daysAgo(30);
                if (endInput) endInput.value = todayStr();
              } else if (v === "90d") {
                if (startInput) startInput.value = daysAgo(90);
                if (endInput) endInput.value = todayStr();
              }
              applyRangeAndReload();
            });
            shell.addEventListener("click", function (e) {
              var actionNode = e.target && e.target.closest ? e.target.closest('[data-obs-action]') : null;
              var action = actionNode && actionNode.getAttribute ? actionNode.getAttribute("data-obs-action") : null;
              if (action === "back") { window.location.hash = "#/usage"; }
              else if (action === "refresh" || action === "load") { loadUsage(); }
              else if (action === "add-price-row") { syncPricingInputs(); state.pricing.push({ modelName: '', inputPricePerM: 0, outputPricePerM: 0, reasoningPricePerM: 0, cachedPricePerM: 0 }); state.pricingMap = buildPricingMap(state.pricing); renderData(); }
              else if (action === "remove-price-row") { syncPricingInputs(); var idx = Number(actionNode.getAttribute('data-obs-price-index') || -1); if (idx >= 0) { state.pricing.splice(idx, 1); state.pricingMap = buildPricingMap(state.pricing); renderData(); } }
              else if (action === "save-pricing") { savePricing(); }
            });
            shell.addEventListener("input", function (e) {
              var field = e.target && e.target.getAttribute ? e.target.getAttribute('data-obs-price-field') : null;
              if (field) {
                syncPricingInputs();
                state.pricingError = '';
              }
            });
          }
          renderData();
        }

        async function loadUsage() {
          if (!originalFetch) return;
          state.loading = true; state.error = ""; state.unauthorized = false;
          var keyInput = document.getElementById("obs-managementKey");
          var cpInput = document.getElementById("obs-costPool");
          if (keyInput) { state.managementKey = String(keyInput.value || "").trim(); localStorage.setItem(STORAGE_KEY, state.managementKey); }
          if (cpInput) localStorage.setItem(COST_POOL_KEY, String(cpInput.value || "100"));
          renderData();
          try {
            var headers = state.managementKey ? { Authorization: /^Bearer\s+/i.test(state.managementKey) ? state.managementKey : "Bearer " + state.managementKey } : {};
            var responses = await Promise.all([
              originalFetch("/v0/management/usage", { method: "GET", headers: headers, credentials: "same-origin" }),
              originalFetch("/v0/management/config", { method: "GET", headers: headers, credentials: "same-origin" }),
              originalFetch("/v0/management/billing-prices", { method: "GET", headers: headers, credentials: "same-origin" })
            ]);
            var usageResponse = responses[0], configResponse = responses[1], pricingResponse = responses[2];
            if (usageResponse.status === 401) { state.usage = null; state.loadedRangeSignature = ""; state.unauthorized = true; state.error = "管理接口未授权，请输入 management key。"; return; }
            if (!usageResponse.ok) throw new Error("HTTP " + usageResponse.status);
            var payload = await usageResponse.json(); state.usage = payload && payload.usage ? payload.usage : payload; state.loadedRangeSignature = getSavedRangeSignature();
            if (configResponse.ok) { state.config = await configResponse.json(); state.providerMap = buildProviderMap(state.config); } else { state.providerMap = {}; }
            if (pricingResponse.ok) { var pricingPayload = await pricingResponse.json(); syncPricingState(pricingPayload && pricingPayload.prices ? pricingPayload.prices : []); } else { syncPricingState(state.pricing); }
            var responses = await Promise.all([
              originalFetch("/v0/management/usage", { method: "GET", headers: headers, credentials: "same-origin" }),
              originalFetch("/v0/management/config", { method: "GET", headers: headers, credentials: "same-origin" })
            ]);
            var usageResponse = responses[0], configResponse = responses[1];
            if (usageResponse.status === 401) { state.usage = null; state.loadedRangeSignature = ""; state.unauthorized = true; state.error = "管理接口未授权，请输入 management key。"; return; }
            if (!usageResponse.ok) throw new Error("HTTP " + usageResponse.status);
            var payload = await usageResponse.json(); state.usage = payload && payload.usage ? payload.usage : payload; state.loadedRangeSignature = getSavedRangeSignature();
            if (configResponse.ok) { state.config = await configResponse.json(); state.providerMap = buildProviderMap(state.config); } else { state.providerMap = {}; }
          } catch (error) { state.usage = null; state.loadedRangeSignature = ""; state.error = "读取计费数据失败：" + (error && error.message ? error.message : "未知错误"); }
          finally { state.loading = false; renderData(); }
        }

        function boot() {
          if (booted) return;
          booted = true;
          wireFetchInterceptor();
          injectMenuEntry();
          syncHashRoute();
          window.addEventListener("hashchange", syncHashRoute);
          var observerTarget = document.getElementById("root") || document.body;
          new MutationObserver(function () { if (!document.getElementById(MENU_ID)) injectMenuEntry(); }).observe(observerTarget, { childList: true, subtree: true });
        }

        if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot, { once: true });
        else boot();
      })();
    