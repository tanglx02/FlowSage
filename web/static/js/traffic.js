// FlowSage 流量分析页：HAR 导入 → 归一化流量 → 接口清单
//
// 后端：POST /api/traffic/har、GET /api/traffic/inventories、GET/DELETE /api/traffic/inventories/:id
// 分析结果结构见 internal/apianalyze（Inventory / Endpoint / Schema）。

const trafficState = {
    inventories: [],
    selectedId: '',
    analysis: null,
    moduleFilter: '',
    selectedEndpoint: '',
    uploading: false
};

function initTrafficPage() {
    loadTrafficInventories();
}

function trafficNotify(message, type) {
    if (typeof notifyApiError === 'function') {
        notifyApiError(message, type || 'info');
    } else {
        console.warn(message);
    }
}

// ---------------------------------------------------------------- 清单列表

async function loadTrafficInventories() {
    const listEl = document.getElementById('traffic-list');
    if (listEl) listEl.innerHTML = '<div class="traffic-empty">加载中…</div>';
    try {
        const res = await apiFetch('/api/traffic/inventories');
        if (!(await ensureApiOk(res, '加载接口清单失败'))) return;
        const data = await res.json();
        trafficState.inventories = data.items || [];
        renderTrafficInventoryList();
    } catch (err) {
        if (listEl) listEl.innerHTML = '<div class="traffic-empty">加载失败</div>';
        trafficNotify('加载接口清单失败: ' + err.message, 'error');
    }
}

function renderTrafficInventoryList() {
    const listEl = document.getElementById('traffic-list');
    const countEl = document.getElementById('traffic-list-count');
    if (!listEl) return;
    if (countEl) countEl.textContent = String(trafficState.inventories.length);
    if (!trafficState.inventories.length) {
        listEl.innerHTML = '<div class="traffic-empty">还没有接口清单<br>先导入一份 .har 抓包文件</div>';
        return;
    }
    listEl.innerHTML = trafficState.inventories.map((item) => {
        const active = item.id === trafficState.selectedId ? ' is-active' : '';
        const host = item.host ? `<span class="traffic-item-host">${escapeHtml(item.host)}</span>` : '';
        return `
            <div class="traffic-item${active}" onclick='selectTrafficInventory(${escapeJsStringAttr(item.id)})'>
                <div class="traffic-item-name" title="${escapeAttr(item.name)}">${escapeHtml(item.name)}</div>
                ${host}
                <div class="traffic-item-meta">
                    <span>${item.endpoint_count} 个接口</span>
                    <span>${item.record_count} 条流量</span>
                </div>
            </div>`;
    }).join('');
}

async function selectTrafficInventory(id) {
    trafficState.selectedId = id;
    trafficState.analysis = null;
    trafficState.moduleFilter = '';
    trafficState.selectedEndpoint = '';
    renderTrafficInventoryList();
    const detailEl = document.getElementById('traffic-detail');
    const placeholderEl = document.getElementById('traffic-placeholder');
    if (placeholderEl) placeholderEl.hidden = true;
    if (detailEl) {
        detailEl.hidden = false;
        detailEl.innerHTML = '<div class="traffic-empty">解析中…</div>';
    }
    try {
        const res = await apiFetch('/api/traffic/inventories/' + encodeURIComponent(id));
        if (!(await ensureApiOk(res, '读取接口清单失败'))) return;
        const data = await res.json();
        trafficState.analysis = data.analysis || null;
        renderTrafficDetail(data);
    } catch (err) {
        if (detailEl) detailEl.innerHTML = '<div class="traffic-empty">读取失败</div>';
        trafficNotify('读取接口清单失败: ' + err.message, 'error');
    }
}

async function deleteTrafficInventory() {
    const id = trafficState.selectedId;
    if (!id) return;
    if (!window.confirm('删除这份接口清单？')) return;
    try {
        const res = await apiFetch('/api/traffic/inventories/' + encodeURIComponent(id), { method: 'DELETE' });
        if (!(await ensureApiOk(res, '删除失败'))) return;
        trafficState.selectedId = '';
        trafficState.analysis = null;
        const detailEl = document.getElementById('traffic-detail');
        const placeholderEl = document.getElementById('traffic-placeholder');
        if (detailEl) { detailEl.hidden = true; detailEl.innerHTML = ''; }
        if (placeholderEl) placeholderEl.hidden = false;
        await loadTrafficInventories();
    } catch (err) {
        trafficNotify('删除失败: ' + err.message, 'error');
    }
}

