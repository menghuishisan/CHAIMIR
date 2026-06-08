/* ============================================================
   pages/teacher/practice.js — 教师·实践域(实验 / 竞赛 / 监控 / 防作弊 / 漏洞源)
   ------------------------------------------------------------
   覆盖:实验管理、实验编排向导(6 步,服务端草稿持久化)、竞赛管理、
        竞赛配置、出题向导(持久化)、实时监控(状态矩阵 + 一键阻断)、
        防作弊审查、漏洞源管理、漏洞题转化工作台(6 步预验证)。
        对应 M7 实验、M8 竞赛、M3 判题/查重、M5 内容(漏洞转化)。
   范式:列表 C.head、子页 C.crumb + C.parentRoute;向导用 .steps +
        ?step=N(C.navigate 带 query),每步可"保存草稿退出"(.autosave)。
   ============================================================ */
(function () {
  const C = window.Chaimir;
  const m = C.mock;

  /* 子页 → 高亮的侧栏项 */
  Object.assign(C.parentRoute, {
    'teacher/exp-wizard': 'teacher/experiments',
    'teacher/contest-edit': 'teacher/contests',
    'teacher/contest-problems': 'teacher/contests',
    'teacher/cheat-review': 'teacher/monitor',
    'teacher/vuln-sources': 'teacher/content',
    'teacher/vuln-transform': 'teacher/content',
  });

  /* ============================================================
     1) 实验管理(列表)
     ============================================================ */
  const experiments = [
    { id: 1, name: 'PoW 挖矿与 51% 算力攻击', course: '区块链原理与智能合约', status: '已发布', collab: '单人', template: 'M5 · evm-hardhat 模板', cp: 3 },
    { id: 2, name: '重入漏洞利用与防护(CEI)', course: '智能合约安全攻防实训', status: '进行中', collab: '单人', template: 'M5 · evm-foundry 模板', cp: 5 },
    { id: 3, name: 'PBFT 拜占庭容错共识交互', course: '密码学基础与共识算法', status: '已发布', collab: '小组(3 人)', template: 'M4 · 仿真包', cp: 4 },
    { id: 4, name: '跨链桥签名验证漏洞', course: 'DeFi 协议开发与套利审计', status: '草稿', collab: '单人', template: '未选择', cp: 0 },
    { id: 5, name: '闪电贷套利与价格操纵', course: 'DeFi 协议开发与套利审计', status: '已下架', collab: '小组(2 人)', template: 'M5 · evm-foundry 模板', cp: 6 },
  ];
  C.tExperiments = experiments;
  const expStatusBadge = (s) => C.badge(s, { '草稿': 'gray', '已发布': 'blue', '进行中': 'green', '已下架': 'gray' }[s] || 'gray');

  function experimentsList(ctx) {
    const filter = ctx.query.st || 'all';
    const tabs = [['all', '全部'], ['进行中', '进行中'], ['已发布', '已发布'], ['草稿', '草稿'], ['已下架', '已下架']];
    const rows = experiments.filter(e => filter === 'all' || e.status === filter);
    return `${C.head('实验管理', '实践', `<button class="btn btn-primary" onclick="Chaimir.navigate('teacher/exp-wizard?step=1')">${C.icon('plus')} 新建实验</button>`)}
      <div class="grid grid-4 mb-4">
        ${C.stat('flask-conical', experiments.length, '实验总数', 'amber')}
        ${C.stat('play-circle', experiments.filter(e => e.status === '进行中').length, '进行中', 'green')}
        ${C.stat('users', experiments.filter(e => e.collab.indexOf('小组') === 0).length, '小组协作', 'blue')}
        ${C.stat('file-edit', experiments.filter(e => e.status === '草稿').length, '草稿', 'gray')}
      </div>
      <div class="tabs">${tabs.map(([k, l]) => `<a class="tab ${k === filter ? 'active' : ''}" onclick="Chaimir.navigate('teacher/experiments?st=${encodeURIComponent(k)}')">${l}</a>`).join('')}</div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>实验名称</th><th>所属课程</th><th>状态</th><th>协作模式</th><th>检查点</th><th>模板引用</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${rows.map(e => `<tr>
          <td class="fw-600">${e.name}</td>
          <td class="muted text-sm">${e.course}</td>
          <td>${expStatusBadge(e.status)}</td>
          <td>${C.badge(e.collab, e.collab.indexOf('小组') === 0 ? 'purple' : 'gray')}</td>
          <td class="mono">${e.cp}</td>
          <td class="muted text-xs">${e.template}</td>
          <td class="row-actions">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/exp-wizard?step=1&id=${e.id}')">${C.icon('pencil')} 编辑</button>
            ${e.status === '草稿' ? `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/exp-wizard?step=6&id=${e.id}')">${C.icon('shield-check')} 校验</button>`
              : e.status === '已下架' ? `<button class="btn btn-primary btn-sm" onclick="Chaimir.toast('success','已重新上架','学生可再次进入实验')">${C.icon('arrow-up-circle')} 上架</button>`
              : `<button class="btn btn-ghost btn-sm" onclick="Chaimir.tExpUnpublish('${C.esc(e.name)}')">${C.icon('arrow-down-circle')} 下架</button>`}
          </td>
        </tr>`).join('')}</tbody></table></div>`;
  }
  C.tExpUnpublish = async function (name) {
    if (await C.confirm({ title: '下架实验', message: '下架后学生无法进入「' + name + '」,进行中的实例不受影响。确认下架?', confirmText: '下架', danger: true }))
      C.toast('success', '已下架', name + ' 已对学生隐藏');
  };

  /* ============================================================
     2) 实验编排向导(6 步 · 服务端草稿持久化)
     ============================================================ */
  const expSteps = ['基础信息', '环境组件', '仿真组件', '检查点', '说明与协作', '校验发布'];

  /* 步骤条 */
  function stepBar(cur, id) {
    return `<div class="steps">${expSteps.map((label, i) => {
      const n = i + 1; const cls = n < cur ? 'done' : n === cur ? 'active' : '';
      return `<div class="step ${cls}" style="cursor:pointer" onclick="Chaimir.navigate('teacher/exp-wizard?step=${n}${id ? '&id=' + id : ''}')">
        <span class="dot-n">${n < cur ? C.icon('check') : n}</span><span class="step-label">${label}</span>
        ${n < expSteps.length ? '<span class="line"></span>' : ''}</div>`;
    }).join('')}</div>`;
  }
  /* 向导底部:上一步 / 保存草稿退出 / 下一步;autosave 指示 */
  function wizardFoot(cur, id, lastLabel) {
    const q = (n) => 'teacher/exp-wizard?step=' + n + (id ? '&id=' + id : '');
    return `<div class="flex justify-between items-center mt-4" style="padding-top:16px;border-top:1px solid var(--color-border)">
      <span class="autosave saved" id="wiz-autosave">${C.icon('cloud')} 草稿已自动保存到服务端</span>
      <div class="flex gap-2">
        ${cur > 1 ? `<button class="btn btn-outline" onclick="Chaimir.navigate('${q(cur - 1)}')">${C.icon('chevron-left')} 上一步</button>` : ''}
        <button class="btn btn-outline" onclick="Chaimir.tWizSaveExit()">${C.icon('save')} 保存草稿退出</button>
        ${cur < expSteps.length ? `<button class="btn btn-primary" onclick="Chaimir.navigate('${q(cur + 1)}')">下一步 ${C.icon('chevron-right')}</button>`
          : `<button class="btn btn-primary" onclick="${lastLabel}">${C.icon('send')} 发布实验</button>`}
      </div></div>`;
  }
  C.tWizSaveExit = function () { C.toast('success', '草稿已保存', '可在实验列表中继续编辑,换设备不丢失'); setTimeout(() => C.navigate('teacher/experiments'), 700); };
  /* 进入向导后,模拟"自动保存"心跳 */
  C.mounts['teacher/exp-wizard'] = function () {
    C._wizTimer && clearInterval(C._wizTimer);
    C._wizTimer = setInterval(() => {
      if (!location.hash.includes('teacher/exp-wizard')) { clearInterval(C._wizTimer); return; }
      const ind = document.getElementById('wiz-autosave'); if (!ind) return;
      ind.className = 'autosave saving'; ind.innerHTML = C.icon('loader') + ' 正在保存…'; C.refreshIcons();
      setTimeout(() => { ind.className = 'autosave saved'; ind.innerHTML = C.icon('cloud') + ' 草稿已自动保存到服务端'; C.refreshIcons(); }, 600);
    }, 45000);
  };

  function expWizard(ctx) {
    const step = parseInt(ctx.query.step || '1', 10);
    const id = ctx.query.id || '';
    let body = '';
    if (step === 1) {
      /* ① 基础信息 / 选 M5 模板 */
      body = `<div class="grid" style="grid-template-columns:1fr 320px">
        <div class="card card-pad">
          <div class="section-title mb-3">基础信息</div>
          <div class="field"><label>实验名称<span class="req">*</span></label><input class="input" placeholder="如:重入漏洞利用与防护(CEI)"></div>
          <div class="field"><label>所属课程<span class="req">*</span></label><select class="select"><option>智能合约安全攻防实训</option><option>区块链原理与智能合约</option></select></div>
          <div class="grid grid-2"><div class="field"><label>难度</label><select class="select"><option>进阶</option><option>高级</option><option>入门</option></select></div>
            <div class="field"><label>预计时长</label><input class="input" placeholder="如 90 分钟"></div></div>
          <div class="field" style="margin-bottom:0"><label>实验目标</label><textarea class="textarea" placeholder="学生完成本实验后应掌握…"></textarea></div>
        </div>
        <div class="card card-pad"><div class="section-title mb-3">从 M5 选择实验模板</div>
          ${[['evm-foundry 重入实验模板', 'v1.3.0', true], ['evm-hardhat 通用合约模板', 'v2.1.0', false], ['不使用模板,从零编排', '', false]].map(([t, v, on], i) => `
            <label class="radio mb-2" style="display:flex;justify-content:space-between;padding:11px;border:1px solid var(--${on ? 'color-primary' : 'color-border'});border-radius:var(--radius-sm);${on ? 'background:var(--color-primary-soft)' : ''}">
              <span class="flex items-center gap-2"><input type="radio" name="tpl" ${on ? 'checked' : ''}> ${C.esc(t)}</span>${v ? `<span class="mono text-xs muted">${v}</span>` : ''}</label>`).join('')}
          <div class="callout info mt-3">${C.icon('info')}<div>选用模板会预填环境组件与检查点骨架,可在后续步骤调整。</div></div>
        </div></div>`;
    } else if (step === 2) {
      /* ② 环境组件:M2 运行时 + 工具集 + 初始代码 */
      body = `<div class="card card-pad mb-3"><div class="section-title mb-3">运行时(M2)</div>
          <div class="grid grid-3">${m.runtimes.map((r, i) => `
            <label class="card card-pad" style="cursor:pointer;border-color:var(--${i === 1 ? 'color-primary' : 'color-border'})">
              <label class="radio"><input type="radio" name="rt" ${i === 1 ? 'checked' : ''}> <span class="fw-600">${r.name}</span></label>
              <div class="muted text-xs mt-2">镜像 ${r.img} · 自检${r.selftest}</div></label>`).join('')}</div></div>
        <div class="card card-pad mb-3"><div class="section-title mb-3">工具集</div>
          <div class="grid grid-2">${[['code-server', 'VS Code 网页版编辑器', true], ['Remix', 'Solidity 在线 IDE', false], ['集成终端', 'Shell / forge / cast', true], ['Blockscout', '区块浏览器', true]].map(([t, d, on]) => `
            <label class="checkbox mb-2" style="display:flex;justify-content:space-between;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm)">
              <span class="flex items-center gap-2"><input type="checkbox" ${on ? 'checked' : ''}> <span class="fw-600">${t}</span></span><span class="muted text-xs">${d}</span></label>`).join('')}</div></div>
        <div class="card card-pad"><div class="section-title mb-2">初始代码 / 脚手架</div>
          <div class="muted text-xs mb-2">学生进入实验时的初始文件(含待修复的漏洞合约)</div>
          <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:14px;font-family:var(--font-mono);font-size:var(--text-xs);color:#cbd5e1;white-space:pre;overflow:auto;line-height:1.7">// Vault.sol — 待修复:存在重入漏洞
function withdraw() public {
    uint amt = balances[msg.sender];
    (bool ok,) = msg.sender.call{value: amt}("");
    balances[msg.sender] = 0;   // TODO(学生): 调整为先更新状态
}</div>
          <button class="btn btn-outline btn-sm mt-3" onclick="Chaimir.demo('上传脚手架压缩包')">${C.icon('upload')} 上传脚手架</button></div>`;
    } else if (step === 3) {
      /* ③ 仿真组件:M4 仿真包 + 参数 */
      body = `<div class="card card-pad mb-3"><div class="section-title mb-3">选择 M4 仿真包(可选)</div>
          <div class="callout info mb-3">${C.icon('info')}<div>仿真组件用于可视化展示链上行为(如重入调用栈、共识投票),帮助学生理解;可与代码环境并存。</div></div>
          <div class="grid grid-2">${[['重入攻击调用栈可视化', '展示递归 receive 与状态变化', true], ['PBFT 三阶段投票矩阵', '可注入拜占庭节点', false], ['默克尔树构建动画', '逐步构建与验证路径', false], ['不使用仿真', '纯代码实验', false]].map(([t, d, on]) => `
            <label class="radio mb-2" style="display:flex;flex-direction:column;align-items:flex-start;gap:4px;padding:12px;border:1px solid var(--${on ? 'color-primary' : 'color-border'});border-radius:var(--radius-sm);${on ? 'background:var(--color-primary-soft)' : ''}">
              <span class="flex items-center gap-2"><input type="radio" name="sim" ${on ? 'checked' : ''}> <span class="fw-600">${t}</span></span><span class="muted text-xs" style="padding-left:24px">${d}</span></label>`).join('')}</div></div>
        <div class="card card-pad"><div class="section-title mb-3">仿真参数</div>
          <div class="grid grid-2">
            <div class="field"><label>初始金库余额(ETH)</label><input class="input" type="number" value="10"></div>
            <div class="field"><label>攻击者初始存款(ETH)</label><input class="input" type="number" value="1"></div>
            <div class="field"><label>最大递归深度</label><input class="input" type="number" value="8"></div>
            <div class="field"><label>播放速度</label><select class="select"><option>正常</option><option>慢速(逐步)</option><option>快速</option></select></div>
          </div></div>`;
    } else if (step === 4) {
      /* ④ 检查点组件:M3 判题(测试用例/链上断言/flag/静态检查/仿真检查点)+ 绑 M5 题目 + 分值 */
      const cpTypes = [
        ['测试用例', 'foundry / hardhat 单测', 'flask-conical'],
        ['链上断言', '校验合约状态 / 事件 / 余额', 'link'],
        ['Flag 校验', 'CTF 提交 flag 比对', 'flag'],
        ['静态检查', 'Slither / 自定义规则', 'scan-search'],
        ['仿真检查点', 'M4 仿真达成目标态', 'activity'],
      ];
      body = `<div class="card card-pad mb-3"><div class="section-title mb-2">检查点配置(M3 判题)</div>
          <div class="muted text-sm mb-3">每个检查点选择判题方式、绑定 M5 题目并设置分值;学生达成即得分。</div>
          <div class="flex gap-2 wrap mb-3">${cpTypes.map(([t, d, ic]) => `<button class="btn btn-outline btn-sm" onclick="Chaimir.demo('添加检查点:${t}')">${C.icon(ic)} + ${t}</button>`).join('')}</div>
          ${[
            { t: '检查点 1 · 正常提款通过', type: '测试用例', col: 'purple', q: 'C-205', sc: 20, ic: 'flask-conical' },
            { t: '检查点 2 · 重入攻击被拦截', type: '链上断言', col: 'blue', q: 'C-205', sc: 40, ic: 'link' },
            { t: '检查点 3 · Slither 无高危告警', type: '静态检查', col: 'teal', q: 'C-118', sc: 20, ic: 'scan-search' },
            { t: '检查点 4 · 提交修复说明 flag', type: 'Flag 校验', col: 'amber', q: 'C-309', sc: 20, ic: 'flag' },
          ].map((cp) => `
            <div class="card card-pad mb-2" style="background:var(--color-surface-sunken)">
              <div class="flex justify-between items-center wrap gap-2">
                <div class="flex items-center gap-2">${C.icon(cp.ic)}<span class="fw-600">${cp.t}</span>${C.badge(cp.type, cp.col)}</div>
                <div class="flex items-center gap-2">
                  <span class="muted text-xs">绑定题目</span><span class="badge badge-gray mono">${cp.q}</span>
                  <span class="muted text-xs">分值</span><input class="input" style="width:60px;text-align:center" type="number" value="${cp.sc}">
                  <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('编辑判题脚本')">${C.icon('settings-2')}</button>
                  <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('删除检查点')">${C.icon('x')}</button>
                </div></div></div>`).join('')}
          <div class="flex justify-end mt-2"><span class="fw-700">满分合计:100 分</span></div>
        </div>
        <div class="callout warn">${C.icon('alert-triangle')}<div>检查点的判题脚本、链上断言与 flag 属于<b>答案黑盒</b>,对学生不可见。请确保题面与判题配置分离。</div></div>`;
    } else if (step === 5) {
      /* ⑤ 说明/报告/协作 */
      body = `<div class="grid" style="grid-template-columns:1fr 320px">
        <div class="card card-pad">
          <div class="section-title mb-3">实验说明(对学生可见)</div>
          <div class="field"><label>实验指南</label><textarea class="textarea" style="min-height:120px" placeholder="分步骤说明实验任务、提示与评分点(不含答案)…">任务:修复 Vault.sol 中的重入漏洞,使其通过全部检查点。提示:遵循 Checks-Effects-Interactions 模式。</textarea></div>
          <div class="field" style="margin-bottom:0"><label>实验报告要求</label>
            <label class="checkbox mb-2" style="display:flex"><input type="checkbox" checked> 要求提交实验报告(教师批改)</label>
            <textarea class="textarea" placeholder="报告需包含:漏洞原理、修复方案、防御对比…"></textarea></div>
        </div>
        <div class="card card-pad"><div class="section-title mb-3">协作模式</div>
          <label class="radio mb-2" style="display:flex;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm)"><input type="radio" name="collab"> 单人独立完成</label>
          <label class="radio mb-3" style="display:flex;padding:11px;border:1px solid var(--color-primary);border-radius:var(--radius-sm);background:var(--color-primary-soft)"><input type="radio" name="collab" checked> 小组协作</label>
          <div class="field"><label>组大小</label><input class="input" type="number" value="3"></div>
          <div class="field" style="margin-bottom:0"><label>组内角色</label>
            ${['攻击方', '防御方', '审计记录'].map(r => `<label class="checkbox mb-2" style="display:flex"><input type="checkbox" checked> ${r}</label>`).join('')}</div>
        </div></div>`;
    } else {
      /* ⑥ 校验/发布:issues 列表(error 必须修复,warning 提示) */
      const issues = [
        { level: 'error', text: '检查点 2「重入攻击被拦截」未绑定 M5 题目,无法计分。', fix: '去配置' },
        { level: 'warning', text: '环境未选择 Blockscout,学生无法在浏览器中查看交易。', fix: '去添加' },
        { level: 'warning', text: '实验报告要求为空,建议补充以指导学生撰写。', fix: '去填写' },
      ];
      const errCount = issues.filter(i => i.level === 'error').length;
      body = `<div class="card card-pad mb-3"><div class="flex justify-between items-center mb-3">
          <div class="section-title">发布前校验</div>
          <button class="btn btn-outline btn-sm" onclick="Chaimir.toast('info','重新校验中','正在检查环境、检查点与题目绑定')">${C.icon('refresh-cw')} 重新校验</button></div>
        ${issues.map(it => `<div class="callout ${it.level === 'error' ? 'danger' : 'warn'} mb-2">
            ${C.icon(it.level === 'error' ? 'alert-circle' : 'alert-triangle')}
            <div style="flex:1">${it.text}</div>
            <button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('teacher/exp-wizard?step=4${id ? '&id=' + id : ''}')">${it.fix}</button></div>`).join('')}
        ${errCount === 0 ? `<div class="callout success">${C.icon('check-circle-2')}<div>所有阻断项已解决,可以发布。</div></div>` : ''}
      </div>
      <div class="card card-pad"><div class="section-title mb-3">发布概览</div>
        <dl class="dl"><dt>实验名称</dt><dd>重入漏洞利用与防护(CEI)</dd>
          <dt>运行时</dt><dd>EVM · Foundry(v0.2.0)</dd>
          <dt>检查点</dt><dd>4 个 · 满分 100</dd>
          <dt>协作</dt><dd>小组(3 人)</dd>
          <dt>校验结果</dt><dd>${errCount > 0 ? C.badge(errCount + ' 个错误待修复', 'red') : C.badge('通过', 'green')} ${C.badge('2 个提示', 'amber')}</dd></dl>
        <div class="callout warn mt-3">${C.icon('lock')}<div>存在 ${errCount} 个错误,发布按钮已禁用。修复全部错误后方可发布。</div></div>
      </div>`;
    }
    const lastAction = "Chaimir.toast('error','尚有错误未修复','请先解决校验中的错误项','TRC-8842-EXP')";
    return `${C.crumb([{ label: '实验管理', to: 'teacher/experiments' }, { label: id ? '编辑实验' : '新建实验' }])}
      ${C.head((id ? '编辑实验' : '新建实验') + ' · 编排向导', '步骤 ' + step + ' / 6 · ' + expSteps[step - 1])}
      ${stepBar(step, id)}
      ${body}
      ${wizardFoot(step, id, lastAction)}`;
  }

  /* ============================================================
     3) 竞赛管理(列表)
     ============================================================ */
  const contests = [
    { id: 1, name: '「链上夺旗」金库重入渗透赛', mode: '对抗赛', match: '天梯 ELO', team: '个人', status: '进行中', players: 64, schedule: '06-05 ~ 06-12' },
    { id: 2, name: '智能合约 Gas 优化挑战赛', mode: '解题赛', match: '—', team: '个人', status: '报名中', players: 41, schedule: '06-10 开赛' },
    { id: 3, name: '跨链桥安全攻防联赛', mode: '对抗赛', match: '循环赛', team: '团队', status: '已结束', players: 120, schedule: '05-01 ~ 05-20' },
    { id: 4, name: 'PBFT 共识理解解题赛', mode: '解题赛', match: '—', team: '个人', status: '草稿', players: 0, schedule: '未排期' },
  ];
  C.tContests = contests;
  const contestStatusBadge = (s) => C.badge(s, { '草稿': 'gray', '报名中': 'blue', '进行中': 'green', '已结束': 'purple', '已归档': 'gray' }[s] || 'gray');

  function contestsList() {
    return `${C.head('竞赛管理', '实践', `<button class="btn btn-primary" onclick="Chaimir.navigate('teacher/contest-edit')">${C.icon('plus')} 新建竞赛</button>`)}
      <div class="grid grid-4 mb-4">
        ${C.stat('trophy', contests.length, '赛事总数', 'amber')}
        ${C.stat('swords', contests.filter(c => c.mode === '对抗赛').length, '对抗赛', 'red')}
        ${C.stat('play-circle', contests.filter(c => c.status === '进行中').length, '进行中', 'green')}
        ${C.stat('users', '225', '累计参赛', 'blue')}
      </div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>赛事名称</th><th>赛制</th><th>撮合</th><th>类型</th><th>赛程</th><th>参赛</th><th>状态</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${contests.map(c => {
          /* 状态机驱动按钮:草稿→发布;报名中→开始;进行中→结束;已结束→归档 */
          let ops = `<button class="btn btn-outline btn-sm" onclick="Chaimir.navigate('teacher/contest-edit?id=${c.id}')">${C.icon('pencil')} 编辑</button>
            <button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('teacher/contest-problems?id=${c.id}')">${C.icon('list-checks')} 出题</button>`;
          if (c.status === '草稿') ops += `<button class="btn btn-primary btn-sm" onclick="Chaimir.tContestAction(${c.id},'发布','已发布','报名通道已开启')">${C.icon('send')} 发布</button>`;
          else if (c.status === '报名中') ops += `<button class="btn btn-primary btn-sm" onclick="Chaimir.tContestAction(${c.id},'开始','进行中','对局撮合已启动')">${C.icon('play')} 开始</button>`;
          else if (c.status === '进行中') ops += `<button class="btn btn-outline btn-sm" onclick="Chaimir.tContestAction(${c.id},'结束','已结束','榜单已封存')">${C.icon('flag')} 结束</button>`;
          else if (c.status === '已结束') ops += `<button class="btn btn-outline btn-sm" onclick="Chaimir.tContestAction(${c.id},'归档','已归档','赛事已存档')">${C.icon('archive')} 归档</button>`;
          return `<tr>
            <td class="fw-600">${c.name}</td>
            <td>${C.badge(c.mode, c.mode === '对抗赛' ? 'red' : 'blue')}</td>
            <td class="muted text-sm">${c.match}</td>
            <td>${C.badge(c.team, c.team === '团队' ? 'purple' : 'gray')}</td>
            <td class="mono text-xs">${c.schedule}</td>
            <td class="mono">${c.players}</td>
            <td>${contestStatusBadge(c.status)}</td>
            <td class="row-actions">${ops}</td></tr>`;
        }).join('')}</tbody></table></div>`;
  }
  C.tContestAction = async function (id, verb, to, sub) {
    if (await C.confirm({ title: verb + '竞赛', message: '确认' + verb + '该竞赛?状态将变更为「' + to + '」。', confirmText: '确认' + verb, danger: verb === '结束' || verb === '归档' })) {
      const c = contests.find(x => x.id == id); if (c) c.status = to;
      C.toast('success', '竞赛已' + verb, sub); C.rerender();
    }
  };

  /* ---------- 竞赛配置 ---------- */
  function contestEdit(ctx) {
    const c = ctx.query.id ? contests.find(x => x.id == ctx.query.id) : null;
    const isNew = !c;
    return `${C.crumb([{ label: '竞赛管理', to: 'teacher/contests' }, { label: isNew ? '新建竞赛' : '竞赛配置' }])}
      ${C.head(isNew ? '新建竞赛' : '竞赛配置', c ? c.name : '设定赛制、撮合与规则',
        `<button class="btn btn-outline" onclick="Chaimir.navigate('teacher/contests')">取消</button>
         <button class="btn btn-outline" onclick="Chaimir.navigate('teacher/contest-problems?id=${c ? c.id : 1}')">${C.icon('list-checks')} 去出题</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','已保存','竞赛配置已保存');setTimeout(()=>Chaimir.navigate('teacher/contests'),700)">${C.icon('save')} 保存</button>`)}
      <div class="grid" style="grid-template-columns:1fr 320px">
        <div class="card card-pad">
          <div class="section-title mb-3">基本信息</div>
          <div class="field"><label>赛事名称<span class="req">*</span></label><input class="input" value="${c ? C.esc(c.name) : ''}" placeholder="如:链上夺旗渗透赛"></div>
          <div class="field"><label>赛事简介</label><textarea class="textarea" placeholder="介绍赛事背景、目标与奖励…"></textarea></div>
          <div class="grid grid-2">
            <div class="field"><label>赛制<span class="req">*</span></label>
              <select class="select" id="contest-mode"><option ${c && c.mode === '解题赛' ? 'selected' : ''}>解题赛</option><option ${c && c.mode === '对抗赛' ? 'selected' : ''}>对抗赛</option></select></div>
            <div class="field"><label>撮合模式</label>
              <select class="select"><option>循环赛(Round-Robin)</option><option>天梯赛(ELO 动态)</option><option>淘汰赛</option></select></div>
            <div class="field"><label>参赛类型</label>
              <select class="select"><option ${c && c.team === '个人' ? 'selected' : ''}>个人</option><option ${c && c.team === '团队' ? 'selected' : ''}>团队</option></select></div>
            <div class="field"><label>封榜时长(赛前 N 小时)</label><input class="input" type="number" value="1"></div>
            <div class="field"><label>报名开始</label><input class="input" type="datetime-local"></div>
            <div class="field"><label>比赛开始</label><input class="input" type="datetime-local"></div>
            <div class="field"><label>比赛结束</label><input class="input" type="datetime-local"></div>
            <div class="field"><label>团队人数上限</label><input class="input" type="number" value="3"></div>
          </div>
          <div class="field" style="margin-bottom:0"><label>赛事规则(对选手可见)</label><textarea class="textarea" placeholder="评分规则、提交限制、违规处理…">解题赛按通过题数与用时排名;对抗赛按攻防对局胜负计 ELO。禁止共享 flag,违规取消资格。</textarea></div>
        </div>
        <div>
          <div class="card card-pad mb-3"><div class="section-title mb-2">赛制说明</div>
            <div class="callout info mb-2">${C.icon('puzzle')}<div><b>解题赛</b>:选手独立解题,按通过题数 / 用时排名。</div></div>
            <div class="callout danger">${C.icon('swords')}<div><b>对抗赛</b>:攻防 / 博弈对局,撮合可选循环赛或天梯 ELO 动态。</div></div>
          </div>
          <div class="card card-pad"><div class="section-title mb-2">参赛设置</div>
            <label class="checkbox mb-2" style="display:flex"><input type="checkbox" checked> 需要审核报名</label>
            <label class="checkbox mb-2" style="display:flex"><input type="checkbox"> 限本校学生参加</label>
            <label class="checkbox" style="display:flex"><input type="checkbox" checked> 开启对局回放</label>
          </div>
        </div>
      </div>`;
  }

  /* ---------- 出题向导(持久化)---------- */
  function contestProblems(ctx) {
    const cid = ctx.query.id || 1;
    const c = contests.find(x => x.id == cid) || contests[0];
    /* 已组卷题目:锁版本 / 分值 / 题序 / 动态分;对抗题设对局规则 */
    const probs = [
      { no: 'A', title: '金库重入渗透(攻防)', ver: 'v1.0.0', score: 300, dyn: true, battle: '攻防对局', col: 'red' },
      { no: 'B', title: 'Gas 优化:批量转账', ver: 'v1.2.0', score: 200, dyn: false, battle: '—', col: 'blue' },
      { no: 'C', title: '价格预言机操纵博弈', ver: 'v1.1.0', score: 400, dyn: true, battle: '博弈对局', col: 'purple' },
    ];
    return `${C.crumb([{ label: '竞赛管理', to: 'teacher/contests' }, { label: c.name, to: 'teacher/contest-edit?id=' + cid }, { label: '出题组卷' }])}
      ${C.head('出题组卷 · ' + c.name, '从 M5 选题、锁版本、设分值与题序',
        `<span class="autosave saved">${C.icon('cloud')} 草稿已自动保存</span>
         <button class="btn btn-outline" onclick="Chaimir.tPickContestProb()">${C.icon('library')} 从题库选题</button>
         <button class="btn btn-primary" onclick="Chaimir.toast('success','组卷已保存','发布竞赛后选手可见题面')">${C.icon('save')} 保存组卷</button>`)}
      <div class="callout warn mb-4">${C.icon('lock')}<div>组卷会<b>锁定题目版本</b>:即使题库后续更新,本场竞赛仍使用锁定版本,保证公平。对抗题需指定对局规则。</div></div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>题序</th><th>题目</th><th>锁定版本</th><th>分值</th><th>动态分</th><th>对局规则</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${probs.map(p => `<tr>
          <td><input class="input" style="width:50px;text-align:center" value="${p.no}"></td>
          <td class="fw-600">${p.title}</td>
          <td><span class="badge badge-gray mono">${p.ver}</span> ${C.icon('lock')}</td>
          <td><input class="input" style="width:72px;text-align:center" type="number" value="${p.score}"></td>
          <td><label class="switch"><input type="checkbox" ${p.dyn ? 'checked' : ''}><span class="track"></span></label></td>
          <td>${p.battle === '—' ? '<span class="muted">—</span>' : `${C.badge(p.battle, p.col)} <button class="btn btn-ghost btn-sm" onclick="Chaimir.tBattleRule('${p.battle}')">${C.icon('settings-2')}</button>`}</td>
          <td class="row-actions"><button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('上移')">${C.icon('chevron-up')}</button>
            <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('下移')">${C.icon('chevron-down')}</button>
            <button class="btn btn-ghost btn-sm btn-icon" onclick="Chaimir.demo('移除')">${C.icon('x')}</button></td>
        </tr>`).join('')}</tbody></table></div>
      <div class="flex justify-between mt-3"><span class="muted text-sm">共 ${probs.length} 题</span><span class="fw-700">总分:900 分</span></div>`;
  }
  C.tPickContestProb = function () {
    const lib = [['P-301', '金库重入渗透(攻防)', '对抗题', '高级'], ['P-145', 'Gas 优化:批量转账', '解题', '进阶'], ['P-410', '价格预言机操纵博弈', '对抗题', '高级'], ['P-208', '签名重放漏洞利用', '解题', '高级']];
    C.modal({
      title: '从题库选题(M5)', size: 'lg',
      body: `<div class="input-icon mb-3">${C.icon('search')}<input class="input" placeholder="搜索竞赛题 / 对抗题"></div>
        <div class="table-wrap"><table class="table"><thead><tr><th><label class="checkbox"><input type="checkbox"></label></th><th>编号</th><th>题目</th><th>类型</th><th>难度</th></tr></thead>
          <tbody>${lib.map(([id, t, ty, d]) => `<tr><td><label class="checkbox"><input type="checkbox"></label></td><td class="mono text-xs">${id}</td><td class="fw-600">${t}</td><td>${C.badge(ty, ty === '对抗题' ? 'red' : 'blue')}</td><td>${C.badge(d, 'gray')}</td></tr>`).join('')}</tbody></table></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','已加入组卷','题目版本已锁定')">加入并锁版本</button>`,
    });
  };
  C.tBattleRule = function (kind) {
    C.modal({
      title: '对局规则 · ' + kind,
      body: `<div class="field"><label>对局类型</label><select class="select"><option ${kind === '攻防对局' ? 'selected' : ''}>攻防对局(一方攻击 / 一方防御)</option><option ${kind === '博弈对局' ? 'selected' : ''}>博弈对局(多方策略竞争)</option></select></div>
        <div class="grid grid-2"><div class="field"><label>单局时长(分钟)</label><input class="input" type="number" value="15"></div>
          <div class="field"><label>每对手对局数</label><input class="input" type="number" value="2"></div></div>
        <div class="field" style="margin-bottom:0"><label>胜负判定</label><textarea class="textarea" placeholder="如:守方资产零损耗即守方胜;攻方掏空金库即攻方胜…">攻方在时限内掏空金库判攻方胜;否则守方胜。平局按 Gas 消耗少者胜。</textarea></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','对局规则已保存')">保存</button>`,
    });
  };

  /* ============================================================
     4) 实时监控(状态矩阵 + 一键阻断)
     ============================================================ */
  function monitor(ctx) {
    const tab = ctx.query.t || 'exp';
    /* 学生实例状态:正常(绿)/落后(琥珀)/报错(红) —— 颜色 + 文字双载体 */
    const stCls = { ok: 'green', warn: 'amber', err: 'red' };
    const stLabel = { ok: '正常', warn: '落后', err: '报错' };
    const grid = m.students.concat([
      { id: 7, name: '何雨欣', no: '2023210462', state: 'ok', stateText: '通过检查点 3/3', cp: 3 },
      { id: 8, name: '罗子轩', no: '2023210463', state: 'warn', stateText: '环境启动慢', cp: 0 },
      { id: 9, name: '高梓涵', no: '2023210464', state: 'ok', stateText: '通过检查点 2/3', cp: 2 },
      { id: 10, name: '梁俊豪', no: '2023210465', state: 'err', stateText: '沙箱 OOM', cp: 1 },
    ]);
    const okN = grid.filter(s => s.state === 'ok').length, warnN = grid.filter(s => s.state === 'warn').length, errN = grid.filter(s => s.state === 'err').length;

    /* 对抗撮合进度(竞赛 Tab) */
    const battleView = `
      <div class="grid grid-4 mb-4">
        ${C.stat('users', '64', '在线选手', 'blue')}
        ${C.stat('swords', '128', '已完成对局', 'amber')}
        ${C.stat('loader', '6', '撮合进行中', 'green')}
        ${C.stat('list', '32', '待撮合队列', 'gray')}
      </div>
      <div class="card mb-4"><div class="card-head"><div class="section-title">天梯撮合进度</div><span class="muted text-xs">每 5 秒刷新</span></div>
        <div class="card-pad">
          <div class="flex justify-between text-sm mb-2"><span>本轮撮合</span><span class="fw-600">128 / 160 局</span></div>
          <div class="progress green mb-4"><span style="width:80%"></span></div>
          ${[['影梭战队', 'ZeroDay', '进行中', 'green'], ['拜占庭幻象', '链链同盟', '已结束 · 攻方胜', 'gray'], ['夜枭', '白帽联盟', '撮合中', 'amber']].map(([a, b, st, col]) => `
            <div class="flex items-center justify-between" style="padding:10px 0;border-bottom:1px solid var(--color-border)">
              <div class="flex items-center gap-2"><span class="fw-600">${a}</span><span class="muted">vs</span><span class="fw-600">${b}</span></div>
              <div class="flex items-center gap-2">${C.statusDot(col, st)}<button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('immersive/battle-replay')">${C.icon('play-circle')} 回放</button></div></div>`).join('')}
        </div></div>`;

    const expView = `
      <div class="grid grid-4 mb-4">
        ${C.stat('box', grid.length, '活跃实例', 'blue')}
        ${C.stat('check-circle-2', okN, '正常', 'green')}
        ${C.stat('alert-triangle', warnN, '落后', 'amber')}
        ${C.stat('alert-circle', errN, '报错', 'red')}
      </div>
      <div class="grid grid-2 mb-4">
        <div class="card card-pad"><div class="section-title mb-3">资源占用</div>
          ${[['CPU 总用量', 68, ''], ['内存总用量', 54, 'green'], ['沙箱配额', 42, 'green']].map(([l, v, col]) => `
            <div class="mb-3"><div class="flex justify-between text-sm mb-2"><span class="muted">${l}</span><span class="fw-600">${v}%</span></div>
              <div class="progress ${col}"><span style="width:${v}%"></span></div></div>`).join('')}
        </div>
        <div class="card card-pad"><div class="section-title mb-3">实时事件流</div>
          <div style="max-height:180px;overflow:auto">${[
            ['err', '梁俊豪 的沙箱内存超限(OOM),实例已重启', '刚刚'],
            ['warn', '罗子轩 的环境启动耗时 48s,超过预期', '1 分钟前'],
            ['ok', '何雨欣 通过全部检查点(3/3)', '2 分钟前'],
            ['ok', '林思远 通过检查点 2「重入拦截」', '3 分钟前'],
          ].map(([lv, t, ago]) => `<div class="flex items-start gap-2 text-sm" style="padding:7px 0;border-bottom:1px solid var(--color-border)">
            <span class="dot dot-${stCls[lv]}" style="margin-top:6px"></span><span style="flex:1">${t}</span><span class="muted text-xs">${ago}</span></div>`).join('')}</div>
        </div>
      </div>
      <div class="card"><div class="card-head"><div class="section-title">学生实例状态矩阵</div>
        <span class="muted text-xs">绿=正常 · 琥珀=落后 · 红=报错</span></div>
        <div class="card-pad">
          <div class="grid" style="grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px">
            ${grid.map(s => `<div class="card card-pad" style="padding:12px;border-left:3px solid var(--${stCls[s.state]}-600)">
              <div class="flex items-center justify-between mb-2"><span class="fw-600 text-sm">${s.name}</span><span class="dot dot-${stCls[s.state]}"></span></div>
              <div class="muted text-xs mono">${s.no}</div>
              <div class="text-xs mt-2">${C.badge(stLabel[s.state], stCls[s.state])} ${s.stateText}</div>
              <div class="flex gap-1 mt-2">
                <button class="btn btn-ghost btn-sm btn-icon" title="查看实例" onclick="Chaimir.demo('查看 ${s.name} 的实例')">${C.icon('eye')}</button>
                <button class="btn btn-ghost btn-sm btn-icon" title="重启实例" onclick="Chaimir.demo('重启 ${s.name} 的实例')">${C.icon('rotate-cw')}</button>
                <button class="btn btn-ghost btn-sm btn-icon" title="阻断" onclick="Chaimir.tBlockOne('${s.name}')">${C.icon('ban')}</button>
              </div></div>`).join('')}
          </div>
        </div></div>`;

    return `${C.head('实时监控', '实践',
      `<button class="btn btn-outline" onclick="Chaimir.toast('info','已刷新','实时数据已更新')">${C.icon('refresh-cw')} 刷新</button>
       <button class="btn btn-danger" onclick="Chaimir.tBlockAll()">${C.icon('shield-alert')} 一键集中阻断</button>`)}
      <div class="tabs">
        <a class="tab ${tab === 'exp' ? 'active' : ''}" onclick="Chaimir.navigate('teacher/monitor?t=exp')">${C.icon('flask-conical')} 实验运行</a>
        <a class="tab ${tab === 'contest' ? 'active' : ''}" onclick="Chaimir.navigate('teacher/monitor?t=contest')">${C.icon('swords')} 竞赛对抗</a>
      </div>
      <div class="card card-pad mb-4 flex items-center justify-between wrap gap-3" style="background:var(--color-surface-sunken)">
        <div class="flex items-center gap-3">${C.icon('radio')}<div><div class="fw-600">${tab === 'exp' ? '重入漏洞利用与防护(CEI)' : '「链上夺旗」金库重入渗透赛'}</div>
          <div class="muted text-xs">${tab === 'exp' ? '智能合约安全攻防实训 · 区块链 2301 班' : '对抗赛 · 天梯 ELO · 64 名选手'}</div></div></div>
        <select class="select" style="width:280px" onchange="Chaimir.demo('切换监控对象')"><option>${tab === 'exp' ? '重入漏洞利用与防护(进行中)' : '链上夺旗渗透赛(进行中)'}</option></select>
      </div>
      ${tab === 'exp' ? expView : battleView}`;
  }
  /* 一键集中阻断:危险确认 + toast */
  C.tBlockAll = async function () {
    if (await C.confirm({ title: '一键集中阻断', message: '将立即冻结本场全部学生实例并断开沙箱网络(用于发现集体作弊或紧急事故)。此操作影响所有在场学生,确认执行?', confirmText: '立即阻断', danger: true }))
      C.toast('success', '已集中阻断', '全部实例已冻结,沙箱网络已断开', 'TRC-BLOCK-9931');
  };
  C.tBlockOne = async function (name) {
    if (await C.confirm({ title: '阻断单个实例', message: '将冻结 ' + name + ' 的实例并断开其沙箱网络。确认?', confirmText: '阻断', danger: true }))
      C.toast('success', '已阻断', name + ' 的实例已冻结');
  };

  /* ---------- 防作弊审查(子页)---------- */
  const cheatCases = [
    { id: 1, name: '孙浩然', no: '2023210458', type: '代码查重', detail: '与「赵雨桐」提交相似度 86%', evidence: 'sim', level: 'A', col: 'red' },
    { id: 2, name: '梁俊豪', no: '2023210465', type: '行为异常', detail: '5 分钟内提交 23 次,疑似暴力试探', evidence: 'freq', level: 'B', col: 'amber' },
    { id: 3, name: '罗子轩', no: '2023210463', type: '环境违规', detail: '检测到沙箱内访问外部网络请求', evidence: 'net', level: 'B', col: 'amber' },
  ];
  function cheatReview(ctx) {
    return `${C.crumb([{ label: '实时监控', to: 'teacher/monitor' }, { label: '防作弊审查' }])}
      ${C.head('防作弊审查', '可疑提交检测 · 证据可视化 · 处理决定')}
      <div class="callout warn mb-4">${C.icon('shield-alert')}<div>系统自动标记可疑提交,最终是否违规由教师认定。处理决定将记入审计日志,请审慎操作。</div></div>
      <div class="grid grid-3 mb-4">
        ${C.stat('copy-check', cheatCases.filter(c => c.type === '代码查重').length, '代码查重预警', 'red')}
        ${C.stat('activity', cheatCases.filter(c => c.type === '行为异常').length, '行为异常', 'amber')}
        ${C.stat('globe', cheatCases.filter(c => c.type === '环境违规').length, '环境违规', 'amber')}
      </div>
      ${cheatCases.map(c => `
        <div class="card card-pad mb-3">
          <div class="flex justify-between items-center wrap gap-2 mb-3">
            <div class="flex items-center gap-2"><span class="fw-700">${c.name}</span><span class="muted text-xs mono">${c.no}</span>
              ${C.badge(c.type, c.col)} ${C.badge('风险 ' + c.level + ' 级', c.col)}</div>
            <div class="muted text-sm">${c.detail}</div>
          </div>
          ${c.evidence === 'sim' ? `<div class="grid grid-2" style="gap:8px">
              <div><div class="muted text-xs mb-2">本人提交</div><div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:10px;font-family:var(--font-mono);font-size:11px;color:#cbd5e1;white-space:pre;overflow:auto">balances[msg.sender]=0;
(bool ok,)=msg.sender.call{value:amt}("");
require(ok,"fail");</div></div>
              <div><div class="muted text-xs mb-2">相似提交(赵雨桐 · 86%)</div><div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:10px;font-family:var(--font-mono);font-size:11px;color:#fca5a5;white-space:pre;overflow:auto">balances[msg.sender]=0;
(bool ok,)=msg.sender.call{value:amt}("");
require(ok,"fail");</div></div></div>`
            : c.evidence === 'freq' ? `<div class="card-pad" style="background:var(--color-surface-sunken);border-radius:var(--radius-sm)">
              <div class="muted text-xs mb-2">提交频率(每分钟)</div>
              <div class="flex items-end gap-1" style="height:60px">${[2, 3, 5, 8, 5].map(v => `<div style="flex:1;background:var(--amber-500);border-radius:3px 3px 0 0;height:${v / 8 * 100}%" title="${v} 次"></div>`).join('')}</div>
              <div class="flex justify-between muted text-xs mt-2"><span>00:00</span><span>05:00</span></div></div>`
            : `<div class="callout danger">${C.icon('globe')}<div>沙箱网络策略为 deny-all,但检测到 3 次出站请求尝试(已被拦截),目标:外部 RPC 节点。</div></div>`}
          <div class="flex justify-end gap-2 mt-3">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.tCheatHandle('${C.esc(c.name)}','警告',false)">${C.icon('bell')} 警告</button>
            <button class="btn btn-outline btn-sm" onclick="Chaimir.tCheatHandle('${C.esc(c.name)}','扣分',true)">${C.icon('minus-circle')} 扣分</button>
            <button class="btn btn-danger btn-sm" onclick="Chaimir.tCheatHandle('${C.esc(c.name)}','取消资格',true)">${C.icon('user-x')} 取消资格</button>
          </div>
        </div>`).join('')}`;
  }
  C.tCheatHandle = async function (name, action, danger) {
    if (await C.confirm({ title: action + '处理', message: '对 ' + name + ' 执行「' + action + '」?该决定会记入审计日志并通知学生。', confirmText: '确认' + action, danger: !!danger }))
      C.toast('success', '已' + action, '已对 ' + name + ' 处理并记入审计');
  };

  /* ============================================================
     5) 漏洞源管理 + 漏洞题转化工作台
     ============================================================ */
  const vulnSources = [
    { id: 1, name: 'SWC Registry(智能合约弱点分类)', kind: 'SWC', status: '已启用', last: '2026-06-06 03:00', count: 37, grade: 'A', key: 'sk_live_8f2a…c91d' },
    { id: 2, name: 'CVE 区块链相关漏洞库', kind: 'CVE', status: '已启用', last: '2026-06-05 03:00', count: 214, grade: 'B', key: 'cve_tok_a18…77ef' },
    { id: 3, name: '威胁情报源(链上攻击事件)', kind: '情报', status: '已暂停', last: '2026-05-20 03:00', count: 58, grade: 'C', key: 'intel_…(未配置)' },
  ];
  function vulnSourcesPage() {
    const gradeBadge = (g) => C.badge(g + ' 级', { A: 'red', B: 'amber', C: 'gray' }[g] || 'gray');
    return `${C.head('漏洞源管理', '资源', `<button class="btn btn-primary" onclick="Chaimir.tAddVulnSource()">${C.icon('plus')} 接入漏洞源</button>`)}
      <div class="callout info mb-4">${C.icon('shield')}<div>外部漏洞源(SWC / CVE / 情报)按 A/B/C 分级同步入库,作为漏洞题转化的素材。密钥仅掩码展示,不在前端明文返回。</div></div>
      <div class="table-wrap"><table class="table">
        <thead><tr><th>漏洞源</th><th>类型</th><th>分级</th><th>状态</th><th>最近同步</th><th>条目</th><th>访问密钥</th><th style="text-align:right">操作</th></tr></thead>
        <tbody>${vulnSources.map(s => `<tr>
          <td class="fw-600">${s.name}</td>
          <td>${C.badge(s.kind, 'blue')}</td>
          <td>${gradeBadge(s.grade)}</td>
          <td>${C.statusDot(s.status === '已启用' ? 'green' : 'gray', s.status)}</td>
          <td class="mono text-xs">${s.last}</td>
          <td class="mono">${s.count}</td>
          <td><span class="mono text-xs muted">${s.key}</span></td>
          <td class="row-actions">
            <button class="btn btn-outline btn-sm" onclick="Chaimir.toast('info','开始同步','正在从 ${s.kind} 拉取最新漏洞条目…')">${C.icon('refresh-cw')} 同步</button>
            <button class="btn btn-ghost btn-sm" onclick="Chaimir.navigate('teacher/vuln-transform')">${C.icon('wand-2')} 转化</button>
          </td></tr>`).join('')}</tbody></table></div>`;
  }
  C.tAddVulnSource = function () {
    C.modal({
      title: '接入漏洞源',
      body: `<div class="field"><label>名称<span class="req">*</span></label><input class="input" placeholder="如:SWC Registry"></div>
        <div class="field"><label>类型</label><select class="select"><option>SWC</option><option>CVE</option><option>情报</option></select></div>
        <div class="field"><label>同步端点(URL)</label><input class="input" placeholder="https://…"></div>
        <div class="field"><label>访问密钥</label><input class="input" type="password" placeholder="存入密钥库,前端仅掩码展示"></div>
        <div class="field" style="margin-bottom:0"><label>默认分级</label><select class="select"><option>A 级(高危)</option><option>B 级(中危)</option><option>C 级(低危)</option></select></div>`,
      foot: `<button class="btn btn-outline" onclick="document.querySelector('.overlay').remove()">取消</button>
             <button class="btn btn-primary" onclick="document.querySelector('.overlay').remove();Chaimir.toast('success','漏洞源已接入','密钥已安全存入密钥库')">接入</button>`,
    });
  };

  /* 漏洞题转化工作台:草稿编辑 + 6 步预验证(正向 PoC 通过 + 反向不误判)+ finalize 入库 M5 */
  function vulnTransform(ctx) {
    const checks = [
      { t: '正向 PoC 复现漏洞', ok: true, d: '攻击合约成功掏空金库,漏洞确实存在' },
      { t: '修复后 PoC 失效', ok: true, d: '应用 CEI 修复后,攻击被拦截' },
      { t: '反向样例不误判', ok: true, d: '已正确实现的合约不被判为有漏洞' },
      { t: '边界用例覆盖', ok: true, d: '零余额 / 重复提款等边界通过' },
      { t: '静态检查规则命中', ok: false, d: 'Slither 规则未命中本漏洞模式,需调整' },
      { t: '判题脚本可重现', ok: true, d: '三次独立运行结果一致' },
    ];
    const passN = checks.filter(c => c.ok).length;
    return `${C.crumb([{ label: '漏洞源管理', to: 'teacher/vuln-sources' }, { label: '漏洞题转化' }])}
      ${C.head('漏洞题转化工作台', '把外部漏洞素材转化为可判题的 M5 题目',
        `<span class="autosave saved">${C.icon('cloud')} 草稿已自动保存</span>
         <button class="btn btn-outline" onclick="Chaimir.toast('info','重新预验证','正在运行正向 PoC 与反向样例…')">${C.icon('play')} 运行预验证</button>
         <button class="btn btn-primary" ${passN < checks.length ? 'aria-disabled="true"' : ''} onclick="Chaimir.tVulnFinalize(${passN === checks.length})">${C.icon('database')} finalize 入库</button>`)}
      <div class="grid" style="grid-template-columns:1fr 360px">
        <div class="card card-pad">
          <div class="section-title mb-3">题目草稿</div>
          <div class="field"><label>来源</label><div class="flex gap-2">${C.badge('SWC-107 重入', 'blue')}${C.badge('A 级', 'red')}</div></div>
          <div class="field"><label>题目标题<span class="req">*</span></label><input class="input" value="金库重入漏洞利用与防护"></div>
          <div class="field"><label>题面(对学生可见)</label><textarea class="textarea" style="min-height:90px">下方 Vault 合约存在重入漏洞,请编写攻击合约证明可掏空金库,并给出修复方案。</textarea></div>
          <div class="field"><label>漏洞合约</label>
            <div style="background:var(--color-editor-bg);border-radius:var(--radius-sm);padding:12px;font-family:var(--font-mono);font-size:11px;color:#cbd5e1;white-space:pre;overflow:auto;line-height:1.6">function withdraw() public {
    uint amt = balances[msg.sender];
    (bool ok,) = msg.sender.call{value: amt}("");
    balances[msg.sender] = 0;
}</div></div>
          <div class="field" style="margin-bottom:0"><label>判题配置(答案黑盒)</label>
            <div class="callout warn">${C.icon('eye-off')}<div>正向 PoC、修复参考与判题脚本对学生不可见,仅用于自动判定。</div></div></div>
        </div>
        <div class="card card-pad">
          <div class="section-title mb-3">6 步预验证</div>
          <div class="callout ${passN === checks.length ? 'success' : 'warn'} mb-3">${C.icon(passN === checks.length ? 'check-circle-2' : 'alert-triangle')}<div>已通过 <b>${passN}/${checks.length}</b> 项。${passN === checks.length ? '可 finalize 入库。' : '存在未通过项,入库已禁用。'}</div></div>
          ${checks.map((c, i) => `<div class="flex items-start gap-2" style="padding:9px 0;border-bottom:1px solid var(--color-border)">
            <span class="dot dot-${c.ok ? 'green' : 'red'}" style="margin-top:6px"></span>
            <div style="flex:1"><div class="text-sm fw-600">${i + 1}. ${c.t}</div><div class="muted text-xs">${c.d}</div></div>
            <span class="badge badge-${c.ok ? 'green' : 'red'}">${c.ok ? '通过' : '未通过'}</span></div>`).join('')}
          ${passN < checks.length ? `<button class="btn btn-outline btn-block mt-3" onclick="Chaimir.demo('修复静态检查规则')">${C.icon('wrench')} 修复未通过项</button>` : ''}
        </div>
      </div>`;
  }
  C.tVulnFinalize = function (ok) {
    if (!ok) { C.toast('error', '尚有验证未通过', '请先通过全部 6 步预验证再入库', 'TRC-VULN-7720'); return; }
    C.toast('success', '已入库 M5 题库', '题目已生成版本 v1.0.0,可用于作业 / 竞赛引用');
    setTimeout(() => C.navigate('teacher/content'), 800);
  };

  C.registerPages({
    'teacher/experiments': experimentsList,
    'teacher/exp-wizard': expWizard,
    'teacher/contests': contestsList,
    'teacher/contest-edit': contestEdit,
    'teacher/contest-problems': contestProblems,
    'teacher/monitor': monitor,
    'teacher/cheat-review': cheatReview,
    'teacher/vuln-sources': vulnSourcesPage,
    'teacher/vuln-transform': vulnTransform,
  });
})();
