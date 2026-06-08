/* ============================================================
   pages/student/account.js — 学生·账户域(成绩 / 预警 / 申诉 / 消息 / 个人)
   ------------------------------------------------------------
   覆盖路由:
     · student/grades         我的成绩(学期明细表 + 学期/累计 GPA)
     · student/warnings        学业预警(挂科/低 GPA,未确认高亮)
     · student/appeals         成绩申诉(子页:提交表单 + 进度状态机)
     · student/transcripts     成绩单(子页:生成/查看/下载 PDF,鉴权防直链)
     · student/notifications   站内信(已读/未读筛选 + 标记已读)
     · student/announcements   系统公告(置顶在前,查看即已读)
     · student/profile         个人中心(只读学籍 + 改密/换绑 + 偏好/会话)
   风格:沿用 pages/student/courses.js 范式;文案面向用户、无 emoji、
        图标用 lucide;成绩可视化用进度条/表格并配文字数值(无障碍)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项 */
  Object.assign(C.parentRoute, {
    'student/appeals': 'student/grades',
    'student/transcripts': 'student/grades',
  });

  /* ============================================================
     成绩域共享假数据(学期课程成绩明细)
     ------------------------------------------------------------
     成绩明细实时取自课程(教师报送 + 学校审核锁定);此处为展示。
     ============================================================ */
  const gradeTerms = [
    { term: '2025-2026 春(本学期)', current: true, courses: [
      { name: '区块链原理与智能合约开发', credits: 3, score: 88, grade: 'B+', gp: 3.7, status: '已锁定' },
      { name: 'DeFi 协议开发与套利审计', credits: 2, score: null, grade: '—', gp: null, status: '进行中' },
    ] },
    { term: '2024-2025 秋', current: false, courses: [
      { name: '密码学基础与共识算法', credits: 3, score: 92, grade: 'A-', gp: 3.9, status: '已锁定' },
      { name: '分布式系统导论', credits: 3, score: 79, grade: 'C+', gp: 2.7, status: '已锁定' },
      { name: '程序设计与数据结构', credits: 4, score: 85, grade: 'B', gp: 3.3, status: '已锁定' },
    ] },
  ];

  /* 计算 GPA(学分加权;仅计入已出分课程) */
  function calcGpa(courses) {
    let cr = 0, sum = 0;
    courses.forEach(c => { if (c.gp != null) { cr += c.credits; sum += c.gp * c.credits; } });
    return cr ? { gpa: (sum / cr).toFixed(2), credits: cr } : { gpa: '—', credits: 0 };
  }
  function gradeBadgeColor(g) {
    if (g === '—') return 'gray';
    if (g.charAt(0) === 'A') return 'green';
    if (g.charAt(0) === 'B') return 'blue';
    if (g.charAt(0) === 'C') return 'amber';
    return 'red';
  }

  /* ============================================================
     一、我的成绩(GPA)
     ============================================================ */
  function grades() {
    const termGpa = calcGpa(gradeTerms[0].courses);
    const allCourses = gradeTerms.reduce((a, t) => a.concat(t.courses), []);
    const cumGpa = calcGpa(allCourses);
    return `${C.head('我的成绩', '成绩', `<button class="btn btn-outline" onclick="Chaimir.navigate('student/transcripts')">${C.icon('file-text')} 成绩单</button>`)}
      <div class="grid grid-3 mb-4">
        ${C.stat('trending-up', termGpa.gpa, '本学期 GPA', 'amber')}
        ${C.stat('award', cumGpa.gpa, '累计 GPA', 'green')}
        ${C.stat('book-check', cumGpa.credits, '已获学分', 'blue')}
      </div>
      <div class="callout info mb-4">${C.icon('info')}<div>成绩明细实时取自各课程(教师报送、学校审核后锁定)。<b>竞赛成绩不计入 GPA</b>。如对某门课成绩有异议,可在该课程行发起申诉。</div></div>
      ${gradeTerms.map(t => {
        const g = calcGpa(t.courses);
        return `<div class="card mb-4">
          <div class="card-head">
            <div class="section-title flex items-center gap-2">${t.term} ${t.current ? C.badge('当前学期', 'amber') : ''}</div>
            <div class="flex items-center gap-3 text-sm"><span class="muted">学期 GPA</span><span class="fw-700" style="font-size:var(--text-lg);color:var(--amber-700)">${g.gpa}</span></div>
          </div>
          <div class="table-wrap" style="border:none"><table class="table">
            <thead><tr><th>课程</th><th>学分</th><th>成绩</th><th>等级</th><th>绩点</th><th>状态</th><th></th></tr></thead>
            <tbody>${t.courses.map(c => `<tr>
              <td class="fw-600">${C.esc(c.name)}</td>
              <td class="mono">${c.credits}</td>
              <td class="mono">${c.score != null ? `<span class="fw-700">${c.score}</span>` : '<span class="muted">—</span>'}</td>
              <td>${C.badge(c.grade, gradeBadgeColor(c.grade))}</td>
              <td class="mono">${c.gp != null ? c.gp.toFixed(1) : '<span class="muted">—</span>'}</td>
              <td>${c.status === '已锁定' ? C.statusDot('green', '已锁定') : C.statusDot('amber', '进行中')}</td>
              <td class="row-actions">
                <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/transcripts')" title="成绩单">${C.icon('file-text')}</button>
                ${c.status === '已锁定' ? `<button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('student/appeals?course=${encodeURIComponent(c.name)}')">${C.icon('gavel')} 申诉</button>` : ''}
              </td>
            </tr>`).join('')}</tbody>
          </table></div>
        </div>`;
      }).join('')}`;
  }

  /* ============================================================
     二、学业预警
     ============================================================ */
  /* 本人预警:挂科 / 低 GPA;未确认者高亮 + "我已知悉"按钮。
     确认态用模块变量记忆,便于演示"知悉后高亮消失"。 */
  C._warnAck = C._warnAck || {};
  const warnings = [
    { id: 'w1', level: 'danger', type: '课程不及格', title: '《分布式系统导论》补考成绩 59,未达及格线', desc: '该课程为学位必修,请尽快联系任课教师了解重修安排。', term: '2024-2025 秋' },
    { id: 'w2', level: 'warn', type: '学业进度', title: '本学期已选课程进度落后,1 门课程未达里程碑', desc: '《DeFi 协议开发与套利审计》当前进度 0%,建议尽快进入学习。', term: '2025-2026 春' },
    { id: 'w3', level: 'warn', type: 'GPA 偏低', title: '上学期 GPA 接近预警线(2.7)', desc: '建议合理规划本学期选课与复习,避免触发学业警示。', term: '2024-2025 秋' },
  ];
  function warnLevelMeta(l) {
    return l === 'danger'
      ? { badge: 'red', ic: 'alert-octagon', label: '严重', color: 'red' }
      : { badge: 'amber', ic: 'alert-triangle', label: '提醒', color: 'amber' };
  }
  function studentWarnings() {
    const unread = warnings.filter(w => !C._warnAck[w.id]).length;
    return `${C.head('学业预警', '成绩')}
      <div class="callout warn mb-4">${C.icon('alert-triangle')}<div>学业预警旨在帮助你及时关注学习风险。请阅读后点击「我已知悉」,你的辅导员可看到确认状态。预警不影响成绩,仅作提示。</div></div>
      <div class="flex items-center gap-2 mb-3 text-sm muted">${C.icon('inbox')} 共 ${warnings.length} 条,其中 <span class="fw-700" style="color:var(--red-700)">${unread}</span> 条待确认</div>
      ${warnings.map(w => {
        const meta = warnLevelMeta(w.level);
        const acked = !!C._warnAck[w.id];
        return `<div class="card card-pad mb-3" style="${acked ? '' : 'border-left:3px solid var(--' + meta.color + '-600)'}">
          <div class="flex justify-between items-center wrap gap-2">
            <div class="flex items-center gap-2">${C.badge(meta.label, meta.badge, meta.ic)}${C.badge(w.type, 'gray')}<span class="muted text-xs">${w.term}</span>
              ${acked ? C.badge('已知悉', 'green', 'check') : C.badge('待确认', 'red', 'circle-alert')}</div>
            ${acked
              ? `<span class="autosave saved">${C.icon('check-circle-2')} 你已确认知悉</span>`
              : `<button class="btn btn-primary btn-sm" onclick="Chaimir.studentAckWarn('${w.id}')">${C.icon('check')} 我已知悉</button>`}
          </div>
          <div class="fw-600 mt-3">${C.esc(w.title)}</div>
          <p class="muted text-sm mt-2" style="line-height:var(--leading-relaxed)">${C.esc(w.desc)}</p>
        </div>`;
      }).join('')}`;
  }
  /* 确认预警:记忆 + 反馈 + 重渲染(高亮消失) */
  C.studentAckWarn = function (id) {
    C._warnAck[id] = true;
    C.toast('success', '已确认知悉', '辅导员将看到你的确认状态');
    C.rerender();
  };

  /* ============================================================
     三、成绩申诉(子页)
     ------------------------------------------------------------
     提交表单(选课程 + 理由,30 天时效)+ 进度状态机展示。
     ============================================================ */
  /* 历史申诉(展示状态机:待处理→受理改分→已完成/已驳回) */
  const appealRecords = [
    { id: 'A-2026-018', course: '分布式系统导论', reason: '期末大题第 3 问评分疑似漏判,申请复核。', state: '受理改分', time: '2026-06-03', reply: '已受理,正在与任课教师复核,预计 3 个工作日内反馈。' },
    { id: 'A-2026-006', course: '程序设计与数据结构', reason: '平时分录入与实际提交次数不符。', state: '已完成', time: '2026-05-20', reply: '复核属实,平时分由 80 调整为 88,总评已更新。' },
    { id: 'A-2026-002', course: '密码学基础与共识算法', reason: '希望复核简答题给分。', state: '已驳回', time: '2026-05-12', reply: '经复核评分无误,维持原成绩。如仍有疑问可联系学院教务。' },
  ];
  /* 申诉状态机阶段(用于进度条可视化) */
  const appealStages = ['提交申诉', '学校受理', '复核改分', '完成'];
  function appealStageIdx(state) {
    return { '待处理': 1, '受理改分': 2, '已完成': 3, '已驳回': 3 }[state] || 0;
  }
  function appealStateBadge(state) {
    return { '待处理': 'amber', '受理改分': 'blue', '已完成': 'green', '已驳回': 'red' }[state] || 'gray';
  }
  function appeals(ctx) {
    const presetCourse = ctx.query.course ? decodeURIComponent(ctx.query.course) : '';
    /* 可申诉课程:取已锁定成绩的课程 */
    const lockedCourses = gradeTerms.reduce((a, t) => a.concat(t.courses.filter(c => c.status === '已锁定')), []);
    return `${C.crumb([{ label: '我的成绩', to: 'student/grades' }, { label: '成绩申诉' }])}
      ${C.head('成绩申诉', '成绩')}
      <div class="callout warn mb-4">${C.icon('clock')}<div>申诉须在成绩锁定后 <b>30 天内</b> 提交。每门课程同一学期限申诉一次,请填写具体、可核查的理由,便于学校与教师复核。</div></div>
      <div class="grid" style="grid-template-columns:380px 1fr">
        <div>
          <div class="card card-pad">
            <div class="section-title mb-3">发起新申诉</div>
            <div class="field"><label>申诉课程 <span class="req">*</span></label>
              <select class="select" id="ap-course">
                <option value="">请选择课程</option>
                ${lockedCourses.map(c => `<option value="${C.esc(c.name)}" ${presetCourse === c.name ? 'selected' : ''}>${C.esc(c.name)} · 当前 ${c.score} 分</option>`).join('')}
              </select>
            </div>
            <div class="field"><label>申诉类型 <span class="req">*</span></label>
              <select class="select" id="ap-type">
                <option>评分疑似漏判/错判</option>
                <option>平时分/作业分录入有误</option>
                <option>缺考/缓考记录有误</option>
                <option>其他(请在理由中说明)</option>
              </select>
            </div>
            <div class="field"><label>申诉理由 <span class="req">*</span></label>
              <textarea class="textarea" id="ap-reason" placeholder="请具体说明:涉及哪道题/哪部分成绩、你认为的问题、佐证(如提交记录、考试作答)…"></textarea>
              <div class="help">理由越具体,复核越快。提交后将进入「待处理」状态。</div>
            </div>
            <div class="field" style="margin-bottom:0"><label>佐证材料(可选)</label>
              <button class="btn btn-outline btn-block" onclick="Chaimir.demo('选择文件上传')">${C.icon('paperclip')} 上传截图/文件</button>
              <div class="help">支持图片/PDF,单个不超过 10 MB;下载经鉴权,不会暴露直链。</div>
            </div>
            <button class="btn btn-primary btn-block mt-4" onclick="Chaimir.studentSubmitAppeal()">${C.icon('send')} 提交申诉</button>
          </div>
        </div>
        <div>
          <div class="section-title mb-3">我的申诉进度</div>
          ${appealRecords.map(a => {
            const idx = appealStageIdx(a.state);
            const rejected = a.state === '已驳回';
            return `<div class="card card-pad mb-3">
              <div class="flex justify-between items-center wrap gap-2 mb-3">
                <div class="flex items-center gap-2"><span class="mono text-xs muted">${a.id}</span><span class="fw-600">${C.esc(a.course)}</span></div>
                ${C.badge(a.state, appealStateBadge(a.state))}
              </div>
              <p class="text-sm muted mb-3">申诉理由:${C.esc(a.reason)}</p>
              <div class="steps" style="margin-bottom:14px">${appealStages.map((s, i) => {
                const done = i < idx;
                const active = i === idx;
                const failHere = rejected && i === idx;
                const cls = failHere ? '' : done ? 'done' : active ? 'active' : '';
                return `<div class="step ${cls}">
                  <span class="dot-n" style="${failHere ? 'background:var(--red-600);color:#fff' : ''}">${failHere ? C.icon('x') : done ? C.icon('check') : i + 1}</span>
                  <span class="step-label">${i === 3 && rejected ? '已驳回' : s}</span>
                  ${i < appealStages.length - 1 ? '<span class="line"></span>' : ''}
                </div>`;
              }).join('')}</div>
              <div class="callout ${rejected ? 'danger' : a.state === '已完成' ? 'success' : 'info'}">${C.icon(rejected ? 'x-circle' : a.state === '已完成' ? 'check-circle-2' : 'message-square')}
                <div><span class="muted text-xs">学校反馈 · ${a.time}</span><div class="text-sm mt-2">${C.esc(a.reply)}</div></div></div>
            </div>`;
          }).join('')}
        </div>
      </div>`;
  }
  C.studentSubmitAppeal = async function () {
    const courseEl = document.getElementById('ap-course');
    const reasonEl = document.getElementById('ap-reason');
    const course = courseEl ? courseEl.value : '';
    const reason = reasonEl ? reasonEl.value.trim() : '';
    if (!course) { C.toast('error', '请选择申诉课程', '需指定要申诉的课程'); return; }
    if (reason.length < 10) { C.toast('error', '申诉理由过于简略', '请填写至少 10 个字的具体理由,便于复核'); return; }
    if (await C.confirm({ title: '提交成绩申诉', message: '提交后将进入学校受理流程,期间成绩保持锁定。确认提交?', confirmText: '确认提交' })) {
      C.toast('success', '申诉已提交', '已进入「待处理」,学校受理后将在站内信通知你');
    }
  };

  /* ============================================================
     四、成绩单(子页)
     ============================================================ */
  function transcripts() {
    const allCourses = gradeTerms.reduce((a, t) => a.concat(t.courses.filter(c => c.gp != null)), []);
    const cum = calcGpa(allCourses);
    return `${C.crumb([{ label: '我的成绩', to: 'student/grades' }, { label: '成绩单' }])}
      ${C.head('成绩单', '成绩', `<button class="btn btn-primary" onclick="Chaimir.studentDownloadTranscript()">${C.icon('download')} 下载 PDF</button>`)}
      <div class="callout info mb-4">${C.icon('shield')}<div>成绩单下载经身份鉴权生成临时链接,<b>不暴露文件直链</b>,链接短时有效。官方盖章版请通过学校教务办理。</div></div>
      <div class="grid" style="grid-template-columns:1fr 300px">
        <div class="card card-pad">
          <div class="flex justify-between items-center mb-4" style="padding-bottom:14px;border-bottom:2px solid var(--color-border-strong)">
            <div><div class="fw-700" style="font-size:var(--text-lg)">学生成绩单(非正式预览)</div>
              <div class="muted text-xs mt-2">示例大学 · 计算机学院</div></div>
            <div style="display:grid;place-items:center;width:48px;height:48px;border-radius:var(--radius);background:var(--amber-100);color:var(--amber-800)">${C.icon('graduation-cap')}</div>
          </div>
          <dl class="dl mb-4">
            <dt>姓名</dt><dd>${C.esc(m.me.name)}</dd>
            <dt>学号</dt><dd class="mono">${C.esc(m.me.no)}</dd>
            <dt>班级</dt><dd>${C.esc(m.me.class)}</dd>
            <dt>累计 GPA</dt><dd class="fw-700" style="color:var(--amber-700)">${cum.gpa} · 已获 ${cum.credits} 学分</dd>
          </dl>
          ${gradeTerms.map(t => `
            <div class="fw-600 text-sm mb-2" style="color:var(--color-text-sub)">${t.term}</div>
            <table class="table mb-4" style="border:1px solid var(--color-border)">
              <thead><tr><th>课程</th><th>学分</th><th>成绩</th><th>等级</th><th>绩点</th></tr></thead>
              <tbody>${t.courses.map(c => `<tr><td>${C.esc(c.name)}</td><td class="mono">${c.credits}</td><td class="mono">${c.score != null ? c.score : '—'}</td><td>${c.grade}</td><td class="mono">${c.gp != null ? c.gp.toFixed(1) : '—'}</td></tr>`).join('')}</tbody>
            </table>`).join('')}
          <div class="muted text-xs">本预览仅供本人查看,水印与防伪信息将在正式 PDF 中生成。竞赛与课外成绩不计入本成绩单。</div>
        </div>
        <div>
          <div class="card card-pad mb-3">
            <div class="section-title mb-3">导出选项</div>
            <label class="checkbox mb-2" style="display:flex"><input type="checkbox" checked> 含课程明细</label>
            <label class="checkbox mb-2" style="display:flex"><input type="checkbox" checked> 含 GPA 与学分统计</label>
            <label class="checkbox mb-3" style="display:flex"><input type="checkbox"> 仅含本学期</label>
            <button class="btn btn-primary btn-block" onclick="Chaimir.studentDownloadTranscript()">${C.icon('download')} 生成并下载</button>
            <button class="btn btn-outline btn-block mt-2" onclick="Chaimir.demo('已发送到邮箱')">${C.icon('mail')} 发送到我的邮箱</button>
          </div>
          <div class="card card-pad">
            <div class="section-title mb-2">下载记录</div>
            ${[['2026-06-06 14:02', '完整成绩单'], ['2026-05-21 09:30', '本学期成绩单']].map(([t, k]) => `
              <div class="flex items-center gap-2 text-sm" style="padding:8px 0;border-bottom:1px solid var(--color-border)">${C.icon('file-text')}<span style="flex:1">${k}</span><span class="muted text-xs">${t}</span></div>`).join('')}
            <div class="muted text-xs mt-3">每次下载均记入审计,可追溯。</div>
          </div>
        </div>
      </div>`;
  }
  C.studentDownloadTranscript = function () {
    C.toast('info', '正在生成成绩单', '正在通过鉴权生成临时下载链接…');
    setTimeout(() => C.toast('success', '成绩单已就绪', '已开始下载(原型为模拟);链接短时有效'), 900);
  };

  /* ============================================================
     五、站内信
     ------------------------------------------------------------
     已读/未读 Tab + 未读高亮(圆点)+ 单条/全部标记已读 + 跳 link。
     用模块变量记忆已读态,体现"实时到达"。
     ============================================================ */
  C._notifRead = C._notifRead || {};   /* id → 已读(覆盖 mock 的 read 字段) */
  function notifRead(n) { return C._notifRead[n.id] != null ? C._notifRead[n.id] : n.read; }
  function notifTypeMeta(t) {
    return {
      '作业': { color: 'amber', ic: 'file-check' },
      '竞赛': { color: 'red', ic: 'trophy' },
      '成绩': { color: 'green', ic: 'bar-chart-3' },
      '系统': { color: 'blue', ic: 'info' }
    }[t] || { color: 'gray', ic: 'bell' };
  }
  function notifications(ctx) {
    const filter = ctx.query.f || 'all';   /* all | unread | read */
    const list = m.notifications.filter(n => filter === 'all' ? true : filter === 'unread' ? !notifRead(n) : notifRead(n));
    const unreadCount = m.notifications.filter(n => !notifRead(n)).length;
    const tabs = [['all', '全部', m.notifications.length], ['unread', '未读', unreadCount], ['read', '已读', m.notifications.length - unreadCount]];
    return `${C.head('站内信', '消息', unreadCount ? `<button class="btn btn-outline" onclick="Chaimir.studentReadAllNotif()">${C.icon('check-check')} 全部已读</button>` : '')}
      <div class="callout info mb-4">${C.icon('bell')}<div>新消息<b>实时到达</b>,无需刷新。点击消息可直接跳转到对应的作业、竞赛或成绩页面。</div></div>
      <div class="tabs">${tabs.map(([k, l, n]) => `<a class="tab ${k === filter ? 'active' : ''}" onclick="Chaimir.navigate('student/notifications?f=${k}')">${l}${n ? ` <span class="count">${n}</span>` : ''}</a>`).join('')}</div>
      ${list.length ? `<div class="card">${list.map((n, i) => {
        const read = notifRead(n);
        const meta = notifTypeMeta(n.type);
        return `<div class="flex items-center gap-3" style="padding:14px 18px;${i < list.length - 1 ? 'border-bottom:1px solid var(--color-border)' : ''};cursor:pointer;${read ? '' : 'background:var(--amber-50)'}" onclick="Chaimir.studentOpenNotif('${n.id}','${n.link}')">
          <span style="display:grid;place-items:center;width:38px;height:38px;border-radius:var(--radius);background:var(--${meta.color}-100);color:var(--${meta.color}-700);flex-shrink:0">${C.icon(meta.ic)}</span>
          <div style="flex:1;min-width:0">
            <div class="flex items-center gap-2">${C.badge(n.type, meta.color)}${read ? '' : '<span class="dot dot-amber" title="未读"></span>'}<span class="muted text-xs" style="margin-left:auto">${n.time}</span></div>
            <div class="${read ? '' : 'fw-600'} text-sm mt-2">${C.esc(n.title)}</div>
          </div>
          ${C.icon('chevron-right')}
        </div>`;
      }).join('')}</div>` : C.empty({ icon: 'mail-check', title: filter === 'unread' ? '没有未读消息' : '暂无消息', desc: '新消息会实时出现在这里' })}`;
  }
  /* 打开消息:标记已读 + 跳转 link */
  C.studentOpenNotif = function (id, link) {
    C._notifRead[id] = true;
    if (link) C.navigate(link); else C.rerender();
  };
  C.studentReadAllNotif = function () {
    m.notifications.forEach(n => { C._notifRead[n.id] = true; });
    C.toast('success', '已全部标记为已读');
    C.rerender();
  };

  /* ============================================================
     六、系统公告
     ------------------------------------------------------------
     列表(含已读状态)+ 置顶在前 + 查看详情即已读。
     ============================================================ */
  C._annRead = C._annRead || {};
  const announcements = [
    { id: 'an1', pin: true, title: '关于 2026 春季学期期末考试安排的通知', time: '2026-06-05', by: '教务处',
      body: '本学期期末考试将于 6 月 16 日至 6 月 27 日进行,具体场次见各课程公告。涉及上机实验的科目在线上沙箱完成,请提前测试环境连通性。' },
    { id: 'an2', pin: true, title: '平台维护通知:本周六凌晨例行升级', time: '2026-06-04', by: '平台运维',
      body: '平台将于本周六 02:00-04:00 进行例行维护升级,期间登录与实验环境可能短暂不可用。请合理安排提交时间,避免在维护窗口内提交作业或参赛。' },
    { id: 'an3', pin: false, title: '「链上夺旗」竞赛报名将于本周五截止', time: '2026-06-03', by: '竞赛组委会',
      body: '本届渗透赛报名将于 6 月 9 日 23:59 截止,个人赛与团队赛均开放,支持跨校组队。名额有限,先到先得。' },
    { id: 'an4', pin: false, title: '学业预警确认提醒', time: '2026-05-28', by: '学生工作处',
      body: '部分同学收到本学期学业预警,请及时在「学业预警」中确认知悉并与辅导员沟通学习计划。' },
  ];
  function studentAnnouncements() {
    const sorted = announcements.slice().sort((a, b) => (b.pin ? 1 : 0) - (a.pin ? 1 : 0));
    const unread = announcements.filter(a => !C._annRead[a.id]).length;
    return `${C.head('系统公告', '消息')}
      <div class="flex items-center gap-2 mb-3 text-sm muted">${C.icon('megaphone')} 共 ${announcements.length} 条公告${unread ? `,${unread} 条未读` : ''}</div>
      <div class="card">${sorted.map((a, i) => {
        const read = !!C._annRead[a.id];
        return `<div class="flex items-center gap-3" style="padding:14px 18px;${i < sorted.length - 1 ? 'border-bottom:1px solid var(--color-border)' : ''};cursor:pointer;${read ? '' : 'background:var(--amber-50)'}" onclick="Chaimir.studentOpenAnn('${a.id}')">
          <span style="display:grid;place-items:center;width:38px;height:38px;border-radius:var(--radius);background:var(--blue-100);color:var(--blue-700);flex-shrink:0">${C.icon('megaphone')}</span>
          <div style="flex:1;min-width:0">
            <div class="flex items-center gap-2">${a.pin ? C.badge('置顶', 'amber', 'pin') : ''}${read ? '' : '<span class="dot dot-amber" title="未读"></span>'}<span class="${read ? '' : 'fw-600'}">${C.esc(a.title)}</span></div>
            <div class="muted text-xs mt-2">${a.by} · ${a.time}</div>
          </div>
          ${read ? `<span class="muted text-xs flex items-center gap-1">${C.icon('check')} 已读</span>` : C.icon('chevron-right')}
        </div>`;
      }).join('')}</div>`;
  }
  /* 查看公告:进入即已读 + 弹出详情;关闭后刷新列表已读态 */
  C.studentOpenAnn = function (id) {
    const a = announcements.find(x => x.id === id); if (!a) return;
    C._annRead[id] = true;
    C.modal({
      title: a.title,
      body: `<div class="flex items-center gap-2 mb-3">${a.pin ? C.badge('置顶', 'amber', 'pin') : ''}<span class="muted text-xs">${a.by} · ${a.time}</span></div>
        <p class="text-sm" style="line-height:var(--leading-relaxed)">${C.esc(a.body)}</p>`,
      foot: `<button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.rerender()">我知道了</button>`,
    });
  };

  /* ============================================================
     七、个人中心
     ------------------------------------------------------------
     只读学籍(由学校维护)+ 改密/换绑(短信验证)+ 通知偏好
     (强制类锁定)+ 登录会话/单端登录说明。
     ============================================================ */
  function profile() {
    return `${C.head('个人中心', '账户')}
      <div class="grid" style="grid-template-columns:1fr 340px">
        <div>
          <div class="card mb-4"><div class="card-head"><div class="section-title">个人信息</div>${C.badge('学校维护', 'gray', 'lock')}</div>
            <div class="card-pad">
              <div class="callout info mb-4">${C.icon('shield')}<div>姓名、学号、班级、学院等学籍信息<b>由学校管理员统一维护</b>,如有错误请联系学校教务,本人不可自行修改。</div></div>
              <dl class="dl">
                <dt>姓名</dt><dd>${C.esc(m.me.name)}</dd>
                <dt>学号</dt><dd class="mono">${C.esc(m.me.no)}</dd>
                <dt>班级</dt><dd>${C.esc(m.me.class)}</dd>
                <dt>学院</dt><dd>${C.esc(m.me.dept)}</dd>
                <dt>学校</dt><dd>示例大学</dd>
              </dl>
            </div>
          </div>
          <div class="card mb-4"><div class="card-head"><div class="section-title">账号与安全</div></div>
            <div class="card-pad">
              <div class="flex items-center justify-between" style="padding:12px 0;border-bottom:1px solid var(--color-border)">
                <div><div class="fw-600 text-sm flex items-center gap-2">${C.icon('smartphone')} 绑定手机</div><div class="muted text-xs mt-2">${C.esc(m.me.phone)} · 用于登录与短信验证</div></div>
                <button class="btn btn-outline btn-sm" onclick="Chaimir.studentRebindPhone()">换绑手机</button>
              </div>
              <div class="flex items-center justify-between" style="padding:12px 0;border-bottom:1px solid var(--color-border)">
                <div><div class="fw-600 text-sm flex items-center gap-2">${C.icon('key-round')} 登录密码</div><div class="muted text-xs mt-2">建议定期更换,使用字母+数字组合</div></div>
                <button class="btn btn-outline btn-sm" onclick="Chaimir.studentChangePwd()">修改密码</button>
              </div>
              <div class="flex items-center justify-between" style="padding:12px 0">
                <div><div class="fw-600 text-sm flex items-center gap-2">${C.icon('building-2')} 学校统一身份(SSO)</div><div class="muted text-xs mt-2">已关联校园账号,可用校园身份登录</div></div>
                ${C.badge('已关联', 'green', 'check')}
              </div>
            </div>
          </div>
          <div class="card"><div class="card-head"><div class="section-title">通知偏好</div></div>
            <div class="card-pad">
              <div class="callout warn mb-3">${C.icon('lock')}<div><b>强制类通知</b>(成绩锁定、学业预警、安全提醒等)关乎学业与账号安全,<b>不可关闭</b>。</div></div>
              ${[
                ['作业与截止提醒', '作业发布、临近截止时提醒', true, false],
                ['竞赛动态', '排名变化、对局结果、报名提醒', true, false],
                ['成绩与申诉(强制)', '成绩锁定、申诉处理结果', true, true],
                ['学业预警(强制)', '挂科、低 GPA 等学业风险', true, true],
                ['账号安全(强制)', '异地登录、密码变更等', true, true],
              ].map(([t, d, on, locked]) => `
                <div class="flex items-center justify-between" style="padding:11px 0;border-bottom:1px solid var(--color-border)">
                  <div><div class="fw-600 text-sm">${t}</div><div class="muted text-xs mt-2">${d}</div></div>
                  ${locked
                    ? `<span class="flex items-center gap-2 text-xs muted">${C.icon('lock')} 已锁定</span>`
                    : `<label class="switch"><input type="checkbox" ${on ? 'checked' : ''} onchange="Chaimir.demo('已更新通知偏好')"><span class="track"></span></label>`}
                </div>`).join('')}
            </div>
          </div>
        </div>
        <div>
          <div class="card card-pad mb-4" style="text-align:center">
            <div style="display:inline-grid;place-items:center;width:72px;height:72px;border-radius:50%;background:linear-gradient(135deg,var(--amber-500),var(--amber-700));color:#fff;font-size:var(--text-2xl);font-weight:700;margin-bottom:12px">${C.esc(m.me.name.slice(0, 1))}</div>
            <div class="fw-700" style="font-size:var(--text-lg)">${C.esc(m.me.name)}</div>
            <div class="muted text-sm mt-2">${C.esc(m.me.class)}</div>
            <div class="flex gap-2 mt-3" style="justify-content:center">${C.badge('在读学生', 'green')}${C.badge('示例大学', 'gray')}</div>
          </div>
          <div class="card"><div class="card-head"><div class="section-title">登录会话</div></div>
            <div class="card-pad">
              <div class="callout info mb-3">${C.icon('monitor-smartphone')}<div>本平台<b>单端登录</b>:同一账号在新设备登录后,旧设备会自动下线,保护账号安全。</div></div>
              ${[['当前设备 · Chrome / Windows', '北京 · 刚刚', true], ['手机 App · iPhone', '北京 · 2 小时前', false]].map(([dev, loc, cur]) => `
                <div class="flex items-center gap-3" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
                  <span style="display:grid;place-items:center;width:34px;height:34px;border-radius:var(--radius-sm);background:var(--slate-100);color:var(--color-text-sub);flex-shrink:0">${C.icon(cur ? 'laptop' : 'smartphone')}</span>
                  <div style="flex:1;min-width:0"><div class="fw-600 text-sm ellipsis">${dev}</div><div class="muted text-xs mt-2">${loc}</div></div>
                  ${cur ? C.badge('本机', 'green') : `<button class="btn btn-ghost btn-sm" onclick="Chaimir.studentKickSession()">下线</button>`}
                </div>`).join('')}
              <button class="btn btn-outline btn-block mt-3" onclick="Chaimir.studentKickSession()">${C.icon('log-out')} 退出其他所有设备</button>
            </div>
          </div>
        </div>
      </div>`;
  }
  /* 换绑手机(短信验证弹窗) */
  C.studentRebindPhone = function () {
    C.modal({
      title: '换绑手机号',
      body: `<div class="field"><label>新手机号 <span class="req">*</span></label>
          <div class="input-icon">${C.icon('smartphone')}<input class="input" id="new-phone" placeholder="请输入新手机号"></div></div>
        <div class="field" style="margin-bottom:0"><label>短信验证码 <span class="req">*</span></label>
          <div class="flex gap-2">
            <div class="input-icon" style="flex:1">${C.icon('shield-check')}<input class="input" id="sms-code" placeholder="6 位验证码"></div>
            <button class="btn btn-outline" onclick="Chaimir.studentSendSms(this)">获取验证码</button>
          </div>
          <div class="help">验证码将发送至新手机号,用于确认本人操作</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','手机号已更新','下次可用新手机号登录与接收验证码')">确认换绑</button>`,
    });
  };
  /* 修改密码弹窗(旧密码 + 新密码) */
  C.studentChangePwd = function () {
    C.modal({
      title: '修改登录密码',
      body: `<div class="field"><label>当前密码 <span class="req">*</span></label>
          <div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="请输入当前密码"></div></div>
        <div class="field"><label>新密码 <span class="req">*</span></label>
          <div class="input-icon">${C.icon('key-round')}<input class="input" type="password" placeholder="8-20 位,字母+数字"></div>
          <div class="help">建议包含大小写字母与数字,避免使用生日、学号</div></div>
        <div class="field" style="margin-bottom:0"><label>确认新密码 <span class="req">*</span></label>
          <div class="input-icon">${C.icon('key-round')}<input class="input" type="password" placeholder="再次输入新密码"></div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','密码已修改','为保护账号,其他设备需重新登录')">确认修改</button>`,
    });
  };
  /* 发送验证码(倒计时演示) */
  C.studentSendSms = function (btn) {
    if (!btn || btn.disabled) return;
    let s = 60; btn.disabled = true; const orig = btn.textContent;
    btn.textContent = s + ' 秒后重发';
    const t = setInterval(() => { s--; if (s <= 0) { clearInterval(t); btn.disabled = false; btn.textContent = orig; } else { btn.textContent = s + ' 秒后重发'; } }, 1000);
    C.toast('info', '验证码已发送', '请查收短信,10 分钟内有效');
  };
  /* 下线其他会话(危险操作,需确认) */
  C.studentKickSession = async function () {
    if (await C.confirm({ title: '退出其他设备', message: '将立即下线除本机外的所有登录会话,确认继续?', confirmText: '全部下线', danger: true })) {
      C.toast('success', '已退出其他设备', '其他设备需重新登录');
    }
  };

  /* ============================================================
     注册路由
     ============================================================ */
  C.registerPages({
    'student/grades': grades,
    'student/warnings': studentWarnings,
    'student/appeals': appeals,
    'student/transcripts': transcripts,
    'student/notifications': notifications,
    'student/announcements': studentAnnouncements,
    'student/profile': profile,
  });
})();