// ---------------------------------------------------------------- 上传 HAR

async function uploadTrafficHAR(input) {
    if (!input || !input.files || !input.files.length) return;
    if (trafficState.uploading) {
        trafficNotify('正在解析上一份文件，请稍候', 'warning');
        input.value = '';
        return;
    }
    const file = input.files[0];
    const statusEl = document.getElementById('traffic-upload-status');
    const form = new FormData();
    form.append('file', file);
    form.append('name', file.name);

    trafficState.uploading = true;
    if (statusEl) {
        statusEl.hidden = false;
        statusEl.textContent = `正在解析 ${file.name}（${formatTrafficSize(file.size)}）…`;
    }
    try {
        const res = typeof apiUploadWithProgress === 'function'
            ? await apiUploadWithProgress('/api/traffic/har', form, {
                onProgress: (p) => {
                    if (statusEl) statusEl.textContent = `上传中 ${p.percent}%（${file.name}）`;
                }
            })
            : await apiFetch('/api/traffic/har', { method: 'POST', body: form });
        if (!(await ensureApiOk(res, 'HAR 解析失败'))) return;
        const data = await res.json();
        if (statusEl) {
            statusEl.textContent = `解析完成：${data.recordCount} 条流量 → ${data.endpointCount} 个接口`;
        }
        await loadTrafficInventories();
        await selectTrafficInventory(data.id);
    } catch (err) {
        if (statusEl) statusEl.textContent = '解析失败：' + err.message;
        trafficNotify('HAR 解析失败: ' + err.message, 'error');
    } finally {
        trafficState.uploading = false;
        input.value = '';
    }
}

