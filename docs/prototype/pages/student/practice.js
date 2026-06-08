/* ============================================================
   pages/student/practice.js — 学生·实践域(实验 / 竞赛 / 仿真实验室)
   ------------------------------------------------------------
   覆盖路由:
     · student/experiments      我的实验(列表:链栈/类型/状态/检查点/得分)
     · student/experiment-detail 实验详情(组件构成+协作模式+进入工作台)
     · student/contests          我的竞赛(列表:赛制/个人团队/报名/排名)
     · student/contest-detail    竞赛详情(规则+赛程时间轴+题目概览)
     · student/contest-signup    竞赛报名向导(个人/团队·邀请·锁定,草稿可续)
     · student/sim-lib           仿真实验室(检索仿真包+对比+回放)
     · student/my-records        我的战绩(跨竞赛名次/ELO/徽章,不计 GPA)
   风格:严格沿用 pages/student/courses.js 范式 —— registerPages 返回
        HTML 字符串、复用 C.* 工具、子页登记 parentRoute 高亮侧栏、
        行为走 C.mounts / 内联工具,文案面向用户、无 emoji、图标用 lucide。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项(沿用 courses.js 做法) */
  Object.assign(C.parentRoute, {
    'student/experiment-detail': 'student/experiments',
    'student/contest-detail': 'student/contests',
    'student/contest-signup': 'student/contests',
    'student/my-records': 'student/contests',
  });

  /* ========== 公共小工具:状态 → 徽章色 ========== */
  const expStatusBadge = { '未开始': 'gray', '进行中': 'amber', '已完成': 'blue', '已通过': 'green' };
  const contestStatusBadge = { '报名中': 'amber', '进行中': 'green', '已结束': 'gray' };
  const signupBadge = { '未报名': 'gray', '已报名': 'green', '已参赛': 'blue' };

  /* ============================================================
     一、我的实验(列表)
     ============================================================ */
  /* 单行实验:既给状态/检查点进度,也给"进入对应工作台"的入口。
     代码实验进 exp-ide,仿真进 sim —— 与导航约定一致。 */
  function experimentRow(e) {
    const isCode = e.kind === 'code';
    const pct = e.checkpoints ? Math.round((e.passed / e.checkpoints) * 100) : 0;
    const enterRoute = isCode ? 'immersive/exp-ide' : 'immersive/sim';
    const enterLabel = isCode ? '进入实验' : '启动仿真';
    const enterIcon = isCode ? 'code-2' : 'activity';
    const canEnter = e.status !== '已通过' && e.status !== '已完成';
    return `<tr>
      <td>
        <a class="fw-600" style="cursor:pointer" onclick="Chaimir.navigate('student/experiment-detail?id=${e.id}')">${C.esc(e.name)}</a>
        <div class="muted text-xs mt-2 flex items-center gap-2">${C.icon(isCode ? 'terminal' : 'box')} ${C.esc(e.stack)}</div>
      </td>
      <td>${isCode ? C.badge('代码实验', 'purple', 'code') : C.badge('仿真实验', 'teal', 'activity')}</td>
      <td>${C.badge(e.status, expStatusBadge[e.status] || 'gray')}</td>
      <td style="min-width:150px">
        <div class="flex items-center gap-2 text-xs muted mb-2"><span>检查点 ${e.passed}/${e.checkpoints}</span><span style="margin-left:auto">${pct}%</span></div>
        <div class="progress ${pct === 100 ? 'green' : ''}"><span style="width:${pct}%"></span></div>
      </td>
      <td class="mono">${e.score != null ? `<span class="fw-700" style="color:var(--green-700)">${e.score}</span><span class="muted text-xs"> 分</span>` : '<span class="muted">—</span>'}</td>
      <td class="row-actions">
        <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/experiment-detail?id=${e.id}')">详情</button>
        ${canEnter
          ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.navigate('${enterRoute}')">${C.icon(enterIcon)} ${enterLabel}</button>`
          : `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/experiment-detail?id=${e.id}')">${C.icon('rotate-ccw')} 查看结果</button>`}
      </td>
    </tr>`;
  }

  function experiments() {
    const total = m.experiments.length;
    const passed = m.experiments.filter(e => e.status === '已通过').length;
    const doing = m.experiments.filter(e => e.status === '进行中').length;
    return `${C.head('我的实验', '学习', `<button class="btn btn-outline" onclick="Chaimir.navigate('student/sim-lib')">${C.icon('activity')} 仿真实验室</button>`)}
      <div class="grid grid-3 mb-4">
        ${C.stat('flask-conical', total, '实验总数', 'amber')}
        ${C.stat('loader', doing, '进行中', 'blue')}
        ${C.stat('check-circle-2', passed, '已通过', 'green')}
      </div>
      <div class="callout info mb-4">${C.icon('info')}<div>实验来自所选课程的实践环节。代码实验在隔离沙箱中作答,环境用后即毁;仿真实验在浏览器内可视化推演,可反复重放。</div></div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>实验名称</th><th>类型</th><th>状态</th><th>检查点进度</th><th>得分</th><th></th></tr></thead>
        <tbody>${m.experiments.map(experimentRow).join('')}</tbody>
      </table></div>`;
  }

  /* ============================================================
     二、实验详情(子页)
     ============================================================ */
  function experimentDetail(ctx) {
    const e = m.experiments.find(x => x.id == ctx.query.id) || m.experiments[0];
    const isCode = e.kind === 'code';
    const enterRoute = isCode ? 'immersive/exp-ide' : 'immersive/sim';
    /* 组件构成(演示):若干环境 + 若干仿真 + 检查点及分值,合计应等于满分。
       不同实验侧重不同,这里按类型给出贴合区块链教学的构成。 */
    const components = isCode
      ? { envs: 1, sims: 0, envList: [['EVM 私链节点', e.stack]], checks: [
          ['部署合约通过编译', 20], ['复现重入/攻击路径', 30], ['修复并通过全部测试', 30], ['Gas 与安全检查', 20] ] }
      : { envs: 0, sims: 1, simList: [['共识推演沙盘', '图网络 + 时序泳道']], checks: [
          ['注入拜占庭节点', 30], ['观察三阶段达成共识', 40], ['给出容错结论', 30] ] };
    const cps = components.checks.slice(0, e.checkpoints);
    const totalScore = cps.reduce((s, c) => s + c[1], 0);
    return `${C.crumb([{ label: '我的实验', to: 'student/experiments' }, { label: '实验详情' }])}
      <div class="card card-pad mb-4" style="background:linear-gradient(120deg,var(--slate-900),var(--slate-800));color:var(--color-dark-text);border:none">
        <div class="flex justify-between wrap gap-3">
          <div>
            <div class="flex gap-2 mb-2">${C.badge(e.status, expStatusBadge[e.status] || 'gray')}${isCode ? C.badge('代码实验', 'purple') : C.badge('仿真实验', 'teal')}</div>
            <div style="font-size:var(--text-2xl);font-weight:700">${C.esc(e.name)}</div>
            <div style="color:var(--color-dark-text-sub);margin-top:6px" class="flex items-center gap-2">${C.icon(isCode ? 'terminal' : 'box')} ${C.esc(e.stack)} · 检查点 ${e.checkpoints} 个 · 满分 ${totalScore}</div>
          </div>
          <div style="text-align:right">
            <div style="font-size:var(--text-3xl);font-weight:700;color:var(--amber-400)">${e.passed}/${e.checkpoints}</div>
            <div style="color:var(--color-dark-text-sub);font-size:var(--text-sm)">检查点完成</div>
          </div>
        </div>
      </div>
      <div class="grid" style="grid-template-columns:1fr 320px">
        <div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">实验说明</div></div>
            <div class="card-pad text-sm" style="line-height:var(--leading-relaxed)">
              <p class="muted">${isCode
                ? '本实验在隔离的 EVM 私链沙箱中进行。你将复现合约重入漏洞的攻击路径,理解状态更新顺序对安全的影响,并采用「检查—生效—交互(CEI)」模式修复合约,直至通过全部检查点。'
                : '本实验提供可交互的共识推演沙盘。你将动手注入拜占庭(作恶)节点,逐步观察预准备、准备、提交三阶段如何在存在作恶节点时仍达成一致,并据此给出容错边界结论。'}</p>
              <div class="divider"></div>
              <div class="section-title mb-2" style="font-size:var(--text-base)">检查点与分值</div>
              ${cps.map((cp, i) => `<div class="flex items-center gap-3 text-sm" style="padding:9px 0;border-bottom:1px solid var(--color-border)">
                <span class="dot dot-${i < e.passed ? 'green' : 'gray'}"></span>
                <span style="flex:1">${i < e.passed ? C.icon('check') : C.icon('circle')} 检查点 ${i + 1} · ${C.esc(cp[0])}</span>
                <span class="badge badge-${i < e.passed ? 'green' : 'gray'}">${i < e.passed ? '已通过' : cp[1] + ' 分'}</span>
              </div>`).join('')}
              <div class="flex justify-between mt-3 fw-600 text-sm"><span>分值合计</span><span class="mono">${totalScore} 分</span></div>
            </div>
          </div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">组件构成预览</div><span class="muted text-xs">实例化时按此编排环境</span></div>
            <div class="card-pad">
              <div class="grid grid-3 mb-3">
                ${miniStat('boxes', components.envs, '链上环境')}
                ${miniStat('activity', components.sims, '仿真场景')}
                ${miniStat('flag', e.checkpoints, '检查点')}
              </div>
              ${(components.envList || []).map(([n, d]) => `<div class="flex items-center gap-3 text-sm" style="padding:8px 0">${C.icon('boxes')}<span class="fw-600">${C.esc(n)}</span><span class="muted" style="margin-left:auto">${C.esc(d)}</span></div>`).join('')}
              ${(components.simList || []).map(([n, d]) => `<div class="flex items-center gap-3 text-sm" style="padding:8px 0">${C.icon('activity')}<span class="fw-600">${C.esc(n)}</span><span class="muted" style="margin-left:auto">${C.esc(d)}</span></div>`).join('')}
            </div>
          </div>
        </div>
        <div>
          <div class="card card-pad mb-3">
            <div class="section-title mb-2">进入工作台</div>
            <p class="muted text-xs mb-3">发起实例后将为你分配独立沙箱,准备就绪即可进入作答。环境用后即毁,不影响他人。</p>
            <button class="btn btn-primary btn-block" onclick="Chaimir.studentStartExp('${enterRoute}')">${C.icon('play')} ${e.passed > 0 ? '进入工作台' : '发起实例并进入'}</button>
            <button class="btn btn-outline btn-block mt-2" onclick="Chaimir.demo('已重置环境')">${C.icon('rotate-ccw')} 重置实验环境</button>
          </div>
          <div class="card card-pad">
            <div class="section-title mb-2">协作模式</div>
            <div class="flex items-center gap-2 text-sm mb-2">${C.badge('单人实验', 'gray', 'user')}</div>
            <p class="muted text-xs" style="line-height:var(--leading-relaxed)">本实验为单人独立完成。每位同学拥有独立环境与检查点记录,提交与判分互不影响。需要小组协作的实验会在此标注「协作」并显示同组成员。</p>
          </div>
        </div>
      </div>`;
  }

  /* 详情页内嵌小统计(比 stat 卡更紧凑,用于组件构成预览) */
  function miniStat(ic, num, label) {
    return `<div class="card card-pad" style="text-align:center;padding:12px">
      <div style="display:inline-grid;place-items:center;width:34px;height:34px;border-radius:var(--radius);background:var(--amber-100);color:var(--amber-800);margin-bottom:6px">${C.icon(ic)}</div>
      <div class="fw-700" style="font-size:var(--text-xl)">${num}</div>
      <div class="muted text-xs">${label}</div></div>`;
  }

  /* 发起实例:模拟"环境准备中"再进入工作台 */
  C.studentStartExp = function (route) {
    C.toast('info', '正在准备实验环境', '正在分配独立沙箱,请稍候…');
    setTimeout(() => C.navigate(route), 900);
  };

  /* ============================================================
     三、我的竞赛(列表)
     ============================================================ */
  /* 单行竞赛:CTA 随状态变化 —— 报名中→报名;进行中→去答题/参战
     (对抗赛看回放走 battle-replay,解题赛走 solve);已结束→看战绩。 */
  function contestRow(c) {
    const isBattle = c.mode === '对抗赛';
    let cta = '';
    if (c.status === '报名中') {
      cta = c.signup === '未报名'
        ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.navigate('student/contest-signup?id=${c.id}')">${C.icon('user-plus')} 去报名</button>`
        : `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/contest-detail?id=${c.id}')">查看详情</button>`;
    } else if (c.status === '进行中') {
      cta = isBattle
        ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.navigate('immersive/battle-replay')">${C.icon('swords')} 去参战</button>`
        : `<button class="btn btn-primary btn-sm" onclick="Chaimir.navigate('immersive/solve')">${C.icon('terminal')} 去答题</button>`;
    } else {
      cta = `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('student/my-records')">${C.icon('bar-chart-3')} 看战绩</button>`;
    }
    return `<tr>
      <td>
        <a class="fw-600" style="cursor:pointer" onclick="Chaimir.navigate('student/contest-detail?id=${c.id}')">${C.esc(c.name)}</a>
        <div class="muted text-xs mt-2 flex items-center gap-2">${C.icon('users')} ${c.players} 人参与</div>
      </td>
      <td>${isBattle ? C.badge('对抗赛', 'red', 'swords') : C.badge('解题赛', 'blue', 'flag')}</td>
      <td>${c.team === '团队' ? C.badge('团队赛', 'purple', 'users') : C.badge('个人赛', 'gray', 'user')}</td>
      <td>${C.badge(c.status, contestStatusBadge[c.status] || 'gray')}</td>
      <td>${C.badge(c.signup, signupBadge[c.signup] || 'gray')}</td>
      <td class="mono">${c.rank != null ? `<span class="fw-700" style="color:var(--amber-700)">第 ${c.rank} 名</span>` : '<span class="muted">—</span>'}</td>
      <td class="row-actions">${cta}</td>
    </tr>`;
  }

  function contests() {
    const joined = m.contests.filter(c => c.signup !== '未报名').length;
    const ranks = m.contests.filter(c => c.rank != null).map(c => c.rank);
    const best = ranks.length ? Math.min.apply(null, ranks) : null;
    return `${C.head('我的竞赛', '学习', `<button class="btn btn-outline" onclick="Chaimir.navigate('student/my-records')">${C.icon('trophy')} 我的战绩</button>`)}
      <div class="grid grid-3 mb-4">
        ${C.stat('trophy', m.contests.length, '可参与赛事', 'amber')}
        ${C.stat('flag', joined, '已报名/参赛', 'blue')}
        ${C.stat('medal', best != null ? '第 ' + best + ' 名' : '—', '历史最佳名次', 'green')}
      </div>
      <div class="callout warn mb-4">${C.icon('info')}<div>竞赛成绩用于天梯与荣誉展示,<b>不计入课程成绩与 GPA</b>。对抗赛在隔离环境对局,解题赛在沙箱中提交判定。</div></div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>赛事名称</th><th>赛制</th><th>形式</th><th>状态</th><th>报名</th><th>我的排名</th><th></th></tr></thead>
        <tbody>${m.contests.map(contestRow).join('')}</tbody>
      </table></div>`;
  }

  /* ============================================================
     四、竞赛详情(子页)
     ============================================================ */
  function contestDetail(ctx) {
    const c = m.contests.find(x => x.id == ctx.query.id) || m.contests[0];
    const isBattle = c.mode === '对抗赛';
    /* 赛程时间轴(演示):报名 → 开赛 → 封榜 → 颁奖,状态点体现当前阶段 */
    const phaseIdx = c.status === '报名中' ? 0 : c.status === '进行中' ? 1 : 3;
    const timeline = [
      ['报名开放', '2026-06-01 ~ 06-09', 'flag'],
      ['正式开赛', '2026-06-10 09:00', 'play'],
      ['封榜阶段', '赛程最后 1 小时', 'lock'],
      ['成绩公布与颁奖', '2026-06-12 20:00', 'award'],
    ];
    /* 题目概览(演示):贴合区块链 CTF / 解题语境 */
    const probs = isBattle
      ? [['金库重入渗透', '攻防', '动态分'], ['闪电贷价格操纵', '攻防', '动态分'], ['访问控制绕过', '攻防', '动态分']]
      : [['Gas 优化:存储打包', '解题', '300'], ['整数溢出修复', '解题', '250'], ['默克尔证明校验', '解题', '400'], ['代理升级漏洞', '解题', '450']];
    const rules = isBattle
      ? [['动态计分', '题目分值随被攻破次数动态衰减,先攻破者得分更高'], ['攻防对局', '红队渗透、蓝队加固,虚拟机实时裁决重入/越权'], ['天梯结算', '对局按 ELO 结算,计入个人/战队天梯']]
      : [['动态分', '解出人数越多,该题分值越低,鼓励攻克难题'], ['封榜', '赛程最后 1 小时隐藏实时榜单,结果公布时揭晓'], ['一血加成', '每题首个通过者获额外荣誉加成(不影响他人分值)']];
    if (c.team === '团队') rules.push(['跨校组队', '允许邀请其他学校的平台用户跨校组队,队长统一提交']);
    return `${C.crumb([{ label: '我的竞赛', to: 'student/contests' }, { label: '竞赛详情' }])}
      <div class="card card-pad mb-4" style="background:linear-gradient(120deg,var(--slate-900),var(--slate-800));color:var(--color-dark-text);border:none">
        <div class="flex justify-between wrap gap-3">
          <div>
            <div class="flex gap-2 mb-2">${C.badge(c.status, contestStatusBadge[c.status] || 'gray')}${isBattle ? C.badge('对抗赛', 'red') : C.badge('解题赛', 'blue')}${c.team === '团队' ? C.badge('团队赛', 'purple') : C.badge('个人赛', 'gray')}</div>
            <div style="font-size:var(--text-2xl);font-weight:700">${C.esc(c.name)}</div>
            <div style="color:var(--color-dark-text-sub);margin-top:6px" class="flex items-center gap-2">${C.icon('users')} ${c.players} 人参与 · 报名状态:${c.signup}</div>
          </div>
          <div style="text-align:right;align-self:center">
            ${c.status === '报名中' && c.signup === '未报名'
              ? `<button class="btn btn-primary btn-lg" onclick="Chaimir.navigate('student/contest-signup?id=${c.id}')">${C.icon('user-plus')} 立即报名</button>`
              : c.status === '进行中'
                ? `<button class="btn btn-primary btn-lg" onclick="Chaimir.navigate('${isBattle ? 'immersive/battle-replay' : 'immersive/solve'}')">${C.icon(isBattle ? 'swords' : 'terminal')} 进入赛场</button>`
                : `<button class="btn btn-on-dark btn-lg" onclick="Chaimir.navigate('student/my-records')">${C.icon('bar-chart-3')} 查看战绩</button>`}
          </div>
        </div>
      </div>
      <div class="grid" style="grid-template-columns:1fr 340px">
        <div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">赛制说明</div></div>
            <div class="card-pad text-sm muted" style="line-height:var(--leading-relaxed)">
              ${isBattle
                ? '对抗赛采用红蓝攻防对局形式:你需在隔离的链上环境中,既要渗透对手的合约金库,又要加固自身合约抵御攻击。系统虚拟机实时裁决每一次交易,并按 ELO 结算天梯名次。'
                : '解题赛采用积分制:在限定赛程内攻克尽可能多的题目。每题提供独立沙箱环境,提交答案后由判题器即时判定;分值随通过人数动态调整。'}
            </div>
          </div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">竞赛规则</div></div>
            <div class="card-pad">${rules.map(([t, d]) => `<div class="flex gap-3 text-sm" style="padding:9px 0;border-bottom:1px solid var(--color-border)">
              <span style="display:grid;place-items:center;width:26px;height:26px;border-radius:var(--radius-sm);background:var(--amber-100);color:var(--amber-800);flex-shrink:0">${C.icon('check')}</span>
              <div><div class="fw-600">${C.esc(t)}</div><div class="muted text-xs mt-2">${C.esc(d)}</div></div></div>`).join('')}</div>
          </div>
          <div class="card"><div class="card-head"><div class="section-title">题目概览</div><span class="muted text-xs">题面在开赛后解锁</span></div>
            <div class="table-wrap" style="border:none"><table class="table">
              <thead><tr><th>题目</th><th>类别</th><th>分值</th><th>状态</th></tr></thead>
              <tbody>${probs.map(([t, cat, sc]) => `<tr><td class="fw-600">${C.esc(t)}</td><td>${C.badge(cat, cat === '攻防' ? 'red' : 'blue')}</td><td class="mono">${C.esc(sc)}</td><td>${c.status === '报名中' ? C.badge('未解锁', 'gray', 'lock') : C.badge('可作答', 'green')}</td></tr>`).join('')}</tbody>
            </table></div>
          </div>
        </div>
        <div>
          <div class="card card-pad mb-3">
            <div class="section-title mb-3">赛程时间轴</div>
            <ol style="position:relative;padding-left:6px">${timeline.map(([t, d, ic], i) => `
              <li class="flex gap-3" style="padding-bottom:${i < timeline.length - 1 ? '16px' : '0'};position:relative">
                <span style="display:grid;place-items:center;width:28px;height:28px;border-radius:50%;flex-shrink:0;background:${i <= phaseIdx ? 'var(--amber-700)' : 'var(--slate-100)'};color:${i <= phaseIdx ? '#fff' : 'var(--slate-500)'};z-index:1">${C.icon(ic)}</span>
                ${i < timeline.length - 1 ? `<span style="position:absolute;left:13px;top:28px;bottom:0;width:2px;background:${i < phaseIdx ? 'var(--amber-500)' : 'var(--color-border)'}"></span>` : ''}
                <div><div class="fw-600 text-sm">${C.esc(t)} ${i === phaseIdx ? C.badge('当前', 'amber') : ''}</div><div class="muted text-xs mt-2">${C.esc(d)}</div></div>
              </li>`).join('')}</ol>
          </div>
          <div class="card card-pad">
            <div class="section-title mb-2">报名入口</div>
            ${c.signup === '未报名'
              ? `<p class="muted text-xs mb-3">${c.team === '团队' ? '本赛为团队赛,可建队并邀请队员(支持跨校)。' : '本赛为个人赛,确认信息后即可报名。'}报名过程支持中断续编。</p>
                 <button class="btn btn-primary btn-block" onclick="Chaimir.navigate('student/contest-signup?id=${c.id}')">${C.icon('user-plus')} 开始报名</button>`
              : `<div class="callout success" style="border-color:var(--color-success)">${C.icon('check-circle-2')}<div>你已完成报名(${c.signup})。开赛后可在「我的竞赛」进入赛场。</div></div>`}
          </div>
        </div>
      </div>`;
  }

  /* ============================================================
     五、竞赛报名向导(子页 · 需持久化)
     ------------------------------------------------------------
     步骤:① 选择形式(个人/团队)② 队伍与邀请 ③ 确认锁定
     持久化:草稿存于服务端(原型用模块内 draft 模拟),刷新/换设备
     不丢失;.autosave 指示保存态,顶部 callout 说明"可中断续编"。
     ============================================================ */
  /* 报名草稿(模拟服务端权威态;真实环境为后端持久化的向导中间态) */
  C._signupDraft = C._signupDraft || { step: 1, type: 'team', teamName: '', invites: [], agree: false };

  /* 标记草稿"已保存到服务端"(与 courses.js 作业草稿同款指示器手法) */
  function signupSave(manual) {
    const ind = document.getElementById('signup-autosave');
    if (ind) {
      ind.className = 'autosave saving'; ind.innerHTML = C.icon('loader') + ' 正在保存…'; C.refreshIcons();
      setTimeout(() => { ind.className = 'autosave saved'; ind.innerHTML = C.icon('cloud') + ' 草稿已保存到服务端'; C.refreshIcons(); if (manual) C.toast('success', '报名草稿已保存', '可随时关闭,稍后或换设备继续'); }, 500);
    } else if (manual) {
      C.toast('success', '报名草稿已保存', '可随时关闭,稍后或换设备继续');
    }
  }

  /* 步骤条:用 components.css 的 .steps/.step 体系(active/done 修饰) */
  function signupSteps(cur) {
    const labels = ['选择形式', '队伍与邀请', '确认锁定'];
    return `<div class="steps">${labels.map((l, i) => {
      const n = i + 1;
      const cls = n < cur ? 'done' : n === cur ? 'active' : '';
      return `<div class="step ${cls}">
        <span class="dot-n">${n < cur ? C.icon('check') : n}</span>
        <span class="step-label">${l}</span>
        ${i < labels.length - 1 ? '<span class="line"></span>' : ''}
      </div>`;
    }).join('')}</div>`;
  }

  function contestSignup(ctx) {
    const c = m.contests.find(x => x.id == ctx.query.id) || m.contests.find(x => x.status === '报名中') || m.contests[1];
    const d = C._signupDraft;
    /* 团队赛默认走团队流程;个人赛锁定为个人形式 */
    const teamAllowed = c.team === '团队';
    if (!teamAllowed) d.type = 'solo';
    const step = d.step;
    let panel = '';

    if (step === 1) {
      panel = `<div class="card card-pad">
        <div class="section-title mb-3">选择参赛形式</div>
        <div class="grid grid-2">
          ${signupTypeCard('solo', 'user', '个人报名', '以个人身份参赛,独立计分与排名。', d.type === 'solo', true)}
          ${signupTypeCard('team', 'users', '团队报名', '创建队伍并邀请队员,队长统一提交,支持跨校组队。', d.type === 'team', teamAllowed)}
        </div>
        ${!teamAllowed ? `<div class="callout info mt-4">${C.icon('info')}<div>本赛事为个人赛,仅支持个人报名。</div></div>` : ''}
      </div>`;
    } else if (step === 2) {
      if (d.type === 'solo') {
        panel = `<div class="card card-pad">
          <div class="section-title mb-3">确认个人信息</div>
          <dl class="dl">
            <dt>姓名</dt><dd>${C.esc(m.me.name)}</dd>
            <dt>学号</dt><dd class="mono">${C.esc(m.me.no)}</dd>
            <dt>班级</dt><dd>${C.esc(m.me.class)}</dd>
            <dt>学院</dt><dd>${C.esc(m.me.dept)}</dd>
          </dl>
          <div class="callout info mt-4">${C.icon('shield')}<div>学籍信息由学校管理员维护,如有错误请联系学校,报名信息以此为准。</div></div>
        </div>`;
      } else {
        panel = `<div class="card card-pad mb-3">
          <div class="field"><label>队伍名称 <span class="req">*</span></label>
            <div class="input-icon">${C.icon('flag')}<input class="input" id="team-name" placeholder="如:拜占庭幻象" value="${C.esc(d.teamName)}" oninput="Chaimir._signupDraft.teamName=this.value;Chaimir.studentSignupDirty()"></div>
            <div class="help">队伍名将展示在天梯与对局中,创建后可由队长修改</div></div>
          <div class="field" style="margin-bottom:0"><label>邀请码</label>
            <div class="flex gap-2">
              <div class="input-icon" style="flex:1">${C.icon('ticket')}<input class="input mono" value="TEAM-7F3K9Q" readonly></div>
              <button class="btn btn-outline" onclick="Chaimir.toast('success','邀请码已复制','把它发给队友,对方输入即可加入')">${C.icon('copy')} 复制</button>
            </div>
            <div class="help">队友可凭邀请码加入;也可在下方按学号/手机号定向邀请</div></div>
        </div>
        <div class="card">
          <div class="card-head"><div class="section-title">邀请队员</div><button class="btn btn-primary btn-sm" onclick="Chaimir.studentInviteMember()">${C.icon('user-plus')} 邀请队员</button></div>
          <div class="card-pad">
            <div class="callout warn mb-3">${C.icon('info')}<div>支持<b>跨校邀请</b>:被邀请人需为本平台已注册用户,接受邀请后方可加入。每队最多 4 人。</div></div>
            <div id="invite-list">${renderInviteList()}</div>
          </div>
        </div>`;
      }
    } else {
      /* 确认锁定 */
      const members = d.type === 'team' ? [{ name: m.me.name, no: m.me.no, role: '队长', school: '本校' }].concat(d.invites) : [];
      panel = `<div class="card card-pad mb-3">
        <div class="section-title mb-3">确认报名信息</div>
        <dl class="dl">
          <dt>参赛赛事</dt><dd class="fw-600">${C.esc(c.name)}</dd>
          <dt>参赛形式</dt><dd>${d.type === 'team' ? C.badge('团队报名', 'purple') : C.badge('个人报名', 'gray')}</dd>
          ${d.type === 'team' ? `<dt>队伍名称</dt><dd class="fw-600">${C.esc(d.teamName || '(未填写)')}</dd>` : ''}
          ${d.type === 'team' ? `<dt>队伍成员</dt><dd>${members.length} 人</dd>` : `<dt>参赛人</dt><dd>${C.esc(m.me.name)} · ${C.esc(m.me.no)}</dd>`}
        </dl>
        ${d.type === 'team' && members.length ? `<div class="table-wrap mt-3"><table class="table">
          <thead><tr><th>成员</th><th>学号</th><th>归属</th><th>角色</th></tr></thead>
          <tbody>${members.map(mm => `<tr><td class="fw-600">${C.esc(mm.name)}</td><td class="mono">${C.esc(mm.no)}</td><td>${mm.school === '本校' ? C.badge('本校', 'gray') : C.badge(mm.school, 'blue')}</td><td>${mm.role === '队长' ? C.badge('队长', 'amber') : C.badge('队员', 'gray')}</td></tr>`).join('')}</tbody>
        </table></div>` : ''}
      </div>
      <div class="card card-pad">
        <label class="checkbox" style="align-items:flex-start">
          <input type="checkbox" ${d.agree ? 'checked' : ''} onchange="Chaimir._signupDraft.agree=this.checked;Chaimir.studentSignupDirty()">
          <span class="text-sm">我已阅读并同意<a style="color:var(--color-primary-text);cursor:pointer" onclick="event.preventDefault();Chaimir.demo('查看竞赛章程')">《竞赛参赛章程与公平竞赛承诺》</a>,确认报名信息真实有效。</span>
        </label>
        <div class="callout warn mt-3">${C.icon('lock')}<div>提交后将<b>锁定报名</b>:个人信息与队伍成员不可再更改。${d.type === 'team' ? '团队赛需所有受邀成员接受邀请后方可正式生效。' : ''}</div></div>
      </div>`;
    }

    return `${C.crumb([{ label: '我的竞赛', to: 'student/contests' }, { label: '竞赛详情', to: 'student/contest-detail?id=' + c.id }, { label: '报名' }])}
      <div class="content-head">
        <div><div class="page-sub">${C.esc(c.name)}</div><h1 class="page-title">竞赛报名</h1></div>
        <div class="content-actions">
          <span class="autosave saved" id="signup-autosave">${C.icon('cloud')} 草稿已保存到服务端</span>
          <button class="btn btn-outline" onclick="Chaimir.studentSignupSave(true)">${C.icon('save')} 保存草稿</button>
        </div>
      </div>
      <div class="callout info mb-4">${C.icon('info')}<div>报名为多步流程,中间状态<b>实时保存到服务端</b>。你可以随时关闭,稍后或换设备从上次步骤继续。</div></div>
      ${signupSteps(step)}
      ${panel}
      <div class="flex justify-between mt-4">
        <button class="btn btn-outline" ${step <= 1 ? 'disabled' : ''} onclick="Chaimir.studentSignupStep(${step - 1})">${C.icon('chevron-left')} 上一步</button>
        ${step < 3
          ? `<button class="btn btn-primary" onclick="Chaimir.studentSignupNext(${step})">下一步 ${C.icon('chevron-right')}</button>`
          : `<button class="btn btn-primary" onclick="Chaimir.studentSignupConfirm()">${C.icon('check')} 确认并锁定报名</button>`}
      </div>`;
  }

  /* 形式选择卡(可点选;不可用时置灰) */
  function signupTypeCard(val, ic, title, desc, active, enabled) {
    return `<label class="card card-pad ${enabled ? 'card-hover' : ''}" style="display:block;${active ? 'border-color:var(--color-primary);box-shadow:0 0 0 1px var(--color-primary)' : ''};${enabled ? '' : 'opacity:.5;pointer-events:none'}"
      onclick="${enabled ? `Chaimir._signupDraft.type='${val}';Chaimir.studentSignupDirty();Chaimir.rerender()` : ''}">
      <div class="flex items-center gap-3 mb-2">
        <span style="display:grid;place-items:center;width:38px;height:38px;border-radius:var(--radius);background:${active ? 'var(--amber-700)' : 'var(--amber-100)'};color:${active ? '#fff' : 'var(--amber-800)'}">${C.icon(ic)}</span>
        <span class="fw-700">${title}</span>
        ${active ? `<span style="margin-left:auto;color:var(--color-primary-text)">${C.icon('check-circle-2')}</span>` : ''}
      </div>
      <p class="muted text-xs" style="line-height:var(--leading-relaxed)">${desc}</p>
    </label>`;
  }

  /* 渲染已邀请队员列表 */
  function renderInviteList() {
    const d = C._signupDraft;
    if (!d.invites.length) {
      return `<div class="empty" style="padding:24px"><div class="empty-ico">${C.icon('user-plus')}</div>
        <div class="empty-title">还没有邀请队员</div><div class="empty-desc">通过邀请码或定向邀请添加队友,可跨校组队</div></div>`;
    }
    return d.invites.map((mm, i) => `<div class="flex items-center gap-3" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
      <span style="width:32px;height:32px;border-radius:50%;background:linear-gradient(135deg,var(--amber-500),var(--amber-700));color:#fff;display:grid;place-items:center;font-size:var(--text-sm);font-weight:600;flex-shrink:0">${C.esc(mm.name.slice(0, 1))}</span>
      <div style="min-width:0"><div class="fw-600 text-sm">${C.esc(mm.name)} ${mm.school !== '本校' ? C.badge(mm.school, 'blue') : ''}</div><div class="muted text-xs mono">${C.esc(mm.no)}</div></div>
      <span style="margin-left:auto" class="badge badge-amber">${C.icon('clock')} 待对方接受</span>
      <button class="btn btn-ghost btn-sm" title="移除" onclick="Chaimir.studentRemoveInvite(${i})">${C.icon('x')}</button>
    </div>`).join('');
  }

  /* 邀请队员弹窗(支持按学号/手机号 + 跨校提示) */
  C.studentInviteMember = function () {
    if (C._signupDraft.invites.length >= 3) { C.toast('info', '队伍已满', '含队长每队最多 4 人'); return; }
    C.modal({
      title: '邀请队员',
      body: `<div class="field"><label>学号或手机号 <span class="req">*</span></label>
          <div class="input-icon">${C.icon('search')}<input class="input" id="inv-key" placeholder="输入对方学号或手机号"></div>
          <div class="help">仅可邀请已注册本平台的用户</div></div>
        <div class="field" style="margin-bottom:0"><label>所属学校</label>
          <select class="select" id="inv-school"><option value="本校">本校(示例大学)</option><option value="滨海理工">滨海理工大学</option><option value="云岭师院">云岭师范学院</option></select>
          <div class="help">支持跨校组队;选择对方所属学校以便定位账号</div></div>
        <div class="callout warn mt-3">${C.icon('info')}<div>发送邀请后,对方需在站内信中确认接受,方可加入队伍。</div></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="Chaimir.studentDoInvite()">${C.icon('send')} 发送邀请</button>`,
      onMount: (root) => { const el = root.querySelector('#inv-key'); if (el) el.focus(); }
    });
  };
  C.studentDoInvite = function () {
    const keyEl = document.getElementById('inv-key');
    const schEl = document.getElementById('inv-school');
    const key = keyEl ? keyEl.value : '';
    const school = schEl ? schEl.value : '本校';
    if (!key.trim()) { C.toast('error', '请输入学号或手机号', '邀请对象不能为空'); return; }
    /* 演示:用输入造一个待接受成员 */
    const names = ['周晓彤', '吴俊杰', '郑梓萱', '何子墨'];
    const nm = names[C._signupDraft.invites.length % names.length];
    C._signupDraft.invites.push({ name: nm, no: key.trim(), school: school, role: '队员' });
    const ov = document.querySelector('.overlay'); if (ov) ov.remove();
    const list = document.getElementById('invite-list'); if (list) { list.innerHTML = renderInviteList(); C.refreshIcons(); }
    signupSave(false);
    C.toast('success', '邀请已发送', school === '本校' ? '等待对方在站内信确认' : '跨校邀请已发送,等待对方确认');
  };
  C.studentRemoveInvite = function (i) {
    C._signupDraft.invites.splice(i, 1);
    const list = document.getElementById('invite-list'); if (list) { list.innerHTML = renderInviteList(); C.refreshIcons(); }
    signupSave(false);
  };

  /* 向导控制:切步 / 下一步(带最小校验)/ 保存 / 提交 */
  C.studentSignupStep = function (n) { C._signupDraft.step = Math.max(1, Math.min(3, n)); C.rerender(); };
  C.studentSignupNext = function (cur) {
    const d = C._signupDraft;
    if (cur === 2 && d.type === 'team' && !d.teamName.trim()) { C.toast('error', '请先填写队伍名称', '队伍名称为必填项'); return; }
    signupSave(false);
    C.studentSignupStep(cur + 1);
  };
  C.studentSignupSave = function (manual) { signupSave(manual); };
  C.studentSignupDirty = function () {
    const ind = document.getElementById('signup-autosave');
    if (ind) { ind.className = 'autosave saving'; ind.innerHTML = C.icon('loader') + ' 未保存的更改…'; C.refreshIcons(); }
    clearTimeout(C._signupDirtyTimer);
    C._signupDirtyTimer = setTimeout(() => signupSave(false), 1200);
  };
  C.studentSignupConfirm = async function () {
    const d = C._signupDraft;
    if (!d.agree) { C.toast('error', '请先勾选同意', '需阅读并同意参赛章程方可锁定报名'); return; }
    if (await C.confirm({ title: '确认锁定报名', message: '锁定后报名信息不可更改,确认提交?', confirmText: '确认锁定' })) {
      C.toast('success', '报名成功', d.type === 'team' ? '队伍已创建,等待受邀成员确认' : '已完成报名,开赛后即可进入赛场');
      /* 提交后清空草稿(对应服务端草稿转正式记录) */
      C._signupDraft = { step: 1, type: 'team', teamName: '', invites: [], agree: false };
      setTimeout(() => C.navigate('student/contests'), 900);
    }
  };

  /* ============================================================
     六、仿真实验室
     ============================================================ */
  /* 仿真包目录(贴合教学:图网络/共识/默克尔树/Gossip 传播等)。
     字段对齐 mock 风格:类别/版本/性能边界提示 → 启动进 sim。 */
  const simPackages = [
    { id: 'p2p-graph', name: 'P2P 节点拓扑与连通性', cat: '图网络', ver: 'v2.1.0', icon: 'share-2',
      desc: '可视化节点加入/退出、连边与分区,观察网络连通度与消息可达性。', limit: '≤ 200 节点 / 60 fps' },
    { id: 'pbft', name: 'PBFT 三阶段共识推演', cat: '共识', ver: 'v3.0.1', icon: 'vote',
      desc: '注入拜占庭节点,逐步推演预准备—准备—提交三阶段如何容错达成一致。', limit: '≤ 16 副本 / 实时' },
    { id: 'merkle', name: '默克尔树与轻节点验证', cat: '数据结构', ver: 'v1.4.2', icon: 'git-merge',
      desc: '构建默克尔树,演示证明路径与篡改检测,理解轻节点如何低成本验证。', limit: '≤ 1024 叶子 / 实时' },
    { id: 'gossip', name: 'Gossip 区块传播仿真', cat: '网络传播', ver: 'v1.2.0', icon: 'radio',
      desc: '模拟区块在网络中按 Gossip 协议扩散,观察传播延迟与覆盖曲线。', limit: '≤ 500 节点 / 30 fps' },
    { id: 'pow-fork', name: 'PoW 出块与分叉竞争', cat: '共识', ver: 'v2.3.0', icon: 'pickaxe',
      desc: '调节算力分布,观察最长链规则下的分叉产生、收敛与孤块。', limit: '≤ 8 矿工 / 实时' },
    { id: 'dht', name: 'Kademlia DHT 路由查找', cat: '图网络', ver: 'v1.0.3', icon: 'network',
      desc: '演示分布式哈希表的 K 桶与异或距离寻址,逐跳收敛到目标节点。', limit: '≤ 256 节点 / 实时' },
  ];

  function simCard(p) {
    return `<div class="card card-hover" style="display:flex;flex-direction:column">
      <div class="card-pad" style="flex:1">
        <div class="flex items-center gap-3 mb-3">
          <span style="display:grid;place-items:center;width:42px;height:42px;border-radius:var(--radius);background:var(--teal-100);color:var(--teal-700);flex-shrink:0">${C.icon(p.icon)}</span>
          <div style="min-width:0"><div class="fw-700 ellipsis" style="font-size:var(--text-md)">${C.esc(p.name)}</div>
            <div class="flex gap-2 mt-2">${C.badge(p.cat, 'teal')}<span class="badge badge-gray mono">${C.esc(p.ver)}</span></div></div>
        </div>
        <p class="muted text-sm" style="line-height:var(--leading-relaxed);min-height:42px">${C.esc(p.desc)}</p>
        <div class="callout info mt-3" style="padding:8px 10px">${C.icon('gauge')}<div class="text-xs">性能边界:${C.esc(p.limit)}。超出规模将提示降采样以保流畅。</div></div>
      </div>
      <div class="flex gap-2" style="padding:12px 18px;border-top:1px solid var(--color-border)">
        <button class="btn btn-primary btn-sm" style="flex:1" onclick="Chaimir.navigate('immersive/sim')">${C.icon('play')} 启动仿真</button>
        <button class="btn btn-outline btn-sm" title="加入对比" onclick="Chaimir.studentSimCompareAdd('${p.id}')">${C.icon('columns-2')}</button>
        <button class="btn btn-ghost btn-sm" title="回放剧本" onclick="Chaimir.studentSimReplay('${C.esc(p.name)}')">${C.icon('history')}</button>
      </div>
    </div>`;
  }

  /* 对比篮(模块态;演示并排对比能力) */
  C._simCompare = C._simCompare || [];
  C.studentSimCompareAdd = function (id) {
    if (C._simCompare.indexOf(id) >= 0) { C.toast('info', '已在对比列表', '该仿真包已加入并排对比'); return; }
    if (C._simCompare.length >= 3) { C.toast('info', '对比已满', '最多并排对比 3 个仿真包'); return; }
    C._simCompare.push(id);
    C.toast('success', '已加入对比', '已选 ' + C._simCompare.length + ' 个,点右上「并排对比」查看');
    const b = document.getElementById('sim-compare-count'); if (b) b.textContent = C._simCompare.length;
  };
  C.studentSimReplay = function (name) {
    C.modal({
      title: '回放与分享剧本',
      body: `<p class="text-sm mb-3">为 <b>${C.esc(name)}</b> 选择一个历史剧本回放,或分享你的操作剧本给同学复现。</p>
        ${[['课堂演示 · 注入 2 个拜占庭节点', '李明远', '4 分 12 秒'], ['我的实验 · 分区后恢复', '我', '2 分 03 秒'], ['共享剧本 · 极端分叉竞争', '王思齐', '6 分 30 秒']].map(([t, who, dur]) => `
          <div class="flex items-center gap-3" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
            <span style="display:grid;place-items:center;width:32px;height:32px;border-radius:var(--radius-sm);background:var(--teal-100);color:var(--teal-700);flex-shrink:0">${C.icon('clapperboard')}</span>
            <div style="flex:1;min-width:0"><div class="fw-600 text-sm">${t}</div><div class="muted text-xs mt-2">${C.icon('user')} ${who} · 时长 ${dur}</div></div>
            <button class="btn btn-outline btn-sm" onclick="document.querySelector('.overlay').remove();Chaimir.navigate('immersive/sim')">${C.icon('play')} 回放</button>
          </div>`).join('')}`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">关闭</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','分享链接已生成','链接已复制,同学打开即可复现你的剧本')">${C.icon('share-2')} 分享我的剧本</button>`,
    });
  };
  C.studentOpenCompare = function () {
    if (C._simCompare.length < 2) { C.toast('info', '请先添加对比项', '在仿真卡片上点对比图标,至少选 2 个'); return; }
    const sel = simPackages.filter(p => C._simCompare.indexOf(p.id) >= 0);
    C.modal({
      title: '并排对比', size: 'lg',
      body: `<div class="grid" style="grid-template-columns:repeat(${sel.length},1fr)">${sel.map(p => `
        <div class="card card-pad"><div class="flex items-center gap-2 mb-2">${C.icon(p.icon)}<span class="fw-700 text-sm">${C.esc(p.name)}</span></div>
          <dl class="dl" style="grid-template-columns:64px 1fr">
            <dt>类别</dt><dd>${C.badge(p.cat, 'teal')}</dd>
            <dt>版本</dt><dd class="mono">${C.esc(p.ver)}</dd>
            <dt>边界</dt><dd class="text-xs">${C.esc(p.limit)}</dd>
          </dl>
          <button class="btn btn-primary btn-sm btn-block mt-3" onclick="document.querySelector('.overlay').remove();Chaimir.navigate('immersive/sim')">${C.icon('play')} 启动</button>
        </div>`).join('')}</div>`,
      foot: `<button class="btn btn-outline" onclick="Chaimir._simCompare=[];document.querySelector('.overlay').remove();Chaimir.toast('info','已清空对比')">清空对比</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove()">完成</button>`,
    });
  };

  function simLib() {
    const cats = ['全部', '图网络', '共识', '数据结构', '网络传播'];
    return `${C.head('仿真实验室', '学习', `<button class="btn btn-outline" onclick="Chaimir.studentOpenCompare()">${C.icon('columns-2')} 并排对比 <span id="sim-compare-count" class="count">${C._simCompare.length}</span></button>`)}
      <div class="callout info mb-4">${C.icon('info')}<div>仿真在浏览器内可视化推演,不消耗链上资源,可反复重放与分享剧本。每个仿真包标注了性能边界,超出规模会自动降采样以保证流畅。</div></div>
      <div class="flex gap-3 mb-4 wrap items-center">
        <div class="input-icon" style="max-width:320px;flex:1">${C.icon('search')}<input class="input" placeholder="搜索仿真包(如 共识 / 默克尔 / 拓扑)" oninput="Chaimir.demo('原型搜索')"></div>
        <div class="tabs" style="border:none;margin:0">${cats.map((t, i) => `<a class="tab ${i === 0 ? 'active' : ''}" onclick="Chaimir.demo('按类别筛选')">${t}</a>`).join('')}</div>
      </div>
      <div class="grid grid-3">${simPackages.map(simCard).join('')}</div>`;
  }

  /* ============================================================
     七、我的战绩(子页 · 从竞赛进入)
     ------------------------------------------------------------
     跨竞赛战绩:竞赛/名次/得分/天梯 ELO/徽章;用 mock.ladder 体现
     天梯;明确说明竞赛成绩不计入 GPA。
     ============================================================ */
  function myRecords() {
    const me = m.ladder.find(x => x.me) || m.ladder[m.ladder.length - 1];
    /* 我参加过的竞赛战绩(贴合 contests 语境 + 解题赛示例) */
    const records = [
      { name: '跨链桥安全攻防联赛', mode: '对抗赛', rank: 3, total: 120, score: 2680, badge: '季军' },
      { name: '「链上夺旗」金库重入渗透赛', mode: '对抗赛', rank: 7, total: 64, score: 1820, badge: '八强' },
      { name: '智能合约审计精英赛', mode: '解题赛', rank: 12, total: 88, score: 3150, badge: '优胜' },
    ];
    const bestRank = Math.min.apply(null, records.map(r => r.rank));
    /* 徽章墙 */
    const badges = [
      ['季军', 'medal', 'amber', '跨链桥安全攻防联赛'],
      ['一血猎手', 'flag', 'red', '首杀 3 道渗透题'],
      ['全勤选手', 'calendar-check', 'blue', '连续 3 届参赛'],
      ['重入克星', 'shield', 'green', '攻防赛拦截 10 次重入'],
    ];
    return `${C.crumb([{ label: '我的竞赛', to: 'student/contests' }, { label: '我的战绩' }])}
      ${C.head('我的战绩', '消息')}
      <div class="callout warn mb-4">${C.icon('info')}<div>以下为跨竞赛的累计战绩与天梯表现,用于荣誉与排名展示,<b>不计入课程成绩与 GPA</b>。</div></div>
      <div class="grid grid-4 mb-4">
        ${C.stat('swords', records.length, '参赛场次', 'amber')}
        ${C.stat('medal', '第 ' + bestRank + ' 名', '最佳名次', 'green')}
        ${C.stat('trending-up', me.elo, '当前天梯 ELO', 'purple')}
        ${C.stat('award', badges.length, '获得徽章', 'blue')}
      </div>
      <div class="grid" style="grid-template-columns:1fr 360px">
        <div>
          <div class="card mb-3"><div class="card-head"><div class="section-title">竞赛战绩</div><span class="muted text-xs">仅展示已结束/已参赛的赛事</span></div>
            <div class="table-wrap" style="border:none"><table class="table">
              <thead><tr><th>赛事</th><th>赛制</th><th>名次</th><th>得分</th><th>荣誉</th></tr></thead>
              <tbody>${records.map(r => `<tr>
                <td class="fw-600">${C.esc(r.name)}</td>
                <td>${r.mode === '对抗赛' ? C.badge('对抗赛', 'red') : C.badge('解题赛', 'blue')}</td>
                <td><span class="fw-700" style="color:var(--amber-700)">第 ${r.rank}</span><span class="muted text-xs"> / ${r.total}</span></td>
                <td class="mono">${r.score}</td>
                <td>${C.badge(r.badge, r.rank <= 3 ? 'amber' : 'gray', 'medal')}</td>
              </tr>`).join('')}</tbody>
            </table></div>
          </div>
          <div class="card"><div class="card-head"><div class="section-title">天梯榜(对抗赛)</div><span class="muted text-xs">按 ELO 排名</span></div>
            <div class="card-pad">${m.ladder.map(l => `
              <div class="flex items-center gap-3" style="padding:10px 0;border-bottom:1px solid var(--color-border);${l.me ? 'background:var(--amber-50);margin:0 -18px;padding-left:18px;padding-right:18px' : ''}">
                <span class="fw-700 mono" style="width:34px;text-align:center;color:${l.rank <= 3 ? 'var(--amber-700)' : 'var(--color-text-sub)'}">${l.rank <= 3 ? C.icon('crown') : '#' + l.rank}</span>
                <span class="fw-600 text-sm" style="flex:1">${C.esc(l.name)} ${l.me ? C.badge('我', 'amber') : ''}</span>
                <span class="muted text-xs">${l.win}胜 ${l.lose}负</span>
                <span class="badge badge-purple mono">ELO ${l.elo}</span>
              </div>`).join('')}</div>
          </div>
        </div>
        <div>
          <div class="card card-pad mb-3">
            <div class="section-title mb-3">我的徽章墙</div>
            <div class="grid grid-2" style="gap:12px">${badges.map(([t, ic, color, tip]) => `
              <div style="text-align:center;padding:14px 8px;border:1px solid var(--color-border);border-radius:var(--radius)" title="${C.esc(tip)}">
                <div style="display:inline-grid;place-items:center;width:48px;height:48px;border-radius:50%;background:var(--${color}-100);color:var(--${color}-700);margin-bottom:8px">${C.icon(ic)}</div>
                <div class="fw-600 text-sm">${t}</div>
                <div class="muted text-xs mt-2">${C.esc(tip)}</div>
              </div>`).join('')}</div>
          </div>
          <div class="card card-pad">
            <div class="section-title mb-2">关于天梯 ELO</div>
            <p class="muted text-xs" style="line-height:var(--leading-relaxed)">ELO 是对抗赛的相对实力评分:每场对局后,战胜更强对手获得更多分,惜败强敌扣分更少。它只反映竞技水平,与课程绩点完全独立。</p>
            <button class="btn btn-outline btn-block mt-3" onclick="Chaimir.navigate('student/contests')">${C.icon('trophy')} 去参加新赛事</button>
          </div>
        </div>
      </div>`;
  }

  /* ============================================================
     注册路由
     ============================================================ */
  C.registerPages({
    'student/experiments': experiments,
    'student/experiment-detail': experimentDetail,
    'student/contests': contests,
    'student/contest-detail': contestDetail,
    'student/contest-signup': contestSignup,
    'student/sim-lib': simLib,
    'student/my-records': myRecords,
  });
})();
