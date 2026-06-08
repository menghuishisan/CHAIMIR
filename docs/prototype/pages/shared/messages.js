/* ============================================================
   pages/shared/messages.js — 通知中心 + 系统公告(四端共用)
   ------------------------------------------------------------
   职责:通知与公告从侧栏移出后,作为顶栏铃铛的"查看全部"落点。
        同一套实现注册到四个角色端,保证体验一致、覆盖完整。
        对应 M10 通知与实时推送(站内信 / 系统公告 / 偏好)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;
  const roles = ['student', 'teacher', 'school-admin', 'platform-admin'];

  const typeColor = { '作业': 'blue', '竞赛': 'purple', '成绩': 'green', '系统': 'gray', '审核': 'amber' };

  /* ---------- 通知中心 ---------- */
  function notifications(ctx) {
    const role = ctx.route.split('/')[0];
    const tab = ctx.query.tab || 'all';
    const all = m.notifications;
    const unread = all.filter(n => !n.read).length;
    let list = all;
    if (tab === 'unread') list = all.filter(n => !n.read);
    if (tab === 'read') list = all.filter(n => n.read);
    const tabs = [['all', '全部', all.length], ['unread', '未读', unread], ['read', '已读', all.length - unread]];
    return C.head('通知中心', '站内信 · 实时到达',
      `<button class="btn btn-outline" onclick="Chaimir.toast('success','已全部标记已读','')">${C.icon('check-check')} 全部已读</button>
       <button class="btn btn-ghost" onclick="Chaimir.navigate('${role}/notif-prefs')">${C.icon('settings-2')} 通知偏好</button>`) + `
      <div class="callout info mb-4">${C.icon('radio')}<div>新通知通过实时通道即时到达并在顶栏铃铛提醒;漏推时以站内信落库为准,双通道保证不丢。</div></div>
      <div class="tabs">${tabs.map(([k, l, n]) => `<a class="tab ${k === tab ? 'active' : ''}" onclick="Chaimir.navigate('${role}/notifications?tab=${k}')">${l}${n ? ` (${n})` : ''}</a>`).join('')}</div>
      ${list.length ? `<div class="card">${list.map((n, i) => `
        <a class="flex gap-3" style="align-items:flex-start;padding:14px 16px;border-bottom:${i < list.length - 1 ? '1px solid var(--color-border)' : 'none'};cursor:pointer;${n.read ? '' : 'background:var(--amber-50)'}"
           onclick="Chaimir.toast('info','已读','原型演示:跳转到「${C.esc(n.type)}」相关页')">
          <span class="stat-icon" style="width:36px;height:36px;background:var(--${typeColor[n.type] || 'gray'}-100);color:var(--${typeColor[n.type] || 'gray'}-700);flex-shrink:0">${C.icon({ '作业': 'file-check', '竞赛': 'trophy', '成绩': 'award', '系统': 'info', '审核': 'clipboard-check' }[n.type] || 'bell')}</span>
          <div style="flex:1;min-width:0"><div class="flex items-center gap-2">${n.read ? '' : '<span class="dot dot-amber"></span>'}<span class="${n.read ? 'muted' : 'fw-600'} text-sm">${C.esc(n.title)}</span></div>
            <div class="muted text-xs" style="margin-top:3px">${C.badge(n.type, typeColor[n.type] || 'gray')} ${C.esc(n.time)}</div></div>
          ${n.read ? '' : `<button class="btn btn-ghost btn-sm" onclick="event.stopPropagation();Chaimir.toast('success','已标记已读')">标记已读</button>`}
        </a>`).join('')}</div>` : C.empty({ icon: 'bell-off', title: '暂无通知', desc: '这里会显示作业、竞赛、成绩与系统消息。' })}`;
  }

  /* ---------- 系统公告 ---------- */
  function announcements(ctx) {
    const list = [
      { title: '平台 6 月安全更新与维护通知', scope: '全平台', time: '2026-06-06', pin: true, read: false, body: '平台将于 2026-06-08 02:00-04:00 进行安全更新,期间沙箱与判题服务短暂不可用,请合理安排实验与竞赛时间。' },
      { title: '关于启用新版 PBFT 仿真场景的说明', scope: '本校', time: '2026-06-02', pin: false, read: false, body: '题库新增「PBFT 三阶段共识」高保真仿真场景,支持注入拜占庭节点,欢迎在课程与实验中引用。' },
      { title: '期末竞赛周日程公布', scope: '本校', time: '2026-05-28', pin: false, read: true, body: '期末「链上夺旗」对抗赛将于第 16 周举行,报名通道现已开放。' },
    ];
    return C.head('系统公告', '平台 / 学校通知') + `
      ${list.map(a => `<div class="card card-pad mb-3" style="${a.read ? '' : 'border-left:3px solid var(--color-accent)'}">
        <div class="flex justify-between items-center wrap gap-2">
          <div class="flex items-center gap-2">${a.pin ? C.badge('置顶', 'amber', 'pin') : ''}${C.badge(a.scope, a.scope === '全平台' ? 'purple' : 'blue')}
            <span class="fw-700">${C.esc(a.title)}</span>${a.read ? '' : '<span class="dot dot-amber"></span>'}</div>
          <span class="muted text-xs">${C.esc(a.time)}</span></div>
        <p class="muted text-sm mt-2" style="line-height:var(--leading-relaxed)">${C.esc(a.body)}</p>
      </div>`).join('')}`;
  }

  /* ---------- 通知偏好 ---------- */
  function notifPrefs(ctx) {
    const role = ctx.route.split('/')[0];
    const prefs = [
      { t: '作业与截止提醒', on: true, force: false }, { t: '竞赛与排名变动', on: true, force: false },
      { t: '讨论区回复', on: false, force: false }, { t: '成绩与审核结果', on: true, force: true },
      { t: '系统与安全通知', on: true, force: true },
    ];
    return C.crumb([{ label: '通知中心', to: role + '/notifications' }, { label: '通知偏好' }]) +
      C.head('通知偏好', '选择希望接收的通知类型') + `
      <div class="card" style="max-width:560px">${prefs.map((p, i) => `
        <div class="flex justify-between items-center" style="padding:14px 18px;border-bottom:${i < prefs.length - 1 ? '1px solid var(--color-border)' : 'none'}">
          <div><div class="fw-600 text-sm">${p.t}</div>${p.force ? '<div class="muted text-xs mt-1">必要通知,不可关闭</div>' : ''}</div>
          <label class="switch"><input type="checkbox" ${p.on ? 'checked' : ''} ${p.force ? 'disabled' : ''} onchange="Chaimir.toast('success','偏好已更新')"><span class="track"></span></label>
        </div>`).join('')}</div>`;
  }

  const reg = {};
  roles.forEach(r => {
    reg[r + '/notifications'] = notifications;
    reg[r + '/announcements'] = announcements;
    reg[r + '/notif-prefs'] = notifPrefs;
    C.parentRoute[r + '/notif-prefs'] = r + '/notifications';
  });
  C.registerPages(reg);
})();
