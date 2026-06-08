/* ============================================================
   core/shell.js — 应用外壳 + 哈希路由
   ------------------------------------------------------------
   职责:① 解析 location.hash → 路由;② 按路由首段决定布局
        (home 导航 / auth 登录前 / 四端 app 外壳 / immersive 全屏);
        ③ 渲染外壳骨架(顶栏+侧栏)并把页面内容挂载进内容区。
   路由形如:#/student/courses 、#/student/course-detail?id=2 、
            #/auth/login 、#/immersive/exp-ide 、#/(原型导航)。
   ============================================================ */
(function () {
  const C = window.Chaimir = window.Chaimir || {};
  C.parentRoute = C.parentRoute || {};   /* 子页 → 高亮的侧栏路由 */
  const app = () => document.getElementById('app');

  /* 解析 hash → { route, query } */
  function parse() {
    let h = location.hash.replace(/^#\/?/, '');
    if (!h) return { route: '', query: {} };
    const [path, qs] = h.split('?');
    const query = {};
    if (qs) qs.split('&').forEach(p => { const [k, v] = p.split('='); query[decodeURIComponent(k)] = decodeURIComponent(v || ''); });
    return { route: path, query };
  }

  C.navigate = (route) => { location.hash = '#/' + route; };

  /* 顶栏(深色) */
  function topnav(role) {
    const r = C.roles[role];
    return `<header class="topnav">
      <div class="brand"><div class="logo-mark">${C.icon('link-2')}</div>Chaimir</div>
      <span class="role-pill">${C.icon(r.icon)} ${r.label}端</span>
      <div class="spacer"></div>
      <button class="nav-icon" title="通知" aria-label="通知" onclick="Chaimir.notifDropdown(event,'${role}')">
        ${C.icon('bell')}<span class="badge-count">3</span></button>
      <button class="nav-icon" title="帮助" onclick="Chaimir.toast('info','帮助中心','原型演示:此处打开帮助与新手引导')">${C.icon('help-circle')}</button>
      <button class="avatar" onclick="Chaimir.navigate('${role}/profile')">
        <span class="ava">林</span><span class="uname">林同学</span></button>
      <button class="nav-icon" title="退出登录" onclick="Chaimir.navigate('')">${C.icon('log-out')}</button>
    </header>`;
  }

  /* 顶栏通知下拉(铃铛触发,代替侧栏的消息分组) */
  C.notifDropdown = function (ev, role) {
    const ex = document.getElementById('notif-pop');
    if (ex) { ex.remove(); return; }
    const r = ev.currentTarget.getBoundingClientRect();
    const items = (C.mock && C.mock.notifications) || [];
    const unread = items.filter(n => !n.read).length;
    const pop = document.createElement('div');
    pop.id = 'notif-pop'; pop.className = 'popover';
    pop.style.cssText = `top:${r.bottom + 6}px;right:${Math.max(12, window.innerWidth - r.right)}px;width:344px`;
    pop.innerHTML = `
      <div style="padding:12px 14px;border-bottom:1px solid var(--color-border);display:flex;align-items:center;justify-content:space-between">
        <span class="fw-700 text-sm">通知 ${unread ? `<span class="badge badge-amber">${unread} 条未读</span>` : ''}</span>
        <a class="text-xs" style="color:var(--color-primary-text);cursor:pointer" onclick="Chaimir.toast('success','已全部标记已读','');document.getElementById('notif-pop').remove()">全部已读</a></div>
      <div style="max-height:340px;overflow:auto">${items.map(n => `
        <a class="menu-item" style="align-items:flex-start;border-radius:0;padding:11px 14px" onclick="Chaimir.navigate('${role}/notifications');document.getElementById('notif-pop').remove()">
          <span class="dot dot-${n.read ? 'gray' : 'amber'}" style="margin-top:5px"></span>
          <div style="min-width:0"><div class="text-sm ${n.read ? 'muted' : 'fw-600'}">${C.esc(n.title)}</div>
            <div class="muted text-xs" style="margin-top:2px">${C.esc(n.type)} · ${C.esc(n.time)}</div></div></a>`).join('')}</div>
      <div style="padding:8px;border-top:1px solid var(--color-border);display:flex;gap:6px">
        <button class="btn btn-outline btn-sm btn-block" onclick="Chaimir.navigate('${role}/announcements');document.getElementById('notif-pop').remove()">${C.icon('megaphone')} 系统公告</button>
        <button class="btn btn-primary btn-sm btn-block" onclick="Chaimir.navigate('${role}/notifications');document.getElementById('notif-pop').remove()">查看全部</button></div>`;
    document.body.appendChild(pop); C.refreshIcons();
    setTimeout(() => { const close = (e) => { if (!pop.contains(e.target)) { pop.remove(); document.removeEventListener('mousedown', close); } }; document.addEventListener('mousedown', close); }, 0);
  };

  /* 侧栏 */
  function sidebar(role, activeRoute) {
    const groups = C.nav[role] || [];
    const active = C.parentRoute[activeRoute] || activeRoute;
    return `<aside class="sidebar">${groups.map(g => `
      <div class="side-group"><div class="side-group-title">${g.group}</div>
        ${g.items.map(([route, ic, label, count]) => `
          <a class="side-item ${route === active ? 'active' : ''}" onclick="Chaimir.navigate('${route}')">
            ${C.icon(ic)}<span>${label}</span>
            ${count ? `<span class="count">${count}</span>` : ''}</a>`).join('')}
      </div>`).join('')}</aside>`;
  }

  /* 主渲染 */
  function render() {
    const { route, query } = parse();
    C.route = route; C.query = query;
    const seg = route.split('/');
    const top = seg[0];

    /* 1) 原型导航首页 */
    if (!route) { app().innerHTML = C.pages['home'] ? C.pages['home']() : '加载中…'; finalize(); return; }

    /* 2) 登录前 / 沉浸式:整屏由页面自绘 */
    if (top === 'auth' || top === 'immersive') {
      const fn = C.pages[route];
      app().innerHTML = fn ? fn({ route, query }) : notFound(route);
      finalize(); return;
    }

    /* 3) 四端 app 外壳 */
    if (C.roles[top]) {
      const fn = C.pages[route];
      const body = fn ? fn({ route, query, role: top }) : notFound(route);
      app().innerHTML = `${topnav(top)}
        <div class="app-body">${sidebar(top, route)}
          <main class="content" id="main-content"><div class="content-inner">${body}</div></main>
        </div>`;
      finalize(); document.getElementById('main-content').scrollTop = 0; return;
    }

    /* 4) 兜底 */
    app().innerHTML = notFound(route); finalize();
  }

  function notFound(route) {
    return `<div style="padding:60px"><div class="content-inner">${C.head('页面建设中', '该路由暂未在原型中实现')}
      <div class="card card-pad">${C.empty({ icon: 'hammer', title: '此页面尚未实现',
        desc: '路由 ' + route + ' 还没有对应的页面模块。返回原型导航查看已完成页面。',
        action: `<button class="btn btn-primary" onclick="Chaimir.navigate('')">返回原型导航</button>` })}</div></div></div>`;
  }

  /* 渲染收尾:实例化图标 + 触发页面 onMount 钩子 */
  function finalize() {
    C.refreshIcons();
    const { route } = parse();
    const hook = C.mounts && C.mounts[route];
    if (hook) try { hook(); } catch (e) { console.error('[mount]', route, e); }
  }
  C.mounts = C.mounts || {};   /* 路由 → 渲染后回调(图表/动画/定时器) */

  window.addEventListener('hashchange', render);
  window.addEventListener('DOMContentLoaded', render);
  C.rerender = render;
})();
