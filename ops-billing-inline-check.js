
(function () {
  const STORAGE_KEY = "opsBilling.managementKey";
  const COST_POOL_KEY = "opsBilling.costPool";
  const EMBED_MODE = new URLSearchParams(window.location.search).get("embed") === "1";
  const fetchNative = window.fetch.bind(window);
  const PALETTE = ["#3f3424", "#8d6f3f", "#1b8a5d", "#b47c2d", "#7f5f78", "#4e86a6", "#c05f47", "#7c7a43", "#6f5a45"];
  const el = {
    managementKey: document.getElementById("managementKey"), costPool: document.getElementById("costPool"), settlementCycle: document.getElementById("settlementCycle"),
    refreshBtn: document.getElementById("refreshBtn"), saveAndLoad: document.getElementById("saveAndLoad"), backToPanel: document.getElementById("backToPanel"),
    statusHint: document.getElementById("statusHint"), authFallback: document.getElementById("authFallback"), totalRequests: document.getElementById("totalRequests"),
    requestsMeta: document.getElementById("requestsMeta"), totalTokens: document.getElementById("totalTokens"), costPoolValue: document.getElementById("costPoolValue"),
    activeKeys: document.getElementById("activeKeys"), keyDonutWrap: document.getElementById("keyDonutWrap"), keyBarsWrap: document.getElementById("keyBarsWrap"),
    sourceBarsWrap: document.getElementById("sourceBarsWrap"), trendWrap: document.getElementById("trendWrap"), modelBarsWrap: document.getElementById("modelBarsWrap"),
    riskWrap: document.getElementById("riskWrap"), keyTableWrap: document.getElementById("keyTableWrap"), detailTableWrap: document.getElementById("detailTableWrap")
  };
  const state = { usage: null, config: null, providerMap: {}, loading: false, unauthorized: false, error: "", managementKey: localStorage.getItem(STORAGE_KEY) || "", costPool: Number(localStorage.getItem(COST_POOL_KEY) || "100") };
  function escapeHtml(v){return String(v==null?"":v).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/\"/g,"&quot;").replace(/'/g,"&#39;");}
  function formatNumber(v){return new Intl.NumberFormat("zh-CN").format(Number.isFinite(v)?v:0)}
  function formatCurrency(v){return new Intl.NumberFormat("zh-CN",{style:"currency",currency:"CNY",minimumFractionDigits:2,maximumFractionDigits:2}).format(Number.isFinite(v)?v:0)}
  function formatCompactTokens(v){const n=Number(v)||0;if(n>=1e6)return (n/1e6).toFixed(2)+"M";if(n>=1e3)return (n/1e3).toFixed(1)+"K";return String(n)}
  function formatPercent(n,d){if(!d)return"0.00%";return((n/d)*100).toFixed(2)+"%"}
  function looksLikeSecret(v){const t=String(v||"").trim();if(!t)return false;if(/^(sk-|nvapi-|LB-|WZ-|XX-|AI-)/i.test(t))return true;return t.length>=24&&!t.includes(" ")&&!t.includes("@");}
  function maskSensitive(v){const t=String(v||"").trim();if(!t)return"-";if(t.includes("@")){const p=t.split("@");const name=p[0]||"";const domain=p[1]||"";const masked=name.length<=4?(name.slice(0,1)+"***"):(name.slice(0,2)+"***"+name.slice(-2));return masked+"@"+domain}if(looksLikeSecret(t)){if(t.length<=10)return t.slice(0,2)+"***";return t.slice(0,6)+"..."+t.slice(-4)}return t}
  function friendlySourceLabel(source){const raw=String(source||"").trim();if(!raw)return {key:"unlabeled-source",label:"未标记来源",raw:"-"};if(looksLikeSecret(raw)){const masked=maskSensitive(raw);return {key:"api-key:"+masked,label:"API Key · "+masked,raw:raw}}return {key:raw,label:raw,raw:raw}}
  function providerToneClass(name){const value=String(name||"").toLowerCase();if(value.includes("minimax"))return"minimax";if(value.includes("nvidia"))return"nvidia";if(value.includes("bailian"))return"bailian";if(value.includes("claude"))return"claude";if(value.includes("codex"))return"codex";if(value.includes("gemini"))return"gemini";if(value.includes("vertex"))return"vertex";return"default"}
  function getFirst(obj,keys){for(let i=0;i<keys.length;i++){const k=keys[i];if(obj&&obj[k]!=null)return obj[k]}return undefined}
  function buildProviderMap(config){const map={};if(!config||typeof config!=="object")return map;const openaiCompat=getFirst(config,["openai-compatibility","openaiCompatibility"])||[];openaiCompat.forEach(function(provider){const providerName=String(getFirst(provider,["name"])||"OpenAI Compatible").trim();const entries=getFirst(provider,["api-key-entries","apiKeyEntries"])||[];entries.forEach(function(entry){const apiKey=String(getFirst(entry,["api-key","apiKey"])||"").trim();if(apiKey)map[apiKey]=providerName})});[{keys:["gemini-api-key","geminiApiKey"],name:"Gemini"},{keys:["claude-api-key","claudeApiKey"],name:"Claude"},{keys:["codex-api-key","codexApiKey"],name:"Codex"},{keys:["vertex-api-key","vertexApiKey"],name:"Vertex"}].forEach(function(entry){const list=getFirst(config,entry.keys)||[];list.forEach(function(item){const apiKey=String(getFirst(item,["api-key","apiKey"])||"").trim();if(apiKey)map[apiKey]=entry.name})});return map}
  function resolveProviderName(apiKey, source){const sourceText=String(source||"").trim();const keyText=String(apiKey||"").trim();if(sourceText&&!looksLikeSecret(sourceText))return sourceText;if(sourceText&&state.providerMap[sourceText])return state.providerMap[sourceText];if(keyText&&state.providerMap[keyText])return state.providerMap[keyText];return friendlySourceLabel(sourceText||keyText).label}
  function syncManagementKey(v){const next=String(v||"").trim();if(!next)return;state.managementKey=next;el.managementKey.value=next;localStorage.setItem(STORAGE_KEY,next)}
  function applySavedInputs(){el.managementKey.value=state.managementKey;el.costPool.value=String(state.costPool)}
  function renderDonut(rows,totalTokens){if(!rows.length||totalTokens<=0)return'<div class="chart-empty">暂无可用于绘制占比图的数据。</div>';const topRows=rows.slice(0,8).map((row,index)=>({label:row.apiKeyLabel||row.apiKey,tokens:row.totalTokens,color:PALETTE[index%PALETTE.length]}));if(rows.length>8){const otherTokens=rows.slice(8).reduce((sum,row)=>sum+row.totalTokens,0);if(otherTokens>0)topRows.push({label:"其他 Key",tokens:otherTokens,color:"#cabda8"})}let current=0;const gradient=topRows.map(function(row){const next=current+row.tokens/totalTokens*100;const seg=row.color+" "+current.toFixed(2)+"% "+next.toFixed(2)+"%";current=next;return seg}).join(", ");const legend=topRows.map(function(row){return '<div class="legend-item"><span class="legend-dot" style="background:'+row.color+';"></span><span class="legend-label" title="'+escapeHtml(row.label)+'">'+escapeHtml(row.label)+'</span><span class="legend-value">'+formatPercent(row.tokens,totalTokens)+'</span></div>'}).join("");return '<div class="donut-layout"><div class="donut-shell" style="background:conic-gradient('+gradient+');"><div class="donut-hole"><strong>'+rows.length+'</strong><span>参与分摊的 Key</span></div></div><div class="legend-list">'+legend+'</div></div>'}
  function renderBarRows(rows,totalTokens,labelKey,tokenKey,subtitleBuilder){if(!rows.length||totalTokens<=0)return '<div class="chart-empty">暂无可用于绘制条形图的数据。</div>';return '<div class="bar-list">'+rows.map(function(row,index){const tokens=row[tokenKey];const share=totalTokens>0?tokens/totalTokens:0;const cost=state.costPool*share;const color=PALETTE[index%PALETTE.length];const subtitle=subtitleBuilder?subtitleBuilder(row,tokens,totalTokens):(formatCompactTokens(tokens)+' Tokens / '+formatPercent(tokens,totalTokens));return '<div class="bar-row"><div class="bar-key"><strong title="'+escapeHtml(row[labelKey])+'">'+escapeHtml(row[labelKey])+'</strong><span>'+escapeHtml(subtitle)+'</span></div><div class="bar-track"><div class="bar-fill" style="width:'+Math.max(2,share*100)+'%;background:'+color+';"></div></div><div class="bar-cost">'+formatCurrency(cost)+'</div></div>'}).join("")+'</div>'}
  function renderProviderStructure(rows,totalTokens){if(!rows.length||totalTokens<=0)return '<div class="chart-empty">暂无可用于绘制供应商费用结构的数据。</div>';return '<div class="provider-list">'+rows.map(function(row){const share=totalTokens>0?row.tokens/totalTokens:0;const cost=state.costPool*share;const tone=providerToneClass(row.sourceLabel);return '<div class="provider-item"><div class="provider-head"><div class="provider-left"><span class="provider-badge '+tone+'">'+escapeHtml(row.sourceLabel)+'</span></div><span class="provider-share">'+formatPercent(row.tokens,totalTokens)+'</span></div><div class="provider-meta"><span class="provider-chip">关联模型数：'+formatNumber(row.modelsSet?row.modelsSet.size:0)+'</span><span class="provider-chip">请求数：'+formatNumber(row.requests)+'</span><span class="provider-chip">承担金额：'+formatCurrency(cost)+'</span></div><div class="bar-track"><div class="bar-fill" style="width:'+Math.max(2,share*100)+'%;"></div></div></div>'}).join("")+'</div>'}
  function renderTrend(days,totalTokens){if(!days.length||totalTokens<=0)return '<div class="chart-empty">最近 7 日暂无足够样本。</div>';const maxTokens=days.reduce((max,item)=>Math.max(max,item.tokens),0)||1;return '<div class="trend-bars">'+days.map(function(item){const ratio=item.tokens/maxTokens;const cost=state.costPool*(item.tokens/totalTokens);return '<div class="trend-col"><div class="trend-track"><div class="trend-fill" style="height:'+Math.max(8,ratio*100)+'%;"></div></div><span class="trend-label">'+escapeHtml(item.day.slice(5))+'</span><span class="trend-value">'+formatCurrency(cost)+'</span></div>'}).join("")+'</div>'}
  function renderModelBars(models,totalTokens){if(!models.length||totalTokens<=0)return '<div class="chart-empty">暂无可用于绘制模型分摊的数据。</div>';return renderBarRows(models.slice(0,10),totalTokens,"modelName","tokens",function(model){return formatNumber(model.requests)+' 次请求 / '+formatNumber(model.keySet.size)+' 个 key 参与'})}
  function renderRiskWarnings(rows,totalTokens){if(!rows.length||totalTokens<=0)return '<div class="chart-empty">暂无可分析的 key 风险。</div>';const risks=rows.map(function(row){const share=totalTokens>0?row.totalTokens/totalTokens:0;const failureRate=row.totalRequests>0?row.failureCount/row.totalRequests:0;const allocatedCost=state.costPool*share;let score=0;const reasons=[];if(share>=0.35){score+=55;reasons.push('成本占比 '+formatPercent(row.totalTokens,totalTokens)+'，单 key 集中度偏高')}else if(share>=0.2){score+=30;reasons.push('成本占比 '+formatPercent(row.totalTokens,totalTokens)+'，需要持续关注')}if(failureRate>=0.3&&row.totalRequests>=5){score+=30;reasons.push('失败率 '+(failureRate*100).toFixed(1)+'%，可能影响稳定交付')}else if(failureRate>=0.15&&row.totalRequests>=5){score+=15;reasons.push('失败率 '+(failureRate*100).toFixed(1)+'%，建议排查')}if(allocatedCost>=state.costPool*0.25){score+=20;reasons.push('单 key 承担金额已达 '+formatCurrency(allocatedCost))}return {apiKey:row.apiKey,score:score,level:score>=60?'high':score>=30?'medium':'low',reasons:reasons}}).filter(r=>r.score>0).sort((a,b)=>b.score-a.score).slice(0,5);if(!risks.length)return '<div class="chart-empty">当前没有明显的高风险 key，成本分布相对均衡。</div>';return '<div class="risk-list">'+risks.map(function(risk){return '<div class="risk-item '+(risk.level==='high'?'high':'medium')+'"><div class="risk-head"><div class="risk-title" title="'+escapeHtml(maskSensitive(risk.apiKey))+'">'+escapeHtml(maskSensitive(risk.apiKey))+'</div><div class="risk-score">风险分 '+risk.score+'</div></div><div class="risk-reason">'+escapeHtml(risk.reasons.join('；'))+'</div></div>'}).join("")+'</div>'}
  function collectSnapshot(snapshot){
    const apis=snapshot&&snapshot.apis?snapshot.apis:{};
    const rows=[]; const recent=[]; const providerMap=new Map(); const dayMap=new Map(); const modelMap=new Map();
    Object.keys(apis).forEach(function(apiKey){
      const api=apis[apiKey]||{}; const models=api.models||{}; let successCount=0; let failureCount=0; let requestCount=0;
      Object.keys(models).forEach(function(modelName){
        const model=models[modelName]||{};
        (model.details||[]).forEach(function(detail){
          const tokens=detail.tokens||{}; const totalTokens=Number(tokens.total_tokens||tokens.totalTokens||0); const failed=Boolean(detail.failed);
          requestCount+=1; if(failed) failureCount+=1; else successCount+=1;
          recent.push({apiKey:apiKey,apiKeyLabel:maskSensitive(apiKey),modelName:modelName,timestamp:detail.timestamp||"",source:detail.source||"-",failed:failed,totalTokens:totalTokens});
          const providerName=resolveProviderName(apiKey,detail.source||"-");
          const provider=providerMap.get(providerName)||{sourceLabel:providerName,tokens:0,requests:0,modelsSet:new Set()};
          provider.tokens+=totalTokens; provider.requests+=1; provider.modelsSet.add(modelName||"-"); providerMap.set(providerName,provider);
          const dayKey=(detail.timestamp||"").slice(0,10)||"未知日期"; const dayEntry=dayMap.get(dayKey)||{day:dayKey,tokens:0,requests:0}; dayEntry.tokens+=totalTokens; dayEntry.requests+=1; dayMap.set(dayKey,dayEntry);
          const modelKey=modelName||"-"; const modelEntry=modelMap.get(modelKey)||{modelName:modelKey,tokens:0,requests:0,keySet:new Set()}; modelEntry.tokens+=totalTokens; modelEntry.requests+=1; modelEntry.keySet.add(apiKey); modelMap.set(modelKey,modelEntry);
        });
      });
      rows.push({apiKey:apiKey,apiKeyLabel:maskSensitive(apiKey),totalRequests:Number(api.total_requests||api.totalRequests||requestCount||0),totalTokens:Number(api.total_tokens||api.totalTokens||0),successCount:successCount,failureCount:failureCount});
    });
    rows.sort((a,b)=>b.totalTokens-a.totalTokens||b.totalRequests-a.totalRequests);
    recent.sort((a,b)=>new Date(b.timestamp).getTime()-new Date(a.timestamp).getTime());
    return {rows:rows,recent:recent.slice(0,20),providers:Array.from(providerMap.values()).sort((a,b)=>b.tokens-a.tokens),models:Array.from(modelMap.values()).sort((a,b)=>b.tokens-a.tokens),daily:Array.from(dayMap.values()).sort((a,b)=>new Date(a.day).getTime()-new Date(b.day).getTime()).slice(-7)};
  }
  function render(){
    const costPool=Number(el.costPool.value||0); state.costPool=Number.isFinite(costPool)?costPool:0; el.costPoolValue.textContent=formatCurrency(state.costPool);
    if(state.loading){el.statusHint.textContent='正在读取使用统计并重新计算各项费用结构...';}
    else if(state.error){el.statusHint.innerHTML='<span class="danger-text">'+escapeHtml(state.error)+'</span>';}
    else {el.statusHint.textContent='按总 token 占比摊成本池，适合快速核算每个 key / 供应商 / 模型应承担多少费用。';}
    el.authFallback.style.display=state.unauthorized?'block':'none';
    if(!state.usage){
      el.totalRequests.textContent='-'; el.requestsMeta.textContent='等待计费数据'; el.totalTokens.textContent='-'; el.activeKeys.textContent='-';
      el.keyDonutWrap.textContent='等待生成占比图表。'; el.keyBarsWrap.textContent='等待生成分摊条形图。'; el.sourceBarsWrap.textContent='等待生成供应商费用结构图。';
      el.trendWrap.textContent='等待生成趋势图。'; el.modelBarsWrap.textContent='等待生成模型分摊图。'; el.riskWrap.textContent='等待风险预警结果。';
      el.keyTableWrap.textContent='还没有可用的分摊数据。'; el.detailTableWrap.textContent='暂无最近样本。'; return;
    }
    const totalRequests=Number(state.usage.total_requests||state.usage.totalRequests||0);
    const successCount=Number(state.usage.success_count||state.usage.successCount||0);
    const failureCount=Number(state.usage.failure_count||state.usage.failureCount||0);
    const totalTokens=Number(state.usage.total_tokens||state.usage.totalTokens||0);
    const data=collectSnapshot(state.usage);
    el.totalRequests.textContent=formatNumber(totalRequests); el.requestsMeta.textContent='成功 '+formatNumber(successCount)+' / 失败 '+formatNumber(failureCount); el.totalTokens.textContent=formatCompactTokens(totalTokens); el.activeKeys.textContent=formatNumber(data.rows.length);
    el.keyDonutWrap.innerHTML=renderDonut(data.rows,totalTokens); el.keyBarsWrap.innerHTML=renderBarRows(data.rows,totalTokens,'apiKeyLabel','totalTokens'); el.sourceBarsWrap.innerHTML=renderProviderStructure(data.providers,totalTokens); el.trendWrap.innerHTML=renderTrend(data.daily,totalTokens); el.modelBarsWrap.innerHTML=renderModelBars(data.models,totalTokens); el.riskWrap.innerHTML=renderRiskWarnings(data.rows,totalTokens);
    if(!data.rows.length||totalTokens<=0){el.keyTableWrap.innerHTML='<div class="chart-empty">当前统计里还没有可用于分摊的 key/token 数据。</div>';} else {
      const body=data.rows.map(function(row){const share=row.totalTokens/totalTokens; const allocatedCost=state.costPool*share; const successRate=row.totalRequests>0?((row.successCount/row.totalRequests)*100).toFixed(1)+'%':'-'; return '<tr><td>'+escapeHtml(row.apiKeyLabel||row.apiKey)+'</td><td>'+formatNumber(row.totalRequests)+'</td><td>'+formatCompactTokens(row.totalTokens)+'</td><td>'+formatPercent(row.totalTokens,totalTokens)+'</td><td>'+successRate+'</td><td>'+formatCurrency(allocatedCost)+'</td></tr>';}).join('');
      el.keyTableWrap.innerHTML='<table><thead><tr><th>API Key</th><th>请求数</th><th>总 Tokens</th><th>成本占比</th><th>成功率</th><th>应承担金额</th></tr></thead><tbody>'+body+'</tbody></table>';
    }
    if(!data.recent.length||totalTokens<=0){el.detailTableWrap.innerHTML='<div class="chart-empty">暂无最近样本。</div>';} else {
      const body=data.recent.map(function(item){const allocatedCost=state.costPool*(item.totalTokens/totalTokens); return '<tr><td>'+escapeHtml(item.timestamp||'-')+'</td><td>'+escapeHtml(item.modelName||'-')+'</td><td>'+escapeHtml(item.apiKeyLabel||item.apiKey||'-')+'</td><td>'+escapeHtml(resolveProviderName(item.apiKey,item.source))+'</td><td><span class="badge '+(item.failed?'failure':'success')+'">'+(item.failed?'失败':'成功')+'</span></td><td>'+formatCompactTokens(item.totalTokens)+'</td><td>'+formatCurrency(allocatedCost)+'</td></tr>';}).join('');
      el.detailTableWrap.innerHTML='<table><thead><tr><th>时间</th><th>模型</th><th>API Key</th><th>来源</th><th>状态</th><th>Tokens</th><th>折算成本</th></tr></thead><tbody>'+body+'</tbody></table>';
    }
  }
  async function loadUsage(){
    state.loading=true; state.error=''; state.unauthorized=false; render();
    try {
      state.managementKey=String(el.managementKey.value||'').trim(); localStorage.setItem(STORAGE_KEY,state.managementKey); localStorage.setItem(COST_POOL_KEY,String(el.costPool.value||'100'));
      const headers=state.managementKey?{Authorization:/^Bearer\s+/i.test(state.managementKey)?state.managementKey:'Bearer '+state.managementKey}:{};
      const [usageResponse, configResponse] = await Promise.all([
        fetchNative('/v0/management/usage',{method:'GET',headers:headers,credentials:'same-origin'}),
        fetchNative('/v0/management/config',{method:'GET',headers:headers,credentials:'same-origin'})
      ]);
      if(usageResponse.status===401){state.usage=null; state.unauthorized=true; state.error='管理接口未授权，请输入 management key。'; return;}
      if(!usageResponse.ok){throw new Error('HTTP '+usageResponse.status)}
      const payload=await usageResponse.json(); state.usage=payload&&payload.usage?payload.usage:payload;
      if(configResponse.ok){state.config=await configResponse.json(); state.providerMap=buildProviderMap(state.config);} else {state.providerMap={};}
    } catch(error){ state.usage=null; state.error='读取计费数据失败：'+(error&&error.message?error.message:'未知错误'); }
    finally { state.loading=false; render(); }
  }
  window.addEventListener('message', function (event) { if(event&&event.data&&event.data.type==='ops-billing-auth'){ syncManagementKey(event.data.key); } });
  if(EMBED_MODE && window.parent && window.parent!==window){ window.parent.postMessage({ type:'ops-billing-request-auth' }, '*'); }
  el.refreshBtn.addEventListener('click', loadUsage); el.saveAndLoad.addEventListener('click', loadUsage); el.costPool.addEventListener('input', render);
  el.backToPanel.addEventListener('click', function(){ if(EMBED_MODE && window.parent && window.parent!==window){ window.parent.postMessage({ type:'ops-billing-close' }, '*'); } else { window.location.href='/management.html#/usage'; } });
  applySavedInputs(); render(); if(state.managementKey){ loadUsage(); }
})();

