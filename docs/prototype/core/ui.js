/* ============================================================
   core/ui.js — Chaimir 原型通用 UI 工具与命名空间
   ------------------------------------------------------------
   职责:① 暴露全局 Chaimir 命名空间(页面注册表 + 工具);
        ② 提供反馈类组件(toast/模态/确认/抽屉)与构建器
           (徽章/空态/骨架/分页/面包屑),供所有页面复用。
   设计原则:页面只产出 HTML 字符串,交互行为经此处统一工具;
            文案面向用户(自然语言,不暴露技术术语)。
   ============================================================ */
(function () {
  const Chaimir = window.Chaimir = window.Chaimir || {};
  Chaimir.pages = Chaimir.pages || {};
  /* 这两个表由各页面模块在加载期写入,而 shell.js 最后加载;
     故必须在最先加载的 ui.js 中先建好,避免页面模块引用到 undefined。 */
  Chaimir.parentRoute = Chaimir.parentRoute || {};   /* 子页 → 高亮的侧栏路由 */
  Chaimir.mounts = Chaimir.mounts || {};             /* 路由 → 渲染后回调 */

  /* 页面注册:各 pages/*.js 调用此函数把"路由→渲染函数"登记进来 */
  Chaimir.registerPages = function (obj) { Object.assign(Chaimir.pages, obj); };

  /* 图标(lucide):统一生成 <i data-lucide>,渲染后由 refreshIcons 实例化 */
  const icon = Chaimir.icon = (name, cls) =>
    `<i data-lucide="${name}"${cls ? ` class="${cls}"` : ''}></i>`;
  Chaimir.refreshIcons = () => { if (window.lucide) window.lucide.createIcons(); };

  /* HTML 转义(防原型里假数据破坏结构) */
  const esc = Chaimir.esc = (s) => String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

  /* ---------- 构建器:返回 HTML 字符串 ---------- */

  /* 徽章 */
  Chaimir.badge = (text, variant = 'gray', ic) =>
    `<span class="badge badge-${variant}">${ic ? icon(ic) : ''}${esc(text)}</span>`;

  /* 状态点 + 文字(颜色非唯一信息载体,无障碍) */
  Chaimir.statusDot = (color, text) =>
    `<span class="flex items-center gap-2"><span class="dot dot-${color}"></span>${esc(text)}</span>`;

  /* 空状态 */
  Chaimir.empty = ({ icon: ic = 'inbox', title = '暂无数据', desc = '', action = '' }) => `
    <div class="empty">
      <div class="empty-ico">${icon(ic)}</div>
      <div class="empty-title">${esc(title)}</div>
      ${desc ? `<div class="empty-desc">${esc(desc)}</div>` : ''}
      ${action || ''}
    </div>`;

  /* 骨架屏列表 */
  Chaimir.skeletonList = (rows = 4) =>
    `<div class="card card-pad">${'<div class="skeleton line" style="width:'
      + '80%"></div><div class="skeleton line" style="width:60%"></div>'.repeat(rows)}</div>`;

  /* 面包屑 */
  Chaimir.crumb = (items) => `<div class="breadcrumb">${items.map((it, i) =>
    (i ? `<span class="sep">/</span>` : '') +
    (it.to ? `<a onclick="Chaimir.navigate('${it.to}')">${esc(it.label)}</a>` : `<span>${esc(it.label)}</span>`)
  ).join('')}</div>`;

  /* 内容页头(标题 + 副标题 + 右侧操作区) */
  Chaimir.head = (title, sub, actions) => `
    <div class="content-head">
      <div>${sub ? (typeof sub === 'object' ? Chaimir.crumb(sub) : `<div class="page-sub">${esc(sub)}</div>`) : ''}
        <h1 class="page-title">${esc(title)}</h1></div>
      ${actions ? `<div class="content-actions">${actions}</div>` : ''}
    </div>`;

  /* 统计卡 */
  Chaimir.stat = (ic, num, label, color = 'amber', delta) => `
    <div class="card card-pad stat">
      <div class="stat-icon" style="background:var(--${color}-100);color:var(--${color}-700)">${icon(ic)}</div>
      <div><div class="num">${esc(num)}</div><div class="label">${esc(label)}</div>
      ${delta ? `<div class="delta ${delta.dir}">${delta.dir === 'up' ? '↑' : '↓'} ${esc(delta.text)}</div>` : ''}</div>
    </div>`;

  /* 简单分页 */
  Chaimir.pagination = (page, total, size = 20) => {
    const pages = Math.max(1, Math.ceil(total / size));
    let btns = '';
    for (let i = 1; i <= Math.min(pages, 5); i++)
      btns += `<button class="${i === page ? 'active' : ''}">${i}</button>`;
    return `<div class="pagination"><button ${page <= 1 ? 'disabled' : ''}>${icon('chevron-left')}</button>${btns}
      <button ${page >= pages ? 'disabled' : ''}>${icon('chevron-right')}</button>
      <span class="muted text-sm" style="margin-left:8px">共 ${total} 条</span></div>`;
  };

  /* ---------- 行为类:toast / 模态 / 确认 / 抽屉 ---------- */

  function ensureLayer(id) {
    let n = document.getElementById(id);
    if (!n) { n = document.createElement('div'); n.id = id; document.body.appendChild(n); }
    return n;
  }

  /* Toast:type ∈ success|error|info;trace 仅错误态展示报障编号 */
  Chaimir.toast = function (type, title, sub, trace) {
    const stack = ensureLayer('toast-stack'); stack.className = 'toast-stack';
    const t = document.createElement('div');
    t.className = `toast ${type}`;
    const ic = { success: 'check-circle-2', error: 'alert-circle', info: 'info' }[type] || 'info';
    t.innerHTML = `${icon(ic)}<div style="flex:1"><div class="t-title">${esc(title)}</div>
      ${sub ? `<div class="t-sub">${esc(sub)}</div>` : ''}
      ${trace ? `<div class="t-trace">报障编号 ${esc(trace)}</div>` : ''}</div>`;
    stack.appendChild(t); Chaimir.refreshIcons();
    setTimeout(() => { t.style.opacity = '0'; setTimeout(() => t.remove(), 200); }, trace ? 6000 : 3200);
  };

  /* 模态:opts={title, body(html), foot(html), size} 返回关闭函数 */
  Chaimir.modal = function (opts) {
    const layer = ensureLayer('modal-layer');
    const o = document.createElement('div'); o.className = 'overlay';
    o.innerHTML = `<div class="modal ${opts.size || ''}" role="dialog" aria-modal="true" aria-label="${esc(opts.title || '')}">
        <div class="modal-head"><div class="title">${esc(opts.title || '')}</div>
          <button class="modal-close" aria-label="关闭">${icon('x')}</button></div>
        <div class="modal-body">${opts.body || ''}</div>
        ${opts.foot ? `<div class="modal-foot">${opts.foot}</div>` : ''}
      </div>`;
    layer.appendChild(o); Chaimir.refreshIcons();
    const close = () => o.remove();
    o.querySelector('.modal-close').onclick = close;
    o.addEventListener('mousedown', (e) => { if (e.target === o && !opts.persistent) close(); });
    if (opts.onMount) opts.onMount(o, close);
    return close;
  };

  /* 二次确认:危险操作统一入口,返回 Promise<boolean> */
  Chaimir.confirm = function ({ title = '确认操作', message = '', confirmText = '确认', danger = false }) {
    return new Promise((resolve) => {
      const close = Chaimir.modal({
        title,
        body: `<p style="font-size:var(--text-sm);color:var(--color-text)">${esc(message)}</p>`,
        foot: `<button class="btn btn-outline" data-act="cancel">取消</button>
               <button class="btn ${danger ? 'btn-danger' : 'btn-primary'}" data-act="ok">${esc(confirmText)}</button>`,
        onMount: (root, _close) => {
          root.querySelector('[data-act="cancel"]').onclick = () => { _close(); resolve(false); };
          root.querySelector('[data-act="ok"]').onclick = () => { _close(); resolve(true); };
        }
      });
    });
  };

  /* 抽屉(右侧滑入) */
  Chaimir.drawer = function (opts) {
    const layer = ensureLayer('drawer-layer');
    const o = document.createElement('div'); o.className = 'overlay'; o.style.placeItems = 'stretch'; o.style.justifyItems = 'end';
    o.innerHTML = `<div class="drawer" role="dialog" aria-modal="true">
        <div class="modal-head"><div class="title">${esc(opts.title || '')}</div>
          <button class="modal-close" aria-label="关闭">${icon('x')}</button></div>
        <div class="modal-body" style="flex:1">${opts.body || ''}</div>
        ${opts.foot ? `<div class="modal-foot">${opts.foot}</div>` : ''}</div>`;
    layer.appendChild(o); Chaimir.refreshIcons();
    const close = () => o.remove();
    o.querySelector('.modal-close').onclick = close;
    o.addEventListener('mousedown', (e) => { if (e.target === o) close(); });
    if (opts.onMount) opts.onMount(o, close);
    return close;
  };

  /* 演示用:统一提示"这是原型,行为已模拟" */
  Chaimir.demo = (msg) => Chaimir.toast('info', msg || '原型演示', '该操作在原型中为模拟效果');

  /* 日期格式化(原型用) */
  Chaimir.fmtDate = (d) => { const x = new Date(d); return isNaN(x) ? d :
    `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, '0')}-${String(x.getDate()).padStart(2, '0')}`; };
})();
