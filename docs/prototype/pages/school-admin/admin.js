/* ============================================================
   pages/school-admin/admin.js — 学校管理员·概览与用户组织域
   ------------------------------------------------------------
   覆盖:数据看板(实时聚合 KPI + 折线/柱/环形图)、运营统计
        (趋势分析 + 时间范围 + 日/周/月切换)、用户管理(多维
        筛选 + 行内/批量账号操作)、账号导入向导(2 步预览校验,
        服务端暂存只持 preview_id)、账号新增/编辑、组织架构
        (院系→专业→班级三级树 CRUD)、导入记录。对应 M1 身份
        与组织、M11 聚合(学校侧)。
   说明:遵循 courses.js 范式 —— registerPages({route: ctx => html})
        + 复用 C.* 工具 + 子页登记 C.parentRoute 高亮侧栏。
        看板/统计图表用内联 SVG(折线/柱/环形)+ 图例 + 数值,
        颜色非唯一信息(状态均配文字 + 圆点)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项(子页无独立菜单,沿用父级高亮) */
  Object.assign(C.parentRoute, {
    'school-admin/account-import': 'school-admin/accounts',
    'school-admin/account-edit': 'school-admin/accounts',
  });

  /* ============================================================
     图表基元:内联 SVG 折线 / 柱 / 环形(无外部库)
     设计:统一在白底卡片内绘制;坐标系内置内边距;图例独立渲染,
          颜色仅作区分,数值始终以文字形式给出(无障碍)。
     ============================================================ */

  /* 折线图:series=[{name,color,data:[..]}],labels=[..] */
  function lineChart(labels, series, opt = {}) {
    const W = 640, H = 220, padL = 36, padR = 14, padT = 14, padB = 26;
    const iw = W - padL - padR, ih = H - padT - padB;
    const all = series.flatMap(s => s.data);
    const max = opt.max || Math.max(1, ...all) * 1.15;
    const xOf = i => padL + (labels.length <= 1 ? iw / 2 : (iw * i) / (labels.length - 1));
    const yOf = v => padT + ih - (ih * v) / max;
    /* 横向网格线 + Y 轴刻度(4 等分) */
    let grid = '';
    for (let g = 0; g <= 4; g++) {
      const v = (max / 4) * g, y = yOf(v);
      grid += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="var(--color-border)" stroke-width="1"/>
        <text x="${padL - 6}" y="${y + 3}" text-anchor="end" font-size="10" fill="var(--color-text-faint)">${Math.round(v)}</text>`;
    }
    /* X 轴标签 */
    let xl = labels.map((l, i) => `<text x="${xOf(i)}" y="${H - 8}" text-anchor="middle" font-size="10" fill="var(--color-text-sub)">${C.esc(l)}</text>`).join('');
    /* 折线 + 数据点 */
    let paths = series.map(s => {
      const pts = s.data.map((v, i) => `${xOf(i)},${yOf(v)}`).join(' ');
      const dots = s.data.map((v, i) => `<circle cx="${xOf(i)}" cy="${yOf(v)}" r="3" fill="${s.color}"><title>${C.esc(s.name)} · ${C.esc(labels[i])}:${v}</title></circle>`).join('');
      return `<polyline points="${pts}" fill="none" stroke="${s.color}" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round"/>${dots}`;
    }).join('');
    const legend = series.map(s => `<span class="flex items-center gap-1 text-xs muted"><span style="width:10px;height:10px;border-radius:3px;background:${s.color};display:inline-block"></span>${C.esc(s.name)}</span>`).join('');
    return `<div><svg viewBox="0 0 ${W} ${H}" width="100%" preserveAspectRatio="xMidYMid meet" role="img" aria-label="${C.esc(opt.title || '折线趋势图')}">${grid}${xl}${paths}</svg>
      <div class="flex gap-4 wrap mt-2" style="justify-content:center">${legend}</div></div>`;
  }

  /* 柱状图(支持分组对比):series=[{name,color,data}],labels */
  function barChart(labels, series, opt = {}) {
    const W = 640, H = 220, padL = 36, padR = 14, padT = 14, padB = 26;
    const iw = W - padL - padR, ih = H - padT - padB;
    const max = Math.max(1, ...series.flatMap(s => s.data)) * 1.15;
    const groups = labels.length, n = series.length;
    const gw = iw / groups, bw = Math.min(26, (gw * 0.62) / n);
    const yOf = v => padT + ih - (ih * v) / max;
    let grid = '';
    for (let g = 0; g <= 4; g++) {
      const v = (max / 4) * g, y = yOf(v);
      grid += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="var(--color-border)" stroke-width="1"/>
        <text x="${padL - 6}" y="${y + 3}" text-anchor="end" font-size="10" fill="var(--color-text-faint)">${Math.round(v)}</text>`;
    }
    let bars = '';
    labels.forEach((l, i) => {
      const cx = padL + gw * i + gw / 2;
      const totalW = bw * n + (n - 1) * 4;
      series.forEach((s, j) => {
        const x = cx - totalW / 2 + j * (bw + 4);
        const y = yOf(s.data[i]), h = padT + ih - y;
        bars += `<rect x="${x}" y="${y}" width="${bw}" height="${h}" rx="3" fill="${s.color}"><title>${C.esc(s.name)} · ${C.esc(l)}:${s.data[i]}</title></rect>`;
      });
      bars += `<text x="${cx}" y="${H - 8}" text-anchor="middle" font-size="10" fill="var(--color-text-sub)">${C.esc(l)}</text>`;
    });
    const legend = series.map(s => `<span class="flex items-center gap-1 text-xs muted"><span style="width:10px;height:10px;border-radius:3px;background:${s.color};display:inline-block"></span>${C.esc(s.name)}</span>`).join('');
    return `<div><svg viewBox="0 0 ${W} ${H}" width="100%" preserveAspectRatio="xMidYMid meet" role="img" aria-label="${C.esc(opt.title || '柱状对比图')}">${grid}${bars}</svg>
      <div class="flex gap-4 wrap mt-2" style="justify-content:center">${legend}</div></div>`;
  }

  /* 环形图(用量 vs 配额):used/total + 颜色阈值 */
  function donutChart(used, total, label, unit) {
    const pct = Math.min(100, Math.round((used / total) * 100));
    const R = 54, C0 = 2 * Math.PI * R, off = C0 * (1 - pct / 100);
    /* 阈值上色:<70 绿,70~90 琥珀,>90 红(语义令牌) */
    const col = pct >= 90 ? 'var(--red-600)' : pct >= 70 ? 'var(--amber-500)' : 'var(--green-600)';
    return `<div class="flex items-center gap-4 wrap" style="justify-content:center">
      <svg viewBox="0 0 140 140" width="140" height="140" role="img" aria-label="${C.esc(label)} 使用率 ${pct}%">
        <circle cx="70" cy="70" r="${R}" fill="none" stroke="var(--slate-200)" stroke-width="14"/>
        <circle cx="70" cy="70" r="${R}" fill="none" stroke="${col}" stroke-width="14" stroke-linecap="round"
          stroke-dasharray="${C0}" stroke-dashoffset="${off}" transform="rotate(-90 70 70)"/>
        <text x="70" y="66" text-anchor="middle" font-size="26" font-weight="700" fill="var(--color-text-strong)">${pct}%</text>
        <text x="70" y="86" text-anchor="middle" font-size="11" fill="var(--color-text-sub)">已使用</text>
      </svg>
      <div class="text-sm">
        <div class="fw-700" style="font-size:var(--text-md)">${C.esc(label)}</div>
        <div class="muted mt-2">${C.statusDot(pct >= 90 ? 'red' : pct >= 70 ? 'amber' : 'green', `${used} / ${total} ${unit || ''}`)}</div>
        <div class="muted text-xs mt-2">剩余 ${total - used} ${unit || ''}</div>
      </div></div>`;
  }

  /* 卡片外壳:统一标题栏 + 实时聚合标记(可选) */
  function chartCard(title, live, body, foot) {
    return `<div class="card"><div class="card-head"><div class="section-title">${title}</div>
      ${live ? `<span class="badge badge-green">${C.icon('radio')} 实时聚合</span>` : (foot || '')}</div>
      <div class="card-pad">${body}</div></div>`;
  }

  /* ============================================================
     ① 数据看板(school-admin/dashboard)— 本校运营总览
     ============================================================ */
  function dashboard() {
    /* KPI 行 */
    const kpis = [
      C.stat('users', '3,280', '师生总数', 'amber', { dir: 'up', text: '本月 +126' }),
      C.stat('book-open', '48', '在授课程', 'blue', { dir: 'up', text: '本周 +3' }),
      C.stat('trophy', '12', '进行中竞赛', 'purple', { dir: 'up', text: '+2' }),
      C.stat('flask-conical', '186', '今日实验活跃', 'green', { dir: 'down', text: '较昨日 -8' }),
      C.stat('gauge', '64%', '资源用量', 'amber'),
    ].join('');

    const months = ['1月', '2月', '3月', '4月', '5月', '6月'];
    const teachTrend = lineChart(months, [
      { name: '活跃学生', color: 'var(--amber-500)', data: [820, 960, 1180, 1240, 1390, 1460] },
      { name: '提交作业', color: 'var(--blue-600)', data: [410, 520, 690, 740, 880, 930] },
    ], { title: '教学活跃趋势' });

    const contestTrend = barChart(['3月', '4月', '5月', '6月'], [
      { name: '竞赛场次', color: 'var(--purple-600)', data: [2, 4, 5, 6] },
      { name: '参赛人次', color: 'var(--amber-500)', data: [120, 260, 410, 520] },
    ], { title: '竞赛趋势' });

    const expTrend = lineChart(['周一', '周二', '周三', '周四', '周五', '周六', '周日'], [
      { name: '代码实验', color: 'var(--green-600)', data: [86, 102, 95, 130, 168, 60, 42] },
      { name: '仿真实验', color: 'var(--teal-700)', data: [40, 55, 48, 62, 80, 30, 22] },
    ], { title: '实验活跃' });

    const resDonut = donutChart(64, 100, '沙箱算力配额', '核·时');

    return `${C.head('数据看板', '概览', `<button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/statistics')">${C.icon('line-chart')} 运营统计</button>`)}
      <div class="callout info mb-4">${C.icon('info')}<div>以下指标为本校范围内实时聚合(M11 聚合层只读跨模块数据,不跨写);最近一次聚合 1 分钟前。</div></div>
      <div class="grid grid-4 mb-4" style="grid-template-columns:repeat(5,1fr)">${kpis}</div>
      <div class="grid grid-2 mb-4">
        ${chartCard('教学活跃趋势(近 6 个月)', true, teachTrend)}
        ${chartCard('竞赛趋势(近 4 个月)', true, contestTrend)}
      </div>
      <div class="grid grid-2">
        ${chartCard('实验活跃(本周)', true, expTrend)}
        ${chartCard('资源用量 vs 配额', true, resDonut, `<button class="btn btn-ghost btn-sm" onclick="Chaimir.demo()">查看明细</button>`)}
      </div>`;
  }

  /* ============================================================
     ② 运营统计(school-admin/statistics)— 趋势分析
     ============================================================ */
  C.saStat = { dim: 'week', metric: 'teach' };
  C.saStatSet = (k, v) => { C.saStat[k] = v; C.rerender(); };

  function statistics() {
    const s = C.saStat;
    const grain = [['day', '按日'], ['week', '按周'], ['month', '按月']];
    const metricTabs = [['teach', '教学'], ['contest', '竞赛'], ['exp', '实验']];
    /* 按粒度切换横轴标签与数据,体现"日/周/月切换" */
    const labelMap = {
      day: ['6-01', '6-02', '6-03', '6-04', '6-05', '6-06', '6-07'],
      week: ['第1周', '第2周', '第3周', '第4周', '第5周', '第6周'],
      month: ['1月', '2月', '3月', '4月', '5月', '6月'],
    };
    const labels = labelMap[s.dim];
    const seed = { teach: 1, contest: 0.4, exp: 0.8 }[s.metric];
    const mk = (base, color, name) => ({ name, color, data: labels.map((_, i) => Math.round(base * seed * (0.8 + 0.12 * i + (i % 2 ? 0.1 : 0)))) });
    const line = lineChart(labels, [
      mk(140, 'var(--amber-500)', '本期'),
      mk(110, 'var(--slate-400)', '上一周期'),
    ], { title: '趋势折线' });
    const bar = barChart(labels.slice(-5), [
      mk(120, 'var(--blue-600)', '完成量'),
      mk(80, 'var(--purple-600)', '新增量'),
    ], { title: '对比柱状' });

    return `${C.head('运营统计', '概览', `<button class="btn btn-outline" onclick="Chaimir.demo()">${C.icon('download')} 导出报表</button>`)}
      <div class="card card-pad mb-4">
        <div class="flex gap-3 wrap items-center justify-between">
          <div class="flex gap-3 wrap items-center">
            <div class="flex gap-2 items-center"><span class="muted text-sm">时间范围</span>
              <input type="date" class="input" style="width:150px" value="2026-05-01" onchange="Chaimir.demo()">
              <span class="muted">至</span>
              <input type="date" class="input" style="width:150px" value="2026-06-07" onchange="Chaimir.demo()"></div>
          </div>
          <div class="flex gap-1" style="background:var(--color-surface-sunken);padding:3px;border-radius:var(--radius-sm)">
            ${grain.map(([k, l]) => `<button class="btn btn-sm ${s.dim === k ? 'btn-primary' : 'btn-ghost'}" onclick="Chaimir.saStatSet('dim','${k}')">${l}</button>`).join('')}
          </div>
        </div>
      </div>
      <div class="tabs">${metricTabs.map(([k, l]) => `<a class="tab ${s.metric === k ? 'active' : ''}" onclick="Chaimir.saStatSet('metric','${k}')">${l}分析</a>`).join('')}</div>
      <div class="grid grid-4 mb-4">
        ${C.stat('trending-up', s.metric === 'teach' ? '1,460' : s.metric === 'contest' ? '520' : '1,180', '本期总量', 'amber', { dir: 'up', text: '环比 +12%' })}
        ${C.stat('activity', s.metric === 'teach' ? '243' : s.metric === 'contest' ? '86' : '186', '日均活跃', 'green')}
        ${C.stat('check-circle-2', s.metric === 'teach' ? '92%' : s.metric === 'contest' ? '78%' : '84%', '完成率', 'blue')}
        ${C.stat('users', s.metric === 'teach' ? '2,140' : s.metric === 'contest' ? '640' : '1,520', '覆盖人数', 'purple')}
      </div>
      <div class="grid grid-2">
        ${chartCard('趋势分析(' + (grain.find(g => g[0] === s.dim)[1]) + ')', false, line)}
        ${chartCard('完成量 / 新增量 对比', false, bar)}
      </div>`;
  }

  /* ============================================================
     ③ 用户管理(school-admin/accounts)
     ============================================================ */
  C.saAcc = { role: '全部', cls: '全部', status: '全部', kw: '', page: 1, sel: {} };
  C.saAccSet = (k, v) => { C.saAcc[k] = v; C.saAcc.page = 1; C.rerender(); };

  /* 手机号脱敏(本校假数据补齐手机号字段) */
  function maskPhone(no, role) {
    const tail = String(no).slice(-4).padStart(4, '0');
    const head = role === '教师' ? '139' : '138';
    return `${head}****${tail}`;
  }

  /* 扩展演示数据集(在 mock.accounts 基础上补充字段:班级/最后登录) */
  function accountRows() {
    return m.accounts.map(a => ({
      ...a,
      phone: maskPhone(a.no, a.role),
      belong: a.dept,
      cls: a.role === '学生' ? a.dept : '—',
    }));
  }

  C.saAccToggleAll = (cb) => {
    accountRows().forEach(a => { C.saAcc.sel[a.id] = cb.checked; });
    C.rerender();
  };
  C.saAccToggle = (id, cb) => { C.saAcc.sel[id] = cb.checked; C.rerender(); };

  /* 行内操作:统一二次确认 + toast 反馈 */
  C.saAccAct = async function (id, act) {
    const a = accountRows().find(x => x.id == id); if (!a) return;
    const map = {
      disable: { t: '停用账号', msg: `停用后「${a.name}」将无法登录,可随时恢复。确认停用?`, d: true, ok: '停用', done: '账号已停用' },
      enable: { t: '启用账号', msg: `恢复「${a.name}」的登录权限?`, ok: '启用', done: '账号已启用' },
      archive: { t: '归档账号', msg: `归档「${a.name}」(通常用于毕业/离职);归档后从在用名单移除,数据保留可恢复。`, d: true, ok: '归档', done: '账号已归档' },
      restore: { t: '恢复账号', msg: `将「${a.name}」从归档恢复为在用状态?`, ok: '恢复', done: '账号已恢复' },
      cancel: { t: '注销账号', msg: `注销「${a.name}」为不可逆操作,将清退该账号在本校的访问权限。确认注销?`, d: true, ok: '确认注销', done: '账号已注销' },
      reset: { t: '重置密码', msg: `将为「${a.name}」生成新的初始密码并通过短信下发,原密码立即失效。`, ok: '重置', done: '密码已重置,初始密码已下发' },
      kick: { t: '强制下线', msg: `立即终止「${a.name}」的所有在线会话,需重新登录。`, d: true, ok: '强制下线', done: '已强制下线' },
      grant: { t: '授予管理员', msg: `将「${a.name}」提升为学校管理员,可管理本校用户/组织/配置。该操作记审计。`, ok: '授予', done: '已授予学校管理员' },
      revoke: { t: '撤销管理员', msg: `撤销「${a.name}」的学校管理员权限,保留其教师身份。`, d: true, ok: '撤销', done: '已撤销管理员权限' },
    };
    const c = map[act]; if (!c) return;
    if (await C.confirm({ title: c.t, message: c.msg, confirmText: c.ok, danger: !!c.d }))
      C.toast('success', c.done, `操作对象:${a.name}(${a.no})`);
  };

  /* 批量操作工具条动作 */
  C.saAccBulk = async function (act) {
    const ids = Object.keys(C.saAcc.sel).filter(k => C.saAcc.sel[k]);
    if (!ids.length) { C.toast('info', '请先勾选账号', '在列表左侧勾选后再执行批量操作'); return; }
    const map = {
      disable: { t: '批量停用', msg: `将停用选中的 ${ids.length} 个账号,可随时恢复。确认?`, d: true, ok: '批量停用', done: '已批量停用' },
      archive: { t: '按学年归档', msg: `将选中的 ${ids.length} 个账号按毕业学年归档,移出在用名单(数据保留)。`, d: true, ok: '归档', done: '已按学年归档' },
      restore: { t: '批量恢复', msg: `将选中的 ${ids.length} 个账号恢复为在用状态?`, ok: '批量恢复', done: '已批量恢复' },
    };
    const c = map[act]; if (!c) return;
    if (await C.confirm({ title: c.t, message: c.msg, confirmText: c.ok, danger: !!c.d })) {
      C.toast('success', c.done, `共处理 ${ids.length} 个账号`);
      C.saAcc.sel = {}; C.rerender();
    }
  };

  /* 模板下载 / 导入记录入口 */
  C.saTpl = () => C.toast('success', '模板已下载', '请按模板列填写后上传,系统会逐行校验');

  function accountsPage() {
    const rows = accountRows();
    const f = C.saAcc;
    /* 多维筛选 */
    const filtered = rows.filter(a =>
      (f.role === '全部' || a.role === f.role) &&
      (f.status === '全部' || a.status === f.status) &&
      (!f.kw || a.name.includes(f.kw) || String(a.no).includes(f.kw)));
    const selCount = Object.values(f.sel).filter(Boolean).length;
    const stBadge = { '正常': 'green', '已停用': 'gray', '已归档': 'blue', '已注销': 'red' };
    const allChecked = filtered.length > 0 && filtered.every(a => f.sel[a.id]);

    const actions = `
      <button class="btn btn-outline" onclick="Chaimir.saTpl()">${C.icon('file-down')} 模板下载</button>
      <button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/import-batches')">${C.icon('history')} 导入记录</button>
      <button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/account-import')">${C.icon('upload')} 批量导入</button>
      <button class="btn btn-primary" onclick="Chaimir.saAccEdit()">${C.icon('user-plus')} 新增账号</button>`;

    if (rows.length === 0) {
      return `${C.head('用户管理', '用户与组织', actions)}
        <div class="card card-pad">${C.empty({
          icon: 'users', title: '本校还没有任何账号',
          desc: '建议先在「组织架构」建好院系/专业/班级,再通过批量导入快速创建师生账号。',
          action: `<div class="flex gap-2" style="justify-content:center">
            <button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/org')">${C.icon('network')} 先建组织</button>
            <button class="btn btn-primary" onclick="Chaimir.navigate('school-admin/account-import')">${C.icon('upload')} 再导入账号</button></div>`
        })}</div>`;
    }

    /* 批量工具条(选中时浮现) */
    const bulkBar = selCount ? `
      <div class="card card-pad mb-3 flex items-center justify-between wrap gap-3" style="border-color:var(--color-primary);background:var(--amber-50)">
        <div class="flex items-center gap-2 text-sm fw-600">${C.icon('check-square')} 已选 ${selCount} 项</div>
        <div class="flex gap-2 wrap">
          <button class="btn btn-outline btn-sm" onclick="Chaimir.saAccBulk('disable')">${C.icon('ban')} 批量停用</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.saAccBulk('archive')">${C.icon('archive')} 按学年归档</button>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.saAccBulk('restore')">${C.icon('archive-restore')} 批量恢复</button>
          <button class="btn btn-ghost btn-sm" onclick="Chaimir.saAcc.sel={};Chaimir.rerender()">取消选择</button>
        </div>
      </div>` : '';

    return `${C.head('用户管理', '用户与组织', actions)}
      <div class="card card-pad mb-4">
        <div class="flex gap-3 wrap items-end">
          <div class="field" style="margin:0;flex:1;min-width:200px"><label>关键词</label>
            <div class="input-icon">${C.icon('search')}<input class="input" placeholder="搜索姓名 / 学号 / 工号" value="${C.esc(f.kw)}" oninput="Chaimir.saAcc.kw=this.value" onkeydown="if(event.key==='Enter')Chaimir.rerender()"></div></div>
          <div class="field" style="margin:0;width:140px"><label>身份</label>
            <select class="select" onchange="Chaimir.saAccSet('role',this.value)">
              ${['全部', '教师', '学生'].map(o => `<option ${f.role === o ? 'selected' : ''}>${o}</option>`).join('')}</select></div>
          <div class="field" style="margin:0;width:160px"><label>班级 / 院系</label>
            <select class="select" onchange="Chaimir.saAccSet('cls',this.value)">
              ${['全部', '区块链 2301', '计算机学院', '网络空间安全学院'].map(o => `<option ${f.cls === o ? 'selected' : ''}>${o}</option>`).join('')}</select></div>
          <div class="field" style="margin:0;width:140px"><label>状态</label>
            <select class="select" onchange="Chaimir.saAccSet('status',this.value)">
              ${['全部', '正常', '已停用', '已归档', '已注销'].map(o => `<option ${f.status === o ? 'selected' : ''}>${o}</option>`).join('')}</select></div>
          <button class="btn btn-primary" onclick="Chaimir.rerender()">${C.icon('filter')} 筛选</button>
        </div>
      </div>
      ${bulkBar}
      <div class="table-wrap"><table class="table">
        <thead><tr>
          <th style="width:36px"><input type="checkbox" ${allChecked ? 'checked' : ''} onclick="Chaimir.saAccToggleAll(this)" aria-label="全选"></th>
          <th>姓名</th><th>手机号</th><th>学号 / 工号</th><th>身份</th><th>所属</th><th>状态</th><th>最后登录</th><th style="text-align:right">操作</th>
        </tr></thead>
        <tbody>
          ${filtered.map(a => {
            const isStudent = a.role === '学生';
            const archived = a.status === '已归档';
            const disabled = a.status === '已停用';
            return `<tr>
              <td><input type="checkbox" ${f.sel[a.id] ? 'checked' : ''} onclick="Chaimir.saAccToggle(${a.id},this)" aria-label="选择 ${C.esc(a.name)}"></td>
              <td class="fw-600">${C.esc(a.name)}</td>
              <td class="mono muted">${a.phone}</td>
              <td class="mono">${C.esc(a.no)}</td>
              <td>${C.badge(a.role, a.role === '教师' ? 'blue' : 'gray')}</td>
              <td>${C.esc(a.belong)}</td>
              <td>${C.statusDot(a.status === '正常' ? 'green' : a.status === '已停用' ? 'gray' : archived ? 'blue' : 'red', a.status)}</td>
              <td class="muted text-sm">${C.esc(a.login)}</td>
              <td class="row-actions">
                <button class="btn btn-ghost btn-sm" title="编辑" onclick="Chaimir.saAccEdit(${a.id})">${C.icon('pencil')}</button>
                ${archived ? `<button class="btn btn-ghost btn-sm" title="恢复" onclick="Chaimir.saAccAct(${a.id},'restore')">${C.icon('archive-restore')}</button>`
                  : disabled ? `<button class="btn btn-ghost btn-sm" title="启用" onclick="Chaimir.saAccAct(${a.id},'enable')">${C.icon('circle-check')}</button>`
                  : `<button class="btn btn-ghost btn-sm" title="停用" onclick="Chaimir.saAccAct(${a.id},'disable')">${C.icon('ban')}</button>`}
                <button class="btn btn-ghost btn-sm" title="更多" onclick="Chaimir.saAccMore(event,${a.id})">${C.icon('more-horizontal')}</button>
              </td>
            </tr>`;
          }).join('')}
        </tbody>
      </table></div>
      ${C.pagination(f.page, filtered.length, 20)}`;
  }

  /* 行内"更多"菜单:归档/恢复/注销/重置密码/强制下线/授予撤销管理员 */
  C.saAccMore = function (ev, id) {
    ev.stopPropagation();
    const a = accountRows().find(x => x.id == id); if (!a) return;
    const isStudent = a.role === '学生';
    const archived = a.status === '已归档';
    /* 关闭已存在菜单 */
    document.querySelectorAll('.sa-more-pop').forEach(n => n.remove());
    const pop = document.createElement('div');
    pop.className = 'popover sa-more-pop';
    pop.style.minWidth = '188px';
    /* 授予/撤销管理员:仅对教师可用,学生禁用并提示 */
    const adminItem = isStudent
      ? `<button class="menu-item" disabled style="opacity:.45;cursor:not-allowed" title="仅教师可被授予管理员">${C.icon('shield')} 授予管理员(仅教师)</button>`
      : `<button class="menu-item" data-act="grant">${C.icon('shield')} 授予学校管理员</button>
         <button class="menu-item" data-act="revoke">${C.icon('shield-off')} 撤销管理员</button>`;
    pop.innerHTML = `
      <button class="menu-item" data-act="reset">${C.icon('key-round')} 重置密码</button>
      <button class="menu-item" data-act="kick">${C.icon('log-out')} 强制下线</button>
      <div class="menu-sep"></div>
      ${archived
        ? `<button class="menu-item" data-act="restore">${C.icon('archive-restore')} 恢复账号</button>`
        : `<button class="menu-item" data-act="archive">${C.icon('archive')} 归档账号</button>`}
      ${adminItem}
      <div class="menu-sep"></div>
      <button class="menu-item danger" data-act="cancel">${C.icon('user-x')} 注销账号</button>`;
    document.body.appendChild(pop);
    const r = ev.currentTarget.getBoundingClientRect();
    pop.style.top = (r.bottom + window.scrollY + 4) + 'px';
    pop.style.left = (r.right + window.scrollX - pop.offsetWidth) + 'px';
    C.refreshIcons();
    pop.querySelectorAll('[data-act]').forEach(b => b.onclick = () => { pop.remove(); C.saAccAct(id, b.dataset.act); });
    setTimeout(() => document.addEventListener('mousedown', function h(e) { if (!pop.contains(e.target)) { pop.remove(); document.removeEventListener('mousedown', h); } }), 0);
  };

  /* ============================================================
     ④ 账号新增 / 编辑(school-admin/account-edit,弹窗形态)
        不可变字段(学号/工号)在编辑态置灰;开通方式三选一。
     ============================================================ */
  C.saAccEdit = function (id) {
    const a = id ? accountRows().find(x => x.id == id) : null;
    const editing = !!a;
    C.modal({
      title: editing ? '编辑账号' : '新增账号', size: 'lg',
      body: `
        <div class="grid grid-2">
          <div class="field"><label>姓名<span class="req">*</span></label>
            <input class="input" id="ae-name" value="${editing ? C.esc(a.name) : ''}" placeholder="真实姓名"></div>
          <div class="field"><label>身份<span class="req">*</span></label>
            <select class="select" id="ae-role" ${editing ? 'disabled' : ''}>
              <option ${editing && a.role === '教师' ? 'selected' : ''}>教师</option>
              <option ${editing && a.role === '学生' ? 'selected' : ''}>学生</option></select></div>
          <div class="field"><label>学号 / 工号<span class="req">*</span></label>
            <input class="input" value="${editing ? C.esc(a.no) : ''}" ${editing ? 'disabled' : ''} placeholder="唯一标识,创建后不可修改">
            ${editing ? `<div class="help">${C.icon('lock')} 学号 / 工号为身份唯一标识,创建后不可修改</div>` : ''}</div>
          <div class="field"><label>手机号</label>
            <input class="input" placeholder="用于短信通知与找回密码" value="${editing ? a.phone : ''}"></div>
          <div class="field"><label>所属院系 / 班级<span class="req">*</span></label>
            <select class="select"><option>区块链 2301</option><option>计算机学院</option><option>网络空间安全学院</option></select></div>
          <div class="field"><label>邮箱</label><input class="input" placeholder="选填"></div>
        </div>
        <div class="divider"></div>
        <div class="field" style="margin-bottom:8px"><label>开通方式<span class="req">*</span></label>
          <div class="help" style="margin-bottom:8px">选择账号如何完成首次登录</div>
          <label class="radio" style="display:flex;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:8px">
            <input type="radio" name="ae-open" checked> <span><b>设置初始密码</b> · 由管理员设定,首次登录强制改密</span></label>
          <label class="radio" style="display:flex;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:8px">
            <input type="radio" name="ae-open"> <span><b>生成激活码</b> · 下发一次性激活码,本人激活自设密码</span></label>
          <label class="radio" style="display:flex;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm)">
            <input type="radio" name="ae-open"> <span><b>加入 SSO 名单</b> · 由学校统一身份认证(CAS/LDAP)登录</span></label>
        </div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="Chaimir.saAccSave(${editing})">${editing ? '保存修改' : '创建账号'}</button>`,
    });
  };
  C.saAccSave = function (editing) {
    const name = (document.getElementById('ae-name') || {}).value;
    if (!name || !name.trim()) { C.toast('error', '请填写姓名', '姓名为必填项'); return; }
    document.querySelector('.overlay').remove();
    C.toast('success', editing ? '修改已保存' : '账号已创建', editing ? '账号信息已更新' : '已按所选开通方式生成登录凭据');
  };

  /* ============================================================
     ⑤ 账号导入向导(school-admin/account-import)— 2 步,需持久化
        强调:预览结果服务端暂存,前端只持 preview_id 防篡改;
              提交仅传 preview_id,服务端只写通过行。
     ============================================================ */
  C.saImport = { step: 1, type: '学生', file: null, previewId: null };
  C.saImportType = (t) => { C.saImport.type = t; C.rerender(); };
  C.saImportPick = () => {
    /* 原型:模拟选取文件 + 生成服务端 preview_id */
    C.saImport.file = (C.saImport.type === '学生' ? '学生名册_区块链2301.xlsx' : '教师名册_2026春.xlsx');
    C.saImport.previewId = 'prev_' + Math.random().toString(36).slice(2, 10);
    C.rerender();
    C.toast('success', '文件已上传', '服务端已生成预览,正在逐行校验…');
  };
  C.saImportStep = (n) => { C.saImport.step = n; C.rerender(); };
  C.saImportSubmit = async function () {
    const total = previewData().length, invalid = previewData().filter(r => !r.ok).length, valid = total - invalid;
    if (await C.confirm({
      title: '确认导入', danger: false, confirmText: `导入 ${valid} 条`,
      message: `校验完成:共 ${total} 条,其中 ${valid} 条将导入、${invalid} 条将跳过(失败行不写库)。仅提交预览编号 ${C.saImport.previewId},确认导入?`,
    })) {
      C.toast('success', '导入完成', `成功导入 ${valid} 条,跳过 ${invalid} 条;可在导入记录中查看`);
      C.saImport = { step: 1, type: C.saImport.type, file: null, previewId: null };
      setTimeout(() => C.navigate('school-admin/import-batches'), 800);
    }
  };

  /* 预览数据(原型:服务端逐行校验结果,前端只读) */
  function previewData() {
    const stu = [
      { row: 2, name: '陈嘉怡', no: '2025210101', cls: '区块链 2501', ok: true },
      { row: 3, name: '黄子轩', no: '2025210102', cls: '区块链 2501', ok: true },
      { row: 4, name: '李婉婷', no: '2025210103', cls: '区块链 2501', ok: true },
      { row: 5, name: '', no: '2025210104', cls: '区块链 2501', ok: false, err: '姓名为空' },
      { row: 6, name: '吴泽宇', no: '2023210456', cls: '区块链 2501', ok: false, err: '学号已存在' },
      { row: 7, name: '林佳琪', no: '20252101', cls: '区块链 2501', ok: false, err: '学号格式不符(应为 10 位)' },
      { row: 8, name: '徐梓涵', no: '2025210106', cls: '不存在的班级', ok: false, err: '班级在组织架构中不存在' },
      { row: 9, name: '何思琪', no: '2025210107', cls: '区块链 2501', ok: true },
    ];
    const tea = [
      { row: 2, name: '罗启明', no: 'T2026001', cls: '计算机学院', ok: true },
      { row: 3, name: '范晓晴', no: 'T2026002', cls: '网络空间安全学院', ok: true },
      { row: 4, name: '杜文博', no: 'T2019033', cls: '计算机学院', ok: false, err: '工号已存在' },
      { row: 5, name: '', no: 'T2026004', cls: '计算机学院', ok: false, err: '姓名为空' },
    ];
    return C.saImport.type === '学生' ? stu : tea;
  }

  function accountImport() {
    Object.assign(C.parentRoute, { 'school-admin/account-import': 'school-admin/accounts' });
    const s = C.saImport;
    const stepsHtml = `<div class="steps">
      <div class="step ${s.step >= 1 ? (s.step > 1 ? 'done' : 'active') : ''}">
        <span class="dot-n">${s.step > 1 ? C.icon('check') : '1'}</span><span class="step-label">选择类型并上传</span><span class="line"></span></div>
      <div class="step ${s.step >= 2 ? 'active' : ''}">
        <span class="dot-n">2</span><span class="step-label">预览校验并提交</span></div>
    </div>`;

    let body;
    if (s.step === 1) {
      /* 步骤①:选类型 + 下载模板 + 上传文件 */
      body = `
        <div class="card card-pad mb-4">
          <div class="section-title mb-3">① 选择导入对象</div>
          <div class="grid grid-2 mb-4">
            ${['学生', '教师'].map(t => `
              <label class="card card-pad card-hover ${s.type === t ? '' : ''}" style="display:flex;gap:12px;align-items:center;cursor:pointer;${s.type === t ? 'border-color:var(--color-primary);background:var(--amber-50)' : ''}" onclick="Chaimir.saImportType('${t}')">
                <div class="stat-icon" style="background:var(--${t === '学生' ? 'blue' : 'amber'}-100);color:var(--${t === '学生' ? 'blue' : 'amber'}-700)">${C.icon(t === '学生' ? 'graduation-cap' : 'presentation')}</div>
                <div><div class="fw-700">${t}名册</div><div class="muted text-xs mt-2">${t === '学生' ? '需含姓名、学号、班级' : '需含姓名、工号、院系'}</div></div>
                ${s.type === t ? `<span style="margin-left:auto;color:var(--color-primary-text)">${C.icon('check-circle-2')}</span>` : ''}
              </label>`).join('')}
          </div>
          <div class="section-title mb-3">② 下载模板并填写</div>
          <div class="callout info mb-3">${C.icon('info')}<div>请下载对应模板,按表头列填写后上传(支持 .xlsx / .csv)。系统将逐行校验,<b>预览阶段不写库</b>。</div></div>
          <button class="btn btn-outline mb-4" onclick="Chaimir.saTpl()">${C.icon('file-down')} 下载${s.type}导入模板</button>
          <div class="section-title mb-3">③ 上传文件</div>
          ${s.file
            ? `<div class="card card-pad flex items-center justify-between" style="border-color:var(--green-600);background:var(--green-50)">
                 <div class="flex items-center gap-3">${C.icon('file-check-2')}<div><div class="fw-600">${C.esc(s.file)}</div>
                   <div class="muted text-xs mt-2">已上传,服务端预览编号 <span class="mono">${s.previewId}</span></div></div></div>
                 <button class="btn btn-ghost btn-sm" onclick="Chaimir.saImport.file=null;Chaimir.saImport.previewId=null;Chaimir.rerender()">重新选择</button></div>`
            : `<div class="card card-pad" style="border-style:dashed;border-color:var(--color-border-strong);text-align:center;cursor:pointer" onclick="Chaimir.saImportPick()">
                 <div class="empty-ico" style="margin:0 auto 10px">${C.icon('upload-cloud')}</div>
                 <div class="fw-600">点击选择文件或拖拽到此处</div>
                 <div class="muted text-xs mt-2">支持 .xlsx / .csv,单次最多 2000 行</div></div>`}
        </div>
        <div class="flex justify-between">
          <button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/accounts')">取消</button>
          <button class="btn btn-primary" ${s.file ? '' : 'aria-disabled="true"'} onclick="${s.file ? "Chaimir.saImportStep(2)" : "Chaimir.toast('info','请先上传文件','上传名册后才能进入预览')"}">下一步:预览校验 ${C.icon('arrow-right')}</button>
        </div>`;
    } else {
      /* 步骤②:逐行校验预览(成功/失败标红 + 原因)+ 统计 + 提交 */
      const data = previewData();
      const total = data.length, invalid = data.filter(r => !r.ok).length, valid = total - invalid;
      body = `
        <div class="callout warn mb-4">${C.icon('shield')}<div>以下为服务端校验结果(暂存于预览编号 <span class="mono">${s.previewId}</span>),<b>前端不持有数据、只持有预览编号</b>;提交时仅回传该编号,服务端按已校验结果只写入通过行,杜绝前端篡改。</div></div>
        <div class="grid grid-3 mb-4">
          ${C.stat('list', String(total), '总行数', 'blue')}
          ${C.stat('check-circle-2', String(valid), '校验通过(将导入)', 'green')}
          ${C.stat('alert-circle', String(invalid), '校验失败(将跳过)', 'red')}
        </div>
        <div class="table-wrap mb-4"><table class="table">
          <thead><tr><th style="width:64px">行号</th><th>姓名</th><th>${s.type === '学生' ? '学号' : '工号'}</th><th>${s.type === '学生' ? '班级' : '院系'}</th><th>校验结果</th></tr></thead>
          <tbody>${data.map(r => `
            <tr ${r.ok ? '' : 'style="background:var(--red-50)"'}>
              <td class="mono muted">${r.row}</td>
              <td class="fw-600">${r.name ? C.esc(r.name) : '<span class="muted">(空)</span>'}</td>
              <td class="mono">${C.esc(r.no)}</td>
              <td>${C.esc(r.cls)}</td>
              <td>${r.ok ? C.statusDot('green', '通过') : `<span class="flex items-center gap-2"><span class="dot dot-red"></span><span style="color:var(--color-danger)">${C.esc(r.err)}</span></span>`}</td>
            </tr>`).join('')}</tbody>
        </table></div>
        <div class="flex justify-between">
          <button class="btn btn-outline" onclick="Chaimir.saImportStep(1)">${C.icon('arrow-left')} 上一步</button>
          <button class="btn btn-primary" onclick="Chaimir.saImportSubmit()">${C.icon('check')} ${valid} 条将导入,${invalid} 条将跳过 — 确认提交</button>
        </div>`;
    }

    return `${C.crumb([{ label: '用户管理', to: 'school-admin/accounts' }, { label: '批量导入' }])}
      ${C.head('批量导入账号', null, '')}
      ${stepsHtml}${body}`;
  }

  /* ============================================================
     ⑥ 组织架构(school-admin/org)— 院系→专业→班级 三级树
     ============================================================ */
  /* 演示树:展开态保存在 expand 集合 */
  C.saOrg = { expand: { '计算机学院': true, '区块链工程': true } };
  C.saOrgTree = () => [
    { name: '计算机学院', kind: '院系', children: [
      { name: '区块链工程', kind: '专业', children: [
        { name: '区块链 2301', kind: '班级', year: 2023, students: 42 },
        { name: '区块链 2401', kind: '班级', year: 2024, students: 45 },
        { name: '区块链 2501', kind: '班级', year: 2025, students: 0, fresh: true },
      ]},
      { name: '软件工程', kind: '专业', children: [
        { name: '软工 2302', kind: '班级', year: 2023, students: 50 },
      ]},
    ]},
    { name: '网络空间安全学院', kind: '院系', children: [
      { name: '信息安全', kind: '专业', children: [
        { name: '信安 2303', kind: '班级', year: 2023, students: 38 },
      ]},
    ]},
  ];
  C.saOrgToggle = (name) => { C.saOrg.expand[name] = !C.saOrg.expand[name]; C.rerender(); };

  /* 各级 CRUD(原型:统一弹窗 + toast) */
  C.saOrgAdd = function (kind, parent) {
    const sub = { '院系': '专业', '专业': '班级', '根': '院系' }[kind] || '院系';
    const isClass = sub === '班级';
    C.modal({
      title: `新增${sub}` + (parent ? `(隶属:${parent})` : ''),
      body: `<div class="field"><label>${sub}名称<span class="req">*</span></label><input class="input" id="org-name" placeholder="如:${sub === '院系' ? '人工智能学院' : sub === '专业' ? '数据科学' : '区块链 2601'}"></div>
        ${parent ? `<div class="field"><label>上级</label><input class="input" value="${C.esc(parent)}" disabled></div>` : ''}
        ${isClass ? `<div class="field"><label>关联入学年份<span class="req">*</span></label>
          <select class="select"><option>2025</option><option>2024</option><option>2023</option></select>
          <div class="help">入学年份用于按学年归档与年级升级</div></div>` : ''}`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="(function(){var v=(document.getElementById('org-name')||{}).value;if(!v||!v.trim()){Chaimir.toast('error','请填写名称','${sub}名称为必填项');return}document.querySelector('.overlay').remove();Chaimir.toast('success','${sub}已创建','已添加到组织架构')})()">创建</button>`,
    });
  };
  C.saOrgEdit = function (name, kind) {
    C.modal({
      title: `编辑${kind}`,
      body: `<div class="field"><label>${kind}名称<span class="req">*</span></label><input class="input" id="org-ename" value="${C.esc(name)}"></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','修改已保存','组织信息已更新')">保存</button>`,
    });
  };
  C.saOrgDel = async function (name, kind, hasChild) {
    const msg = hasChild
      ? `「${name}」下还包含下级节点(${kind === '院系' ? '专业 / 班级' : '班级'}),删除将一并移除其全部下级,且需先迁出在籍人员。此操作不可恢复,确认删除?`
      : `确认删除${kind}「${name}」?此操作不可恢复。`;
    if (await C.confirm({ title: `删除${kind}`, message: msg, confirmText: '确认删除', danger: true }))
      C.toast('success', `${kind}已删除`, `「${name}」及其下级已移除`);
  };
  C.saOrgArchive = async function (name) {
    if (await C.confirm({ title: '归档班级', message: `归档班级「${name}」(通常用于该班毕业);归档后不再参与开课,数据保留。`, confirmText: '归档', danger: true }))
      C.toast('success', '班级已归档', `「${name}」已移入归档`);
  };
  C.saOrgUpgrade = async function () {
    if (await C.confirm({ title: '年级升级', message: '将按入学年份把全校班级整体升一个年级(如 2023 级升入下一学年)。该操作影响全部在籍班级,建议在学年初执行。', confirmText: '执行升级' }))
      C.toast('success', '年级升级完成', '全校班级已按入学年份整体升级');
  };

  function orgNode(node, depth) {
    const pad = 12 + depth * 22;
    const hasChild = node.children && node.children.length;
    const open = C.saOrg.expand[node.name];
    const kindBadge = { '院系': 'blue', '专业': 'purple', '班级': 'gray' }[node.kind];
    let row = `<div class="side-item" style="padding-left:${pad}px;border-radius:var(--radius-sm);margin-bottom:0">
      ${hasChild
        ? `<button class="btn-icon btn-ghost" style="padding:2px;width:20px;height:20px" onclick="Chaimir.saOrgToggle('${C.esc(node.name)}')" aria-label="${open ? '折叠' : '展开'}">${C.icon(open ? 'chevron-down' : 'chevron-right')}</button>`
        : `<span style="width:20px;display:inline-block"></span>`}
      ${C.icon(node.kind === '班级' ? 'users' : node.kind === '专业' ? 'git-branch' : 'building-2')}
      <span class="fw-600">${C.esc(node.name)}</span>
      ${C.badge(node.kind, kindBadge)}
      ${node.kind === '班级' ? `<span class="muted text-xs">${node.fresh ? C.badge('新生待导入', 'amber') : `${node.year} 级 · ${node.students} 人`}</span>` : ''}
      <span style="margin-left:auto" class="flex gap-1">
        ${node.kind !== '班级' ? `<button class="btn btn-ghost btn-sm" title="新增下级" onclick="Chaimir.saOrgAdd('${node.kind}','${C.esc(node.name)}')">${C.icon('plus')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" title="编辑" onclick="Chaimir.saOrgEdit('${C.esc(node.name)}','${node.kind}')">${C.icon('pencil')}</button>
        ${node.kind === '班级' ? `<button class="btn btn-ghost btn-sm" title="归档" onclick="Chaimir.saOrgArchive('${C.esc(node.name)}')">${C.icon('archive')}</button>` : ''}
        <button class="btn btn-ghost btn-sm" title="删除" onclick="Chaimir.saOrgDel('${C.esc(node.name)}','${node.kind}',${hasChild ? 'true' : 'false'})">${C.icon('trash-2')}</button>
      </span></div>`;
    let kids = '';
    if (hasChild && open) kids = node.children.map(c => orgNode(c, depth + 1)).join('');
    return row + kids;
  }

  function orgPage() {
    const tree = C.saOrgTree();
    const actions = `
      <button class="btn btn-outline" onclick="Chaimir.saOrgUpgrade()">${C.icon('arrow-up-circle')} 年级升级</button>
      <button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/account-import')">${C.icon('upload')} 批量导入</button>
      <button class="btn btn-primary" onclick="Chaimir.saOrgAdd('根')">${C.icon('plus')} 新增院系</button>`;

    if (tree.length === 0) {
      return `${C.head('组织架构', '用户与组织', actions)}
        <div class="card card-pad">${C.empty({
          icon: 'network', title: '本校还没有组织架构',
          desc: '作为首个管理员,请先建立院系 → 专业 → 班级三级结构,后续导入师生才能正确归属。',
          action: `<button class="btn btn-primary" onclick="Chaimir.saOrgAdd('根')">${C.icon('plus')} 创建第一个院系</button>`
        })}</div>`;
    }

    return `${C.head('组织架构', '用户与组织', actions)}
      <div class="callout info mb-4">${C.icon('info')}<div>三级结构:院系 → 专业 → 班级。班级须关联入学年份,以支持按学年归档与年级升级。点击节点左侧箭头可展开 / 折叠。</div></div>
      <div class="card" style="padding:8px">
        ${tree.map(n => orgNode(n, 0)).join('')}
      </div>`;
  }

  /* ============================================================
     ⑦ 导入记录(school-admin/import-batches)
     ============================================================ */
  function importBatches() {
    const batches = [
      { id: 'IMP-20260606-03', type: '学生', file: '学生名册_区块链2501.xlsx', imported: 43, skipped: 5, time: '2026-06-06 14:22', op: '王校管' },
      { id: 'IMP-20260603-02', type: '教师', file: '教师名册_2026春.csv', imported: 12, skipped: 1, time: '2026-06-03 09:10', op: '王校管' },
      { id: 'IMP-20260520-01', type: '学生', file: '学生名册_软工2302.xlsx', imported: 50, skipped: 0, time: '2026-05-20 16:40', op: '李教务' },
    ];
    return `${C.head('导入记录', '用户与组织', `<button class="btn btn-primary" onclick="Chaimir.navigate('school-admin/account-import')">${C.icon('upload')} 新建导入</button>`)}
      <div class="table-wrap"><table class="table">
        <thead><tr><th>批次编号</th><th>类型</th><th>文件</th><th>导入数</th><th>跳过数</th><th>时间</th><th>操作人</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${batches.map(b => `
          <tr>
            <td class="mono">${b.id}</td>
            <td>${C.badge(b.type, b.type === '学生' ? 'blue' : 'amber')}</td>
            <td class="ellipsis" style="max-width:200px">${C.esc(b.file)}</td>
            <td>${C.statusDot('green', String(b.imported))}</td>
            <td>${b.skipped ? C.statusDot('amber', String(b.skipped)) : C.statusDot('gray', '0')}</td>
            <td class="muted text-sm mono">${b.time}</td>
            <td>${C.esc(b.op)}</td>
            <td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.demo('查看导入明细')">${C.icon('eye')} 明细</button></td>
          </tr>`).join('')}</tbody>
      </table></div>
      ${C.pagination(1, batches.length, 20)}`;
  }

  /* ---------- 注册路由 ---------- */
  C.registerPages({
    'school-admin/dashboard': dashboard,
    'school-admin/statistics': statistics,
    'school-admin/accounts': accountsPage,
    'school-admin/account-import': accountImport,
    'school-admin/org': orgPage,
    'school-admin/import-batches': importBatches,
  });
})();
