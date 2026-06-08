/* ============================================================
   pages/platform-admin/tenants.js — 平台管理员·租户与平台大盘域
   ------------------------------------------------------------
   覆盖:学校管理(租户列表 + 状态/续期 强二次确认)、租户详情、
        入驻审核(按状态 Tab + 审核详情:通过建租户发激活码 / 驳回填因)、
        平台看板(跨校 KPI + 内联 SVG 图表)、平台统计(趋势 + 分布 + 范围筛选)。
   说明:遵循 courses.js 范式 —— registerPages({route: ctx => htmlString}),
        复用 C.* 工具;子页登记 C.parentRoute 以高亮侧栏;
        危险操作(停用/续期)必经 C.confirm 强二次确认 + C.toast;
        图表为纯内联 SVG/CSS 并附图例数值,颜色非唯一信息载体。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项 */
  Object.assign(C.parentRoute, {
    'platform-admin/tenant-detail': 'platform-admin/tenants',
    'platform-admin/application-review': 'platform-admin/applications',
  });

  /* ---------- 小工具:状态 → 徽章 ---------- */
  const tenantBadge = (s) => ({ '正常': 'green', '停用': 'red', '到期': 'amber' }[s] || 'gray');
  const tenantDot = (s) => ({ '正常': 'green', '停用': 'red', '到期': 'amber' }[s] || 'gray');
  const appBadge = (s) => ({ '待审': 'amber', '通过': 'green', '驳回': 'red' }[s] || 'gray');

  /* 派生:到期日是否临近(原型用,统一阈值演示) */
  function expireHint(expire) {
    const d = new Date(expire); if (isNaN(d)) return '';
    const days = Math.round((d - new Date('2026-06-07')) / 86400000);
    if (days < 0) return `<span class="badge badge-red">已过期 ${-days} 天</span>`;
    if (days <= 60) return `<span class="badge badge-amber">${days} 天后到期</span>`;
    return '';
  }

  /* ============================================================
     内联 SVG 图表工具(无外部库;均带图例/数值)
     ============================================================ */

  /* 折线图:points=[{x:'标签', y:数值}],单序列;带网格、点、数值标注 */
  function lineChart(points, opts = {}) {
    const W = 520, H = 180, pad = 30;
    const max = Math.max(...points.map(p => p.y)) * 1.15 || 1;
    const stepX = (W - pad * 2) / (points.length - 1 || 1);
    const sx = (i) => pad + i * stepX;
    const sy = (v) => H - pad - (v / max) * (H - pad * 2);
    const color = opts.color || 'amber';
    const line = points.map((p, i) => `${i ? 'L' : 'M'}${sx(i).toFixed(1)},${sy(p.y).toFixed(1)}`).join(' ');
    const area = `M${sx(0)},${(H - pad).toFixed(1)} ` + points.map((p, i) => `L${sx(i).toFixed(1)},${sy(p.y).toFixed(1)}`).join(' ') + ` L${sx(points.length - 1).toFixed(1)},${(H - pad).toFixed(1)} Z`;
    const grid = [0, .25, .5, .75, 1].map(t => {
      const y = (pad + t * (H - pad * 2)).toFixed(1);
      return `<line x1="${pad}" y1="${y}" x2="${W - pad}" y2="${y}" stroke="var(--color-border)" stroke-width="1"/>`;
    }).join('');
    return `<svg viewBox="0 0 ${W} ${H}" width="100%" role="img" aria-label="${C.esc(opts.label || '趋势图')}" preserveAspectRatio="none" style="overflow:visible">
      ${grid}
      <path d="${area}" fill="var(--${color}-100)" opacity=".5"/>
      <path d="${line}" fill="none" stroke="var(--${color}-600)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
      ${points.map((p, i) => `<circle cx="${sx(i).toFixed(1)}" cy="${sy(p.y).toFixed(1)}" r="3.5" fill="var(--color-surface)" stroke="var(--${color}-600)" stroke-width="2"/>`).join('')}
      ${points.map((p, i) => `<text x="${sx(i).toFixed(1)}" y="${(H - pad + 16).toFixed(1)}" text-anchor="middle" font-size="11" fill="var(--color-text-sub)">${C.esc(p.x)}</text>`).join('')}
      ${points.map((p, i) => `<text x="${sx(i).toFixed(1)}" y="${(sy(p.y) - 8).toFixed(1)}" text-anchor="middle" font-size="11" font-weight="600" fill="var(--color-text)">${C.esc(p.y)}</text>`).join('')}
    </svg>`;
  }

  /* 水平柱状排行:rows=[{name, value}],带数值标签 */
  function barRank(rows, opts = {}) {
    const max = Math.max(...rows.map(r => r.value)) || 1;
    const color = opts.color || 'amber';
    return `<div class="flex" style="flex-direction:column;gap:12px">${rows.map((r, i) => `
      <div>
        <div class="flex justify-between text-sm mb-2">
          <span class="fw-600">${C.esc((i + 1) + '. ' + r.name)}</span>
          <span class="mono muted">${C.esc(r.value)}</span>
        </div>
        <div class="progress ${color === 'green' ? 'green' : ''}" style="height:9px"><span style="width:${(r.value / max * 100).toFixed(1)}%"></span></div>
      </div>`).join('')}</div>`;
  }

  /* 环形占比图:items=[{label, value, color}],中心显示总量 */
  function donut(items, opts = {}) {
    const total = items.reduce((s, x) => s + x.value, 0) || 1;
    const R = 60, C0 = 2 * Math.PI * R, cx = 80, cy = 80;
    let acc = 0;
    const arcs = items.map(it => {
      const frac = it.value / total;
      const seg = `<circle cx="${cx}" cy="${cy}" r="${R}" fill="none" stroke="var(--${it.color}-600)" stroke-width="18"
        stroke-dasharray="${(frac * C0).toFixed(2)} ${C0.toFixed(2)}" stroke-dashoffset="${(-acc * C0).toFixed(2)}"
        transform="rotate(-90 ${cx} ${cy})"/>`;
      acc += frac; return seg;
    }).join('');
    const legend = items.map(it => `
      <div class="flex items-center justify-between text-sm" style="gap:10px">
        <span class="flex items-center gap-2"><span class="dot" style="background:var(--${it.color}-600)"></span>${C.esc(it.label)}</span>
        <span class="mono muted">${C.esc(it.value)} · ${((it.value / total) * 100).toFixed(0)}%</span>
      </div>`).join('');
    return `<div class="flex items-center gap-4 wrap">
      <svg viewBox="0 0 160 160" width="160" height="160" role="img" aria-label="${C.esc(opts.label || '占比图')}">
        <circle cx="${cx}" cy="${cy}" r="${R}" fill="none" stroke="var(--color-border)" stroke-width="18"/>
        ${arcs}
        <text x="${cx}" y="${cy - 4}" text-anchor="middle" font-size="22" font-weight="700" fill="var(--color-text-strong)">${C.esc(opts.center || total)}</text>
        <text x="${cx}" y="${cy + 16}" text-anchor="middle" font-size="11" fill="var(--color-text-sub)">${C.esc(opts.centerLabel || '总计')}</text>
      </svg>
      <div style="flex:1;min-width:160px;display:flex;flex-direction:column;gap:8px">${legend}</div>
    </div>`;
  }

  /* 图表卡片外壳 */
  function chartCard(title, sub, inner, badge) {
    return `<div class="card">
      <div class="card-head"><div><div class="section-title">${C.esc(title)}</div>${sub ? `<div class="muted text-xs mt-2">${C.esc(sub)}</div>` : ''}</div>${badge || ''}</div>
      <div class="card-pad">${inner}</div>
    </div>`;
  }

  /* ============================================================
     1) 学校管理(租户列表)
     ============================================================ */
  function tenants() {
    const rows = m.tenants.map(t => `
      <tr>
        <td><a class="fw-600" style="cursor:pointer;color:var(--color-primary-text)" onclick="Chaimir.navigate('platform-admin/tenant-detail?id=${t.id}')">${C.esc(t.name)}</a></td>
        <td><div class="mono text-sm">${C.esc(t.code)}</div><div class="muted text-xs">租户 #${t.id}</div></td>
        <td>${C.statusDot(tenantDot(t.status), t.status)}</td>
        <td><div class="flex items-center gap-2"><span class="mono text-sm">${C.fmtDate(t.expire)}</span>${expireHint(t.expire)}</div></td>
        <td class="mono">${t.users.toLocaleString()}</td>
        <td class="muted text-sm">${C.fmtDate(t.create || '2024-09-01')}</td>
        <td class="row-actions">
          <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('platform-admin/tenant-detail?id=${t.id}')">详情</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paRenew(${t.id})">${C.icon('calendar-clock')} 续期</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.paToggleTenant(${t.id})">${t.status === '停用' ? '启用' : '停用'}</button>
        </td>
      </tr>`).join('');

    const total = m.tenants.length;
    const active = m.tenants.filter(t => t.status === '正常').length;
    const users = m.tenants.reduce((s, t) => s + t.users, 0);
    return `${C.head('学校管理', '租户', `<button class="btn btn-outline" onclick="Chaimir.navigate('platform-admin/applications')">${C.icon('clipboard-check')} 入驻审核</button>
        <button class="btn btn-primary" onclick="Chaimir.paCreateTenant()">${C.icon('plus')} 录入学校</button>`)}
      <div class="grid grid-3 mb-4">
        ${C.stat('building-2', total, '接入学校', 'amber')}
        ${C.stat('check-circle-2', active, '正常运营', 'green')}
        ${C.stat('users', users.toLocaleString(), '平台用户总数', 'blue')}
      </div>
      <div class="card mb-3"><div class="card-pad flex items-center gap-3 wrap" style="padding-bottom:0;border-bottom:none">
        <div class="input-icon" style="max-width:260px">${C.icon('search')}<input class="input" placeholder="搜索学校名 / 短码"></div>
        <select class="select" style="max-width:160px"><option>全部状态</option><option>正常</option><option>停用</option><option>临近到期</option></select>
        <div style="flex:1"></div>
        <span class="muted text-sm">${C.icon('info')} 停用或到期将导致该校全部用户无法登录</span>
      </div>
      <div class="table-wrap" style="border:none;border-radius:0">
        <table class="table"><thead><tr>
          <th>学校名称</th><th>租户 ID / 短码</th><th>状态</th><th>到期时间</th><th>用户数</th><th>创建时间</th><th></th>
        </tr></thead><tbody>${rows}</tbody></table>
      </div>
      <div class="card-pad" style="padding-top:0">${C.pagination(1, total)}</div>
      </div>`;
  }

  /* 录入学校(弹窗;原型模拟提交) */
  C.paCreateTenant = function () {
    C.modal({
      title: '录入学校', size: 'lg',
      body: `<div class="callout info mb-4">${C.icon('info')}<div>录入即新建租户并隔离数据(启用行级隔离);保存后可在详情页生成校管激活码完成开通。</div></div>
        <div class="grid grid-2">
          <div class="field"><label>学校名称 <span class="req">*</span></label><input class="input" placeholder="如:江南科技大学"></div>
          <div class="field"><label>租户短码 <span class="req">*</span></label><div class="input-icon">${C.icon('hash')}<input class="input mono" placeholder="小写字母/数字,如 jnu"></div><div class="help">用于登录选校与子域名,创建后不可更改</div></div>
          <div class="field"><label>办学类型</label><select class="select"><option>本科</option><option>高职</option><option>研究院所</option><option>企业培训</option></select></div>
          <div class="field"><label>授权到期日 <span class="req">*</span></label><input class="input" type="date" value="2027-08-31"></div>
          <div class="field"><label>联系人</label><input class="input" placeholder="教务处 / 张老师"></div>
          <div class="field"><label>联系方式</label><input class="input" placeholder="手机号 / 邮箱"></div>
        </div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','学校已录入','已新建租户并隔离数据,可在详情页生成校管激活码')">保存并新建租户</button>`,
    });
  };

  /* 续期(强二次确认 —— 影响全校可用性) */
  C.paRenew = function (id) {
    const t = m.tenants.find(x => x.id == id); if (!t) return;
    C.modal({
      title: '续期 / 调整到期', size: '',
      body: `<div class="dl mb-4"><dt>学校</dt><dd class="fw-600">${C.esc(t.name)}</dd>
          <dt>当前到期</dt><dd class="mono">${C.fmtDate(t.expire)}</dd></div>
        <div class="field"><label>新到期日 <span class="req">*</span></label><input class="input" type="date" id="pa-renew-date" value="${C.esc(t.expire)}"></div>
        <div class="callout warn">${C.icon('alert-triangle')}<div>到期日早于今天将立即停用该校,全体师生<b>无法登录</b>。请谨慎设置。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="Chaimir.paRenewConfirm(${id})">确认续期</button>`,
    });
  };
  C.paRenewConfirm = async function (id) {
    const t = m.tenants.find(x => x.id == id); if (!t) return;
    const v = (document.getElementById('pa-renew-date') || {}).value || t.expire;
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    if (await C.confirm({
      title: '确认调整到期时间', danger: true, confirmText: '确认变更',
      message: `将把「${t.name}」到期时间改为 ${C.fmtDate(v)}。此操作直接影响全校师生能否登录,请二次确认。`,
    })) {
      C.toast('success', '到期时间已更新', `「${t.name}」新到期 ${C.fmtDate(v)},已写入审计`);
    }
  };

  /* 停用 / 启用(停用为强二次确认) */
  C.paToggleTenant = async function (id) {
    const t = m.tenants.find(x => x.id == id); if (!t) return;
    const disabling = t.status !== '停用';
    if (disabling) {
      if (await C.confirm({
        title: '停用学校', danger: true, confirmText: '确认停用',
        message: `停用「${t.name}」后,该校 ${t.users.toLocaleString()} 名用户将立即无法登录、所有进行中实验与竞赛被冻结。确认停用?`,
      })) C.toast('success', '学校已停用', `「${t.name}」已停用,操作已记入审计`);
    } else {
      if (await C.confirm({
        title: '启用学校', confirmText: '确认启用',
        message: `将恢复「${t.name}」的登录与全部服务。确认启用?`,
      })) C.toast('success', '学校已启用', `「${t.name}」已恢复服务`);
    }
  };

  /* ============================================================
     2) 租户详情(子页)
     ============================================================ */
  function tenantDetail(ctx) {
    const t = m.tenants.find(x => x.id == ctx.query.id) || m.tenants[0];
    return `${C.crumb([{ label: '学校管理', to: 'platform-admin/tenants' }, { label: t.name }])}
      <div class="content-head">
        <div><div class="page-sub">${C.icon('hash')} 短码 ${C.esc(t.code)} · 租户 #${t.id}</div><h1 class="page-title">${C.esc(t.name)}</h1></div>
        <div class="content-actions">
          <button class="btn btn-outline" onclick="Chaimir.paRenew(${t.id})">${C.icon('calendar-clock')} 续期</button>
          <button class="btn ${t.status === '停用' ? 'btn-primary' : 'btn-danger'}" onclick="Chaimir.paToggleTenant(${t.id})">${t.status === '停用' ? '启用学校' : '停用学校'}</button>
        </div>
      </div>
      <div class="grid grid-4 mb-4">
        ${C.stat('activity', t.status, '当前状态', tenantBadge(t.status) === 'green' ? 'green' : tenantBadge(t.status) === 'red' ? 'red' : 'amber')}
        ${C.stat('users', t.users.toLocaleString(), '用户总数', 'blue')}
        ${C.stat('calendar', C.fmtDate(t.expire), '授权到期', 'amber')}
        ${C.stat('hard-drive', '38.6 GB', '存储用量', 'purple')}
      </div>
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">基本信息</div></div><div class="card-pad">
          <dl class="dl">
            <dt>学校名称</dt><dd class="fw-600">${C.esc(t.name)}</dd>
            <dt>租户短码</dt><dd class="mono">${C.esc(t.code)}</dd>
            <dt>办学类型</dt><dd>本科</dd>
            <dt>运营状态</dt><dd>${C.statusDot(tenantDot(t.status), t.status)}</dd>
            <dt>授权到期</dt><dd class="mono">${C.fmtDate(t.expire)} ${expireHint(t.expire)}</dd>
            <dt>创建时间</dt><dd class="mono">${C.fmtDate(t.create || '2024-09-01')}</dd>
            <dt>主联系人</dt><dd>教务处 / 张老师 · 139****2200</dd>
          </dl>
        </div></div>
        <div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">改状态 / 到期</div></div><div class="card-pad">
            <div class="field"><label>运营状态</label>
              <select class="select"><option ${t.status === '正常' ? 'selected' : ''}>正常</option><option ${t.status === '停用' ? 'selected' : ''}>停用</option></select>
              <div class="help">停用立即阻断该校全部登录</div></div>
            <div class="field"><label>授权到期日</label><input class="input" type="date" value="${C.esc(t.expire)}"></div>
            <div class="callout danger mb-3">${C.icon('shield-alert')}<div>状态与到期变更直接影响全校可用性,保存时需二次确认。</div></div>
            <button class="btn btn-primary btn-block" onclick="Chaimir.paSaveTenant('${C.esc(t.name)}')">保存变更</button>
          </div></div>
          <div class="card"><div class="card-head"><div class="section-title">配额与资源</div><button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('platform-admin/quota')">前往配额管理</button></div><div class="card-pad">
            <dl class="dl"><dt>最大并发沙箱</dt><dd class="mono">40</dd><dt>当前用量</dt><dd class="mono">12 / 40</dd><dt>快照保留</dt><dd class="mono">7 天</dd></dl>
          </div></div>
        </div>
      </div>`;
  }
  C.paSaveTenant = async function (name) {
    if (await C.confirm({
      title: '确认保存变更', danger: true, confirmText: '确认保存',
      message: `「${name}」的状态/到期变更将影响全校登录与服务。确认保存?`,
    })) C.toast('success', '变更已保存', '已生效并写入平台审计');
  };

  /* ============================================================
     3) 入驻审核(列表,按状态 Tab)
     ============================================================ */
  function applications(ctx) {
    const tab = ctx.query.tab || '待审';
    /* 原型补充几条非待审样例,体现 Tab 切换 */
    const extra = [
      { id: 11, school: '示例大学', type: '本科', contact: '教务处 / 李老师', phone: '138****6677', time: '2026-05-20', status: '通过' },
      { id: 12, school: '滨海理工大学', type: '本科', contact: '信息中心 / 赵老师', phone: '137****2299', time: '2026-05-12', status: '通过' },
      { id: 13, school: '测试培训机构', type: '企业培训', contact: '运营 / 孙经理', phone: '135****7788', time: '2026-05-08', status: '驳回' },
    ];
    const all = m.applications.concat(extra);
    const counts = { '待审': all.filter(a => a.status === '待审').length, '通过': all.filter(a => a.status === '通过').length, '驳回': all.filter(a => a.status === '驳回').length };
    const list = all.filter(a => a.status === tab);
    const tabs = [['待审', '待审核'], ['通过', '已通过'], ['驳回', '已驳回']];

    const body = list.length ? `<div class="table-wrap"><table class="table"><thead><tr>
        <th>学校名称</th><th>办学类型</th><th>联系人</th><th>联系方式</th><th>提交时间</th><th>状态</th><th></th>
      </tr></thead><tbody>${list.map(a => `
        <tr>
          <td class="fw-600">${C.esc(a.school)}</td>
          <td>${C.badge(a.type, 'gray')}</td>
          <td>${C.esc(a.contact)}</td>
          <td class="mono text-sm">${C.esc(a.phone)}</td>
          <td class="mono text-sm">${C.esc(a.time)}</td>
          <td><span class="badge badge-${appBadge(a.status)}">${a.status}</span></td>
          <td class="row-actions"><button class="btn ${a.status === '待审' ? 'btn-primary' : 'btn-outline'} btn-sm" onclick="Chaimir.navigate('platform-admin/application-review?id=${a.id}')">${a.status === '待审' ? '审核' : '查看'}</button></td>
        </tr>`).join('')}</tbody></table></div>`
      : C.empty({ icon: 'clipboard-check', title: '该状态暂无申请', desc: '切换上方标签查看其他状态的入驻申请。' });

    return `${C.head('入驻审核', '租户', `<button class="btn btn-outline" onclick="Chaimir.navigate('platform-admin/tenants')">${C.icon('building-2')} 学校管理</button>`)}
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${k === tab ? 'active' : ''}" onclick="Chaimir.navigate('platform-admin/applications?tab=${k}')">${l} <span class="count" style="margin-left:6px">${counts[k]}</span></a>`).join('')}</div>
      ${body}`;
  }

  /* ============================================================
     4) 申请审核详情(子页)
     ============================================================ */
  function applicationReview(ctx) {
    const extra = { 11: { school: '示例大学', type: '本科', contact: '教务处 / 李老师', phone: '138****6677', time: '2026-05-20', status: '通过' },
      13: { school: '测试培训机构', type: '企业培训', contact: '运营 / 孙经理', phone: '135****7788', time: '2026-05-08', status: '驳回' } };
    const a = m.applications.find(x => x.id == ctx.query.id) || extra[ctx.query.id] || m.applications[0];
    const pending = a.status === '待审';
    return `${C.crumb([{ label: '入驻审核', to: 'platform-admin/applications' }, { label: a.school }])}
      <div class="content-head">
        <div><div class="page-sub">入驻申请 · 提交于 ${C.esc(a.time)}</div><h1 class="page-title">${C.esc(a.school)}</h1></div>
        <div class="content-actions"><span class="badge badge-${appBadge(a.status)}" style="align-self:center">${a.status}</span></div>
      </div>
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">申请信息</div></div><div class="card-pad">
          <dl class="dl">
            <dt>学校名称</dt><dd class="fw-600">${C.esc(a.school)}</dd>
            <dt>办学类型</dt><dd>${C.esc(a.type)}</dd>
            <dt>所在地区</dt><dd>华东 · 示例市</dd>
            <dt>联系人</dt><dd>${C.esc(a.contact)}</dd>
            <dt>联系方式</dt><dd class="mono">${C.esc(a.phone)}</dd>
            <dt>预计规模</dt><dd>师生约 3,000 人</dd>
            <dt>用途说明</dt><dd>面向区块链方向开设教学、实验与竞赛,需独立租户与判题沙箱。</dd>
            <dt>资质材料</dt><dd><a style="cursor:pointer;color:var(--color-primary-text)" onclick="Chaimir.toast('info','查看材料','原型演示:此处打开办学许可扫描件')">${C.icon('paperclip')} 办学许可证.pdf</a></dd>
          </dl>
        </div></div>
        <div class="card"><div class="card-head"><div class="section-title">审核操作</div></div><div class="card-pad">
          ${pending ? `
            <div class="callout info mb-4">${C.icon('info')}<div>通过将自动:① 新建租户并隔离数据;② 分配短码;③ 生成校管激活码(用于首位学校管理员激活)。</div></div>
            <div class="field"><label>分配短码 <span class="req">*</span></label><div class="input-icon">${C.icon('hash')}<input class="input mono" id="pa-rev-code" placeholder="小写字母/数字,如 jnu" value="jnu"></div></div>
            <div class="field"><label>授权到期日 <span class="req">*</span></label><input class="input" type="date" value="2027-08-31"></div>
            <div class="flex gap-3 mt-4">
              <button class="btn btn-primary" style="flex:1" onclick="Chaimir.paApprove('${C.esc(a.school)}')">${C.icon('check')} 通过并开通</button>
              <button class="btn btn-danger" style="flex:1" onclick="Chaimir.paReject('${C.esc(a.school)}')">${C.icon('x')} 驳回</button>
            </div>`
          : a.status === '通过' ? `
            <div class="callout success mb-3">${C.icon('check-circle-2')}<div>申请已通过,租户已开通。</div></div>
            <dl class="dl"><dt>分配短码</dt><dd class="mono">jnu</dd><dt>审核人</dt><dd>平台管理员</dd><dt>审核时间</dt><dd class="mono">${C.esc(a.time)} 16:40</dd></dl>
            <button class="btn btn-outline btn-block mt-4" onclick="Chaimir.navigate('platform-admin/tenants')">前往学校管理</button>`
          : `
            <div class="callout danger mb-3">${C.icon('x-circle')}<div>申请已驳回。</div></div>
            <dl class="dl"><dt>驳回原因</dt><dd>提交的办学资质材料不完整,缺少有效办学许可证扫描件。</dd><dt>审核人</dt><dd>平台管理员</dd><dt>审核时间</dt><dd class="mono">${C.esc(a.time)} 09:12</dd></dl>`}
        </div></div>
      </div>`;
  }

  /* 通过:展示生成的激活码 / 短码 */
  C.paApprove = function (school) {
    const code = (document.getElementById('pa-rev-code') || {}).value || 'jnu';
    const act = 'ACT-' + code.toUpperCase() + '-' + Math.random().toString(36).slice(2, 8).toUpperCase();
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    C.modal({
      title: '已通过并开通租户', size: '', persistent: true,
      body: `<div class="callout success mb-4">${C.icon('check-circle-2')}<div>「${C.esc(school)}」已开通。请将下方激活码与登录短码交付学校管理员,用于激活首个管理员账号。</div></div>
        <div class="field"><label>租户短码</label><div class="flex gap-2"><input class="input mono" value="${C.esc(code)}" readonly><button class="btn btn-outline" onclick="Chaimir.toast('success','已复制','短码已复制到剪贴板')">${C.icon('copy')}</button></div></div>
        <div class="field" style="margin-bottom:0"><label>校管激活码</label><div class="flex gap-2"><input class="input mono" value="${act}" readonly><button class="btn btn-outline" onclick="Chaimir.toast('success','已复制','激活码已复制到剪贴板')">${C.icon('copy')}</button></div><div class="help">激活码 7 天内有效,仅可使用一次</div></div>`,
      foot: `<button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.navigate('platform-admin/tenants')">完成</button>`,
    });
  };

  /* 驳回:填原因 */
  C.paReject = function (school) {
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    C.modal({
      title: '驳回入驻申请', size: '',
      body: `<p class="text-sm mb-3">驳回「${C.esc(school)}」的申请,原因将通知申请联系人。</p>
        <div class="field"><label>驳回原因 <span class="req">*</span></label><textarea class="textarea" id="pa-reject-reason" placeholder="如:办学资质材料不完整,请补充有效办学许可证后重新提交"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-danger" onclick="Chaimir.paRejectConfirm()">确认驳回</button>`,
    });
  };
  C.paRejectConfirm = function () {
    const r = (document.getElementById('pa-reject-reason') || {}).value || '';
    if (!r.trim()) { C.toast('error', '请填写驳回原因', '原因将通知申请方,不能为空'); return; }
    document.querySelector('.overlay') && document.querySelector('.overlay').remove();
    C.toast('success', '已驳回申请', '驳回原因已通知申请联系人');
    setTimeout(() => C.navigate('platform-admin/applications?tab=驳回'), 600);
  };

  /* ============================================================
     5) 平台看板(跨校聚合大盘)
     ============================================================ */
  function dashboard() {
    const growth = [{ x: '1月', y: 18 }, { x: '2月', y: 21 }, { x: '3月', y: 24 }, { x: '4月', y: 28 }, { x: '5月', y: 31 }, { x: '6月', y: 34 }];
    const users = [{ x: '1月', y: 2.1 }, { x: '2月', y: 3.4 }, { x: '3月', y: 4.8 }, { x: '4月', y: 6.2 }, { x: '5月', y: 7.5 }, { x: '6月', y: 8.6 }];
    const topSchools = [{ name: '示例大学', value: 3280 }, { name: '滨海理工大学', value: 1560 }, { name: '北辰大学', value: 980 }, { name: '江南科技大学', value: 760 }, { name: '海川职业技术学院', value: 540 }];
    const resource = [{ label: 'EVM 沙箱', value: 46, color: 'amber' }, { label: '仿真渲染', value: 28, color: 'blue' }, { label: 'Fabric 节点', value: 16, color: 'purple' }, { label: '空闲', value: 10, color: 'green' }];

    return `${C.head('平台看板', '概览', `<span class="badge badge-green" style="align-self:center">${C.icon('radio')} 实时聚合</span>
        <button class="btn btn-outline" onclick="Chaimir.navigate('platform-admin/statistics')">${C.icon('line-chart')} 平台统计</button>`)}
      <div class="callout info mb-4">${C.icon('zap')}<div>大盘数据为<b>跨校实时聚合</b>;高基数维度走预聚合<b>快照加速</b>,默认 5 分钟刷新一次,右上角可手动刷新。</div></div>
      <div class="grid grid-4 mb-4">
        ${C.stat('building-2', '34', '接入学校', 'amber', { dir: 'up', text: '本月 +3' })}
        ${C.stat('users', '8.6 万', '平台用户总数', 'blue', { dir: 'up', text: '本月 +1.1 万' })}
        ${C.stat('activity', '6,240', '今日活跃用户', 'green', { dir: 'up', text: '较昨日 +8%' })}
        ${C.stat('cpu', '64%', '沙箱资源用量', 'purple', { dir: 'up', text: '峰值 82%' })}
      </div>
      <div class="grid grid-2 mb-4">
        ${chartCard('学校增长', '近 6 个月接入学校累计数', lineChart(growth, { color: 'amber', label: '学校增长趋势' }), C.badge('快照', 'gray'))}
        ${chartCard('用户增长', '近 6 个月平台用户(万)', lineChart(users, { color: 'blue', label: '用户增长趋势' }), C.badge('快照', 'gray'))}
      </div>
      <div class="grid grid-2">
        ${chartCard('跨校活跃 Top 5', '按校在线/活跃用户数排行', barRank(topSchools, { color: 'amber' }), C.badge('实时', 'green'))}
        ${chartCard('沙箱资源占比', '当前算力分配(按用途)', donut(resource, { center: '64%', centerLabel: '已占用', label: '资源占比' }), C.badge('实时', 'green'))}
      </div>`;
  }

  /* ============================================================
     6) 平台统计(趋势 + 分布 + 范围筛选 + 日/周/月)
     ============================================================ */
  function statistics(ctx) {
    const gran = ctx.query.g || 'month';
    const grans = [['day', '日'], ['week', '周'], ['month', '月']];
    const series = {
      day: [{ x: '周一', y: 5.2 }, { x: '周二', y: 5.6 }, { x: '周三', y: 6.1 }, { x: '周四', y: 5.9 }, { x: '周五', y: 6.4 }, { x: '周六', y: 4.1 }, { x: '周日', y: 3.8 }],
      week: [{ x: 'W18', y: 5.4 }, { x: 'W19', y: 5.9 }, { x: 'W20', y: 6.3 }, { x: 'W21', y: 6.8 }, { x: 'W22', y: 7.1 }, { x: 'W23', y: 7.6 }],
      month: [{ x: '1月', y: 4.6 }, { x: '2月', y: 5.2 }, { x: '3月', y: 6.0 }, { x: '4月', y: 6.9 }, { x: '5月', y: 7.8 }, { x: '6月', y: 8.6 }],
    }[gran];
    const dist = [{ label: '本科院校', value: 21, color: 'amber' }, { label: '高职院校', value: 8, color: 'blue' }, { label: '研究院所', value: 3, color: 'purple' }, { label: '企业培训', value: 2, color: 'green' }];
    const usage = [{ name: '代码实验', value: 4820 }, { name: '图形仿真', value: 3160 }, { name: '竞赛对局', value: 1740 }, { name: '理论判题', value: 980 }];

    return `${C.head('平台统计', '概览', `<button class="btn btn-outline" onclick="Chaimir.toast('success','已导出','统计报表(CSV)已生成,原型演示')">${C.icon('download')} 导出报表</button>`)}
      <div class="card mb-4"><div class="card-pad flex items-center gap-3 wrap" style="padding:14px 18px">
        ${C.icon('filter')}<span class="fw-600 text-sm">时间范围</span>
        <input class="input" type="date" value="2026-01-01" style="max-width:160px"><span class="muted">至</span><input class="input" type="date" value="2026-06-07" style="max-width:160px">
        <div style="flex:1"></div>
        <div class="flex gap-1">${grans.map(([k, l]) => `<button class="btn ${k === gran ? 'btn-primary' : 'btn-outline'} btn-sm" onclick="Chaimir.navigate('platform-admin/statistics?g=${k}')">${l}</button>`).join('')}</div>
      </div></div>
      <div class="grid grid-4 mb-4">
        ${C.stat('trending-up', '+18%', '用户环比增长', 'green')}
        ${C.stat('flask-conical', '10.7 万', '累计实验次数', 'amber')}
        ${C.stat('trophy', '1,420', '累计竞赛场次', 'purple')}
        ${C.stat('clock', '32 分', '人均日活时长', 'blue')}
      </div>
      <div class="card mb-4"><div class="card-head"><div class="section-title">活跃用户趋势(${grans.find(x => x[0] === gran)[1]}粒度)</div><div class="muted text-xs">单位:万人</div></div>
        <div class="card-pad">${lineChart(series, { color: 'amber', label: '活跃用户趋势' })}</div></div>
      <div class="grid grid-2">
        ${chartCard('学校类型分布', '按办学类型统计接入学校', donut(dist, { center: '34', centerLabel: '学校数', label: '学校类型分布' }))}
        ${chartCard('使用功能分布', '近 30 天各能力使用次数', barRank(usage, { color: 'blue' }))}
      </div>`;
  }

  /* ---------- 注册 ---------- */
  C.registerPages({
    'platform-admin/tenants': tenants,
    'platform-admin/tenant-detail': tenantDetail,
    'platform-admin/applications': applications,
    'platform-admin/application-review': applicationReview,
    'platform-admin/dashboard': dashboard,
    'platform-admin/statistics': statistics,
  });
})();
