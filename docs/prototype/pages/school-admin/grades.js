/* ============================================================
   pages/school-admin/grades.js — 学校管理员·成绩治理与系统配置域
   ------------------------------------------------------------
   覆盖:成绩审核(教师报送 → 通过锁定 / 驳回 / 解锁回退)、申诉处理
        (受理解锁 → M6 改分 → 事件 → M11 重算重锁)、学业预警(规则
        配置 + 监控列表 + 周期扫描)、成绩配置(等级映射 / 学期管理 /
        成绩单生成)、学校配置(logo/展示名/功能开关/认证方式)、认证
        配置(CAS / LDAP + 连接测试)、审计日志(本校查询 + 导出)、
        告警(规则 + 事件)、个人中心。对应 M11 聚合(成绩与治理)、
        M1 身份与配置。
   说明:成绩状态机:待审 → 已通过(锁定)/ 已驳回;通过即锁定,防止
        绕过审核旁路改分。所有危险操作经 C.confirm 二次确认。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* ============================================================
     ① 成绩审核(school-admin/grade-reviews)
        状态机:待审 → 已通过(锁定,触发 GPA 聚合)/ 已驳回;
                已通过可解锁回退(回到待审,供教师修正后重报)。
     ============================================================ */
  C.saReviews = () => ([
    { id: 1, course: '区块链原理与智能合约开发', teacher: '李明远', cls: '区块链 2301', count: 42, avg: 82.6, fail: 3, submit: '2026-06-06 18:20', status: '待审' },
    { id: 2, course: '密码学基础与共识算法', teacher: '陈雪', cls: '区块链 2301', count: 41, avg: 78.1, fail: 6, submit: '2026-06-05 11:40', status: '待审' },
    { id: 3, course: 'DeFi 协议开发与套利审计', teacher: '王思齐', cls: '区块链 2401', count: 38, avg: 85.3, fail: 1, submit: '2026-06-04 09:00', status: '已通过' },
    { id: 4, course: '智能合约安全实训', teacher: '李明远', cls: '区块链 2301', count: 40, avg: 73.2, fail: 9, submit: '2026-06-03 15:10', status: '已驳回' },
  ]);
  C.saReviewFilter = 'all';
  C.saReviewSet = (v) => { C.saReviewFilter = v; C.rerender(); };

  C.saReviewAct = async function (id, act) {
    const r = C.saReviews().find(x => x.id == id); if (!r) return;
    if (act === 'pass') {
      if (await C.confirm({ title: '通过并锁定成绩', confirmText: '通过并锁定',
        message: `通过《${r.course}》(${r.cls})成绩后将立即锁定该批成绩并触发 GPA 聚合;锁定后不可由教师直接改分,只能经申诉解锁。确认通过?` }))
        C.toast('success', '成绩已通过并锁定', `已触发 ${r.cls} 的 GPA 重算`);
    } else if (act === 'reject') {
      C.saReviewReject(r);
    } else if (act === 'unlock') {
      if (await C.confirm({ title: '解锁回退', danger: true, confirmText: '解锁回退',
        message: `解锁《${r.course}》将回退到待审状态,本批成绩重新可改;请仅在确认录入有误时使用。该操作记审计。` }))
        C.toast('success', '已解锁回退至待审', '教师可修正后重新报送');
    } else if (act === 'detail') {
      C.saReviewDetail(r);
    }
  };
  /* 驳回需填写原因(回退给教师) */
  C.saReviewReject = function (r) {
    C.modal({
      title: '驳回成绩报送',
      body: `<p class="text-sm mb-3">驳回《${C.esc(r.course)}》(${C.esc(r.cls)})的成绩报送,将退回任课教师修正后重报。</p>
        <div class="field"><label>驳回原因<span class="req">*</span></label>
          <textarea class="textarea" id="rj-reason" placeholder="如:不及格人数异常偏高,请核对评分标准与登分是否有误"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-danger" onclick="(function(){var v=(document.getElementById('rj-reason')||{}).value;if(!v||!v.trim()){Chaimir.toast('error','请填写驳回原因','原因将随通知发送给教师');return}document.querySelector('.overlay').remove();Chaimir.toast('success','已驳回','已通知任课教师修正后重报')})()">确认驳回</button>`,
    });
  };
  C.saReviewDetail = function (r) {
    C.drawer({
      title: '成绩报送明细',
      body: `<div class="dl mb-4">
          <dt>课程</dt><dd class="fw-600">${C.esc(r.course)}</dd>
          <dt>任课教师</dt><dd>${C.esc(r.teacher)}</dd>
          <dt>班级</dt><dd>${C.esc(r.cls)}</dd>
          <dt>人数</dt><dd>${r.count} 人</dd>
          <dt>平均分</dt><dd>${r.avg}</dd>
          <dt>不及格</dt><dd>${r.fail} 人</dd>
          <dt>报送时间</dt><dd class="mono">${r.submit}</dd>
          <dt>状态</dt><dd>${reviewBadge(r.status)}</dd>
        </div>
        <div class="section-title mb-2">成绩构成(权重)</div>
        <div class="callout info mb-3">${C.icon('info')}<div>总评 = 作业 40% + 实验 40% + 考试 20%,由教学模块按规则计算后报送,审核端只读不改分。</div></div>
        <div class="section-title mb-2">分数分布</div>
        ${gradeHistogram([2, 1, 4, 12, 15, 8])}`,
      foot: r.status === '待审' ? `<button class="btn btn-danger" onclick="document.querySelector('.overlay').remove();Chaimir.saReviewAct(${r.id},'reject')">驳回</button>
        <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.saReviewAct(${r.id},'pass')">通过并锁定</button>` : '',
    });
  };

  function reviewBadge(s) {
    return { '待审': C.badge('待审核', 'amber'), '已通过': C.badge('已通过 · 已锁定', 'green'), '已驳回': C.badge('已驳回', 'red') }[s] || C.badge(s, 'gray');
  }
  /* 分数分布直方图(内联 SVG):buckets 对应 <60/60-/70-/80-/90-/=100 */
  function gradeHistogram(buckets) {
    const labels = ['<60', '60+', '70+', '80+', '90+', '100'];
    const W = 380, H = 140, padL = 24, padB = 22, padT = 8;
    const iw = W - padL - 10, ih = H - padB - padT;
    const max = Math.max(1, ...buckets);
    const bw = iw / buckets.length * 0.6, gap = iw / buckets.length;
    let bars = buckets.map((v, i) => {
      const x = padL + gap * i + (gap - bw) / 2, h = (ih * v) / max, y = padT + ih - h;
      const col = i === 0 ? 'var(--red-600)' : 'var(--amber-500)';
      return `<rect x="${x}" y="${y}" width="${bw}" height="${h}" rx="3" fill="${col}"><title>${labels[i]}:${v} 人</title></rect>
        <text x="${x + bw / 2}" y="${y - 3}" text-anchor="middle" font-size="9" fill="var(--color-text-sub)">${v}</text>
        <text x="${x + bw / 2}" y="${H - 6}" text-anchor="middle" font-size="9" fill="var(--color-text-faint)">${labels[i]}</text>`;
    }).join('');
    return `<svg viewBox="0 0 ${W} ${H}" width="100%" role="img" aria-label="分数分布直方图">${bars}</svg>`;
  }

  function gradeReviews() {
    const all = C.saReviews();
    const tabs = [['all', '全部'], ['待审', '待审核'], ['已通过', '已通过'], ['已驳回', '已驳回']];
    const list = C.saReviewFilter === 'all' ? all : all.filter(r => r.status === C.saReviewFilter);
    const pending = all.filter(r => r.status === '待审').length;
    return `${C.head('成绩审核', '成绩', `<button class="btn btn-outline" onclick="Chaimir.navigate('school-admin/grade-config')">${C.icon('settings-2')} 成绩配置</button>`)}
      <div class="callout warn mb-4">${C.icon('lock')}<div>审核通过即<b>锁定</b>该批成绩并触发 GPA 聚合;锁定后教师不能直接改分,任何变更须经学生申诉 → 管理员解锁的受控回路,防止旁路改分。</div></div>
      <div class="grid grid-4 mb-4">
        ${C.stat('clock', String(pending), '待审批次', 'amber')}
        ${C.stat('lock', String(all.filter(r => r.status === '已通过').length), '已锁定批次', 'green')}
        ${C.stat('x-circle', String(all.filter(r => r.status === '已驳回').length), '已驳回批次', 'red')}
        ${C.stat('users', String(all.reduce((s, r) => s + r.count, 0)), '覆盖学生人次', 'blue')}
      </div>
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${C.saReviewFilter === k ? 'active' : ''}" onclick="Chaimir.saReviewSet('${k}')">${l}${k === '待审' && pending ? ` (${pending})` : ''}</a>`).join('')}</div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>课程</th><th>教师</th><th>班级</th><th>人数</th><th>平均分</th><th>不及格</th><th>报送时间</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${list.map(r => `
          <tr>
            <td class="fw-600">${C.esc(r.course)}</td>
            <td>${C.esc(r.teacher)}</td>
            <td>${C.esc(r.cls)}</td>
            <td>${r.count}</td>
            <td class="mono">${r.avg}</td>
            <td>${r.fail >= 6 ? C.statusDot('red', String(r.fail)) : C.statusDot('gray', String(r.fail))}</td>
            <td class="muted text-sm mono">${r.submit}</td>
            <td>${reviewBadge(r.status)}</td>
            <td class="row-actions">
              <button class="btn btn-ghost btn-sm" onclick="Chaimir.saReviewAct(${r.id},'detail')">${C.icon('eye')} 明细</button>
              ${r.status === '待审'
                ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.saReviewAct(${r.id},'reject')">驳回</button>
                   <button class="btn btn-primary btn-sm" onclick="Chaimir.saReviewAct(${r.id},'pass')">通过锁定</button>`
                : r.status === '已通过'
                ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.saReviewAct(${r.id},'unlock')">${C.icon('unlock')} 解锁回退</button>`
                : `<span class="muted text-xs">待教师重报</span>`}
            </td>
          </tr>`).join('')}</tbody>
      </table></div>
      ${C.pagination(1, list.length, 20)}`;
  }

  /* ============================================================
     ② 申诉处理(school-admin/appeals)
        回路:M11 解锁 → M6 改分 → 事件 → M11 重算重锁。
     ============================================================ */
  C.saAppeals = () => ([
    { id: 1, student: '孙浩然', no: '2023210458', course: '密码学基础与共识算法', item: '期末考试', cur: 58, claim: '主观题分数偏低', time: '2026-06-06 20:10', status: '待受理' },
    { id: 2, student: '周晓彤', no: '2023210459', course: '区块链原理与智能合约开发', item: '实验 3.3', cur: 72, claim: '判题用例环境异常', time: '2026-06-06 09:30', status: '处理中' },
    { id: 3, student: '郑梓萱', no: '2023210461', course: 'DeFi 协议开发与套利审计', item: '总评', cur: 81, claim: '作业分未计入', time: '2026-06-04 14:00', status: '已完成' },
    { id: 4, student: '吴俊杰', no: '2023210460', course: '密码学基础与共识算法', item: '期末考试', cur: 66, claim: '与标准答案一致', time: '2026-06-03 10:20', status: '已驳回' },
  ]);
  C.saAppealFilter = 'all';
  C.saAppealSet = (v) => { C.saAppealFilter = v; C.rerender(); };

  C.saAppealAct = async function (id, act) {
    const a = C.saAppeals().find(x => x.id == id); if (!a) return;
    if (act === 'accept') {
      if (await C.confirm({ title: '受理申诉(解锁改分通道)', confirmText: '受理并解锁',
        message: `受理「${a.student}」对《${a.course} · ${a.item}》的申诉:系统将解锁该项成绩,开放改分通道给任课教师(M6);教师改分后产生事件,系统自动重算 GPA 并重新锁定。确认受理?` }))
        C.toast('success', '已受理 · 改分通道已解锁', '已通知任课教师复核改分,改分后将自动重算重锁');
    } else if (act === 'reject') {
      C.saAppealReject(a);
    } else if (act === 'withdraw') {
      if (await C.confirm({ title: '撤回受理', danger: true, confirmText: '撤回',
        message: `撤回对「${a.student}」申诉的受理,将关闭已开放的改分通道并恢复原锁定状态。确认撤回?` }))
        C.toast('success', '已撤回受理', '改分通道已关闭,成绩恢复锁定');
    } else if (act === 'detail') {
      C.saAppealDetail(a);
    }
  };
  C.saAppealReject = function (a) {
    C.modal({
      title: '驳回申诉',
      body: `<p class="text-sm mb-3">驳回「${C.esc(a.student)}」对《${C.esc(a.course)} · ${C.esc(a.item)}》的申诉。</p>
        <div class="field"><label>驳回理由(将告知学生)<span class="req">*</span></label>
          <textarea class="textarea" id="ap-reason" placeholder="如:经复核评分与标准答案一致,维持原分"></textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-danger" onclick="(function(){var v=(document.getElementById('ap-reason')||{}).value;if(!v||!v.trim()){Chaimir.toast('error','请填写驳回理由','理由将通过通知告知学生');return}document.querySelector('.overlay').remove();Chaimir.toast('success','申诉已驳回','已通知学生处理结果')})()">确认驳回</button>`,
    });
  };
  C.saAppealDetail = function (a) {
    /* 受控回路可视化 */
    const steps = [
      { t: 'M11 解锁', d: '管理员受理 → 解锁该项成绩', icon: 'unlock' },
      { t: 'M6 改分', d: '任课教师复核并改分', icon: 'edit-3' },
      { t: '事件广播', d: '改分事件经事件总线发布', icon: 'radio' },
      { t: 'M11 重算重锁', d: '聚合层重算 GPA 并重新锁定', icon: 'lock' },
    ];
    C.drawer({
      title: '申诉详情',
      body: `<div class="dl mb-4">
          <dt>学生</dt><dd class="fw-600">${C.esc(a.student)} <span class="muted mono">${a.no}</span></dd>
          <dt>课程</dt><dd>${C.esc(a.course)}</dd>
          <dt>申诉项</dt><dd>${C.esc(a.item)}</dd>
          <dt>当前分数</dt><dd class="mono">${a.cur}</dd>
          <dt>申诉理由</dt><dd>${C.esc(a.claim)}</dd>
          <dt>提交时间</dt><dd class="mono">${a.time}</dd>
          <dt>状态</dt><dd>${appealBadge(a.status)}</dd>
        </div>
        <div class="section-title mb-3">改分受控回路</div>
        <div class="card card-pad">
          ${steps.map((s, i) => `<div class="flex items-center gap-3" style="padding:8px 0${i < steps.length - 1 ? ';border-bottom:1px solid var(--color-border)' : ''}">
            <div class="stat-icon" style="width:34px;height:34px;background:var(--amber-100);color:var(--amber-800)">${C.icon(s.icon)}</div>
            <div><div class="fw-600 text-sm">${i + 1}. ${s.t}</div><div class="muted text-xs">${s.d}</div></div></div>`).join('')}
        </div>
        <div class="callout info mt-3">${C.icon('shield')}<div>全程不允许直接改库:任何分数变更都经"解锁 → 改分 → 事件 → 重算重锁"闭环,且每一步记审计,确保成绩可追溯、不可旁路。</div></div>`,
      foot: a.status === '待受理' ? `<button class="btn btn-danger" onclick="document.querySelector('.overlay').remove();Chaimir.saAppealAct(${a.id},'reject')">驳回</button>
        <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.saAppealAct(${a.id},'accept')">受理并解锁</button>` : '',
    });
  };
  function appealBadge(s) {
    return { '待受理': C.badge('待受理', 'amber'), '处理中': C.badge('改分中', 'blue'), '已完成': C.badge('已完成', 'green'), '已驳回': C.badge('已驳回', 'red') }[s] || C.badge(s, 'gray');
  }

  function appeals() {
    const all = C.saAppeals();
    const tabs = [['all', '全部'], ['待受理', '待受理'], ['处理中', '改分中'], ['已完成', '已完成'], ['已驳回', '已驳回']];
    const list = C.saAppealFilter === 'all' ? all : all.filter(a => a.status === C.saAppealFilter);
    const pending = all.filter(a => a.status === '待受理').length;
    return `${C.head('申诉处理', '成绩')}
      <div class="callout info mb-4">${C.icon('info')}<div>申诉完整回路:<b>M11 解锁 → M6 改分 → 事件 → M11 重算重锁</b>。受理即解锁改分通道,改分后系统自动重算 GPA 并重新锁定;受理后、改分前可撤回。</div></div>
      <div class="grid grid-4 mb-4">
        ${C.stat('inbox', String(pending), '待受理', 'amber')}
        ${C.stat('edit-3', String(all.filter(a => a.status === '处理中').length), '改分处理中', 'blue')}
        ${C.stat('check-circle-2', String(all.filter(a => a.status === '已完成').length), '已完成', 'green')}
        ${C.stat('x-circle', String(all.filter(a => a.status === '已驳回').length), '已驳回', 'red')}
      </div>
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${C.saAppealFilter === k ? 'active' : ''}" onclick="Chaimir.saAppealSet('${k}')">${l}${k === '待受理' && pending ? ` (${pending})` : ''}</a>`).join('')}</div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>学生</th><th>课程</th><th>申诉项</th><th>当前分</th><th>理由</th><th>提交时间</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${list.map(a => `
          <tr>
            <td class="fw-600">${C.esc(a.student)}<div class="muted text-xs mono">${a.no}</div></td>
            <td>${C.esc(a.course)}</td>
            <td>${C.esc(a.item)}</td>
            <td class="mono">${a.cur}</td>
            <td class="ellipsis muted" style="max-width:160px">${C.esc(a.claim)}</td>
            <td class="muted text-sm mono">${a.time}</td>
            <td>${appealBadge(a.status)}</td>
            <td class="row-actions">
              <button class="btn btn-ghost btn-sm" onclick="Chaimir.saAppealAct(${a.id},'detail')">${C.icon('eye')} 详情</button>
              ${a.status === '待受理'
                ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.saAppealAct(${a.id},'reject')">驳回</button>
                   <button class="btn btn-primary btn-sm" onclick="Chaimir.saAppealAct(${a.id},'accept')">受理</button>`
                : a.status === '处理中'
                ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.saAppealAct(${a.id},'withdraw')">${C.icon('undo-2')} 撤回</button>`
                : `<span class="muted text-xs">已归档</span>`}
            </td>
          </tr>`).join('')}</tbody>
      </table></div>
      ${C.pagination(1, list.length, 20)}`;
  }

  /* ============================================================
     ③ 学业预警(school-admin/warnings)
        规则配置(挂科数 / GPA 阈值)+ 监控列表 + 周期扫描。
     ============================================================ */
  C.saWarnRules = { fail: 3, gpa: 2.0, scan: '每周一 02:00' };
  C.saWarnEdit = function () {
    C.modal({
      title: '预警规则配置',
      body: `<div class="callout info mb-4">${C.icon('info')}<div>满足任一规则即触发预警;预警将通知学生、班主任与学院,并计入学业预警监控。</div></div>
        <div class="field"><label>挂科门数阈值<span class="req">*</span></label>
          <div class="flex gap-2 items-center"><input class="input" type="number" id="wr-fail" value="${C.saWarnRules.fail}" style="width:120px"> <span class="muted text-sm">门及以上不及格触发预警</span></div></div>
        <div class="field"><label>GPA 阈值<span class="req">*</span></label>
          <div class="flex gap-2 items-center"><input class="input" type="number" step="0.1" id="wr-gpa" value="${C.saWarnRules.gpa}" style="width:120px"> <span class="muted text-sm">学期 GPA 低于此值触发预警</span></div></div>
        <div class="field"><label>扫描周期</label>
          <select class="select" id="wr-scan">
            <option ${C.saWarnRules.scan.includes('周一') ? 'selected' : ''}>每周一 02:00</option>
            <option>每日 02:00</option>
            <option>每月 1 日 02:00</option></select>
          <div class="help">周期扫描自动比对全校成绩,生成 / 更新预警名单</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="(function(){Chaimir.saWarnRules.fail=+(document.getElementById('wr-fail').value)||3;Chaimir.saWarnRules.gpa=+(document.getElementById('wr-gpa').value)||2;Chaimir.saWarnRules.scan=document.getElementById('wr-scan').value;document.querySelector('.overlay').remove();Chaimir.toast('success','预警规则已保存','下次周期扫描将按新规则执行');Chaimir.rerender()})()">保存规则</button>`,
    });
  };
  C.saWarnScan = async function () {
    if (await C.confirm({ title: '立即触发周期扫描', confirmText: '立即扫描',
      message: '将按当前规则对全校成绩进行一次全量扫描,生成 / 更新预警名单并发送通知。确认执行?' }))
      C.toast('success', '扫描已启动', '约需 1~2 分钟,完成后预警名单将自动刷新');
  };

  function warnings() {
    const r = C.saWarnRules;
    const list = [
      { name: '孙浩然', no: '2023210458', cls: '区块链 2301', fail: 4, gpa: 1.8, level: '严重', reason: '挂科 4 门 / GPA 1.8' },
      { name: '周晓彤', no: '2023210459', cls: '区块链 2301', fail: 3, gpa: 2.1, level: '一般', reason: '挂科 3 门' },
      { name: '吴俊杰', no: '2023210460', cls: '区块链 2401', fail: 2, gpa: 1.9, level: '一般', reason: 'GPA 1.9' },
    ];
    return `${C.head('学业预警', '成绩', `
      <button class="btn btn-outline" onclick="Chaimir.saWarnScan()">${C.icon('refresh-cw')} 立即扫描</button>
      <button class="btn btn-primary" onclick="Chaimir.saWarnEdit()">${C.icon('settings-2')} 预警规则</button>`)}
      <div class="grid grid-3 mb-4">
        <div class="card card-pad"><div class="muted text-sm mb-2">挂科门数阈值</div><div class="num" style="font-size:var(--text-2xl);font-weight:700">≥ ${r.fail} 门</div></div>
        <div class="card card-pad"><div class="muted text-sm mb-2">GPA 阈值</div><div class="num" style="font-size:var(--text-2xl);font-weight:700">< ${r.gpa.toFixed(1)}</div></div>
        <div class="card card-pad"><div class="muted text-sm mb-2">扫描周期</div><div class="flex items-center gap-2 mt-2">${C.icon('clock')}<span class="fw-600">${r.scan}</span></div></div>
      </div>
      <div class="card-head" style="border:1px solid var(--color-border);border-bottom:none;border-radius:var(--radius) var(--radius) 0 0">
        <div class="section-title">预警监控名单</div>
        <span class="badge badge-red">${C.icon('alert-triangle')} 命中 ${list.length} 人</span></div>
      <div class="table-wrap" style="border-radius:0 0 var(--radius) var(--radius)"><table class="table">
        <thead><tr><th>学生</th><th>班级</th><th>挂科门数</th><th>GPA</th><th>预警级别</th><th>命中原因</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${list.map(s => `
          <tr>
            <td class="fw-600">${C.esc(s.name)}<div class="muted text-xs mono">${s.no}</div></td>
            <td>${C.esc(s.cls)}</td>
            <td>${s.fail >= r.fail ? C.statusDot('red', String(s.fail)) : C.statusDot('gray', String(s.fail))}</td>
            <td class="mono">${s.gpa.toFixed(1)}</td>
            <td>${s.level === '严重' ? C.badge('严重', 'red') : C.badge('一般', 'amber')}</td>
            <td class="muted text-sm">${C.esc(s.reason)}</td>
            <td class="row-actions">
              <button class="btn btn-outline btn-sm" onclick="Chaimir.toast('success','已发送预警通知','已通知学生本人、班主任与学院')">${C.icon('send')} 通知</button>
              <button class="btn btn-ghost btn-sm" onclick="Chaimir.demo('查看学生成绩档案')">${C.icon('eye')}</button>
            </td>
          </tr>`).join('')}</tbody>
      </table></div>`;
  }

  /* ============================================================
     ④ 成绩配置(school-admin/grade-config)— Tab 切换
        等级映射 / 学期管理 / 成绩单批量生成。
     ============================================================ */
  C.saGcTab = 'grade';
  C.saGcSet = (v) => { C.saGcTab = v; C.rerender(); };
  C.saGradeMap = () => ([
    { min: 90, max: 100, level: 'A', gp: 4.0 },
    { min: 85, max: 89, level: 'A-', gp: 3.7 },
    { min: 80, max: 84, level: 'B+', gp: 3.3 },
    { min: 70, max: 79, level: 'B', gp: 3.0 },
    { min: 60, max: 69, level: 'C', gp: 2.0 },
    { min: 0, max: 59, level: 'F', gp: 0.0 },
  ]);
  C.saGcMapEdit = function () {
    C.modal({
      title: '新增 / 编辑等级映射',
      body: `<div class="grid grid-2">
          <div class="field"><label>分数下限<span class="req">*</span></label><input class="input" type="number" placeholder="如 90"></div>
          <div class="field"><label>分数上限<span class="req">*</span></label><input class="input" type="number" placeholder="如 100"></div>
          <div class="field"><label>等级<span class="req">*</span></label><input class="input" placeholder="如 A"></div>
          <div class="field"><label>绩点<span class="req">*</span></label><input class="input" type="number" step="0.1" placeholder="如 4.0"></div>
        </div>
        <div class="callout warn">${C.icon('alert-triangle')}<div>分数段需连续且不重叠,覆盖 0~100;保存后影响后续 GPA 计算,已锁定成绩不追溯重算。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','等级映射已保存','新规则将用于后续成绩计算')">保存</button>`,
    });
  };
  C.saGcSemEdit = function (name) {
    C.modal({
      title: name ? '编辑学期' : '创建学期',
      body: `<div class="field"><label>学期名称<span class="req">*</span></label><input class="input" value="${name ? C.esc(name) : ''}" placeholder="如 2026-2027 秋"></div>
        <div class="grid grid-2">
          <div class="field"><label>开始日期<span class="req">*</span></label><input class="input" type="date" value="2026-09-01"></div>
          <div class="field"><label>结束日期<span class="req">*</span></label><input class="input" type="date" value="2027-01-15"></div>
        </div>
        <label class="checkbox"><input type="checkbox" checked> 设为当前学期</label>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','${name ? '学期已更新' : '学期已创建'}','学期信息已保存')">保存</button>`,
    });
  };
  C.saGcGenTranscript = async function () {
    if (await C.confirm({ title: '批量生成成绩单', confirmText: '开始生成',
      message: '将为所选范围内全部学生生成本学期成绩单(PDF),生成期间不影响成绩查询。确认开始?' }))
      C.toast('success', '成绩单生成中', '完成后可在导出记录下载,约需数分钟');
  };

  function gradeConfig() {
    const tab = C.saGcTab;
    const tabs = [['grade', '等级映射'], ['semester', '学期管理'], ['transcript', '成绩单生成']];
    let body;
    if (tab === 'grade') {
      body = `<div class="flex justify-between items-center mb-3">
          <div class="muted text-sm">分数段 → 等级 → 绩点对照表(用于 GPA 计算)</div>
          <button class="btn btn-primary btn-sm" onclick="Chaimir.saGcMapEdit()">${C.icon('plus')} 新增等级</button></div>
        <div class="table-wrap"><table class="table">
          <thead><tr><th>分数区间</th><th>等级</th><th>绩点</th><th style="text-align:right">操作</th></tr></thead>
          <tbody>${C.saGradeMap().map(g => `
            <tr>
              <td class="mono">${g.min} ~ ${g.max}</td>
              <td>${C.badge(g.level, g.level === 'F' ? 'red' : g.gp >= 3.7 ? 'green' : 'gray')}</td>
              <td class="mono fw-600">${g.gp.toFixed(1)}</td>
              <td class="row-actions">
                <button class="btn btn-ghost btn-sm" onclick="Chaimir.saGcMapEdit()">${C.icon('pencil')}</button>
                <button class="btn btn-ghost btn-sm" onclick="Chaimir.saGcDelRow('${g.level}')">${C.icon('trash-2')}</button></td>
            </tr>`).join('')}</tbody>
        </table></div>`;
    } else if (tab === 'semester') {
      const sems = [
        { name: '2025-2026 春', range: '2026-02-24 ~ 2026-07-10', cur: true },
        { name: '2025-2026 秋', range: '2025-09-01 ~ 2026-01-15', cur: false },
        { name: '2024-2025 春', range: '2025-02-26 ~ 2025-07-12', cur: false },
      ];
      body = `<div class="flex justify-between items-center mb-3">
          <div class="muted text-sm">管理学期周期,当前学期决定成绩报送与统计归属</div>
          <button class="btn btn-primary btn-sm" onclick="Chaimir.saGcSemEdit()">${C.icon('plus')} 创建学期</button></div>
        <div class="table-wrap"><table class="table">
          <thead><tr><th>学期</th><th>起止日期</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
          <tbody>${sems.map(s => `
            <tr>
              <td class="fw-600">${C.esc(s.name)}</td>
              <td class="mono muted">${s.range}</td>
              <td>${s.cur ? C.statusDot('green', '当前学期') : C.statusDot('gray', '已结束')}</td>
              <td class="row-actions"><button class="btn btn-ghost btn-sm" onclick="Chaimir.saGcSemEdit('${C.esc(s.name)}')">${C.icon('pencil')}</button></td>
            </tr>`).join('')}</tbody>
        </table></div>`;
    } else {
      body = `<div class="card card-pad">
          <div class="section-title mb-3">批量生成成绩单</div>
          <div class="grid grid-2 mb-4">
            <div class="field"><label>学期</label><select class="select"><option>2025-2026 春</option><option>2025-2026 秋</option></select></div>
            <div class="field"><label>范围</label><select class="select"><option>全校</option><option>按学院</option><option>按班级</option></select></div>
          </div>
          <div class="callout info mb-4">${C.icon('info')}<div>成绩单按学校配置的展示名与 logo 生成 PDF,仅含已锁定成绩;未审核通过的成绩不计入。</div></div>
          <button class="btn btn-primary" onclick="Chaimir.saGcGenTranscript()">${C.icon('file-text')} 开始批量生成</button>
        </div>
        <div class="card mt-4"><div class="card-head"><div class="section-title">最近生成记录</div></div>
          <div class="table-wrap" style="border:none"><table class="table">
            <thead><tr><th>批次</th><th>范围</th><th>份数</th><th>状态</th><th>时间</th><th style="text-align:right">操作</th></tr></thead>
            <tbody>
              <tr><td class="mono">TR-20260601</td><td>区块链 2301</td><td>42</td><td>${C.statusDot('green', '已完成')}</td><td class="mono muted">2026-06-01 10:20</td><td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.demo('下载成绩单压缩包')">${C.icon('download')} 下载</button></td></tr>
              <tr><td class="mono">TR-20260520</td><td>全校</td><td>3,280</td><td>${C.statusDot('green', '已完成')}</td><td class="mono muted">2026-05-20 22:00</td><td class="row-actions"><button class="btn btn-outline btn-sm" onclick="Chaimir.demo('下载成绩单压缩包')">${C.icon('download')} 下载</button></td></tr>
            </tbody></table></div></div>`;
    }
    return `${C.head('成绩配置', '成绩')}
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${tab === k ? 'active' : ''}" onclick="Chaimir.saGcSet('${k}')">${l}</a>`).join('')}</div>
      ${body}`;
  }
  C.saGcDelRow = async function (level) {
    if (await C.confirm({ title: '删除等级', message: `删除等级「${level}」后分数段将出现空缺,请确保重新覆盖 0~100。确认删除?`, confirmText: '删除', danger: true }))
      C.toast('success', '等级已删除', '请检查分数段是否连续覆盖');
  };

  /* ============================================================
     ⑤ 学校配置(school-admin/config)
     ============================================================ */
  C.saCfgSwitch = (name) => C.toast('success', '设置已更新', `「${name}」已${event && event.target && event.target.checked ? '开启' : '关闭'}`);
  function config() {
    const features = [
      ['教学模块', '课程 / 作业 / 学习', true],
      ['实验模块', '代码实验与沙箱', true],
      ['竞赛模块', '解题赛 / 对抗赛', true],
      ['仿真实验室', '可视化共识 / 攻防仿真', true],
      ['学业预警', '挂科 / GPA 自动预警', true],
      ['讨论区', '课程内讨论与问答', false],
    ];
    return `${C.head('学校配置', '系统', `<button class="btn btn-primary" onclick="Chaimir.toast('success','配置已保存','学校配置已生效')">${C.icon('save')} 保存配置</button>`)}
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">基本信息</div></div>
          <div class="card-pad">
            <div class="field"><label>学校 Logo</label>
              <div class="flex items-center gap-3">
                <div style="width:64px;height:64px;border-radius:var(--radius);background:linear-gradient(135deg,var(--amber-400),var(--amber-600));display:grid;place-items:center;color:#1a1206;font-weight:800;font-size:22px">示</div>
                <button class="btn btn-outline btn-sm" onclick="Chaimir.demo('上传 Logo')">${C.icon('upload')} 更换 Logo</button></div>
              <div class="help">建议 256×256 PNG,用于登录页与成绩单页眉</div></div>
            <div class="field"><label>对外展示名<span class="req">*</span></label><input class="input" value="示例大学 · 区块链实训平台"></div>
            <div class="field"><label>学校标识码</label><input class="input mono" value="demo-univ" disabled>
              <div class="help">${C.icon('lock')} 标识码由平台分配,不可修改</div></div>
            <div class="field" style="margin:0"><label>联系邮箱</label><input class="input" value="edu@demo-univ.edu.cn"></div>
          </div></div>
        <div class="card"><div class="card-head"><div class="section-title">功能开关</div></div>
          <div class="card-pad">
            ${features.map(([n, d, on]) => `<div class="flex items-center justify-between" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
              <div><div class="fw-600 text-sm">${n}</div><div class="muted text-xs">${d}</div></div>
              <label class="switch"><input type="checkbox" ${on ? 'checked' : ''} onchange="Chaimir.saCfgSwitch('${n}')"><span class="track"></span></label></div>`).join('')}
          </div></div>
        <div class="card"><div class="card-head"><div class="section-title">认证方式</div></div>
          <div class="card-pad">
            <label class="radio" style="display:flex;padding:12px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:10px">
              <input type="radio" name="auth-mode" checked> <span><b>本地账号</b> · 平台自管账号与密码</span></label>
            <label class="radio" style="display:flex;padding:12px;border:1px solid var(--color-border);border-radius:var(--radius-sm);margin-bottom:10px">
              <input type="radio" name="auth-mode"> <span><b>学校 SSO</b> · CAS / LDAP 统一身份</span></label>
            <div class="flex items-center justify-between mt-2" style="padding:10px 0">
              <div><div class="fw-600 text-sm">允许混合登录</div><div class="muted text-xs">本地账号与 SSO 可同时启用</div></div>
              <label class="switch"><input type="checkbox" checked><span class="track"></span></label></div>
            <button class="btn btn-outline btn-block mt-2" onclick="Chaimir.navigate('school-admin/sso')">${C.icon('key-round')} 前往认证配置(CAS / LDAP)</button>
          </div></div>
        <div class="card"><div class="card-head"><div class="section-title">激活与开通</div></div>
          <div class="card-pad">
            <div class="flex items-center justify-between" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
              <div><div class="fw-600 text-sm">激活码开通</div><div class="muted text-xs">允许通过激活码自助激活账号</div></div>
              <label class="switch"><input type="checkbox" checked><span class="track"></span></label></div>
            <div class="flex items-center justify-between" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
              <div><div class="fw-600 text-sm">首次登录强制改密</div><div class="muted text-xs">初始密码开通的账号首登须改密</div></div>
              <label class="switch"><input type="checkbox" checked><span class="track"></span></label></div>
            <div class="flex items-center justify-between" style="padding:10px 0">
              <div><div class="fw-600 text-sm">禁止学生自助注册</div><div class="muted text-xs">账号仅由管理员导入(建议开启)</div></div>
              <label class="switch"><input type="checkbox" checked><span class="track"></span></label></div>
          </div></div>
      </div>`;
  }

  /* ============================================================
     ⑥ 认证配置(school-admin/sso)— CAS / LDAP,Tab 切换
        敏感字段(bind_password)脱敏;连接测试。
     ============================================================ */
  C.saSsoTab = 'cas';
  C.saSsoSet = (v) => { C.saSsoTab = v; C.rerender(); };
  C.saSsoTest = function (kind) {
    C.toast('info', '正在测试连接…', `向${kind === 'cas' ? 'CAS 服务器' : 'LDAP 目录'}发起连通性探测`);
    setTimeout(() => C.toast('success', '连接测试通过', `${kind === 'cas' ? 'CAS' : 'LDAP'} 服务可达,认证参数有效`), 1200);
  };
  function sso() {
    const tab = C.saSsoTab;
    const tabs = [['cas', 'CAS 单点登录'], ['ldap', 'LDAP 目录']];
    let body;
    if (tab === 'cas') {
      body = `<div class="card card-pad">
          <div class="callout info mb-4">${C.icon('info')}<div>配置学校 CAS Server,用户经学校统一认证页登录;系统按匹配字段核验本平台名单内账号。</div></div>
          <div class="field"><label>CAS Server URL<span class="req">*</span></label>
            <input class="input mono" value="https://sso.demo-univ.edu.cn/cas" placeholder="https://…/cas">
            <div class="help">学校统一身份认证服务地址</div></div>
          <div class="field"><label>匹配字段(match_field)<span class="req">*</span></label>
            <select class="select"><option>学号 / 工号(uid)</option><option>邮箱(mail)</option><option>姓名(cn)</option></select>
            <div class="help">用于将 CAS 返回的身份与本平台账号关联</div></div>
          <div class="field"><label>服务回调地址(Service)</label><input class="input mono" value="https://demo-univ.chaimir.edu/auth/cas/callback" disabled>
            <div class="help">${C.icon('lock')} 由平台固定生成,请在学校 CAS 白名单中放行</div></div>
          <div class="flex gap-2 mt-3">
            <button class="btn btn-outline" onclick="Chaimir.saSsoTest('cas')">${C.icon('plug-zap')} 测试连接</button>
            <button class="btn btn-primary" onclick="Chaimir.toast('success','CAS 配置已保存','下次登录将经学校统一认证')">保存配置</button></div>
        </div>`;
    } else {
      body = `<div class="card card-pad">
          <div class="callout warn mb-4">${C.icon('shield')}<div>LDAP 连接强制使用 <b>ldaps://</b> 加密通道;绑定密码以密文存储,界面仅脱敏展示,不回显明文。</div></div>
          <div class="field"><label>LDAP URL<span class="req">*</span></label>
            <input class="input mono" value="ldaps://ldap.demo-univ.edu.cn:636" placeholder="ldaps://…:636">
            <div class="help">${C.icon('lock')} 仅允许 ldaps(加密),拒绝明文 ldap</div></div>
          <div class="grid grid-2">
            <div class="field"><label>Bind DN<span class="req">*</span></label><input class="input mono" value="cn=admin,dc=demo-univ,dc=edu,dc=cn"></div>
            <div class="field"><label>Bind 密码<span class="req">*</span></label>
              <div class="input-icon">${C.icon('key-round')}<input class="input mono" type="password" value="************"></div>
              <div class="help">${C.icon('eye-off')} 已配置 · 脱敏显示,留空表示不修改</div></div>
          </div>
          <div class="field"><label>Base DN<span class="req">*</span></label><input class="input mono" value="ou=people,dc=demo-univ,dc=edu,dc=cn"></div>
          <div class="field"><label>用户过滤器(user_filter)<span class="req">*</span></label>
            <input class="input mono" value="(&(objectClass=person)(uid=%s))">
            <div class="help">%s 将替换为登录标识</div></div>
          <div class="field"><label>匹配字段(match)<span class="req">*</span></label>
            <select class="select"><option>uid</option><option>sAMAccountName</option><option>mail</option></select></div>
          <div class="flex gap-2 mt-3">
            <button class="btn btn-outline" onclick="Chaimir.saSsoTest('ldap')">${C.icon('plug-zap')} 测试连接</button>
            <button class="btn btn-primary" onclick="Chaimir.toast('success','LDAP 配置已保存','绑定密码已加密存储')">保存配置</button></div>
        </div>`;
    }
    return `${C.head('认证配置', '系统')}
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${tab === k ? 'active' : ''}" onclick="Chaimir.saSsoSet('${k}')">${l}</a>`).join('')}</div>
      ${body}`;
  }

  /* ============================================================
     ⑦ 审计日志(school-admin/audit)— 本校查询 + 导出
     ============================================================ */
  function audit() {
    const logs = [
      { op: '王校管', action: '通过并锁定成绩', obj: '成绩批次 · 区块链 2301', objType: '成绩', time: '2026-06-06 18:42', ip: '10.20.3.11' },
      { op: '王校管', action: '受理学生申诉', obj: '申诉 #1 · 孙浩然', objType: '申诉', time: '2026-06-06 20:15', ip: '10.20.3.11' },
      { op: '李教务', action: '批量导入学生', obj: '批次 IMP-20260606-03 · 43 人', objType: '账号', time: '2026-06-06 14:22', ip: '10.20.3.27' },
      { op: '王校管', action: '授予学校管理员', obj: '李明远(T2019033)', objType: '账号', time: '2026-06-05 09:10', ip: '10.20.3.11' },
      { op: '王校管', action: '修改 LDAP 配置', obj: '认证配置 · LDAP', objType: '配置', time: '2026-06-04 16:30', ip: '10.20.3.11' },
    ];
    const otBadge = { '成绩': 'green', '申诉': 'amber', '账号': 'blue', '配置': 'purple' };
    return `${C.head('审计日志', '系统', `<button class="btn btn-outline" onclick="Chaimir.toast('success','导出已开始','审计记录将导出为 CSV,完成后自动下载')">${C.icon('download')} 导出</button>`)}
      <div class="callout info mb-4">${C.icon('info')}<div>本校范围审计(全平台统一写入 identity 的 audit_log,此处只读本校记录);敏感值(密钥 / 密码 / token)已脱敏,不在审计中明文留存。</div></div>
      <div class="card card-pad mb-4">
        <div class="flex gap-3 wrap items-end">
          <div class="field" style="margin:0;width:160px"><label>操作人</label>
            <select class="select" onchange="Chaimir.demo('按操作人筛选')"><option>全部</option><option>王校管</option><option>李教务</option></select></div>
          <div class="field" style="margin:0;width:170px"><label>动作</label>
            <select class="select" onchange="Chaimir.demo('按动作筛选')"><option>全部</option><option>通过并锁定成绩</option><option>受理学生申诉</option><option>批量导入学生</option><option>修改配置</option></select></div>
          <div class="field" style="margin:0;width:140px"><label>对象类型</label>
            <select class="select" onchange="Chaimir.demo('按对象类型筛选')"><option>全部</option><option>成绩</option><option>申诉</option><option>账号</option><option>配置</option></select></div>
          <div class="field" style="margin:0;width:150px"><label>开始日期</label><input class="input" type="date" value="2026-06-01"></div>
          <div class="field" style="margin:0;width:150px"><label>结束日期</label><input class="input" type="date" value="2026-06-07"></div>
          <button class="btn btn-primary" onclick="Chaimir.demo('执行审计查询')">${C.icon('search')} 查询</button>
        </div>
      </div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>操作人</th><th>动作</th><th>对象</th><th>对象类型</th><th>来源 IP</th><th>时间</th></tr></thead>
        <tbody>${logs.map(l => `
          <tr>
            <td class="fw-600">${C.esc(l.op)}</td>
            <td>${C.esc(l.action)}</td>
            <td class="muted">${C.esc(l.obj)}</td>
            <td>${C.badge(l.objType, otBadge[l.objType] || 'gray')}</td>
            <td class="mono muted text-xs">${l.ip}</td>
            <td class="mono muted text-sm">${l.time}</td>
          </tr>`).join('')}</tbody>
      </table></div>
      ${C.pagination(1, logs.length, 20)}`;
  }

  /* ============================================================
     ⑧ 告警(school-admin/alerts)— 规则 / 事件 Tab
     ============================================================ */
  C.saAlertTab = 'events';
  C.saAlertEvFilter = 'all';
  C.saAlertSet = (k, v) => { C.saAlert_[k] = v; C.rerender(); };
  C.saAlert_ = { tab: 'events', ev: 'all' };
  C.saAlertEvAct = async function (id, act) {
    if (act === 'handle') {
      if (await C.confirm({ title: '处理告警', confirmText: '标记已处理', message: '确认该告警已处理完毕?处理结果将经通知同步给相关人员。' }))
        C.toast('success', '告警已处理', '已记录处理结果并通知相关人员');
    } else {
      if (await C.confirm({ title: '忽略告警', danger: true, confirmText: '忽略', message: '忽略后该告警不再提醒;如为误报可忽略。确认?' }))
        C.toast('success', '告警已忽略', '已从待处理列表移除');
    }
  };
  function alerts() {
    const s = C.saAlert_;
    const tabs = [['events', '告警事件'], ['rules', '告警规则']];
    let body;
    if (s.tab === 'events') {
      const evs = [
        { id: 1, level: '严重', title: '沙箱算力配额即将耗尽', detail: '本月已用 92%,预计 3 天内触顶', time: '20 分钟前', status: '待处理', icon: 'gauge' },
        { id: 2, level: '警告', title: '批量判题任务积压', detail: '判题队列等待超过 50 个任务', time: '1 小时前', status: '待处理', icon: 'list-checks' },
        { id: 3, level: '提示', title: 'LDAP 连接抖动', detail: '过去 1 小时出现 2 次超时', time: '3 小时前', status: '已处理', icon: 'plug' },
      ];
      const evFilter = [['all', '全部'], ['待处理', '待处理'], ['已处理', '已处理']];
      const list = s.ev === 'all' ? evs : evs.filter(e => e.status === s.ev);
      const lvBadge = { '严重': 'red', '警告': 'amber', '提示': 'blue' };
      body = `<div class="flex gap-1 mb-4" style="background:var(--color-surface-sunken);padding:3px;border-radius:var(--radius-sm);width:fit-content">
          ${evFilter.map(([k, l]) => `<button class="btn btn-sm ${s.ev === k ? 'btn-primary' : 'btn-ghost'}" onclick="Chaimir.saAlertSet('ev','${k}')">${l}</button>`).join('')}</div>
        ${list.map(e => `<div class="card card-pad mb-3 flex items-start gap-3">
          <div class="stat-icon" style="background:var(--${lvBadge[e.level] === 'red' ? 'red' : lvBadge[e.level] === 'amber' ? 'amber' : 'blue'}-100);color:var(--${lvBadge[e.level] === 'red' ? 'red' : lvBadge[e.level] === 'amber' ? 'amber' : 'blue'}-700)">${C.icon(e.icon)}</div>
          <div style="flex:1">
            <div class="flex items-center gap-2 wrap"><span class="fw-700">${C.esc(e.title)}</span>${C.badge(e.level, lvBadge[e.level])}${e.status === '已处理' ? C.badge('已处理', 'green') : C.badge('待处理', 'gray')}</div>
            <div class="muted text-sm mt-2">${C.esc(e.detail)}</div>
            <div class="muted text-xs mt-2">${C.icon('clock')} ${e.time}</div></div>
          ${e.status === '待处理' ? `<div class="flex gap-2">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.saAlertEvAct(${e.id},'handle')">${C.icon('check')} 处理</button>
            <button class="btn btn-ghost btn-sm" onclick="Chaimir.saAlertEvAct(${e.id},'ignore')">忽略</button></div>` : ''}
        </div>`).join('')}`;
    } else {
      const rules = [
        { name: '沙箱配额预警', cond: '月度用量 ≥ 90%', notify: '邮件 + 站内信', on: true },
        { name: '判题队列积压', cond: '等待任务 > 50', notify: '站内信', on: true },
        { name: 'SSO 连接异常', cond: '1 小时超时 ≥ 2 次', notify: '邮件', on: true },
        { name: '异常登录', cond: '异地登录 / 暴力破解', notify: '邮件 + 短信', on: false },
      ];
      body = `<div class="flex justify-between items-center mb-3">
          <div class="muted text-sm">触发条件命中即生成告警事件,并经通知模块下发</div>
          <button class="btn btn-primary btn-sm" onclick="Chaimir.demo('新增告警规则')">${C.icon('plus')} 新增规则</button></div>
        <div class="table-wrap"><table class="table">
          <thead><tr><th>规则名称</th><th>触发条件</th><th>通知方式</th><th>启用</th><th style="text-align:right">操作</th></tr></thead>
          <tbody>${rules.map(r => `
            <tr>
              <td class="fw-600">${C.esc(r.name)}</td>
              <td class="muted">${C.esc(r.cond)}</td>
              <td>${C.esc(r.notify)}</td>
              <td><label class="switch"><input type="checkbox" ${r.on ? 'checked' : ''} onchange="Chaimir.toast('success','规则已更新','${r.name} 已'+(this.checked?'启用':'停用'))"><span class="track"></span></label></td>
              <td class="row-actions"><button class="btn btn-ghost btn-sm" onclick="Chaimir.demo('编辑规则')">${C.icon('pencil')}</button></td>
            </tr>`).join('')}</tbody>
        </table></div>`;
    }
    return `${C.head('告警', '系统')}
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${s.tab === k ? 'active' : ''}" onclick="Chaimir.saAlertSet('tab','${k}')">${l}</a>`).join('')}</div>
      ${body}`;
  }

  /* ============================================================
     ⑨ 个人中心(school-admin/profile)
     ============================================================ */
  C.saPwd = function () {
    C.modal({
      title: '修改密码',
      body: `<div class="field"><label>当前密码<span class="req">*</span></label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="请输入当前密码"></div></div>
        <div class="field"><label>新密码<span class="req">*</span></label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="至少 8 位,含字母与数字"></div></div>
        <div class="field" style="margin:0"><label>确认新密码<span class="req">*</span></label><div class="input-icon">${C.icon('lock')}<input class="input" type="password" placeholder="再次输入"></div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','密码已修改','下次登录请使用新密码')">保存</button>`,
    });
  };
  C.saPhone = function () {
    C.modal({
      title: '换绑手机号',
      body: `<div class="field"><label>新手机号<span class="req">*</span></label><div class="input-icon">${C.icon('smartphone')}<input class="input" placeholder="请输入新手机号"></div></div>
        <div class="field" style="margin:0"><label>短信验证码<span class="req">*</span></label>
          <div class="flex gap-2"><div class="input-icon" style="flex:1">${C.icon('shield-check')}<input class="input" placeholder="6 位验证码"></div>
          <button class="btn btn-outline" onclick="Chaimir.authFn.sendSms(this)">获取验证码</button></div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','手机号已换绑','新手机号已生效')">确认换绑</button>`,
    });
  };
  function profile() {
    return `${C.head('个人中心', '账户')}
      <div class="grid grid-2">
        <div class="card"><div class="card-head"><div class="section-title">个人信息</div></div>
          <div class="card-pad">
            <div class="flex items-center gap-4 mb-4">
              <div style="width:64px;height:64px;border-radius:50%;background:linear-gradient(135deg,var(--amber-500),var(--amber-700));display:grid;place-items:center;color:#fff;font-weight:700;font-size:24px">王</div>
              <div><div class="fw-700" style="font-size:var(--text-lg)">王校管</div>
                <div class="muted text-sm mt-2">${C.badge('学校管理员', 'amber')} 示例大学</div></div>
            </div>
            <dl class="dl">
              <dt>姓名</dt><dd>王校管</dd>
              <dt>工号</dt><dd class="mono">A2018001</dd>
              <dt>所属</dt><dd>教务处</dd>
              <dt>手机号</dt><dd class="mono">139****2018</dd>
              <dt>邮箱</dt><dd>admin@demo-univ.edu.cn</dd>
              <dt>最近登录</dt><dd class="mono muted">2026-06-07 09:02 · 10.20.3.11</dd>
            </dl>
          </div></div>
        <div class="card"><div class="card-head"><div class="section-title">账号安全</div></div>
          <div class="card-pad">
            <div class="flex items-center justify-between" style="padding:12px 0;border-bottom:1px solid var(--color-border)">
              <div><div class="fw-600 text-sm">登录密码</div><div class="muted text-xs mt-2">上次修改 32 天前</div></div>
              <button class="btn btn-outline btn-sm" onclick="Chaimir.saPwd()">${C.icon('key-round')} 修改密码</button></div>
            <div class="flex items-center justify-between" style="padding:12px 0;border-bottom:1px solid var(--color-border)">
              <div><div class="fw-600 text-sm">绑定手机</div><div class="muted text-xs mt-2 mono">139****2018</div></div>
              <button class="btn btn-outline btn-sm" onclick="Chaimir.saPhone()">${C.icon('smartphone')} 换绑手机</button></div>
            <div class="flex items-center justify-between" style="padding:12px 0">
              <div><div class="fw-600 text-sm">登录设备</div><div class="muted text-xs mt-2">当前 1 台设备在线</div></div>
              <button class="btn btn-ghost btn-sm" onclick="Chaimir.demo('查看登录设备')">查看</button></div>
            <div class="callout info mt-3">${C.icon('shield')}<div>学校管理员为敏感角色,建议定期更换密码并开启异常登录告警。</div></div>
          </div></div>
      </div>`;
  }

  /* ---------- 注册路由 ---------- */
  C.registerPages({
    'school-admin/grade-reviews': gradeReviews,
    'school-admin/appeals': appeals,
    'school-admin/warnings': warnings,
    'school-admin/grade-config': gradeConfig,
    'school-admin/config': config,
    'school-admin/sso': sso,
    'school-admin/audit': audit,
    'school-admin/alerts': alerts,
    'school-admin/profile': profile,
  });
})();