function formatTrafficSize(bytes) {
    if (!bytes && bytes !== 0) return '';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

// ---------------------------------------------------------------- 详情渲染

function renderTrafficDetail(meta) {
    const detailEl = document.getElementById('traffic-detail');
    if (!detailEl) return;
    const analysis = trafficState.analysis || {};

    const chips = [
        `<span class="traffic-chip">${escapeHtml(meta.source || 'har')}</span>`,
        meta.host ? `<span class="traffic-chip">${escapeHtml(meta.host)}</span>` : '',
        `<span class="traffic-chip">${meta.recordCount} 条流量</span>`,
        `<span class="traffic-chip traffic-chip-accent">${meta.endpointCount} 个接口</span>`
    ].filter(Boolean).join('');

    const activeEndpoint = trafficState.selectedEndpoint;

    detailEl.innerHTML = `
        <header class="traffic-detail-header">
            <div>
                <h3 class="traffic-detail-title">${escapeHtml(meta.name || '接口清单')}</h3>
                <div class="traffic-chips">${chips}</div>
            </div>
            <div class="traffic-detail-actions">
                <button class="btn-secondary btn-small" onclick="exportTrafficInventory()">导出 JSON</button>
                <button class="btn-danger btn-small" data-require-permission="traffic:delete" onclick="deleteTrafficInventory()">删除</button>
            </div>
        </header>
        <div class="traffic-cards">
            ${renderEnvelopeCard(analysis.envelope)}
            ${renderAuthCard(analysis.auth)}
        </div>
        <div class="traffic-endpoints">
            <div class="traffic-module-filter" id="traffic-module-filter"></div>
            <div class="traffic-table-wrap" id="traffic-endpoint-table"></div>
        </div>
        <div id="traffic-endpoint-detail" class="traffic-endpoint-detail"></div>
    `;

    renderTrafficModuleFilter();
    renderTrafficEndpointTable();
    if (activeEndpoint) renderTrafficEndpointDetail(activeEndpoint);
    if (typeof applyRBACToUI === 'function') applyRBACToUI(detailEl);
}

function renderEnvelopeCard(envelope) {
    if (!envelope || !envelope.detected) {
        return `<div class="traffic-card"><div class="traffic-card-title">统一外壳</div>
            <div class="traffic-card-body traffic-muted">未识别到统一响应外壳</div></div>`;
    }
    const rows = [
        ['成功判定', envelope.successCheck || '-'],
        ['数据字段', envelope.dataField || '-'],
        ['消息字段', envelope.messageField || '-'],
        ['覆盖率', envelope.coverage || '-']
    ].map(([k, v]) => `<div class="traffic-kv"><span>${k}</span><b>${escapeHtml(String(v))}</b></div>`).join('');
    return `<div class="traffic-card">
        <div class="traffic-card-title">统一外壳</div>
        <div class="traffic-card-body">${rows}</div>
    </div>`;
}

function renderAuthCard(auth) {
    if (!auth || (!auth.modes || !auth.modes.length)) {
        return `<div class="traffic-card"><div class="traffic-card-title">鉴权与固定头</div>
            <div class="traffic-card-body traffic-muted">样本里没有明显的鉴权特征</div></div>`;
    }
    const modes = (auth.modes || []).join(' / ');
    const fixed = Object.entries(auth.fixedHeaders || {})
        .map(([k, v]) => `<div class="traffic-kv"><span>${escapeHtml(k)}</span><b>${escapeHtml(v)}</b></div>`).join('');
    return `<div class="traffic-card">
        <div class="traffic-card-title">鉴权与固定头</div>
        <div class="traffic-card-body">
            <div class="traffic-kv"><span>方式</span><b>${escapeHtml(modes)}</b></div>
            ${auth.tokenPath ? `<div class="traffic-kv"><span>取 token</span><b>${escapeHtml(auth.tokenPath)}</b></div>` : ''}
            ${fixed}
        </div>
    </div>`;
}

function renderTrafficModuleFilter() {
    const el = document.getElementById('traffic-module-filter');
    const analysis = trafficState.analysis;
    if (!el || !analysis) return;
    const groups = analysis.groups || [];
    const current = trafficState.moduleFilter;
    const buttons = [`<button class="traffic-module-btn${current === '' ? ' is-active' : ''}" onclick="filterTrafficModule('')">全部 ${analysis.endpoints ? analysis.endpoints.length : 0}</button>`];
    groups.forEach((g) => {
        const count = (g.endpoints || []).length;
        buttons.push(`<button class="traffic-module-btn${current === g.name ? ' is-active' : ''}" onclick='filterTrafficModule(${escapeJsStringAttr(g.name)})'>${escapeHtml(g.name)} ${count}</button>`);
    });
    el.innerHTML = buttons.join('');
}

function filterTrafficModule(name) {
    trafficState.moduleFilter = name;
    renderTrafficModuleFilter();
    renderTrafficEndpointTable();
}

function renderTrafficEndpointTable() {
    const el = document.getElementById('traffic-endpoint-table');
    const analysis = trafficState.analysis;
    if (!el || !analysis) return;
    const endpoints = (analysis.endpoints || []).filter((ep) => !trafficState.moduleFilter || ep.module === trafficState.moduleFilter);
    if (!endpoints.length) {
        el.innerHTML = '<div class="traffic-empty">没有接口</div>';
        return;
    }
    const rows = endpoints.map((ep) => {
        const key = ep.method + ' ' + ep.pathPattern;
        const active = key === trafficState.selectedEndpoint ? ' is-active' : '';
        const status = (ep.statusCodes || []).join(', ');
        const pageTag = ep.pagination && ep.pagination.detected ? '<span class="traffic-tag">分页</span>' : '';
        return `<tr class="traffic-row${active}" onclick='renderTrafficEndpointDetail(${escapeJsStringAttr(key)})'>
            <td><span class="traffic-method traffic-method-${ep.method.toLowerCase()}">${escapeHtml(ep.method)}</span></td>
            <td class="traffic-path">${escapeHtml(ep.pathPattern)}</td>
            <td>${escapeHtml(ep.module || '')}</td>
            <td>${ep.callCount}</td>
            <td>${escapeHtml(status)}</td>
            <td>${pageTag}</td>
        </tr>`;
    }).join('');
    el.innerHTML = `<table class="traffic-table">
        <thead><tr><th>方法</th><th>路径</th><th>模块</th><th>次数</th><th>状态码</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
    </table>`;
}

function findTrafficEndpoint(key) {
    const analysis = trafficState.analysis;
    if (!analysis) return null;
    return (analysis.endpoints || []).find((ep) => (ep.method + ' ' + ep.pathPattern) === key) || null;
}

function renderTrafficEndpointDetail(key) {
    trafficState.selectedEndpoint = key;
    renderTrafficEndpointTable();
    const el = document.getElementById('traffic-endpoint-detail');
    const ep = findTrafficEndpoint(key);
    if (!el || !ep) return;

    const queryRows = (ep.query || []).map((f) => `<tr>
        <td>${escapeHtml(f.name)}</td><td>${escapeHtml(f.type)}</td>
        <td>${f.required ? '是' : ''}</td><td>${escapeHtml(f.sample || '')}</td>
    </tr>`).join('');

    const pagination = ep.pagination && ep.pagination.detected ? `
        <div class="traffic-sub-title">分页</div>
        <div class="traffic-kv"><span>模式</span><b>${escapeHtml(ep.pagination.mode || '')}</b></div>
        <div class="traffic-kv"><span>页码参数</span><b>${escapeHtml(ep.pagination.pageParam || '')} / ${escapeHtml(ep.pagination.sizeParam || '')}</b></div>
        <div class="traffic-kv"><span>列表字段</span><b>${escapeHtml(ep.pagination.listField || '')} / ${escapeHtml(ep.pagination.totalField || '')}</b></div>` : '';

    el.innerHTML = `
        <div class="traffic-sub-title">接口详情</div>
        <div class="traffic-endpoint-head">
            <span class="traffic-method traffic-method-${ep.method.toLowerCase()}">${escapeHtml(ep.method)}</span>
            <code>${escapeHtml(ep.pathPattern)}</code>
        </div>
        <div class="traffic-chips">
            <span class="traffic-chip">调用 ${ep.callCount} 次</span>
            <span class="traffic-chip">状态码 ${(ep.statusCodes || []).join(', ')}</span>
            ${(ep.sampleIds || []).length ? `<span class="traffic-chip">样本 ${ep.sampleIds.length} 条</span>` : ''}
        </div>
        ${pagination}
        ${queryRows ? `<div class="traffic-sub-title">查询参数</div>
            <table class="traffic-table"><thead><tr><th>名称</th><th>类型</th><th>必填</th><th>样本</th></tr></thead><tbody>${queryRows}</tbody></table>` : ''}
        ${renderTrafficSchema('请求结构', ep.request)}
        ${renderTrafficSchema('响应结构', ep.response)}
    `;
    el.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

function renderTrafficSchema(title, schema) {
    const fields = schema && schema.fields ? schema.fields : [];
    if (!fields.length) return '';
    return `<div class="traffic-sub-title">${title}</div>
        <div class="traffic-schema">${renderTrafficFields(fields, 0)}</div>`;
}

function renderTrafficFields(fields, depth) {
    return fields.map((f) => {
        const tags = [];
        if (f.required) tags.push('<span class="traffic-tag traffic-tag-required">必填</span>');
        if (f.fixed) tags.push('<span class="traffic-tag">固定值</span>');
        if (f.enum && f.enum.length) tags.push(`<span class="traffic-tag">候选 ${f.enum.length} 个</span>`);
        const itemType = f.items && f.items.type ? f.items.type : '';
        const typeText = f.type === 'array' && itemType ? `array&lt;${escapeHtml(itemType)}&gt;` : escapeHtml(f.type);
        const row = `<div class="traffic-field" style="padding-left:${depth * 16}px">
            <span class="traffic-field-name">${escapeHtml(f.name)}</span>
            <span class="traffic-field-type">${typeText}</span>
            ${tags.join('')}
            ${f.sample !== undefined && f.sample !== '' ? `<span class="traffic-field-sample">${escapeHtml(f.sample)}</span>` : ''}
        </div>`;
        const children = [];
        if (f.fields && f.fields.length) children.push(renderTrafficFields(f.fields, depth + 1));
        if (f.items && f.items.fields && f.items.fields.length) {
            children.push(`<div class="traffic-field traffic-field-items" style="padding-left:${depth * 16}px">[] 元素</div>`);
            children.push(renderTrafficFields(f.items.fields, depth + 1));
        }
        return row + children.join('');
    }).join('');
}

function exportTrafficInventory() {
    const analysis = trafficState.analysis;
    if (!analysis) return;
    const blob = new Blob([JSON.stringify(analysis, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = (trafficState.selectedId || 'inventory') + '.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

window.initTrafficPage = initTrafficPage;
window.loadTrafficInventories = loadTrafficInventories;
window.selectTrafficInventory = selectTrafficInventory;
window.deleteTrafficInventory = deleteTrafficInventory;
window.uploadTrafficHAR = uploadTrafficHAR;
window.filterTrafficModule = filterTrafficModule;
window.renderTrafficEndpointDetail = renderTrafficEndpointDetail;
window.exportTrafficInventory = exportTrafficInventory;