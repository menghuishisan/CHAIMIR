/* ============================================================
   pages/home.js — 原型导航首页(Launcher)
   ------------------------------------------------------------
   职责:原型总入口,分组展示四端、登录前页面、沉浸式工作台,
        以及设计决策落地说明。点击进入对应路由。
   ============================================================ */
(function () {
  const C = window.Chaimir;

  function roleCard(role) {
    const r = C.roles[role];
    const colors = { student: 'blue', teacher: 'amber', 'school-admin': 'green', 'platform-admin': 'purple' };
    const c = colors[role];
    const count = C.nav[role].reduce((n, g) => n + g.items.length, 0);
    return `<a class="card card-hover card-pad" onclick="Chaimir.navigate('${r.home}')">
      <div class="role-ico" style="width:48px;height:48px;border-radius:12px;display:grid;place-items:center;margin-bottom:14px;background:var(--${c}-100);color:var(--${c}-700)">${C.icon(r.icon)}</div>
      <div class="fw-700" style="font-size:var(--text-lg)">${r.label}端</div>
      <div class="muted text-sm mt-2">默认落点:${r.home.split('/')[1]} · 共 ${count} 个功能入口</div>
    </a>`;
  }

  function linkRow(list) {
    return list.map(([route, ic, label, desc]) => `
      <a class="card card-hover" style="display:flex;gap:12px;align-items:flex-start;padding:14px 16px" onclick="Chaimir.navigate('${route}')">
        <div style="width:36px;height:36px;border-radius:9px;background:var(--color-surface-sunken);display:grid;place-items:center;color:var(--color-primary-text);flex-shrink:0">${C.icon(ic)}</div>
        <div style="min-width:0"><div class="fw-600 text-sm">${label}</div><div class="muted text-xs mt-2">${desc}</div></div>
      </a>`).join('');
  }

  C.registerPages({
    'home': () => `
      <div style="max-width:1080px;margin:0 auto;padding:48px 24px">
        <div style="text-align:center;margin-bottom:40px">
          <div style="width:64px;height:64px;border-radius:16px;background:linear-gradient(135deg,var(--amber-400),var(--amber-600));display:inline-grid;place-items:center;color:#1a1206;margin-bottom:16px">${C.icon('link-2', '')}</div>
          <h1 style="font-size:var(--text-4xl);font-weight:800;letter-spacing:-.02em">Chaimir</h1>
          <div style="color:var(--color-primary-text);font-weight:700;font-size:var(--text-xl);letter-spacing:.1em;margin-top:8px">构建 · 验证 · 对抗</div>
          <p class="muted" style="max-width:560px;margin:12px auto 0">区块链教学·实验·竞赛三位一体平台 — 四端高保真交互参考。可按角色端查看功能结构,也可查看登录前页面与全屏沉浸式工作台。</p>
        </div>

        <div class="flex items-center gap-2 mb-3"><span class="badge badge-amber">${C.icon('layout-grid')} 四个角色端</span></div>
        <div class="grid grid-4 mb-4">${['student', 'teacher', 'school-admin', 'platform-admin'].map(roleCard).join('')}</div>

        <div class="flex items-center gap-2 mb-3 mt-4"><span class="badge badge-gray">${C.icon('log-in')} 登录前页面</span></div>
        <div class="grid grid-3 mb-4">${linkRow(C.authPages)}</div>

        <div class="flex items-center gap-2 mb-3 mt-4"><span class="badge badge-purple">${C.icon('maximize')} 沉浸式工作台(全屏)</span></div>
        <div class="grid grid-2 mb-4">${linkRow(C.immersivePages)}</div>

        <div class="card card-pad mt-4">
          <div class="fw-600 mb-3 flex items-center gap-2">${C.icon('palette')} 设计决策落地</div>
          <div class="grid grid-2 text-sm" style="gap:10px 24px">
            <div>· <b>暗夜琥珀(无障碍版)</b>:深色框架 + 白底内容,琥珀 CTA/激活态已达 WCAG AA 对比</div>
            <div>· <b>单页 + 哈希路由</b>:四端共壳,顶栏切端、侧栏切页,120+ 页统一管理</div>
            <div>· <b>设计令牌单一真相源</b>:tokens.css 原始→语义,组件零裸 hex</div>
            <div>· <b>Lucide 图标,全程无 emoji</b>;键盘可达 + 焦点环 + 减少动态</div>
            <div>· <b>模拟关键行为</b>:向导草稿、实时红点、回放时间轴、检查点判分流</div>
            <div>· <b>文案面向用户</b>:错误分层(用户向提示 + 报障编号),不暴露技术细节</div>
          </div>
        </div>
        <p class="muted text-xs" style="text-align:center;margin-top:24px">Chaimir 高保真设计参考 · 示例数据驱动 · 用于设计评审与交互验证</p>
      </div>`,
  });
})();
