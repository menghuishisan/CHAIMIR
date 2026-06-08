/* ============================================================
   pages/student/courses.js — 学生·课程域(参考实现/范式样板)
   ------------------------------------------------------------
   覆盖:我的课程(列表+邀请码加入)、课程详情(章节课时树+多Tab)、
        课时学习页(按内容类型渲染)、作业作答(草稿自动保存)、
        作业结果反馈。对应 M6 教学(学生侧)。
   说明:本文件作为四端页面的"风格与结构范式",其余页面遵循同款:
        registerPages({route: ctx => htmlString}) + 复用 C.* 工具 +
        子页登记 C.parentRoute 以高亮侧栏 + 行为走 C.mounts/内联工具。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项 */
  Object.assign(C.parentRoute, {
    'student/course-detail': 'student/courses',
    'student/lesson': 'student/courses',
    'student/assignment': 'student/courses',
    'student/submission': 'student/courses',
  });

  const coverColors = { amber: 'amber', purple: 'purple', blue: 'blue' };

  /* ---------- 我的课程(列表)---------- */
  function joinModal() {
    C.modal({
      title: '加入课程', size: '',
      body: `<div class="field"><label>课程邀请码</label>
          <div class="input-icon">${C.icon('ticket')}<input class="input" id="invite" placeholder="向任课教师获取,如 BC-3F9K2"></div>
          <div class="help">输入邀请码后将校验课程是否存在、是否已结束</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已加入课程','课程已添加到「我的课程」')">确认加入</button>`,
    });
  }
  C.studentJoin = joinModal;

  function courseCard(c) {
    const cc = coverColors[c.cover] || 'amber';
    const stBadge = { '进行中': 'green', '未开始': 'gray', '已结束': 'blue' }[c.status] || 'gray';
    return `<a class="card card-hover" onclick="Chaimir.navigate('student/course-detail?id=${c.id}')">
      <div style="height:84px;border-radius:var(--radius) var(--radius) 0 0;background:linear-gradient(120deg,var(--${cc}-600),var(--${cc}-700));position:relative">
        <span class="badge badge-${stBadge}" style="position:absolute;top:10px;left:12px">${c.status}</span>
        ${c.progress > 0 && c.progress < 100 ? `<span class="badge badge-amber" style="position:absolute;top:10px;right:12px">已学 ${c.progress}%</span>` : ''}
      </div>
      <div class="card-pad">
        <div class="fw-700" style="font-size:var(--text-md)">${c.name}</div>
        <div class="muted text-sm mt-2">${C.icon('user')} ${c.teacher} · ${c.members} 人选修</div>
        <div class="flex gap-2 mt-3">${C.badge(c.type, 'gray')}${C.badge(c.difficulty, 'amber')}${C.badge(c.credits + ' 学分', 'gray')}</div>
        ${c.progress > 0 ? `<div class="progress mt-3"><span style="width:${c.progress}%"></span></div>` : ''}
      </div></a>`;
  }

  /* ---------- 课程详情 ---------- */
  function lessonIcon(type) { return { video: 'play-circle', sim: 'activity', experiment: 'flask-conical', assignment: 'file-check', text: 'file-text' }[type] || 'circle'; }
  function lessonStatusDot(s) { return { done: 'green', doing: 'amber', todo: 'gray' }[s] || 'gray'; }

  function courseDetail(ctx) {
    const c = m.courses.find(x => x.id == ctx.query.id) || m.courses[0];
    const tab = ctx.query.tab || 'outline';
    const tabs = [['outline', '目录'], ['assignments', '作业'], ['discuss', '讨论'], ['announce', '公告'], ['grade', '成绩']];
    let body = '';
    if (tab === 'outline') {
      body = m.chapters.map(ch => `
        <div class="card mb-3"><div class="card-head"><div class="section-title">${ch.title}</div>
          <span class="muted text-sm">${ch.lessons.filter(l => l.status === 'done').length}/${ch.lessons.length} 已完成</span></div>
          <div style="padding:6px">${ch.lessons.map(l => `
            <a class="side-item" onclick="Chaimir.navigate('student/lesson?c=${c.id}')" style="border-radius:var(--radius-sm)">
              <span class="dot dot-${lessonStatusDot(l.status)}"></span>${C.icon(lessonIcon(l.type))}
              <span style="flex:1">${l.title}</span>
              ${l.dur ? `<span class="muted text-xs">${l.dur}</span>` : ''}
              ${l.type === 'experiment' ? C.badge('代码实验', 'purple') : l.type === 'sim' ? C.badge('仿真', 'teal') : ''}
            </a>`).join('')}</div></div>`).join('');
    } else if (tab === 'assignments') {
      body = `<div class="table-wrap"><table class="table"><thead><tr><th>作业</th><th>截止时间</th><th>状态</th><th></th></tr></thead><tbody>
        <tr><td class="fw-600">第一章测验</td><td>2026-05-20 23:59</td><td>${C.badge('已批改 · 92', 'green')}</td><td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/submission')">查看</button></td></tr>
        <tr><td class="fw-600">智能合约安全作业</td><td class="mono">2026-06-08 23:59</td><td>${C.badge('待提交', 'amber')}</td><td class="row-actions"><button class="btn btn-primary btn-sm" onclick="Chaimir.navigate('student/assignment')">去作答</button></td></tr>
      </tbody></table></div>`;
    } else if (tab === 'discuss') {
      body = `<div class="card card-pad mb-3"><textarea class="textarea" placeholder="发表你的看法或提问…"></textarea>
        <div class="flex justify-between mt-2"><span class="muted text-xs">支持 Markdown</span><button class="btn btn-primary btn-sm" onclick="Chaimir.demo()">发布</button></div></div>
        ${[['李明远(教师)', '本周重点是 Checks-Effects-Interactions 模式,做实验前先理解重入的本质。', true, 24],
           ['赵雨桐', '请问 3.3 实验里 reset 之后状态会保留吗?', false, 3]].map(([a, t, pin, like]) => `
          <div class="card card-pad mb-2"><div class="flex justify-between"><div class="fw-600 text-sm">${a} ${pin ? C.badge('置顶', 'amber') : ''}</div><span class="muted text-xs">2 小时前</span></div>
            <div class="text-sm mt-2">${t}</div><div class="muted text-xs mt-2 flex gap-3"><span>${C.icon('thumbs-up')} ${like}</span><span style="cursor:pointer">回复</span></div></div>`).join('')}`;
    } else if (tab === 'announce') {
      body = `<div class="card card-pad mb-2"><div class="flex justify-between"><div class="fw-600">${C.badge('置顶', 'amber')} 期末项目说明已发布</div><span class="muted text-xs">3 天前</span></div>
        <div class="text-sm mt-2 muted">请各位同学查看课程附件中的期末项目要求,组队截止 6 月 20 日。</div></div>`;
    } else {
      body = `<div class="grid grid-3 mb-4">${C.stat('check-circle-2', '88', '当前总评', 'green')}${C.stat('clipboard-list', '4/6', '作业完成', 'amber')}${C.stat('flask-conical', '2/3', '实验完成', 'blue')}</div>
        <div class="callout info">${C.icon('info')}<div>总评成绩由作业(40%)、实验(40%)、考试(20%)加权;最终成绩以教师报送、学校审核锁定为准。如有异议可发起成绩申诉。</div></div>`;
    }
    return `${C.crumb([{ label: '我的课程', to: 'student/courses' }, { label: '课程详情' }])}
      <div class="card card-pad mb-4" style="background:linear-gradient(120deg,var(--slate-900),var(--slate-800));color:#fff;border:none">
        <div class="flex justify-between wrap gap-3">
          <div><div class="flex gap-2 mb-2">${C.badge(c.status, 'amber')}${C.badge(c.type, 'gray')}</div>
            <div style="font-size:var(--text-2xl);font-weight:700">${c.name}</div>
            <div style="color:var(--color-dark-text-sub);margin-top:6px">${C.icon('user')} ${c.teacher} · ${c.semester} · ${c.credits} 学分</div></div>
          <div style="text-align:right"><div style="font-size:var(--text-3xl);font-weight:700;color:var(--amber-400)">${c.progress}%</div><div style="color:var(--color-dark-text-sub);font-size:var(--text-sm)">学习进度</div></div>
        </div></div>
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${k === tab ? 'active' : ''}" onclick="Chaimir.navigate('student/course-detail?id=${c.id}&tab=${k}')">${l}</a>`).join('')}</div>
      ${body}`;
  }

  /* ---------- 课时学习页 ---------- */
  function lesson(ctx) {
    return `${C.crumb([{ label: '我的课程', to: 'student/courses' }, { label: '课程详情', to: 'student/course-detail?id=' + (ctx.query.c || 1) }, { label: '课时学习' }])}
      ${C.head('3.1 PoW 与 PBFT', '第三章 · 共识算法与重入漏洞')}
      <div class="grid" style="grid-template-columns:1fr 300px">
        <div class="card" style="overflow:hidden">
          <div style="aspect-ratio:16/9;background:var(--slate-900);display:grid;place-items:center;color:var(--color-dark-text-sub)">
            <div style="text-align:center">${C.icon('play-circle')}<div class="mt-2 text-sm">课程视频(原型占位)· 24:10</div></div></div>
          <div class="card-pad"><div class="section-title mb-2">本节要点</div>
            <p class="text-sm muted" style="line-height:var(--leading-relaxed)">对比工作量证明(PoW)与实用拜占庭容错(PBFT)在安全假设、性能与适用场景上的差异。完成视频后,进入 3.2 仿真亲手注入拜占庭节点观察共识。</p></div>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-2">本节进度</div>
            <div class="autosave saved">${C.icon('check-circle-2')} 观看至 18:02,已记录</div>
            <button class="btn btn-primary btn-block mt-3" onclick="Chaimir.navigate('immersive/sim')">${C.icon('activity')} 进入 3.2 共识仿真</button>
            <button class="btn btn-outline btn-block mt-2" onclick="Chaimir.navigate('immersive/exp-ide')">${C.icon('code-2')} 进入 3.3 代码实验</button></div>
          <div class="card card-pad"><div class="section-title mb-2">课时列表</div>
            ${m.chapters[1].lessons.map(l => `<div class="side-item"><span class="dot dot-${lessonStatusDot(l.status)}"></span><span style="flex:1">${l.title}</span></div>`).join('')}</div>
        </div></div>`;
  }

  /* ---------- 作业作答(草稿自动保存)---------- */
  C.asgSave = function (manual) {
    const ind = document.getElementById('autosave-ind'); if (!ind) return;
    ind.className = 'autosave saving'; ind.innerHTML = C.icon('loader') + ' 正在保存…'; C.refreshIcons();
    setTimeout(() => { ind.className = 'autosave saved'; ind.innerHTML = C.icon('check-circle-2') + ' 草稿已保存到服务端'; C.refreshIcons(); if (manual) C.toast('success', '草稿已保存', '换设备或刷新都不会丢失'); }, 600);
  };
  C.mounts['student/assignment'] = function () {
    C._asgTimer && clearInterval(C._asgTimer);
    C._asgTimer = setInterval(() => { if (location.hash.includes('student/assignment')) C.asgSave(false); else clearInterval(C._asgTimer); }, 60000);
  };
  function assignment() {
    return `${C.crumb([{ label: '我的课程', to: 'student/courses' }, { label: '课程详情', to: 'student/course-detail?id=1&tab=assignments' }, { label: '作业作答' }])}
      <div class="content-head"><div><div class="page-sub">区块链原理与智能合约 · 截止 2026-06-08 23:59</div><h1 class="page-title">智能合约安全作业</h1></div>
        <div class="content-actions"><span class="autosave saved" id="autosave-ind">${C.icon('check-circle-2')} 草稿已保存到服务端</span>
          <button class="btn btn-outline" onclick="Chaimir.asgSave(true)">保存草稿</button>
          <button class="btn btn-primary" onclick="Chaimir.studentSubmitAsg()">提交作业</button></div></div>
      <div class="callout info mb-4">${C.icon('info')}<div>系统每 60 秒自动保存草稿;本作业允许提交 <b>3</b> 次,当前第 1 次。超过截止时间提交将按迟交规则处理。</div></div>
      <div class="card card-pad mb-3"><div class="flex justify-between mb-2"><div class="fw-700">第 1 题 · 单选(20 分)</div>${C.badge('自动判题', 'blue')}</div>
        <p class="text-sm mb-3">以下哪种模式能从根本上防止重入攻击?</p>
        ${['先更新状态再外部调用(Checks-Effects-Interactions)', '提高 gas limit', '使用 tx.origin 鉴权', '增加合约余额'].map((o, i) => `<label class="radio" style="display:flex;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:8px"><input type="radio" name="q1"> ${o}</label>`).join('')}</div>
      <div class="card card-pad mb-3"><div class="flex justify-between mb-2"><div class="fw-700">第 2 题 · 编程题(50 分)</div>${C.badge('沙箱判题', 'purple')}</div>
        <p class="text-sm mb-3">修复下方合约的重入漏洞,使其通过全部测试用例。</p>
        <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:14px;font-family:var(--font-mono);font-size:var(--text-xs);color:#cbd5e1;white-space:pre;overflow:auto;line-height:1.7">function withdraw() public {
    uint amt = balances[msg.sender];
    (bool ok,) = msg.sender.call{value: amt}("");   <span style="color:var(--red-400,#fca5a5)">// 漏洞:状态更新在外部调用之后</span>
    balances[msg.sender] = 0;
}</div>
        <button class="btn btn-outline btn-sm mt-3" onclick="Chaimir.navigate('immersive/exp-ide')">${C.icon('code-2')} 在沙箱中作答</button></div>
      <div class="card card-pad"><div class="flex justify-between mb-2"><div class="fw-700">第 3 题 · 简答(30 分)</div>${C.badge('教师批改', 'amber')}</div>
        <p class="text-sm mb-3">简述 Checks-Effects-Interactions 模式的执行顺序及其防御原理。</p>
        <textarea class="textarea" placeholder="在此作答…" oninput="Chaimir.asgDirty&&Chaimir.asgDirty()"></textarea></div>`;
  }
  C.studentSubmitAsg = async function () {
    if (await C.confirm({ title: '提交作业', message: '提交后将进入判题,本次为第 1/3 次提交。确认提交?', confirmText: '确认提交' }))
    { C.toast('success', '作业已提交', '客观题与编程题正在自动判题…'); setTimeout(() => C.navigate('student/submission'), 800); }
  };

  /* ---------- 作业结果 ---------- */
  function submission() {
    return `${C.crumb([{ label: '我的课程', to: 'student/courses' }, { label: '作业结果' }])}
      ${C.head('智能合约安全作业 · 第 1 次提交', '提交于 2026-06-07 16:20')}
      <div class="grid grid-3 mb-4">${C.stat('award', '92', '本次得分', 'green')}${C.stat('check-circle-2', '70/70', '自动判题', 'blue')}${C.stat('clock', '22/30', '教师评分', 'amber')}</div>
      <div class="card mb-3"><div class="card-head"><div class="section-title">第 2 题 · 编程题 · 判题详情</div>${C.badge('通过', 'green')}</div>
        <div class="card-pad">${[['用例 1 · 正常提款', true], ['用例 2 · 重入攻击被拦截', true], ['用例 3 · 余额一致性', true]].map(([t, ok]) => `
          <div class="flex items-center gap-2 text-sm" style="padding:8px 0;border-bottom:1px solid var(--color-border)">${C.statusDot(ok ? 'green' : 'red', t)}<span style="margin-left:auto" class="badge badge-${ok ? 'green' : 'red'}">${ok ? '通过' : '失败'}</span></div>`).join('')}</div></div>
      <div class="card card-pad"><div class="section-title mb-2">教师评语</div>
        <p class="text-sm muted">思路正确,CEI 模式应用到位。简答题对"为什么先更新状态能阻断递归"的解释可以再深入一点。</p>
        <div class="mt-3 flex gap-2"><button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/assignment')">重新作答(剩 2 次)</button>
        <button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('student/grades')">查看课程成绩</button></div></div>`;
  }

  C.registerPages({
    'student/courses': () => `${C.head('我的课程', '学习', `<button class="btn btn-primary" onclick="Chaimir.studentJoin()">${C.icon('plus')} 加入课程</button>`)}
      <div class="grid grid-3">${m.courses.map(courseCard).join('')}</div>`,
    'student/course-detail': courseDetail,
    'student/lesson': lesson,
    'student/assignment': assignment,
    'student/submission': submission,
  });
})();
