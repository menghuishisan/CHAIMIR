/* ============================================================
   pages/teacher/courses.js — 教师·教学域(课程 / 作业 / 批改)
   ------------------------------------------------------------
   覆盖:课程管理(列表+状态机操作)、课程编辑(基本信息+周课表)、
        章节课时编排(增删/拖拽视觉/内容类型)、选课成员、作业管理、
        作业编辑(从题库 M5 选题)、批改中心(主观/实验报告抽屉批改 +
        编程题 M3 查重 + M3 自动判题结果)。对应 M6 教学(教师侧)。
   范式:沿用 student/courses.js —— 列表页 C.head 开头、子页 C.crumb 开头,
        子页统一登记 C.parentRoute 高亮侧栏;行为走 C.* 工具,文案面向用户。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项(教学组) */
  Object.assign(C.parentRoute, {
    'teacher/course-edit': 'teacher/courses',
    'teacher/chapters': 'teacher/courses',
    'teacher/members': 'teacher/courses',
    'teacher/assignments': 'teacher/courses',
    'teacher/assignment-edit': 'teacher/courses',
  });

  /* 教师课程数据(在共享 courses 基础上补充教师视角:可见性/状态机)。
     状态:草稿/已发布/进行中/已结束/已归档 —— 驱动操作按钮可用性。 */
  const tCourses = [
    { id: 1, name: '区块链原理与智能合约开发', type: '混合', status: '进行中', members: 128, visibility: '本校可见', invite: 'BC-3F9K2', difficulty: '进阶', credits: 3, semester: '2025-2026 春' },
    { id: 2, name: 'DeFi 协议开发与套利审计', type: '实验', status: '已发布', members: 86, visibility: '班级可见', invite: 'DF-77AQ1', difficulty: '高级', credits: 2, semester: '2025-2026 春' },
    { id: 3, name: '智能合约安全攻防实训', type: '实验', status: '草稿', members: 0, visibility: '仅自己', invite: '—', difficulty: '高级', credits: 2, semester: '2025-2026 春' },
    { id: 4, name: '密码学基础与共识算法', type: '理论', status: '已结束', members: 152, visibility: '本校可见', invite: 'CR-2K8M5', difficulty: '入门', credits: 3, semester: '2024-2025 秋' },
    { id: 5, name: '区块链导论(2023 春存档)', type: '混合', status: '已归档', members: 96, visibility: '本校可见', invite: '—', difficulty: '入门', credits: 2, semester: '2022-2023 春' },
  ];
  C.tCourses = tCourses;

  const statusBadge = (s) => {
    const map = { '草稿': 'gray', '已发布': 'blue', '进行中': 'green', '已结束': 'purple', '已归档': 'gray' };
    return C.badge(s, map[s] || 'gray');
  };

  /* ---------- 课程管理(列表)---------- */
  function courseRow(c) {
    /* 操作按钮随状态机变化:草稿可发布、进行中可结束、已结束可归档 */
    let ops = `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/course-edit?id=${c.id}')">${C.icon('pencil')} 编辑</button>`;
    if (c.status === '草稿' || c.status === '已发布')
      ops += `<button class="btn btn-primary btn-sm" onclick="Chaimir.tCoursePublish(${c.id})">${C.icon('send')} 发布</button>`;
    if (c.status === '进行中')
      ops += `<button class="btn btn-outline btn-sm" onclick="Chaimir.tCourseEnd(${c.id})">${C.icon('flag')} 结束</button>`;
    if (c.status === '已结束')
      ops += `<button class="btn btn-outline btn-sm" onclick="Chaimir.tCourseArchive(${c.id})">${C.icon('archive')} 归档</button>`;
    return `<tr>
      <td><div class="fw-600">${c.name}</div><div class="muted text-xs mt-2">${c.semester} · ${c.credits} 学分 · ${c.difficulty}</div></td>
      <td>${C.badge(c.type, 'gray')}</td>
      <td>${statusBadge(c.status)}</td>
      <td class="mono">${c.members}</td>
      <td><span class="muted text-sm">${C.icon('eye')} ${c.visibility}</span></td>
      <td><span class="mono text-sm">${c.invite}</span></td>
      <td class="row-actions">${ops}
        <button class="btn btn-ghost btn-sm btn-icon" title="更多" onclick="Chaimir.tCourseMore(event,${c.id})">${C.icon('more-vertical')}</button></td>
    </tr>`;
  }

  /* 列表页:状态筛选 Tab + 表格 */
  function coursesList(ctx) {
    const filter = ctx.query.st || 'all';
    const tabs = [['all', '全部'], ['进行中', '进行中'], ['已发布', '已发布'], ['草稿', '草稿'], ['已结束', '已结束'], ['已归档', '已归档']];
    const rows = tCourses.filter(c => filter === 'all' || c.status === filter);
    const counts = { '进行中': tCourses.filter(c => c.status === '进行中').length, '草稿': tCourses.filter(c => c.status === '草稿').length };
    return `${C.head('课程管理', '教学', `<button class="btn btn-primary" onclick="Chaimir.navigate('teacher/course-edit')">${C.icon('plus')} 新建课程</button>`)}
      <div class="grid grid-4 mb-4">
        ${C.stat('book-open', tCourses.length, '我的课程', 'amber')}
        ${C.stat('play-circle', counts['进行中'], '进行中', 'green')}
        ${C.stat('users', '462', '累计选修', 'blue')}
        ${C.stat('file-edit', counts['草稿'], '草稿待发布', 'gray')}
      </div>
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${k === filter ? 'active' : ''}" onclick="Chaimir.navigate('teacher/courses?st=${encodeURIComponent(k)}')">${l}</a>`).join('')}</div>
      ${rows.length ? `<div class="table-wrap"><table class="table">
        <thead><tr><th>课程名称</th><th>类型</th><th>状态</th><th>选修</th><th>可见性</th><th>邀请码</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${rows.map(courseRow).join('')}</tbody></table></div>`
      : C.empty({ icon: 'book-open', title: '该状态下暂无课程', desc: '切换上方筛选,或新建一门课程开始教学。' })}`;
  }

  /* 列表行更多菜单:克隆 / 共享课程库 / 刷新邀请码 */
  C.tCourseMore = function (ev, id) {
    ev.stopPropagation();
    const c = tCourses.find(x => x.id == id);
    C.modal({
      title: '课程操作 · ' + c.name,
      body: `<div class="grid" style="gap:8px">
        <button class="menu-item" onclick="Chaimir.tCourseClone(${id})">${C.icon('copy')} 克隆课程(深拷贝章节与作业)</button>
        <button class="menu-item" onclick="Chaimir.tCourseShare(${id})">${C.icon('share-2')} 共享到课程库(供本校教师复用)</button>
        <button class="menu-item" onclick="Chaimir.tCourseRefreshInvite(${id})">${C.icon('refresh-cw')} 刷新邀请码(旧码立即失效)</button>
        <button class="menu-item" onclick="Chaimir.navigate('teacher/members?id=${id}')">${C.icon('user-plus')} 管理选课成员</button>
      </div>`,
    });
  };
  C.tCoursePublish = async function (id) {
    if (await C.confirm({ title: '发布课程', message: '发布后学生可凭邀请码加入,课程将进入「进行中」。确认发布?', confirmText: '确认发布' })) {
      const c = tCourses.find(x => x.id == id); if (c) c.status = '进行中';
      C.toast('success', '课程已发布', '学生现在可以加入这门课程了'); C.rerender();
    }
  };
  C.tCourseEnd = async function (id) {
    if (await C.confirm({ title: '结束课程', message: '结束后学生不能再提交作业与实验,但可查看成绩。确认结束?', confirmText: '确认结束', danger: true })) {
      const c = tCourses.find(x => x.id == id); if (c) c.status = '已结束';
      C.toast('success', '课程已结束', '可前往成绩报送汇总本课程成绩'); C.rerender();
    }
  };
  C.tCourseArchive = async function (id) {
    if (await C.confirm({ title: '归档课程', message: '归档后课程进入存档区,默认不在列表展示,可随时恢复。确认归档?', confirmText: '确认归档' })) {
      const c = tCourses.find(x => x.id == id); if (c) c.status = '已归档';
      C.toast('success', '课程已归档', '可在「已归档」筛选中查看'); C.rerender();
    }
  };
  C.tCourseClone = function (id) { document.querySelector('.overlay').remove(); C.toast('success', '克隆任务已创建', '正在深拷贝章节、课时与作业到新草稿'); };
  C.tCourseShare = function (id) { document.querySelector('.overlay').remove(); C.toast('success', '已共享到课程库', '本校教师可在共享库中检索并克隆'); };
  C.tCourseRefreshInvite = async function (id) {
    document.querySelector('.overlay').remove();
    if (await C.confirm({ title: '刷新邀请码', message: '刷新后旧邀请码立即失效,已加入的学生不受影响。确认刷新?', confirmText: '确认刷新', danger: true })) {
      const c = tCourses.find(x => x.id == id);
      if (c) c.invite = 'BC-' + Math.random().toString(36).slice(2, 7).toUpperCase();
      C.toast('success', '邀请码已刷新', '新邀请码:' + (c ? c.invite : '')); C.rerender();
    }
  };

  /* ---------- 课程编辑(基本信息 + 周课表)---------- */
  function courseEdit(ctx) {
    const c = ctx.query.id ? tCourses.find(x => x.id == ctx.query.id) : null;
    const isNew = !c;
    const title = isNew ? '新建课程' : '编辑课程';
    const days = ['周一', '周二', '周三', '周四', '周五'];
    const slots = ['1-2 节', '3-4 节', '5-6 节', '7-8 节'];
    /* 周课表:演示在「周二 3-4 节」「周四 5-6 节」有课 */
    const scheduled = { '周二|3-4 节': '理论 · A302', '周四|5-6 节': '实验 · 沙箱机房' };
    return `${C.crumb([{ label: '课程管理', to: 'teacher/courses' }, { label: title }])}
      ${C.head(title, isNew ? '填写基本信息后可继续编排章节与作业' : (c ? c.name : ''),
        `<button class="btn btn-outline" onclick="Chaimir.navigate('teacher/courses')">取消</button>
         <button class="btn btn-primary" onclick="Chaimir.tCourseSave(${isNew})">${C.icon('save')} 保存</button>`)}
      <div class="grid" style="grid-template-columns:1fr 300px">
        <div class="card card-pad">
          <div class="section-title mb-3">基本信息</div>
          <div class="field"><label>课程名称<span class="req">*</span></label>
            <input class="input" placeholder="如:区块链原理与智能合约开发" value="${c ? C.esc(c.name) : ''}"></div>
          <div class="field"><label>课程简介</label>
            <textarea class="textarea" placeholder="一句话介绍课程目标与适合人群…">${c ? '系统讲解区块链底层原理,结合 EVM 合约开发与安全攻防实验。' : ''}</textarea></div>
          <div class="grid grid-2">
            <div class="field"><label>课程类型<span class="req">*</span></label>
              <select class="select"><option ${c && c.type === '混合' ? 'selected' : ''}>混合</option><option ${c && c.type === '理论' ? 'selected' : ''}>理论</option><option ${c && c.type === '实验' ? 'selected' : ''}>实验</option></select></div>
            <div class="field"><label>难度</label>
              <select class="select"><option ${c && c.difficulty === '入门' ? 'selected' : ''}>入门</option><option ${c && c.difficulty === '进阶' ? 'selected' : ''}>进阶</option><option ${c && c.difficulty === '高级' ? 'selected' : ''}>高级</option></select></div>
            <div class="field"><label>学期</label><input class="input" value="${c ? C.esc(c.semester) : '2025-2026 春'}"></div>
            <div class="field"><label>学分</label><input class="input" type="number" value="${c ? c.credits : 3}"></div>
          </div>
          <div class="divider"></div>
          <div class="section-title mb-3">每周课程表</div>
          <div class="callout info mb-3">${C.icon('info')}<div>点击格子安排上课时段;实验课时段会优先调度沙箱资源。</div></div>
          <div class="table-wrap"><table class="table"><thead><tr><th>时段</th>${days.map(d => `<th>${d}</th>`).join('')}</tr></thead>
            <tbody>${slots.map(s => `<tr><td class="fw-600">${s}</td>${days.map(d => {
              const k = d + '|' + s; const v = scheduled[k];
              return `<td style="cursor:pointer" onclick="Chaimir.demo()">${v ? C.badge(v, 'amber') : `<span class="muted text-xs">+ 安排</span>`}</td>`;
            }).join('')}</tr>`).join('')}</tbody></table></div>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-2">课程封面</div>
            <div style="aspect-ratio:16/9;border:2px dashed var(--color-border-strong);border-radius:var(--radius);display:grid;place-items:center;color:var(--color-text-faint);cursor:pointer" onclick="Chaimir.demo('封面上传(原型占位)')">
              <div style="text-align:center">${C.icon('image-plus')}<div class="text-sm mt-2">点击上传封面</div><div class="text-xs">建议 16:9,JPG/PNG ≤ 2MB</div></div></div>
          </div>
          <div class="card card-pad mb-3"><div class="section-title mb-2">下一步</div>
            <button class="btn btn-outline btn-block mb-2" onclick="Chaimir.navigate('teacher/chapters?id=${c ? c.id : 1}')">${C.icon('list-tree')} 编排章节课时</button>
            <button class="btn btn-outline btn-block mb-2" onclick="Chaimir.navigate('teacher/assignments?id=${c ? c.id : 1}')">${C.icon('clipboard-list')} 管理作业</button>
            <button class="btn btn-outline btn-block" onclick="Chaimir.navigate('teacher/members?id=${c ? c.id : 1}')">${C.icon('users')} 选课成员</button>
          </div>
          <div class="card card-pad"><div class="section-title mb-2">可见性</div>
            <label class="radio mb-2" style="display:flex"><input type="radio" name="vis" checked> 本校可见</label>
            <label class="radio mb-2" style="display:flex"><input type="radio" name="vis"> 仅选课班级可见</label>
            <label class="radio" style="display:flex"><input type="radio" name="vis"> 仅自己(草稿)</label>
          </div>
        </div>
      </div>`;
  }
  C.tCourseSave = function (isNew) {
    C.toast('success', isNew ? '课程已创建' : '已保存', isNew ? '可继续编排章节与作业' : '修改已保存到服务端');
    setTimeout(() => C.navigate('teacher/courses'), 700);
  };

  /* ---------- 章节课时编排 ---------- */
  /* 课时内容类型:视频/图文/附件/实验引用(M7→M4)/仿真引用(M4)。
     评分实验必须走「课时 → M7 实验 → M4 仿真」;直挂 M4 仅讲解不计分。 */
  const lessonTypeMeta = {
    video: { ic: 'play-circle', label: '视频', badge: 'gray' },
    text: { ic: 'file-text', label: '图文', badge: 'gray' },
    file: { ic: 'paperclip', label: '附件', badge: 'gray' },
    experiment: { ic: 'flask-conical', label: '实验引用(M7)', badge: 'purple' },
    sim: { ic: 'activity', label: '仿真引用(M4)', badge: 'teal' },
  };
  const editChapters = [
    { title: '第一章 · 区块链与分布式账本', lessons: [
      { title: '1.1 从比特币说起', type: 'video', meta: '18:24' },
      { title: '1.2 哈希与默克尔树', type: 'sim', meta: 'M4 · 默克尔树可视化(讲解)' },
      { title: '1.3 课堂讲义', type: 'text', meta: '图文' } ] },
    { title: '第三章 · 共识算法与重入漏洞', lessons: [
      { title: '3.1 PoW 与 PBFT', type: 'video', meta: '24:10' },
      { title: '3.2 PBFT 共识仿真(讲解)', type: 'sim', meta: 'M4 · 不计分' },
      { title: '3.3 重入漏洞代码实验', type: 'experiment', meta: 'M7 实验 #1024 · 计分 30' },
      { title: '3.4 参考资料.pdf', type: 'file', meta: '2.1 MB' } ] },
  ];

  function chapters(ctx) {
    const cid = ctx.query.id || 1;
    return `${C.crumb([{ label: '课程管理', to: 'teacher/courses' }, { label: '编辑课程', to: 'teacher/course-edit?id=' + cid }, { label: '章节课时' }])}
      ${C.head('章节课时编排', '拖拽调整顺序 · 配置每个课时的内容类型',
        `<button class="btn btn-outline" onclick="Chaimir.demo('新增章节')">${C.icon('folder-plus')} 新增章节</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','已保存','章节结构已保存')">${C.icon('save')} 保存</button>`)}
      <div class="callout warn mb-4">${C.icon('alert-triangle')}<div><b>评分规则</b>:需要计分的实验请走「课时 → 引用 M7 实验 → 由实验编排绑定 M4 仿真/M3 判题」。直接挂 M4 仿真包的课时<b>仅用于课堂讲解,不计入成绩</b>。</div></div>
      ${editChapters.map((ch, ci) => `
        <div class="card mb-3">
          <div class="card-head">
            <div class="flex items-center gap-2"><span style="cursor:grab;color:var(--color-text-faint)" title="拖拽排序">${C.icon('grip-vertical')}</span>
              <input class="input" style="font-weight:700;border-color:transparent;background:transparent;padding:4px 6px" value="${C.esc(ch.title)}"></div>
            <div class="flex gap-2">
              <button class="btn btn-ghost btn-sm" onclick="Chaimir.tAddLesson(${ci})">${C.icon('plus')} 加课时</button>
              <button class="btn btn-ghost btn-sm btn-icon" title="删除章节" onclick="Chaimir.tDelChapter(${ci})">${C.icon('trash-2')}</button></div>
          </div>
          <div style="padding:8px">${ch.lessons.map((l) => {
            const meta = lessonTypeMeta[l.type];
            return `<div class="side-item" style="border-radius:var(--radius-sm)">
              <span style="cursor:grab;color:var(--color-text-faint)" title="拖拽排序">${C.icon('grip-vertical')}</span>
              ${C.icon(meta.ic)}<span style="flex:1">${C.esc(l.title)}</span>
              <span class="muted text-xs">${C.esc(l.meta)}</span>
              ${C.badge(meta.label, meta.badge)}
              <button class="btn btn-ghost btn-sm btn-icon" title="设置内容类型" onclick="Chaimir.tLessonType()">${C.icon('settings-2')}</button>
              <button class="btn btn-ghost btn-sm btn-icon" title="删除课时" onclick="Chaimir.demo('删除课时')">${C.icon('x')}</button>
            </div>`;
          }).join('')}</div>
        </div>`).join('')}`;
  }
  C.tDelChapter = async function (ci) {
    if (await C.confirm({ title: '删除章节', message: '删除后该章节下的课时编排将一并移除(已发布课时的学生学习记录保留)。确认删除?', confirmText: '删除', danger: true }))
      C.toast('success', '已删除章节', '章节结构已更新');
  };
  /* 加课时:选择内容类型,实验/仿真需引用 M7/M4 */
  C.tAddLesson = function (ci) {
    C.modal({
      title: '新增课时',
      body: `<div class="field"><label>课时标题<span class="req">*</span></label><input class="input" placeholder="如:3.5 闪电贷攻击剖析"></div>
        <div class="field"><label>内容类型<span class="req">*</span></label>
          <div class="grid grid-2" style="gap:8px">
            ${Object.entries(lessonTypeMeta).map(([k, v]) => `<label class="radio" style="display:flex;padding:10px;border:1px solid var(--color-border);border-radius:var(--radius-sm)"><input type="radio" name="lt" ${k === 'video' ? 'checked' : ''}> ${C.icon(v.ic)} ${v.label}</label>`).join('')}
          </div></div>
        <div class="callout info">${C.icon('info')}<div>选择「实验引用」可绑定一个 M7 实验模板用于计分;选择「仿真引用」直接挂 M4 仿真包仅用于讲解。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','课时已添加','可拖拽调整顺序')">添加</button>`,
    });
  };
  C.tLessonType = function () {
    C.modal({
      title: '设置课时内容',
      body: `<div class="field"><label>引用资源</label>
          <select class="select"><option>M7 实验 #1024 · 重入漏洞利用与防护(计分)</option><option>M4 仿真 · PBFT 共识可视化(讲解)</option></select>
          <div class="help">实验引用走 M7→M4/M3 链路,可计分;仿真直挂仅讲解不计分。</div></div>
        <div class="field"><label>计分</label><label class="checkbox"><input type="checkbox" checked> 本课时计入成绩(仅实验引用可用)</label></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已保存','课时内容已更新')">保存</button>`,
    });
  };

  /* ---------- 选课成员 ---------- */
  function members(ctx) {
    const cid = ctx.query.id || 1;
    const c = tCourses.find(x => x.id == cid) || tCourses[0];
    return `${C.crumb([{ label: '课程管理', to: 'teacher/courses' }, { label: c.name, to: 'teacher/course-edit?id=' + cid }, { label: '选课成员' }])}
      ${C.head('选课成员', c.name + ' · 共 ' + m.students.length + ' 人',
        `<button class="btn btn-outline" onclick="Chaimir.tRefreshInviteCard()">${C.icon('refresh-cw')} 刷新邀请码</button>
         <button class="btn btn-primary" onclick="Chaimir.tBatchAdd()">${C.icon('user-plus')} 按班级批量添加</button>`)}
      <div class="card card-pad mb-4 flex items-center justify-between wrap gap-3" style="background:var(--color-surface-sunken)">
        <div class="flex items-center gap-3">${C.icon('ticket')}<div><div class="fw-600">课程邀请码</div><div class="muted text-xs">学生在「加入课程」处输入此码即可加入</div></div></div>
        <div class="flex items-center gap-2"><span class="mono fw-700" style="font-size:var(--text-lg);letter-spacing:.1em">${c.invite}</span>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.demo('已复制邀请码')">${C.icon('copy')} 复制</button></div>
      </div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th><label class="checkbox"><input type="checkbox" onclick="Chaimir.demo('全选')"></label></th><th>姓名</th><th>学号</th><th>班级</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${m.students.map(s => `<tr>
          <td><label class="checkbox"><input type="checkbox"></label></td>
          <td class="fw-600">${s.name}</td><td class="mono">${s.no}</td><td>${s.class}</td>
          <td>${C.statusDot('green', '已加入')}</td>
          <td class="row-actions"><button class="btn btn-ghost btn-sm" onclick="Chaimir.tRemoveMember('${C.esc(s.name)}')">${C.icon('user-minus')} 移除</button></td>
        </tr>`).join('')}</tbody></table></div>`;
  }
  C.tBatchAdd = function () {
    C.modal({
      title: '按班级批量添加',
      body: `<div class="field"><label>选择班级</label>
          ${['区块链 2301 班(46 人)', '区块链 2302 班(44 人)', '网络安全 2301 班(40 人)'].map((b, i) => `<label class="checkbox mb-2" style="display:flex;padding:9px;border:1px solid var(--color-border);border-radius:var(--radius-sm)"><input type="checkbox" ${i === 0 ? 'checked' : ''}> ${b}</label>`).join('')}</div>
        <div class="callout info">${C.icon('info')}<div>已在课程中的学生会自动跳过,不会重复添加。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已添加 46 名学生','已跳过 2 名重复学生')">确认添加</button>`,
    });
  };
  C.tRemoveMember = async function (name) {
    if (await C.confirm({ title: '移除成员', message: '将 ' + name + ' 移出本课程?该生的提交记录会保留但不再可见。', confirmText: '移除', danger: true }))
      C.toast('success', '已移除', name + ' 已移出本课程');
  };
  C.tRefreshInviteCard = async function () {
    if (await C.confirm({ title: '刷新邀请码', message: '刷新后旧邀请码立即失效,已加入学生不受影响。确认刷新?', confirmText: '确认刷新', danger: true }))
      C.toast('success', '邀请码已刷新', '请把新邀请码发给尚未加入的学生');
  };

  /* ---------- 作业管理(列表)---------- */
  const assignments = [
    { id: 1, title: '第一章 · 区块链基础测验', course: '区块链原理与智能合约', status: '已发布', due: '2026-05-20 23:59', submitted: 118, total: 128, attempts: 2 },
    { id: 2, title: '智能合约安全作业', course: '区块链原理与智能合约', status: '进行中', due: '2026-06-08 23:59', submitted: 64, total: 128, attempts: 3 },
    { id: 3, title: '重入漏洞修复编程作业', course: '智能合约安全攻防实训', status: '草稿', due: '—', submitted: 0, total: 0, attempts: 3 },
    { id: 4, title: 'DeFi 套利策略实验报告', course: 'DeFi 协议开发与套利审计', status: '已截止', due: '2026-05-30 23:59', submitted: 82, total: 86, attempts: 1 },
  ];
  C.tAssignments = assignments;

  function assignmentsList(ctx) {
    const cid = ctx.query.id;
    const badge = (s) => C.badge(s, { '已发布': 'blue', '进行中': 'green', '草稿': 'gray', '已截止': 'purple' }[s] || 'gray');
    return `${C.crumb([{ label: '课程管理', to: 'teacher/courses' }, { label: '作业管理' }])}
      ${C.head('作业管理', '从题库 M5 引用题目组成作业,设置截止与迟交策略',
        `<button class="btn btn-primary" onclick="Chaimir.navigate('teacher/assignment-edit')">${C.icon('plus')} 新建作业</button>`)}
      <div class="table-wrap"><table class="table">
        <thead><tr><th>作业</th><th>所属课程</th><th>状态</th><th>截止时间</th><th>提交</th><th>允许次数</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${assignments.map(a => `<tr>
          <td class="fw-600">${a.title}</td>
          <td class="muted text-sm">${a.course}</td>
          <td>${badge(a.status)}</td>
          <td class="mono text-sm">${a.due}</td>
          <td><div class="flex items-center gap-2"><div class="progress" style="width:64px"><span style="width:${a.total ? Math.round(a.submitted / a.total * 100) : 0}%"></span></div><span class="text-xs muted">${a.submitted}/${a.total}</span></div></td>
          <td class="mono">${a.attempts}</td>
          <td class="row-actions">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/assignment-edit?id=${a.id}')">${C.icon('pencil')} 编辑</button>
            ${a.status === '草稿' ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.toast('success','作业已发布','学生现在可以作答了')">${C.icon('send')} 发布</button>`
              : `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/grading?aid=${a.id}')">${C.icon('check-square')} 批改</button>`}
          </td>
        </tr>`).join('')}</tbody></table></div>`;
  }

  /* ---------- 作业编辑(从题库 M5 选题)---------- */
  /* 已选题目:题型(客观/主观/编程/实验报告)+ 分值 */
  const draftQuestions = [
    { id: 'Q-1', title: '重入攻击根本防护方式(单选)', type: '客观', score: 20, judge: '自动判题' },
    { id: 'Q-2', title: '修复合约重入漏洞使其通过测试(编程)', type: '编程', score: 50, judge: '沙箱判题 M3' },
    { id: 'Q-3', title: '简述 CEI 模式执行顺序与防御原理(简答)', type: '主观', score: 30, judge: '教师批改' },
  ];

  function assignmentEdit(ctx) {
    const a = ctx.query.id ? assignments.find(x => x.id == ctx.query.id) : null;
    const isNew = !a;
    const total = draftQuestions.reduce((n, q) => n + q.score, 0);
    return `${C.crumb([{ label: '课程管理', to: 'teacher/courses' }, { label: '作业管理', to: 'teacher/assignments' }, { label: isNew ? '新建作业' : '编辑作业' }])}
      ${C.head(isNew ? '新建作业' : '编辑作业', a ? a.title : '从题库选题、设定策略后发布',
        `<button class="btn btn-outline" onclick="Chaimir.navigate('teacher/assignments')">取消</button>
         <button class="btn btn-outline" onclick="Chaimir.toast('success','草稿已保存','可稍后继续编辑')">存草稿</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','作业已发布','学生现在可以作答了');setTimeout(()=>Chaimir.navigate('teacher/assignments'),700)">${C.icon('send')} 发布</button>`)}
      <div class="grid" style="grid-template-columns:1fr 320px">
        <div>
          <div class="card card-pad mb-3">
            <div class="section-title mb-3">基本信息</div>
            <div class="field"><label>作业标题<span class="req">*</span></label><input class="input" value="${a ? C.esc(a.title) : ''}" placeholder="如:智能合约安全作业"></div>
            <div class="field"><label>作业说明</label><textarea class="textarea" placeholder="向学生说明本次作业目标、提交要求…"></textarea></div>
          </div>
          <div class="card mb-3">
            <div class="card-head"><div class="section-title">题目(共 ${draftQuestions.length} 题 · ${total} 分)</div>
              <button class="btn btn-primary btn-sm" onclick="Chaimir.tPickQuestions()">${C.icon('library')} 从题库选题</button></div>
            <div style="padding:8px">${draftQuestions.map((q, i) => `
              <div class="side-item" style="border-radius:var(--radius-sm)">
                <span style="cursor:grab;color:var(--color-text-faint)">${C.icon('grip-vertical')}</span>
                <span class="muted text-xs mono">${i + 1}</span>
                <span style="flex:1">${q.title}</span>
                ${C.badge(q.type, q.type === '编程' ? 'purple' : q.type === '主观' ? 'amber' : q.type === '实验报告' ? 'teal' : 'blue')}
                <span class="badge badge-gray">${q.judge}</span>
                <input class="input" style="width:64px;text-align:center" type="number" value="${q.score}" title="分值">
                <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('移除题目')">${C.icon('x')}</button>
              </div>`).join('')}</div>
          </div>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-3">提交策略</div>
            <div class="field"><label>截止时间</label><input class="input" type="datetime-local" value="2026-06-08T23:59"></div>
            <div class="field"><label>允许提交次数</label><input class="input" type="number" value="3"></div>
            <div class="field"><label>迟交策略</label>
              <select class="select"><option>允许迟交,按比例扣分</option><option>允许迟交,标记但不扣分</option><option>不允许迟交</option></select></div>
            <div class="field" style="margin-bottom:0"><label class="checkbox"><input type="checkbox" checked> 提交后立即返回客观题与编程题判题结果</label></div>
          </div>
          <div class="callout info">${C.icon('info')}<div>主观题与实验报告需教师在批改中心人工评分;编程题由 M3 沙箱自动判题并查重。</div></div>
        </div>
      </div>`;
  }
  /* 从题库 M5 选题(C.modal 选题器)*/
  C.tPickQuestions = function () {
    const lib = [
      { id: 'C-118', t: '重入攻击根本防护方式', type: '客观', d: '进阶' },
      { id: 'C-205', t: '修复合约重入漏洞(编程)', type: '编程', d: '高级' },
      { id: 'C-309', t: 'CEI 模式执行顺序简答', type: '主观', d: '进阶' },
      { id: 'C-417', t: '闪电贷套利原理分析(实验报告)', type: '实验报告', d: '高级' },
      { id: 'C-522', t: 'PBFT 三阶段共识选择题', type: '客观', d: '入门' },
    ];
    C.modal({
      title: '从题库选题(M5)', size: 'lg',
      body: `<div class="input-icon mb-3">${C.icon('search')}<input class="input" placeholder="按标题 / 知识点 / 标签搜索题目"></div>
        <div class="table-wrap"><table class="table"><thead><tr><th><label class="checkbox"><input type="checkbox"></label></th><th>编号</th><th>题目</th><th>题型</th><th>难度</th></tr></thead>
          <tbody>${lib.map(q => `<tr>
            <td><label class="checkbox"><input type="checkbox"></label></td>
            <td class="mono text-xs">${q.id}</td><td class="fw-600">${q.t}</td>
            <td>${C.badge(q.type, q.type === '编程' ? 'purple' : q.type === '主观' ? 'amber' : q.type === '实验报告' ? 'teal' : 'blue')}</td>
            <td>${C.badge(q.d, 'gray')}</td></tr>`).join('')}</tbody></table></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已加入 2 道题','可在题目列表设置分值与题序')">加入作业</button>`,
    });
  };

  /* ---------- 批改中心 ---------- */
  Object.assign(C.parentRoute, { 'teacher/grading': 'teacher/grading' });
  /* 某作业的提交情况:含自动判题(M3)+ 编程题查重相似度(M3)+ 待人工批改项 */
  const submissions = [
    { id: 1, name: '林思远', no: '2023210456', time: '2026-06-07 16:20', auto: '70/70', sim: 8, manualDone: true, manual: '22/30', total: 92, state: 'graded' },
    { id: 2, name: '赵雨桐', no: '2023210457', time: '2026-06-07 18:02', auto: '70/70', sim: 12, manualDone: false, manual: '—', total: null, state: 'pending' },
    { id: 3, name: '孙浩然', no: '2023210458', time: '2026-06-07 21:40', auto: '50/70', sim: 86, manualDone: false, manual: '—', total: null, state: 'flag' },
    { id: 4, name: '周晓彤', no: '2023210459', time: '2026-06-08 09:11', auto: '70/70', sim: 5, manualDone: false, manual: '—', total: null, state: 'pending' },
    { id: 5, name: '吴俊杰', no: '2023210460', time: '2026-06-08 10:30', auto: '64/70', sim: 9, manualDone: true, manual: '28/30', total: 92, state: 'graded' },
  ];

  function grading(ctx) {
    const tab = ctx.query.t || 'list';
    const pending = submissions.filter(s => !s.manualDone).length;
    return `${C.head('批改中心', '教学', `<button class="btn btn-outline" onclick="Chaimir.demo('导出成绩 Excel')">${C.icon('download')} 导出</button>`)}
      <div class="card card-pad mb-4">
        <div class="flex justify-between wrap gap-3 items-center">
          <div><div class="muted text-xs">当前作业</div><div class="fw-700" style="font-size:var(--text-lg)">智能合约安全作业 · 区块链原理与智能合约</div></div>
          <select class="select" style="width:280px" onchange="Chaimir.demo('切换作业')">
            <option>智能合约安全作业(待批 ${pending})</option><option>第一章 · 区块链基础测验</option><option>DeFi 套利策略实验报告</option></select>
        </div>
      </div>
      <div class="grid grid-4 mb-4">
        ${C.stat('inbox', submissions.length, '已提交', 'blue')}
        ${C.stat('clock', pending, '待人工批改', 'amber')}
        ${C.stat('bot', '70', '自动判题满分项', 'green')}
        ${C.stat('copy-check', '1', '查重高相似', 'red')}
      </div>
      <div class="callout info mb-4">${C.icon('info')}<div>客观题与编程题已由 M3 自动判题;<b>主观题与实验报告</b>需在此人工评分。编程题的<b>查重相似度</b>来自 M3,超过阈值的提交已标红,请人工认定是否抄袭。</div></div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>学生</th><th>提交时间</th><th>自动判题(M3)</th><th>查重相似度</th><th>人工评分</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${submissions.map(s => {
          const simBadge = s.sim >= 80 ? 'red' : s.sim >= 40 ? 'amber' : 'green';
          const stMap = { graded: ['已批改', 'green'], pending: ['待批改', 'amber'], flag: ['查重预警', 'red'] };
          const st = stMap[s.state];
          return `<tr>
            <td><div class="fw-600">${s.name}</div><div class="muted text-xs mono">${s.no}</div></td>
            <td class="mono text-sm">${s.time}</td>
            <td><span class="mono">${s.auto}</span></td>
            <td><div class="flex items-center gap-2"><div class="progress" style="width:56px"><span style="width:${s.sim}%;background:var(--${simBadge === 'red' ? 'red' : simBadge === 'amber' ? 'amber' : 'green'}-600)"></span></div>${C.badge(s.sim + '%', simBadge)}</div></td>
            <td class="mono">${s.manual}</td>
            <td>${C.statusDot(st[1], st[0])}</td>
            <td class="row-actions">
              ${s.sim >= 80 ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/cheat-review?sid=${s.id}')">${C.icon('search')} 查重</button>` : ''}
              <button class="btn btn-primary btn-sm" onclick="Chaimir.tGradeDrawer('${C.esc(s.name)}', ${s.sim})">${C.icon('pen-line')} 批改</button>
            </td>
          </tr>`;
        }).join('')}</tbody></table></div>`;
  }
  /* 批改抽屉:主观题/实验报告给分 + 评语;编程题展示 M3 查重对比 */
  C.tGradeDrawer = function (name, sim) {
    C.drawer({
      title: '批改 · ' + name,
      body: `<div class="mb-4">
          <div class="flex justify-between mb-2"><div class="fw-700">第 1 题 · 单选(20 分)</div>${C.badge('自动判题 20/20', 'green')}</div>
          <p class="text-sm muted">选择「先更新状态再外部调用(CEI)」—— 正确。</p></div>
        <div class="divider"></div>
        <div class="mb-4">
          <div class="flex justify-between mb-2"><div class="fw-700">第 2 题 · 编程(50 分)</div>${C.badge('沙箱判题 50/50', 'green')}</div>
          <div class="callout ${sim >= 80 ? 'danger' : 'success'} mb-2">${C.icon(sim >= 80 ? 'alert-circle' : 'check-circle-2')}<div>M3 代码查重相似度 <b>${sim}%</b>${sim >= 80 ? ' —— 与「孙浩然」提交高度相似,请认定。' : ' —— 在正常范围内。'}</div></div>
          ${sim >= 80 ? `<div class="grid grid-2" style="gap:8px">
            <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:10px;font-family:var(--font-mono);font-size:11px;color:#cbd5e1;white-space:pre;overflow:auto">balances[msg.sender]=0;
(bool ok,)=msg.sender.call{value:amt}("");</div>
            <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:10px;font-family:var(--font-mono);font-size:11px;color:#cbd5e1;white-space:pre;overflow:auto">balances[msg.sender]=0;
(bool ok,)=msg.sender.call{value:amt}("");</div></div>
            <div class="flex gap-2 mt-2"><button class="btn btn-danger btn-sm" onclick="Chaimir.navigate('teacher/cheat-review')">${C.icon('flag')} 移交防作弊处理</button></div>` : ''}
        </div>
        <div class="divider"></div>
        <div>
          <div class="flex justify-between mb-2"><div class="fw-700">第 3 题 · 简答(30 分)</div>${C.badge('待人工评分', 'amber')}</div>
          <div class="card card-pad mb-3" style="background:var(--color-surface-sunken)"><div class="muted text-xs mb-2">学生作答</div>
            <p class="text-sm">CEI 即 Checks(校验)→ Effects(改状态)→ Interactions(外部调用)。先把余额清零再转账,递归回调时余额已为 0,从而阻断重入。</p></div>
          <div class="field"><label>得分(满分 30)</label><input class="input" type="number" placeholder="如 25" value="25"></div>
          <div class="field"><label>评语(对学生可见)</label><textarea class="textarea" placeholder="给出针对性反馈…">思路正确,CEI 顺序解释清晰;可补充对 ReentrancyGuard 互斥锁的对比。</textarea></div>
        </div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已提交批改','该生总评已更新')">${C.icon('check')} 提交评分</button>`,
    });
  };

  C.registerPages({
    'teacher/courses': coursesList,
    'teacher/course-edit': courseEdit,
    'teacher/chapters': chapters,
    'teacher/members': members,
    'teacher/assignments': assignmentsList,
    'teacher/assignment-edit': assignmentEdit,
    'teacher/grading': grading,
  });
})();
